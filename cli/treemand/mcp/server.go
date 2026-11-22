// Package mcp is treemand's Model Context Protocol server: it offers CLI
// discovery to AI agents as tools over stdio (JSON-RPC 2.0, one message per
// line). It never runs a discovered command; the only processes started are
// the --help probes discovery already makes.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/aallbrig/treemand/models"
)

// LatestProtocolVersion is offered to clients that ask for a version this
// server does not know.
const LatestProtocolVersion = "2025-06-18"

// supportedVersions are the MCP revisions this server speaks; the transport
// and the methods it implements are the same in each.
var supportedVersions = []string{LatestProtocolVersion, "2025-03-26", "2024-11-05"}

// JSON-RPC error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// maxMessage caps one incoming line, so a runaway client cannot grow memory.
const maxMessage = 4 << 20

// Discoverer returns the command tree of cli, discovered depth levels deep.
type Discoverer func(ctx context.Context, cli string, depth int) (*models.Node, error)

// Expander discovers one stub node of cli's tree in place (its flags,
// positionals and children).
type Expander func(ctx context.Context, cli string, node *models.Node) error

// Server answers MCP requests. Discover is required; Expand is optional and
// lets assemble reach commands below a stub.
type Server struct {
	Name, Version string
	Discover      Discoverer
	Expand        Expander
	// Timeout bounds each tool call; zero means 30 seconds and a negative
	// value means no limit.
	Timeout time.Duration
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve reads requests from r and writes responses to w until r ends or ctx
// is cancelled. Requests are answered in order.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), maxMessage)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if resp := s.handle(ctx, line); resp != nil {
			if err := enc.Encode(resp); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

// handle answers one message, or returns nil for a notification.
func (s *Server) handle(ctx context.Context, line []byte) *response {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return errorResponse(json.RawMessage("null"), codeParseError, "parse error: "+err.Error())
	}
	if len(req.ID) == 0 {
		return nil // notifications (initialized, cancelled) need no answer
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		return errorResponse(req.ID, codeInvalidRequest, "invalid request: want jsonrpc 2.0 and a method")
	}
	switch req.Method {
	case "initialize":
		return result(req.ID, s.initialize(req.Params))
	case "ping":
		return result(req.ID, struct{}{})
	case "tools/list":
		return result(req.ID, map[string]any{"tools": toolList})
	case "tools/call":
		res, rerr := s.callTool(ctx, req.Params)
		if rerr != nil {
			return &response{JSONRPC: "2.0", ID: req.ID, Error: rerr}
		}
		return result(req.ID, res)
	default:
		return errorResponse(req.ID, codeMethodNotFound, "method not found: "+req.Method)
	}
}

func (s *Server) initialize(params json.RawMessage) map[string]any {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &p)
	version := LatestProtocolVersion
	if slices.Contains(supportedVersions, p.ProtocolVersion) {
		version = p.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
		"instructions": "treemand maps a command-line tool's subcommands, flags and positional arguments " +
			"by reading its --help output. Use discover for the tree, search to find a command or flag " +
			"by keyword, and assemble to build a correctly quoted command line. Nothing is ever executed " +
			"beyond --help probes.",
	}
}

func result(id json.RawMessage, v any) *response {
	return &response{JSONRPC: "2.0", ID: id, Result: v}
}

func errorResponse(id json.RawMessage, code int, msg string) *response {
	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

// toolResult is the result of tools/call. A failing tool is a result with
// IsError set, so the agent sees the message and can correct itself.
type toolResult struct {
	Content           []textContent `json:"content"`
	StructuredContent any           `json:"structuredContent,omitempty"`
	IsError           bool          `json:"isError,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func toolError(format string, a ...any) *toolResult {
	return &toolResult{Content: []textContent{{Type: "text", Text: fmt.Sprintf(format, a...)}}, IsError: true}
}

// jsonResult returns v as indented JSON text, and as structured content when
// v encodes as an object.
func jsonResult(v any, structured bool) *toolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolError("encoding the result failed: %v", err)
	}
	res := &toolResult{Content: []textContent{{Type: "text", Text: string(b)}}}
	if structured {
		res.StructuredContent = v
	}
	return res
}

func (s *Server) callTool(ctx context.Context, params json.RawMessage) (*toolResult, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
	}
	tool, ok := tools[p.Name]
	if !ok {
		return nil, &rpcError{Code: codeInvalidParams, Message: "unknown tool: " + p.Name}
	}
	if len(p.Arguments) == 0 || string(p.Arguments) == "null" {
		p.Arguments = json.RawMessage("{}")
	}
	if s.Timeout < 0 {
		return tool(s, ctx, p.Arguments), nil
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res := tool(s, cctx, p.Arguments)
	if cctx.Err() != context.DeadlineExceeded {
		return res, nil
	}
	if res.IsError {
		return toolError("%s timed out after %v", p.Name, timeout), nil
	}
	// Discovery keeps what it read before the deadline; say the result is partial.
	res.Content = append(res.Content, textContent{Type: "text", Text: fmt.Sprintf(
		"%s timed out after %v, so the result is partial: commands with a discovery_err were not read. "+
			"Ask for less depth, or start the server with a longer --timeout.", p.Name, timeout)})
	return res, nil
}

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aallbrig/treemand/models"
)

// fakeTree is a small two-level CLI: tool {build, remote {add}}.
func fakeTree() *models.Node {
	add := &models.Node{
		Name: "add", FullPath: []string{"tool", "remote", "add"}, Description: "Add a remote", Discovered: true,
		Flags:       []models.Flag{{Name: "--track", ShortName: "-t", ValueType: "string", Description: "branch to track"}},
		Positionals: []models.Positional{{Name: "name", Required: true}, {Name: "url", Required: true}},
		HelpText:    "usage: tool remote add [-t branch] <name> <url>",
	}
	remote := &models.Node{
		Name: "remote", FullPath: []string{"tool", "remote"}, Description: "Manage remotes", Discovered: true,
		Children: []*models.Node{add},
	}
	build := &models.Node{
		Name: "build", FullPath: []string{"tool", "build"}, Description: "Build the project", Discovered: true,
		Flags: []models.Flag{{Name: "--release", ValueType: "bool", Description: "optimised build"}},
	}
	return &models.Node{
		Name: "tool", FullPath: []string{"tool"}, Description: "A tool", Discovered: true,
		Flags:    []models.Flag{{Name: "--verbose", ShortName: "-v", Description: "verbose output"}},
		Children: []*models.Node{build, remote},
		HelpText: "tool does things",
	}
}

func newTestServer() *Server {
	return &Server{
		Name: "treemand", Version: "test",
		Discover: func(_ context.Context, cli string, _ int) (*models.Node, error) {
			if cli != "tool" {
				return nil, errors.New("command not found: " + cli)
			}
			return fakeTree(), nil
		},
		Timeout: 5 * time.Second,
	}
}

// exchange sends each request line to the server and returns one decoded
// response per non-empty output line.
func exchange(t *testing.T, s *Server, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var res []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("response is not JSON: %q: %v", l, err)
		}
		res = append(res, m)
	}
	return res
}

func call(id int, tool string, args map[string]any) string {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	return string(b)
}

// toolText returns the text of a tools/call result and whether it is an error.
func toolText(t *testing.T, resp map[string]any) (string, bool) {
	t.Helper()
	r, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", resp)
	}
	content := r["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	isErr, _ := r["isError"].(bool)
	return text, isErr
}

func TestInitialize_negotiatesVersionAndAdvertisesTools(t *testing.T) {
	res := exchange(t, newTestServer(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
	)
	if len(res) != 2 {
		t.Fatalf("want 2 responses (the notification gets none), got %d: %v", len(res), res)
	}
	r := res[0]["result"].(map[string]any)
	if r["protocolVersion"] != "2025-03-26" {
		t.Errorf("protocolVersion = %v, want the client's supported version", r["protocolVersion"])
	}
	if _, ok := r["capabilities"].(map[string]any)["tools"]; !ok {
		t.Errorf("capabilities lack tools: %v", r["capabilities"])
	}
	if r["serverInfo"].(map[string]any)["name"] != "treemand" {
		t.Errorf("serverInfo = %v", r["serverInfo"])
	}
	if v := res[1]["result"].(map[string]any)["protocolVersion"]; v != LatestProtocolVersion {
		t.Errorf("unknown client version: got %v, want %s", v, LatestProtocolVersion)
	}
}

func TestToolsList_namesTheThreeTools(t *testing.T) {
	res := exchange(t, newTestServer(), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	tools := res[0]["result"].(map[string]any)["tools"].([]any)
	var names []string
	for _, x := range tools {
		tool := x.(map[string]any)
		names = append(names, tool["name"].(string))
		if tool["inputSchema"].(map[string]any)["type"] != "object" {
			t.Errorf("%s inputSchema is not an object schema", tool["name"])
		}
		if tool["description"] == "" {
			t.Errorf("%s has no description", tool["name"])
		}
	}
	if strings.Join(names, ",") != "discover,search,assemble" {
		t.Errorf("tools = %v", names)
	}
}

func TestProtocolErrors(t *testing.T) {
	res := exchange(t, newTestServer(),
		`not json`,
		`{"jsonrpc":"2.0","id":7,"method":"nope"}`,
		`{"jsonrpc":"2.0","id":8,"method":"ping"}`,
		call(9, "missing", nil),
	)
	if code := res[0]["error"].(map[string]any)["code"]; code != float64(-32700) {
		t.Errorf("parse error code = %v", code)
	}
	if code := res[1]["error"].(map[string]any)["code"]; code != float64(-32601) || res[1]["id"] != float64(7) {
		t.Errorf("unknown method: %v", res[1])
	}
	if _, ok := res[2]["result"].(map[string]any); !ok {
		t.Errorf("ping: %v", res[2])
	}
	if code := res[3]["error"].(map[string]any)["code"]; code != float64(-32602) {
		t.Errorf("unknown tool code = %v", code)
	}
}

func TestDiscover_returnsTheTreeWithoutHelpText(t *testing.T) {
	res := exchange(t, newTestServer(),
		call(1, "discover", map[string]any{"cli": "tool"}),
		call(2, "discover", map[string]any{"cli": "tool", "include_help": true}),
	)
	text, isErr := toolText(t, res[0])
	if isErr {
		t.Fatalf("discover failed: %s", text)
	}
	var root models.Node
	if err := json.Unmarshal([]byte(text), &root); err != nil {
		t.Fatalf("discover text is not a tree: %v\n%s", err, text)
	}
	if root.SchemaVersion != models.SchemaVersion || len(root.Children) != 2 || root.Children[1].Children[0].Name != "add" {
		t.Errorf("unexpected tree: %+v", root)
	}
	if strings.Contains(text, "help_text") {
		t.Errorf("help text should be omitted by default")
	}
	if sc, ok := res[0]["result"].(map[string]any)["structuredContent"].(map[string]any); !ok || sc["name"] != "tool" {
		t.Errorf("structuredContent = %v", res[0]["result"])
	}
	text, _ = toolText(t, res[1])
	if !strings.Contains(text, "help_text") {
		t.Errorf("include_help should keep help text")
	}
}

func TestDiscover_rejectsUnsafeOrMissingCLI(t *testing.T) {
	for _, cli := range []string{"", "/bin/sh", "../x", "-rf", "a b", `..\x`} {
		res := exchange(t, newTestServer(), call(1, "discover", map[string]any{"cli": cli}))
		text, isErr := toolText(t, res[0])
		if !isErr || !strings.Contains(text, "cli") {
			t.Errorf("cli %q: want a tool error naming cli, got %q (isError=%v)", cli, text, isErr)
		}
	}
	res := exchange(t, newTestServer(), call(1, "discover", map[string]any{"cli": "nosuch"}))
	if text, isErr := toolText(t, res[0]); !isErr || !strings.Contains(text, "not found") {
		t.Errorf("missing CLI: %q", text)
	}
}

func TestDiscover_timesOut(t *testing.T) {
	s := newTestServer()
	s.Timeout = 20 * time.Millisecond
	s.Discover = func(ctx context.Context, _ string, _ int) (*models.Node, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	res := exchange(t, s, call(1, "discover", map[string]any{"cli": "tool"}))
	if text, isErr := toolText(t, res[0]); !isErr || !strings.Contains(text, "timed out") {
		t.Errorf("want a timeout error, got %q", text)
	}
}

// A negative Timeout (--timeout=0) puts no deadline on a tool call.
func TestDiscover_negativeTimeoutMeansNoLimit(t *testing.T) {
	s := newTestServer()
	s.Timeout = -1
	s.Discover = func(ctx context.Context, _ string, _ int) (*models.Node, error) {
		if _, ok := ctx.Deadline(); ok {
			return nil, errors.New("call has a deadline")
		}
		return fakeTree(), nil
	}
	res := exchange(t, s, call(1, "discover", map[string]any{"cli": "tool"}))
	if text, isErr := toolText(t, res[0]); isErr {
		t.Errorf("want a tree, got error %q", text)
	}
}

func TestSearch_findsCommandsAndFlags(t *testing.T) {
	res := exchange(t, newTestServer(),
		call(1, "search", map[string]any{"cli": "tool", "query": "remote add"}),
		call(2, "search", map[string]any{"cli": "tool", "query": "track"}),
		call(3, "search", map[string]any{"cli": "tool"}),
	)
	text, _ := toolText(t, res[0])
	var hits []Hit
	if err := json.Unmarshal([]byte(text), &hits); err != nil {
		t.Fatalf("search text: %v\n%s", err, text)
	}
	if len(hits) == 0 || hits[0].Command != "tool remote add" || hits[0].Kind != "command" {
		t.Errorf("first hit = %+v", hits)
	}
	text, _ = toolText(t, res[1])
	if !strings.Contains(text, `"kind": "flag"`) || !strings.Contains(text, `"name": "--track"`) {
		t.Errorf("flag search: %s", text)
	}
	if _, isErr := toolText(t, res[2]); !isErr {
		t.Errorf("search without query should be a tool error")
	}
}

func TestAssemble_buildsAQuotedCommand(t *testing.T) {
	res := exchange(t, newTestServer(),
		call(1, "assemble", map[string]any{
			"cli": "tool", "path": []string{"remote", "add"},
			"flags": []map[string]any{{"name": "track", "value": "main branch"}, {"name": "-v"}},
			"args":  []string{"origin", "git@host:o/r.git"},
		}),
		call(2, "assemble", map[string]any{"cli": "tool", "path": []string{"remote", "nope"}}),
		call(3, "assemble", map[string]any{"cli": "tool", "path": []string{"build"}, "flags": []map[string]any{{"name": "--track"}}}),
		call(4, "assemble", map[string]any{"cli": "tool", "args": []string{"it's"}}),
	)
	text, isErr := toolText(t, res[0])
	if isErr {
		t.Fatalf("assemble failed: %s", text)
	}
	want := `tool remote add --track 'main branch' --verbose origin git@host:o/r.git`
	var out Assembled
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("assemble text: %v\n%s", err, text)
	}
	if out.Command != want {
		t.Errorf("command = %q, want %q", out.Command, want)
	}
	if text, isErr := toolText(t, res[1]); !isErr || !strings.Contains(text, `"nope"`) || !strings.Contains(text, "add") {
		t.Errorf("unknown subcommand: %q", text)
	}
	text, isErr = toolText(t, res[2])
	var withWarning Assembled
	_ = json.Unmarshal([]byte(text), &withWarning)
	if isErr || withWarning.Command != "tool build --track" || len(withWarning.Warnings) != 1 ||
		!strings.Contains(withWarning.Warnings[0], "--release") {
		t.Errorf("a flag the help lacks should be kept with a warning naming the known flags: %q", text)
	}
	text, _ = toolText(t, res[3])
	if !strings.Contains(text, `tool 'it'\\''s'`) {
		t.Errorf("single quote not escaped: %s", text)
	}
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"plain": "plain", "": "''", "a b": "'a b'", "it's": `'it'\''s'`,
		"$HOME": "'$HOME'", "k=v,x/y.z:1@h%-+_": "k=v,x/y.z:1@h%-+_", "*": "'*'",
	} {
		if got := ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

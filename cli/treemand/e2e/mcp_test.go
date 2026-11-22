package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mcpSession is a running `treemand mcp` driven over stdio.
type mcpSession struct {
	t      *testing.T
	in     io.WriteCloser
	out    *bufio.Scanner
	cmd    *exec.Cmd
	stderr *strings.Builder
	nextID int
}

func startMCP(t *testing.T, extraEnv []string, args ...string) *mcpSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, binPath, append([]string{"mcp"}, args...)...)
	cmd.Env = env(t, extraEnv...)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &mcpSession{t: t, in: in, out: bufio.NewScanner(stdout), cmd: cmd, stderr: stderr}
	s.out.Buffer(make([]byte, 1<<20), 8<<20)
	t.Cleanup(func() { _ = in.Close(); _ = cmd.Wait() })
	return s
}

// request sends one JSON-RPC request and returns its decoded response.
func (s *mcpSession) request(method string, params any) map[string]any {
	s.t.Helper()
	s.nextID++
	msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": s.nextID, "method": method, "params": params})
	if _, err := fmt.Fprintf(s.in, "%s\n", msg); err != nil {
		s.t.Fatalf("write: %v", err)
	}
	if !s.out.Scan() {
		s.t.Fatalf("no response to %s (stderr: %s)", method, s.stderr.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(s.out.Bytes(), &resp); err != nil {
		s.t.Fatalf("stdout carried a non-JSON line %q: %v", s.out.Text(), err)
	}
	if resp["id"] != float64(s.nextID) {
		s.t.Fatalf("response id %v, want %d", resp["id"], s.nextID)
	}
	return resp
}

// tool calls a tool and returns its text and whether it reported an error.
func (s *mcpSession) tool(name string, args map[string]any) (string, bool) {
	s.t.Helper()
	resp := s.request("tools/call", map[string]any{"name": name, "arguments": args})
	r, ok := resp["result"].(map[string]any)
	if !ok {
		s.t.Fatalf("tools/call %s: %v", name, resp)
	}
	isErr, _ := r["isError"].(bool)
	var texts []string
	for _, c := range r["content"].([]any) {
		texts = append(texts, c.(map[string]any)["text"].(string))
	}
	return strings.Join(texts, "\n"), isErr
}

// An agent starts `treemand mcp`, lists its tools and uses each one on a CLI.
func TestMCP_agentDiscoversSearchesAndAssembles(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "calls.log")
	s := startMCP(t, []string{"FAKE_LOG=" + logFile})

	init := s.request("initialize", map[string]any{
		"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "e2e", "version": "1"},
	})
	if v := init["result"].(map[string]any)["protocolVersion"]; v != "2025-06-18" {
		t.Errorf("protocolVersion = %v", v)
	}
	fmt.Fprintln(s.in, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	list := s.request("tools/list", nil)
	var names []string
	for _, x := range list["result"].(map[string]any)["tools"].([]any) {
		names = append(names, x.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "discover,search,assemble" {
		t.Errorf("tools = %v", names)
	}

	text, isErr := s.tool("discover", map[string]any{"cli": "fakecache"})
	if isErr {
		t.Fatalf("discover: %s", text)
	}
	var tree struct {
		SchemaVersion int `json:"schema_version"`
		Children      []struct {
			Name  string `json:"name"`
			Flags []struct {
				Name string `json:"name"`
			} `json:"flags"`
		} `json:"children"`
	}
	if err := json.Unmarshal([]byte(text), &tree); err != nil {
		t.Fatalf("discover text: %v\n%s", err, text)
	}
	if tree.SchemaVersion != 1 || len(tree.Children) != 3 || tree.Children[0].Name != "alpha" {
		t.Errorf("tree = %+v", tree)
	}
	if strings.Contains(text, "help_text") {
		t.Errorf("help text should be left out by default")
	}

	text, _ = s.tool("search", map[string]any{"cli": "fakecache", "query": "gamma-only"})
	mustContain(t, "search result", text, `"command": "fakecache gamma"`, `"name": "--gamma-only"`)

	text, isErr = s.tool("assemble", map[string]any{
		"cli": "fakecache", "path": []string{"beta"},
		"flags": []map[string]any{{"name": "beta-only", "value": "two words"}},
		"args":  []string{"it's"},
	})
	if isErr {
		t.Fatalf("assemble: %s", text)
	}
	mustContain(t, "assembled command", text, `"command": "fakecache beta --beta-only 'two words' 'it'\\''s'"`)

	text, isErr = s.tool("assemble", map[string]any{"cli": "fakecache", "path": []string{"delta"}})
	if !isErr {
		t.Errorf("an unknown subcommand should be a tool error: %s", text)
	}
	mustContain(t, "unknown subcommand error", text, "alpha, beta, gamma")

	assertOnlyHelpProbes(t, logFile, "two words")
}

// assertOnlyHelpProbes fails unless every call the fake CLI logged was a help
// probe, and none carried built, never-run text.
func assertOnlyHelpProbes(t *testing.T, logFile, neverRun string) {
	t.Helper()
	logged, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(logged)), "\n") {
		if !strings.HasSuffix(line, "--help") && !strings.HasSuffix(line, "-h") && line != "help" &&
			!strings.HasPrefix(line, "help ") && !strings.Contains(line, "version") {
			t.Errorf("the server ran something other than a help probe: %q", line)
		}
	}
	if strings.Contains(string(logged), neverRun) {
		t.Errorf("assemble must never run the command it builds")
	}
}

// A tool call naming a path, a flag-like word or a missing program fails as a
// tool error, and the server keeps answering.
func TestMCP_rejectsUnsafeAndMissingCLIs(t *testing.T) {
	s := startMCP(t, nil)
	for _, cli := range []string{"/bin/sh", "-rf", "../fakecache"} {
		text, isErr := s.tool("discover", map[string]any{"cli": cli})
		if !isErr || !strings.Contains(text, "bare command name") {
			t.Errorf("cli %q: want a bare-name error, got %q", cli, text)
		}
	}
	text, isErr := s.tool("discover", map[string]any{"cli": "treemand-no-such-cli"})
	if !isErr || !strings.Contains(text, "treemand-no-such-cli") {
		t.Errorf("missing CLI: %q", text)
	}
	if r := s.request("ping", nil); r["result"] == nil {
		t.Errorf("server stopped answering after errors: %v", r)
	}
}

// Help probes slower than --timeout end the call on time, and the partial
// tree says it timed out.
func TestMCP_toolCallTimesOut(t *testing.T) {
	s := startMCP(t, []string{"FAKE_DELAY=5"}, "--timeout", "1", "--no-cache")
	start := time.Now()
	text, _ := s.tool("discover", map[string]any{"cli": "fakeslow", "depth": 2})
	mustContain(t, "timed-out discover", text, `"name": "fakeslow"`, "timed out after 1s", "partial")
	if d := time.Since(start); d > 4*time.Second {
		t.Errorf("timeout took %v", d)
	}
}

package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/cmd"
)

// runMCP sends requests, one JSON-RPC message per line, to `treemand mcp`
// and returns the responses keyed by id.
func runMCP(t *testing.T, requests []string, args ...string) map[float64]map[string]any {
	t.Helper()
	c := cmd.NewRootCmd()
	var out, errb bytes.Buffer
	c.SetIn(strings.NewReader(strings.Join(requests, "\n") + "\n"))
	c.SetOut(&out)
	c.SetErr(&errb)
	c.SetArgs(append(append([]string{}, args...), "mcp"))
	if err := c.Execute(); err != nil {
		t.Fatalf("mcp: %v\n%s", err, errb.String())
	}
	res := map[float64]map[string]any{}
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("response is not JSON: %q", l)
		}
		id, _ := m["id"].(float64)
		res[id] = m
	}
	return res
}

func mcpCall(id int, tool string, args map[string]any) string {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	return string(b)
}

// mcpText returns the text of a tools/call result and whether it is an error.
func mcpText(t *testing.T, resp map[string]any) (string, bool) {
	t.Helper()
	r, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", resp)
	}
	text := r["content"].([]any)[0].(map[string]any)["text"].(string)
	isErr, _ := r["isError"].(bool)
	return text, isErr
}

func TestMCP_discoverReturnsTheTreeAndCachesIt(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "faketool")
	res := runMCP(t, []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		mcpCall(2, "discover", map[string]any{"cli": "faketool", "depth": 2}),
		mcpCall(3, "discover", map[string]any{"cli": "faketool", "depth": 2}),
	})
	if v, _ := res[1]["result"].(map[string]any)["serverInfo"].(map[string]any)["name"].(string); v != "treemand" {
		t.Errorf("serverInfo name = %q", v)
	}
	text, isErr := mcpText(t, res[2])
	if isErr {
		t.Fatalf("discover failed: %s", text)
	}
	root := decodeTree(t, text)
	if a := root.child("alpha"); a == nil || a.child("deep") == nil {
		t.Errorf("discover missed nested commands: %s", text)
	}
	if again, _ := mcpText(t, res[3]); again != text {
		t.Errorf("the second discover differs from the first")
	}
	// The second call was served from the cache: help ran once per command.
	if calls := e.calls(t); strings.Count(calls, "alpha --help") != 1 {
		t.Errorf("alpha --help ran %d times, want 1 (cache):\n%s", strings.Count(calls, "alpha --help"), calls)
	}
}

func TestMCP_discoverOfAMissingCLIIsAToolError(t *testing.T) {
	newFakeEnv(t)
	res := runMCP(t, []string{mcpCall(1, "discover", map[string]any{"cli": "no-such-cli-xyz"})}, "--no-cache")
	text, isErr := mcpText(t, res[1])
	if !isErr || !strings.Contains(text, "no-such-cli-xyz") {
		t.Errorf("want a tool error naming the CLI, got %v %q", isErr, text)
	}
}

func TestMCP_assembleExpandsStubsOnTheWay(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "faketool")
	cfgPath := filepath.Join(e.dir, "config.yaml")
	// A stub threshold of 1 leaves alpha and beta undiscovered stubs, so
	// assemble has to expand alpha to check its flag.
	if err := os.WriteFile(cfgPath, []byte("stub_threshold: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res := runMCP(t, []string{mcpCall(1, "assemble", map[string]any{
		"cli": "faketool", "path": []string{"alpha"},
		"flags": []map[string]any{{"name": "alpha-flag", "value": "x y"}},
	})}, "--config="+cfgPath, "--no-cache", "--timeout=0")
	text, isErr := mcpText(t, res[1])
	if isErr {
		t.Fatalf("assemble failed: %s", text)
	}
	var got struct {
		Command  string   `json:"command"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("assemble result is not JSON: %q", text)
	}
	if got.Command != "faketool alpha --alpha-flag 'x y'" {
		t.Errorf("command = %q", got.Command)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("alpha's flag was not found, so the stub was not expanded: %v", got.Warnings)
	}
}

func TestMCP_rejectsANegativeTimeout(t *testing.T) {
	newFakeEnv(t)
	_, err := runCmd("--timeout=-1", "mcp")
	if err == nil || cmd.ExitCodeFor(err) != 2 {
		t.Errorf("want a usage error (exit 2), got %v", err)
	}
}

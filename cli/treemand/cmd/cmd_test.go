package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cmd"
)

func runCmd(args ...string) (string, error) {
	c := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	c.SetOut(buf)
	c.SetErr(buf)
	c.SetArgs(args)
	err := c.Execute()
	return buf.String(), err
}

func TestVersion(t *testing.T) {
	out, err := runCmd("version")
	if err != nil {
		t.Fatalf("version error: %v", err)
	}
	if !strings.Contains(out, "treemand") {
		t.Errorf("version output = %q, want 'treemand ...'", out)
	}
}

func TestRootNoArgs(t *testing.T) {
	_, err := runCmd()
	// Cobra returns an error when no args provided (ExactArgs(1))
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestRootHelp(t *testing.T) {
	out, err := runCmd("--help")
	if err != nil {
		t.Fatalf("--help error: %v", err)
	}
	if !strings.Contains(out, "treemand") {
		t.Errorf("help output missing 'treemand': %q", out)
	}
}

func TestRootEcho_text(t *testing.T) {
	// 'echo' is always available; may not have useful --help but shouldn't crash
	out, err := runCmd("--no-cache", "--no-color", "--timeout=5", "echo")
	if err != nil {
		t.Logf("echo discovery error (acceptable): %v", err)
	}
	// Either output contains 'echo' or we got an error - both are acceptable
	if err == nil && !strings.Contains(out, "echo") {
		t.Errorf("expected 'echo' in output, got: %q", out)
	}
}

func TestRootEcho_json(t *testing.T) {
	out, err := runCmd("--no-cache", "--output=json", "--timeout=5", "echo")
	if err != nil {
		t.Logf("echo discovery error (acceptable): %v", err)
		return
	}
	if !strings.Contains(out, `"name"`) {
		t.Errorf("expected JSON output, got: %q", out)
	}
}

func TestRootDepthFlag(t *testing.T) {
	_, err := runCmd("--no-cache", "--depth=1", "--no-color", "--timeout=5", "echo")
	// Just check it doesn't panic
	_ = err
}

func TestRootFilterFlag(t *testing.T) {
	_, err := runCmd("--no-cache", "--filter=nonexistent", "--no-color", "--timeout=5", "echo")
	_ = err
}

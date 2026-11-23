//go:build !windows

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A command run from the TUI is remembered for that CLI: `treemand history`
// prints it, and Ctrl+R in a later session loads it back into the preview.
func TestHistory_aCommandRunFromTheTUIIsRemembered(t *testing.T) {
	cacheEnv := []string{"TREEMAND_CACHE_DIR=" + filepath.Join(t.TempDir(), "cache")}

	s := startWithEnv(t, cacheEnv, "-i", "--no-cache", "fakecobra")
	s.waitFor("deploy", 15*time.Second)
	s.stepDownUntil("fakecobra build", 12)
	s.send("\r")
	s.waitFor("set: fakecobra build", 5*time.Second)
	s.send("\x05") // Ctrl+E
	s.waitFor("Execute Command", 5*time.Second)
	s.send("r")
	s.exitCode(10 * time.Second) // the fake's own exit code does not matter

	res := run(t, 10*time.Second, cacheEnv, "history", "fakecobra")
	if res.exit != 0 || strings.TrimSpace(res.stdout) != "fakecobra build" {
		t.Fatalf("history: exit %d stdout %q stderr %q", res.exit, res.stdout, res.stderr)
	}
	res = run(t, 10*time.Second, cacheEnv, "history", "fakecobra", "--output", "json")
	var entries []struct {
		Command string `json:"command"`
		Count   int    `json:"count"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &entries); err != nil || len(entries) != 1 || entries[0].Count != 1 {
		t.Fatalf("history json = %q (%v)", res.stdout, err)
	}

	s = startWithEnv(t, cacheEnv, "-i", "--no-cache", "fakecobra")
	s.waitFor("deploy", 15*time.Second)
	s.stepDownUntil("fakecobra --", 6) // the TUI is reading keys
	s.send("\x12")                     // Ctrl+R
	s.waitFor("Recent Commands", 5*time.Second)
	s.send("\r")
	s.waitFor("loaded: fakecobra build", 5*time.Second)
	s.send("\x03")
	s.exitCode(10 * time.Second)
}

func TestHistory_emptyAndOptOut(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	cacheEnv := []string{"TREEMAND_CACHE_DIR=" + cacheDir}

	res := run(t, 10*time.Second, cacheEnv, "history", "fakecobra")
	if res.exit != 0 || res.stdout != "" || !strings.Contains(res.stderr, "No history for fakecobra yet") {
		t.Fatalf("empty history: exit %d stdout %q stderr %q", res.exit, res.stdout, res.stderr)
	}
	res = run(t, 10*time.Second, cacheEnv, "history")
	if res.exit != 2 {
		t.Errorf("history without a CLI: exit %d, want 2 (usage)", res.exit)
	}

	// With history off, running a command from the TUI writes nothing.
	off := []string{cacheEnv[0], "TREEMAND_NO_HISTORY=1"}
	s := startWithEnv(t, off, "-i", "--no-cache", "fakecobra")
	s.waitFor("deploy", 15*time.Second)
	s.stepDownUntil("fakecobra --", 6) // the TUI is reading keys
	s.send("\x12")                     // Ctrl+R
	s.waitFor("history is off", 5*time.Second)
	s.send("\x05")
	s.waitFor("Execute Command", 5*time.Second)
	s.send("r")
	s.exitCode(10 * time.Second)

	if _, err := os.Stat(filepath.Join(cacheDir, "history")); !os.IsNotExist(err) {
		t.Errorf("history dir exists with history off (stat err %v)", err)
	}
	res = run(t, 10*time.Second, off, "history", "fakecobra")
	if !strings.Contains(res.stderr, "history is off") {
		t.Errorf("history with no_history: stderr %q", res.stderr)
	}
}

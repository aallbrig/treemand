package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aallbrig/treemand/history"
)

// historyEnv isolates the cache and config so no real history is read.
func historyEnv(t *testing.T) string {
	t.Helper()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	t.Setenv("TREEMAND_CACHE_DIR", cacheDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TREEMAND_NO_HISTORY", "")
	return cacheDir
}

func TestHistoryCmd_printsNewestFirst(t *testing.T) {
	cacheDir := historyEnv(t)
	s := history.Open(history.Dir(cacheDir), "git")
	now := time.Now()
	_ = s.Add("git status", now)
	_ = s.Add("git log --oneline", now.Add(time.Second))
	_ = s.Add("git status", now.Add(2*time.Second))

	out, _, err := runCmdSplit("history", "git")
	if err != nil {
		t.Fatal(err)
	}
	if out != "git status\ngit log --oneline\n" {
		t.Fatalf("text = %q", out)
	}

	out, _, err = runCmdSplit("history", "/usr/bin/git", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []history.Entry
	if err := json.Unmarshal([]byte(out), &entries); err != nil || len(entries) != 2 || entries[0].Count != 2 {
		t.Fatalf("json = %q (%v)", out, err)
	}

	out, _, err = runCmdSplit("history", "git", "--output", "yaml")
	if err != nil || !strings.Contains(out, "command: git status") {
		t.Fatalf("yaml = %q (%v)", out, err)
	}
}

func TestHistoryCmd_emptyOffAndErrors(t *testing.T) {
	cacheDir := historyEnv(t)

	out, errOut, err := runCmdSplit("history", "git")
	if err != nil || out != "" || !strings.Contains(errOut, "No history for git yet") {
		t.Fatalf("empty: out %q err %q (%v)", out, errOut, err)
	}
	out, _, err = runCmdSplit("history", "git", "--output", "json")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty json: %q (%v)", out, err)
	}
	if _, _, err := runCmdSplit("history", "git", "--output", "xml"); err == nil {
		t.Error("an unknown output format should fail")
	}
	if _, _, err := runCmdSplit("history"); err == nil {
		t.Error("history without a CLI should fail")
	}

	// A corrupt file is reported, not printed as empty.
	if err := history.Open(history.Dir(cacheDir), "git").Add("git status", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(history.Dir(cacheDir), "git.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runCmdSplit("history", "git"); err == nil {
		t.Error("a corrupt history file should fail")
	}

	t.Setenv("TREEMAND_NO_HISTORY", "1")
	out, errOut, err = runCmdSplit("history", "git")
	if err != nil || out != "" || !strings.Contains(errOut, "history is off") {
		t.Fatalf("off: out %q err %q (%v)", out, errOut, err)
	}
}

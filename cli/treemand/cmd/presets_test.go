package cmd_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cmd"
	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/savedpresets"
)

// presetsEnv isolates the config directory and returns the git store.
func presetsEnv(t *testing.T) *savedpresets.Store {
	t.Helper()
	historyEnv(t)
	dir, err := config.PresetsDir()
	if err != nil {
		t.Fatal(err)
	}
	return savedpresets.Open(dir, "git")
}

func TestPresetsCmd_listsInEveryFormat(t *testing.T) {
	s := presetsEnv(t)
	_ = s.Save("amend", "git commit --amend")
	_ = s.Save("st", "git status -sb")

	out, _, err := runCmdSplit("presets", "/usr/bin/git")
	if err != nil {
		t.Fatal(err)
	}
	if out != "amend  git commit --amend\nst     git status -sb\n" {
		t.Fatalf("text = %q", out)
	}

	out, _, err = runCmdSplit("presets", "git", "--output", "json")
	var list []savedpresets.Preset
	if err != nil || json.Unmarshal([]byte(out), &list) != nil || len(list) != 2 || list[1].Name != "st" {
		t.Fatalf("json = %q (%v)", out, err)
	}

	out, _, err = runCmdSplit("presets", "git", "--output", "yaml")
	if err != nil || !strings.Contains(out, "name: amend") {
		t.Fatalf("yaml = %q (%v)", out, err)
	}
}

func TestPresetsCmd_emptyAndErrors(t *testing.T) {
	s := presetsEnv(t)

	out, errOut, err := runCmdSplit("presets", "git")
	if err != nil || out != "" || !strings.Contains(errOut, "No presets for git yet") {
		t.Fatalf("empty: out %q err %q (%v)", out, errOut, err)
	}
	out, _, err = runCmdSplit("presets", "git", "--output", "json")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty json = %q (%v)", out, err)
	}
	if _, _, err = runCmdSplit("presets", "git", "--output", "xml"); cmd.ExitCodeFor(err) != cmd.ExitUsage {
		t.Errorf("bad output: %v", err)
	}
	if _, _, err = runCmdSplit("presets"); cmd.ExitCodeFor(err) != cmd.ExitUsage {
		t.Errorf("no CLI: %v", err)
	}
	// A directory where the file should be cannot be read.
	if err := os.MkdirAll(s.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err = runCmdSplit("presets", "git"); err == nil || cmd.ExitCodeFor(err) != cmd.ExitFailure {
		t.Errorf("unreadable: %v", err)
	}
}

func TestRootPreset_unknownNameIsAUsageError(t *testing.T) {
	s := presetsEnv(t)

	_, _, err := runCmdSplit("--preset", "nope", "git")
	if cmd.ExitCodeFor(err) != cmd.ExitUsage || !strings.Contains(cmd.FormatError(err), "Save one in") {
		t.Fatalf("no presets: %v", cmd.FormatError(err))
	}
	_ = s.Save("amend", "git commit --amend")
	_ = s.Save("st", "git status")
	_, _, err = runCmdSplit("--preset", "nope", "git")
	if msg := cmd.FormatError(err); cmd.ExitCodeFor(err) != cmd.ExitUsage ||
		!strings.Contains(msg, `no saved preset "nope" for git`) || !strings.Contains(msg, "Saved presets: amend, st") {
		t.Fatalf("unknown preset: %s", msg)
	}

	// An unreadable presets file is a runtime failure, not a usage error.
	if err := os.Remove(s.Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err = runCmdSplit("--preset", "amend", "git"); err == nil || cmd.ExitCodeFor(err) != cmd.ExitFailure {
		t.Errorf("unreadable: %v", err)
	}
}

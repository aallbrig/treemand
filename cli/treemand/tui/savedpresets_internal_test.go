package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/savedpresets"
)

func TestPresets_saveFromTheDialog(t *testing.T) {
	m, root := newModalModel(t)
	store := savedpresets.Open(t.TempDir(), "git")
	m.SetSavedPresets(store)

	selectNode(t, m, root.Children[1])
	press(m, "enter", "ctrl+o")
	if view := m.View(); !m.pm.active || !strings.Contains(view, "Saved Presets") || !strings.Contains(view, "No presets saved") {
		t.Fatalf("ctrl+o should open the empty presets dialog:\n%s", view)
	}
	press(m, "a")
	typeText(m, "bad name")
	press(m, "enter")
	if !m.pm.naming || !strings.Contains(m.View(), "may only use") {
		t.Fatalf("an invalid name should keep the prompt open with the reason:\n%s", m.View())
	}
	press(m, "ctrl+u") // clear the input
	typeText(m, "rem")
	press(m, "enter")
	if m.pm.naming || !strings.Contains(m.statusMsg, "saved preset rem") {
		t.Fatalf("after save: naming=%v status=%q", m.pm.naming, m.statusMsg)
	}
	if p, ok, _ := store.Get("rem"); !ok || p.Command != "git remote" {
		t.Fatalf("stored = %+v %v", p, ok)
	}
}

func TestPresets_loadAndDelete(t *testing.T) {
	store := savedpresets.Open(t.TempDir(), "git")
	_ = store.Save("rem", "git remote")
	m, _ := newModalModel(t)
	m.SetSavedPresets(store)

	press(m, "ctrl+o")
	if view := m.View(); !strings.Contains(view, "rem") || !strings.Contains(view, "git remote") {
		t.Fatalf("list view:\n%s", view)
	}
	press(m, "enter")
	if m.pm.active || previewText(m) != "git remote" || !strings.Contains(m.statusMsg, "loaded preset rem") {
		t.Fatalf("after enter: active=%v preview=%q status=%q", m.pm.active, previewText(m), m.statusMsg)
	}

	press(m, "ctrl+o", "x")
	if _, ok, _ := store.Get("rem"); ok {
		t.Fatal("x did not delete the preset")
	}
	if !m.pm.active || !strings.Contains(m.View(), "deleted preset rem") {
		t.Fatalf("after delete:\n%s", m.View())
	}
	press(m, "ctrl+o")
	if m.pm.active {
		t.Error("a second ctrl+o should close the dialog")
	}
}

func TestPresets_navigateCancelAndNothingToSave(t *testing.T) {
	m, _ := newModalModel(t)
	store := savedpresets.Open(t.TempDir(), "git")
	_ = store.Save("a", "git status")
	_ = store.Save("b", "git log")
	m.SetSavedPresets(store)

	press(m, "ctrl+o", "up", "down", "down", "down")
	if m.pm.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.pm.cursor)
	}
	press(m, "k", "j", "a")
	if !m.pm.naming {
		t.Fatal("a should open the name prompt")
	}
	press(m, "esc")
	if m.pm.naming || !m.pm.active {
		t.Fatal("esc in the prompt should go back to the list")
	}

	// With an empty assembled command there is nothing to save.
	press(m, "ctrl+k")
	m.preview.ClearAll()
	press(m, "a")
	if m.pm.naming || !strings.Contains(m.View(), "nothing to save") {
		t.Fatalf("empty command:\n%s", m.View())
	}
	press(m, "q")
	if m.pm.active {
		t.Error("q should close the dialog")
	}
}

func TestPresets_offAndUnreadable(t *testing.T) {
	m, _ := newModalModel(t)
	press(m, "ctrl+o")
	if m.pm.active || !strings.Contains(m.statusMsg, "presets are unavailable") {
		t.Fatalf("no store: active=%v status=%q", m.pm.active, m.statusMsg)
	}

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "git.txt"), 0o700); err != nil {
		t.Fatal(err)
	}
	m.SetSavedPresets(savedpresets.Open(dir, "git"))
	press(m, "ctrl+o")
	if m.pm.active || !strings.Contains(m.statusMsg, "presets unreadable") {
		t.Fatalf("unreadable: active=%v status=%q", m.pm.active, m.statusMsg)
	}
}

func TestPresets_loadPresetAtStart(t *testing.T) {
	m, _ := newModalModel(t)
	m.loadStartPreset(savedpresets.Preset{Name: "amend", Command: "git commit --amend"})
	if previewText(m) != "git commit --amend" || !strings.Contains(m.timedMsg, "loaded preset amend") {
		t.Fatalf("preview=%q timed=%q", previewText(m), m.timedMsg)
	}
	if m.Init() == nil {
		t.Error("Init should schedule clearing the message")
	}
	if view := m.View(); !strings.Contains(view, "loaded preset amend") {
		t.Errorf("the message should survive a render:\n%s", view)
	}
}

func TestPresets_saveErrorStaysInPrompt(t *testing.T) {
	m, root := newModalModel(t)
	dir := t.TempDir()
	store := savedpresets.Open(dir, "git")
	m.SetSavedPresets(store)
	selectNode(t, m, root.Children[1])
	press(m, "enter", "ctrl+o", "a")
	// The directory turns into a file, so the save fails.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	typeText(m, "x")
	press(m, "enter")
	if !m.pm.naming || m.pm.err == "" {
		t.Fatalf("a failed save should keep the prompt with the error: naming=%v err=%q", m.pm.naming, m.pm.err)
	}
	if _, err := os.Stat(store.Path()); !errors.Is(err, os.ErrNotExist) && err == nil {
		t.Error("file written despite the error")
	}
}

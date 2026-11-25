package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/config"
)

func newSettingsFile(t *testing.T, content string) *config.Settings {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return &config.Settings{Path: path, Getenv: func(string) string { return "" }}
}

// selectSetting moves the settings cursor to key.
func selectSetting(t *testing.T, m *Model, key string) {
	t.Helper()
	for i, s := range m.st.list {
		if s.Key == key {
			m.st.cursor = i
			return
		}
	}
	t.Fatalf("no setting %s", key)
}

// A theme or layout key applies to the running TUI as soon as it is saved.
func TestSettings_layoutKeyAppliesLive(t *testing.T) {
	m, _ := newModalModel(t)
	s := newSettingsFile(t, "# mine\n")
	m.SetSettings(s)

	press(m, ",")
	view := m.View()
	if !m.st.active || !strings.Contains(view, "Settings") || !strings.Contains(view, "tree_style") {
		t.Fatalf(", should open the settings screen:\n%s", view)
	}
	selectSetting(t, m, "tree_style")
	if !strings.Contains(m.View(), "one of: default, columns, compact, graph") {
		t.Errorf("the detail line should list the allowed values:\n%s", m.View())
	}
	press(m, "enter", "ctrl+u")
	typeText(m, "graph")
	press(m, "enter")
	if m.st.editing || !strings.Contains(m.st.note, "tree_style = graph; applied") {
		t.Fatalf("after save: editing=%v note=%q err=%q", m.st.editing, m.st.note, m.st.err)
	}
	if m.cfg.TreeStyle != config.StyleGraph {
		t.Errorf("TreeStyle = %v, want graph live", m.cfg.TreeStyle)
	}
	if got, _ := s.Get("tree_style"); got.Value != "graph" || got.Source != config.SourceFile {
		t.Errorf("file value = %+v", got)
	}
	press(m, "q")
	if m.st.active {
		t.Error("q should close the settings screen")
	}
}

// A key a flag sets for this run is saved, and the note says the flag wins.
func TestSettings_flagStillWinsAndEditCanBeAbandoned(t *testing.T) {
	m, _ := newModalModel(t)
	s := newSettingsFile(t, "")
	s.Flags = map[string]string{"icons": "nerd"}
	m.SetSettings(s)
	press(m, ",")
	selectSetting(t, m, "icons")
	press(m, "enter", "ctrl+u")
	typeText(m, "ascii")
	press(m, "enter")
	if !strings.Contains(m.st.note, "this run's flag still wins") || m.cfg.IconPreset == "ascii" {
		t.Errorf("note = %q preset = %q", m.st.note, m.cfg.IconPreset)
	}

	press(m, "enter", "esc")
	if m.st.editing || !m.st.active {
		t.Errorf("esc in the prompt should go back to the list: editing=%v active=%v", m.st.editing, m.st.active)
	}
	press(m, "down", "up", "enter", "ctrl+c")
	if m.st.active {
		t.Error("ctrl+c in the prompt should close the screen")
	}
}

func TestSettings_unavailableOrUnreadable(t *testing.T) {
	m, _ := newModalModel(t)
	press(m, ",")
	if m.st.active || m.statusMsg != "settings are unavailable" {
		t.Errorf("no settings: active=%v status=%q", m.st.active, m.statusMsg)
	}
	m.SetSettings(newSettingsFile(t, "icons: [\n"))
	press(m, ",")
	if m.st.active || !strings.HasPrefix(m.statusMsg, "config unreadable") {
		t.Errorf("bad YAML: active=%v status=%q", m.st.active, m.statusMsg)
	}
}

func TestSettings_writeFailureIsShown(t *testing.T) {
	m, _ := newModalModel(t)
	s := newSettingsFile(t, "icons: ascii\n")
	m.SetSettings(s)
	press(m, ",")
	// The file turns into a list after the screen opened: the write is refused.
	if err := os.WriteFile(s.Path, []byte("- a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	press(m, "enter", "enter")
	if !m.st.editing || !strings.Contains(m.View(), "not a mapping") {
		t.Errorf("a refused write should keep the prompt open with the reason:\n%s", m.View())
	}
}

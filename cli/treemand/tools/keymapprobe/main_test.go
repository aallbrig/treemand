package main

import (
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/tui"
)

func fixtureConfig() Config {
	return Config{
		OverlayConst: "keybindText",
		Reference:    "reference.md",
		Pages:        []string{"reference.md"},
		Modes: []Mode{
			{Name: "tree:vim", Scheme: "vim", Overlay: true, Reference: true,
				Chain: []SwitchRef{{"updateKeys", 0}, {"handleVim", 0}},
				Hints: []HintRef{{Func: "schemeHints", Clause: "SchemeVim"}}},
			{Name: "exec", Chain: []SwitchRef{{"updateModal", 0}},
				Hints: []HintRef{{Func: "renderModal", Contains: "Esc"}}},
		},
	}
}

// The fixture plants each kind of finding a known number of times.
func TestScanCountsEachKindOfFinding(t *testing.T) {
	rep, err := Scan("testdata/tui", "testdata/site", fixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{
		"conflicts":                       1, // j in handleVim, shadowed by updateKeys
		"unreachable_bindings":            1, // shift+l: the terminal sends L
		"undocumented_bindings.overlay":   3, // q, d, z
		"undocumented_bindings.reference": 3, // q, d, z
		"undocumented_bindings.dialogs":   1, // c in the exec modal
		"documented_unbound.overlay":      2, // Shift+L (L), x
		"documented_unbound.hints":        1, // y in the vim footer
		"documented_unbound.website":      3, // Shift+→, Shift+L, w (vim column)
	}
	for k, v := range want {
		if rep.Metrics[k] != v {
			t.Errorf("%s = %v, want %v", k, rep.Metrics[k], v)
		}
	}
	if len(rep.Metrics) != len(want)+1 || rep.Metrics["bindings"] == 0 { // + bindings, informational
		t.Errorf("metrics %v, want exactly %v", rep.Metrics, want)
	}
	var sawSends bool
	for _, d := range rep.Details {
		if d.Metric == "unreachable_bindings" && d.Key == "shift+l" && strings.Contains(d.Note, `"L"`) {
			sawSends = true
		}
		if d.Where == "" {
			t.Errorf("finding without a location: %+v", d)
		}
	}
	if !sawSends {
		t.Errorf("unreachable shift+l should say the terminal sends \"L\": %+v", rep.Details)
	}
}

// A key switch the mode table does not name is drift the probe must refuse
// to ignore, as is a table entry that no longer matches the code.
func TestScanRejectsDrift(t *testing.T) {
	cfg := fixtureConfig()
	cfg.Modes = cfg.Modes[:1]
	if _, err := Scan("testdata/tui", "testdata/site", cfg); err == nil || !strings.Contains(err.Error(), "updateModal") {
		t.Errorf("an unmapped key switch should fail naming it, got %v", err)
	}
	cfg = fixtureConfig()
	cfg.Modes[0].Chain = append(cfg.Modes[0].Chain, SwitchRef{"gone", 0})
	if _, err := Scan("testdata/tui", "testdata/site", cfg); err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("a missing function should fail naming it, got %v", err)
	}
	cfg = fixtureConfig()
	cfg.Modes[1].Hints[0].Contains = "no such hint"
	if _, err := Scan("testdata/tui", "testdata/site", cfg); err == nil {
		t.Error("a hint that matches no literal should fail")
	}
}

func TestDocKeysNormaliseToWhatTheTerminalSends(t *testing.T) {
	cases := map[string][]string{
		"Shift+h/l": {"H", "L"},
		"Shift+←→":  {"shift+left", "shift+right"},
		"↑↓/jk":     {"up", "down", "j", "k"},
		"PgUp/PgDn": {"pgup", "pgdown"},
		"Ctrl+E":    {"ctrl+e"},
		"gg/G":      {"g", "G"},
		"/":         {"/"},
		"Enter/R":   {"enter", "R"},
		"Space":     {" "},
		"Shift+Tab": {"shift+tab"},
		"Backspace": {"backspace"},
		"Type":      nil,
	}
	for in, want := range cases {
		got := docKeys(in)
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("docKeys(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHintKeysReadEachHintStyle(t *testing.T) {
	cases := map[string][]string{
		"j/k:nav  Shift+h/l:subtree  /:filter  ?:keys": {"j", "k", "H", "L", "/", "?"},
		"↑↓/jk scroll · Esc close":                     {"up", "down", "j", "k", "esc"},
		"[Enter/R] Run  [C] Copy  [Esc] Cancel":        {"enter", "R", "C", "esc"},
		"Type value · Enter confirm · Esc cancel":      {"enter", "esc"},
		"↑↓:scroll  PgUp/PgDn  g/G:top/bottom":         {"up", "down", "pgup", "pgdown", "g", "G"},
	}
	for in, want := range cases {
		got := hintKeys(in)
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("hintKeys(%q) = %q, want %q", in, got, want)
		}
	}
}

// The real TUI scans without drift, and the scan finds the key handlers.
func TestRealTUIScans(t *testing.T) {
	rep, err := Scan("../../tui", "../../../../www/treemand/content", DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Bound < 60 {
		t.Errorf("found only %d bindings; the scan is probably broken", rep.Bound)
	}
}

// Modes that bind from the key map are read from the table: a key bound
// twice in one layer conflicts, an upper layer may override a lower one, and
// the footer text the key map renders is checked like any hint.
func TestScanReadsTheKeyMap(t *testing.T) {
	cfg := Config{
		OverlayText: "Keys\n  j / k   Move\n  x       Nothing binds this",
		Table: []tui.Binding{
			{Mode: "base", Keys: []string{"j", "k"}},
			{Mode: "top", Keys: []string{"j"}},            // overrides base: fine
			{Mode: "top", Keys: []string{"j", "shift+q"}}, // j again: a conflict
		},
		Modes: []Mode{{Name: "top", Overlay: true, Layers: []string{"top", "base"},
			Hints: []HintRef{{Text: "j/k:move  y:nothing"}}}},
	}
	rep, err := Scan("testdata/keymaptui", "testdata/site", cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{
		"conflicts":                  1, // j twice in top
		"unreachable_bindings":       1, // shift+q
		"documented_unbound.overlay": 1, // x
		"documented_unbound.hints":   1, // y
	}
	for k, v := range want {
		if rep.Metrics[k] != v {
			t.Errorf("%s = %v, want %v (details %+v)", k, rep.Metrics[k], v, rep.Details)
		}
	}
}

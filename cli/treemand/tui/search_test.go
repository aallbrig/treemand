package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/tui"
)

func typeRunes(m *tui.Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func openSearch(t *testing.T) *tui.Model {
	t.Helper()
	m := tui.NewModel(sampleTree(), config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	return m
}

func TestSearchModal_opensAndShowsResults(t *testing.T) {
	m := openSearch(t)
	typeRunes(m, "amend")
	view := m.View()
	if !strings.Contains(view, "Find") || !strings.Contains(view, "--amend") {
		t.Errorf("expected finder with --amend hit, got:\n%s", view)
	}
}

func TestSearchModal_enterJumpsToFlag(t *testing.T) {
	m := openSearch(t)
	typeRunes(m, "amend")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.TreeModel().SelectedItem()
	if sel == nil || sel.Kind != tui.SelFlag || sel.Flag.Name != "--amend" {
		t.Fatalf("selection = %+v, want flag --amend", sel)
	}
	if strings.Contains(m.View(), "Find") {
		t.Error("modal should be closed after Enter")
	}
}

func TestSearchModal_enterJumpsToNestedCommand(t *testing.T) {
	m := openSearch(t)
	typeRunes(m, "remote add")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	n := m.TreeModel().Selected()
	if n == nil || n.Name != "add" {
		t.Fatalf("selected = %+v, want add", n)
	}
}

func TestSearchModal_jumpsToPositional(t *testing.T) {
	m := openSearch(t)
	typeRunes(m, "msg")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.TreeModel().SelectedItem()
	if sel == nil || sel.Kind != tui.SelPositional || sel.Positional.Name != "msg" {
		t.Fatalf("selection = %+v, want positional msg", sel)
	}
}

func TestSearchModal_escCloses(t *testing.T) {
	m := openSearch(t)
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if strings.Contains(m.View(), "Find") {
		t.Error("Esc should close the finder")
	}
}

func TestSearchModal_noMatches(t *testing.T) {
	m := openSearch(t)
	typeRunes(m, "zzzqqq")
	if !strings.Contains(m.View(), "no matches") {
		t.Error("expected 'no matches'")
	}
}

func TestFilter_isFuzzyAndEscClears(t *testing.T) {
	m := tui.NewModel(sampleTree(), config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	full := m.TreeModel().RowCount()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	typeRunes(m, "cmt")
	if got := m.TreeModel().RowCount(); got >= full {
		t.Errorf("fuzzy 'cmt' should narrow rows (%d >= %d)", got, full)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := m.TreeModel().RowCount(); got != full {
		t.Errorf("Esc should clear filter: rows %d, want %d", got, full)
	}
}

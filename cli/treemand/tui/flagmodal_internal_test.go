package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/aallbrig/treemand/cli/treemand/config"
	"github.com/aallbrig/treemand/cli/treemand/models"
)

var updateFlagModalGolden = flag.Bool("update-flag-modal", false, "rewrite testdata/flag_modal.golden")

// flagModalEntries covers every row style the picker draws: each value-type
// colour, short names, an added flag, long descriptions and global flags.
func flagModalEntries() []flagEntry {
	long := "a description long enough to be truncated at every modal width we draw here"
	local := []models.Flag{
		{Name: "--verbose", ShortName: "v", Description: "print more"},
		{Name: "--force", ValueType: "bool", Description: long},
		{Name: "--name", ShortName: "n", ValueType: "string", Description: "the name"},
		{Name: "--count", ValueType: "int", Description: long},
		{Name: "--path", ValueType: "path"},
		{Name: "--timeout", ValueType: "duration", Description: "how long"},
		{Name: "--output", ShortName: "o", ValueType: "string", Description: long},
		{Name: "--dry-run", Description: "do nothing"},
	}
	global := []models.Flag{
		{Name: "--config", ValueType: "string", Description: "config file"},
		{Name: "--debug", Description: long},
		{Name: "--log-level", ValueType: "level", Description: "log level"},
		{Name: "--help", ShortName: "h", Description: "show help"},
		{Name: "--quiet", ShortName: "q", Description: "less output"},
		{Name: "--color", ValueType: "string", Description: "when to colour"},
		{Name: "--no-cache", Description: "skip the cache"},
		{Name: "--profile", ValueType: "string", Description: long},
	}
	var entries []flagEntry
	for i, f := range local {
		entries = append(entries, flagEntry{flag: f, added: i == 2 || i == 5})
	}
	for i, f := range global {
		entries = append(entries, flagEntry{flag: f, global: true, added: i == 1})
	}
	return entries
}

func TestRenderFlagModalGolden(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	all := flagModalEntries()
	localOnly := all[:5]
	cases := []struct {
		name          string
		width, height int
		entries       []flagEntry
		cursor        int
		offset        int
		awaiting      bool
		awaitingIdx   int
	}{
		{name: "top of a list with globals", width: 100, height: 30, entries: all},
		{name: "cursor on an added flag", width: 100, height: 30, entries: all, cursor: 2},
		{name: "scrolled into globals", width: 100, height: 30, entries: all, cursor: 13},
		{name: "viewport starts on a global", width: 100, height: 30, entries: all, cursor: 15, offset: 15},
		{name: "cursor above the offset", width: 100, height: 30, entries: all, cursor: 1, offset: 6},
		{name: "narrow terminal", width: 30, height: 10, entries: all, cursor: 3},
		{name: "medium width selected long row", width: 60, height: 40, entries: all, cursor: 6},
		{name: "local flags only", width: 80, height: 20, entries: localOnly, cursor: 4},
		{name: "awaiting a value", width: 80, height: 30, entries: all, cursor: 2, awaiting: true, awaitingIdx: 2},
	}

	var b strings.Builder
	for _, c := range cases {
		m := NewModel(flattenFixture(), config.DefaultConfig())
		m.width, m.height = c.width, c.height
		vi := textinput.New()
		vi.CharLimit = 128
		vi.SetValue("abc")
		m.fm = flagModal{
			active: true, entries: c.entries, cursor: c.cursor, offset: c.offset,
			awaitingValue: c.awaiting, awaitingIdx: c.awaitingIdx, valueInput: vi,
		}
		out := m.renderFlagModal()
		fmt.Fprintf(&b, "== %s (offset after=%d)\n", c.name, m.fm.offset)
		for _, line := range strings.Split(out, "\n") {
			fmt.Fprintf(&b, "%q\n", line)
		}
	}

	checkFlagModalGolden(t, b.String())
}

// checkFlagModalGolden compares got with testdata/flag_modal.golden, or
// rewrites the golden when -update-flag-modal is set.
func checkFlagModalGolden(t *testing.T, got string) {
	t.Helper()
	path := filepath.Join("testdata", "flag_modal.golden")
	if *updateFlagModalGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update-flag-modal to create it): %v", err)
	}
	if got != string(want) {
		gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
			if gotLines[i] != wantLines[i] {
				t.Fatalf("flag modal differs at golden line %d:\n got: %s\nwant: %s", i+1, gotLines[i], wantLines[i])
			}
		}
		t.Fatalf("flag modal differs in length: got %d lines, want %d", len(gotLines), len(wantLines))
	}
}

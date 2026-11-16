// Command tuiprobe renders the TUI at several terminal sizes and states and
// reports layout measurements as JSON, for the tui-layout probe
// (scripts/metrics/probes/tui-layout.sh).
//
// Per size it counts, across states: lines wider than the terminal
// (overflow), frames taller than it (excess), status-bar hints cut short
// with an ellipsis (truncated), and the share of blank rows in the first
// frame (blank_pct), which shows space the layout leaves unused.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/tui"
)

func sampleTree() *models.Node {
	root := &models.Node{Name: "tool", FullPath: []string{"tool"}, Discovered: true,
		Description: "a sample command-line tool with a long description to exercise wrapping in narrow terminals"}
	for i := 0; i < 6; i++ {
		root.Flags = append(root.Flags, models.Flag{Name: fmt.Sprintf("--global-option-%d", i), ValueType: "string", Description: "a global option with a reasonably long description"})
	}
	for s := 0; s < 40; s++ {
		n := fmt.Sprintf("subcommand-%02d", s)
		c := &models.Node{Name: n, FullPath: []string{"tool", n}, Discovered: true, Description: "does the " + n + " thing across several resources"}
		for f := 0; f < 4; f++ {
			c.Flags = append(c.Flags, models.Flag{Name: fmt.Sprintf("--%s-flag-%d", n, f), ValueType: "string", Description: "flag description"})
		}
		for k := 0; k < 3; k++ {
			kn := fmt.Sprintf("child-%d", k)
			c.Children = append(c.Children, &models.Node{Name: kn, FullPath: []string{"tool", n, kn}, Discovered: true, Description: "nested"})
		}
		root.Children = append(root.Children, c)
	}
	return root
}

func keyMsg(s string) tea.Msg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

type frameStats struct{ overflow, excess, truncated, blank, rows int }

func measure(view string, w, h int) frameStats {
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	st := frameStats{rows: len(lines)}
	for _, l := range lines {
		if lipgloss.Width(l) > w {
			st.overflow++
		}
		if strings.TrimSpace(stripBorders(l)) == "" {
			st.blank++
		}
	}
	if len(lines) > h {
		st.excess = len(lines) - h
	}
	if len(lines) > 0 && strings.Contains(lines[len(lines)-1], "…") {
		st.truncated++
	}
	return st
}

// stripBorders drops box-drawing characters so a bordered empty row counts as blank.
func stripBorders(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 0x2500 && r <= 0x257F {
			return ' '
		}
		return r
	}, s)
}

func main() {
	states := []struct {
		name string
		keys []string
	}{
		{"initial", nil},
		{"expanded_all", []string{"e"}},
		{"help_pane_off", []string{"H"}},
		{"key_overlay", []string{"?"}},
		{"filter_open", []string{"/", "s", "u", "b"}},
		{"deep_scroll", []string{"e", "pgdown", "pgdown", "pgdown", "down"}},
	}
	metrics := map[string]float64{}
	var details []map[string]any
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 50}} {
		w, h := size[0], size[1]
		tag := fmt.Sprintf("%dx%d", w, h)
		var overflow, excess, truncated int
		for i, st := range states {
			m := tui.NewModel(sampleTree(), config.DefaultConfig())
			m.Update(tea.WindowSizeMsg{Width: w, Height: h})
			for _, k := range st.keys {
				m.Update(keyMsg(k))
			}
			fs := measure(m.View(), w, h)
			overflow += fs.overflow
			excess += fs.excess
			truncated += fs.truncated
			if i == 0 {
				metrics["blank_pct."+tag] = float64(100 * fs.blank / max(fs.rows, 1))
			}
			if fs.overflow+fs.excess+fs.truncated > 0 {
				details = append(details, map[string]any{"size": tag, "state": st.name,
					"overflow_lines": fs.overflow, "excess_lines": fs.excess, "truncated_hint": fs.truncated > 0})
			}
		}
		metrics["overflow_lines."+tag] = float64(overflow)
		metrics["excess_lines."+tag] = float64(excess)
		metrics["truncated_hints."+tag] = float64(truncated)
	}
	out, _ := json.Marshal(map[string]any{"probe": "tui-layout", "metrics": metrics, "details": details})
	fmt.Println(string(out))
	_ = os.Stdout.Sync()
}

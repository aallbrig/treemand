package tui_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/tui"
)

// scaleTree builds an aws-shaped tree. With cmds == 0 the services are
// undiscovered stubs (what a plain `treemand -i aws` shows); otherwise every
// service has cmds commands with flags flags each (a full --expand-stubs
// snapshot).
func scaleTree(services, cmds, flags int) *models.Node {
	root := &models.Node{Name: "aws", FullPath: []string{"aws"}, Discovered: true}
	for i := 0; i < 18; i++ {
		root.Flags = append(root.Flags, models.Flag{Name: fmt.Sprintf("--global-%d", i), ValueType: "string", Description: "a global flag"})
	}
	for s := 0; s < services; s++ {
		sn := fmt.Sprintf("svc%03d", s)
		svc := &models.Node{Name: sn, FullPath: []string{"aws", sn}, Description: "service " + sn, Discovered: cmds > 0, Stub: cmds == 0}
		for c := 0; c < cmds; c++ {
			cn := fmt.Sprintf("cmd-%02d", c)
			cmd := &models.Node{Name: cn, FullPath: []string{"aws", sn, cn}, Description: "does " + cn, Discovered: true}
			for f := 0; f < flags; f++ {
				cmd.Flags = append(cmd.Flags, models.Flag{Name: fmt.Sprintf("--flag-%d", f), ValueType: "string", Description: "a flag"})
			}
			svc.Children = append(svc.Children, cmd)
		}
		root.Children = append(root.Children, svc)
	}
	return root
}

func key(s string) tea.Msg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "ctrl+f":
		return tea.KeyMsg{Type: tea.KeyCtrlF}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// allocsPerStep returns the average allocations of one Update plus View of msg.
// Allocation counts do not depend on how busy the machine is, so they let these
// tests check how work grows with the tree without timing anything. Absolute
// latency is judged by the tui benchmark probe (frame_ms) on a calm host.
func allocsPerStep(m *tui.Model, msg tea.Msg) float64 {
	return testing.AllocsPerRun(3, func() {
		m.Update(msg)
		m.View()
	})
}

// Every key press must cost about the same on a big tree as on a small one:
// the work follows the rows on screen, not the size of the tree. A tree four
// times larger may cost at most three times the allocations. Filter keystrokes
// scan every node, so they may grow with the tree but no faster (under six
// times). A quadratic path would cost sixteen times.
func TestKeyPressCostOnAwsSizedTreeGrowsLessThanTheTree(t *testing.T) {
	steps := []struct {
		name string
		msg  func(w, h int) tea.Msg
	}{
		{"down", fixed(key("down"))}, {"up", fixed(key("up"))}, {"pgdown", fixed(key("pgdown"))}, {"pgup", fixed(key("pgup"))},
		{"G", fixed(key("G"))}, {"right", fixed(key("right"))}, {"left", fixed(key("left"))},
		{"e expand all", fixed(key("e"))}, {"down after e", fixed(key("down"))}, {"E collapse all", fixed(key("E"))},
		{"S sections", fixed(key("S"))}, {"S again", fixed(key("S"))}, {"T style", fixed(key("T"))},
		{"H help pane", fixed(key("H"))}, {"H again", fixed(key("H"))}, {"? keys", fixed(key("?"))}, {"esc", fixed(key("esc"))},
		{"/ filter", fixed(key("/"))}, {"type s", fixed(key("s"))}, {"type v", fixed(key("v"))}, {"enter", fixed(key("enter"))},
		{"esc clears", fixed(key("esc"))},
		{"resize", func(w, h int) tea.Msg { return tea.WindowSizeMsg{Width: w + 9, Height: h + 4} }},
		{"resize back", func(w, h int) tea.Msg { return tea.WindowSizeMsg{Width: w, Height: h} }},
		{"wheel", fixed(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, X: 5, Y: 5})},
		{"click", fixed(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 5, Y: 6})},
	}
	scansTree := map[string]bool{"/ filter": true, "type s": true, "type v": true}
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 50}} {
		small := newSized(scaleTree(105, 0, 0), size[0], size[1])
		big := newSized(scaleTree(422, 0, 0), size[0], size[1])
		for _, st := range steps {
			msg := st.msg(size[0], size[1])
			limit := 3.0
			if scansTree[st.name] {
				limit = 6
			}
			a, b := allocsPerStep(small, msg), allocsPerStep(big, msg)
			if b > limit*a+100 {
				t.Errorf("%dx%d: %q made %.0f allocations on a 422-service tree and %.0f on a 105-service one, want under %.0fx",
					size[0], size[1], st.name, b, a, limit)
			}
		}
	}
}

func fixed(msg tea.Msg) func(int, int) tea.Msg { return func(int, int) tea.Msg { return msg } }

func newSized(root *models.Node, w, h int) *tui.Model {
	m := tui.NewModel(root, config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.View()
	return m
}

// Expanding everything on a fully expanded aws snapshot (about 9,000 commands
// and 42,000 flags) used to take about 300ms; row building now reuses its
// buffers. Expand-all must grow at most linearly with the tree (four times the
// tree, under six times the allocations; quadratic would be sixteen), and a key
// press after it must not depend on how many rows were expanded.
func TestExpandAllOnAFullyExpandedAwsTree(t *testing.T) {
	small := newSized(scaleTree(105, 20, 5), 120, 40)
	big := newSized(scaleTree(422, 20, 5), 120, 40)
	expandAll := func(m *tui.Model) float64 {
		return testing.AllocsPerRun(3, func() {
			m.Update(key("E")) // collapse so the next e does real work
			m.Update(key("e"))
			m.View()
		})
	}
	if a, b := expandAll(small), expandAll(big); b > 6*a {
		t.Errorf("expand-all made %.0f allocations on ~60,000 rows and %.0f on ~15,000, want under 6x", b, a)
	}
	if a, b := allocsPerStep(small, key("down")), allocsPerStep(big, key("down")); b > 1.5*a+100 {
		t.Errorf("a key press after expand-all made %.0f allocations on ~60,000 rows and %.0f on ~15,000, want about the same", b, a)
	}
}

// No state of the UI may draw a line wider than the terminal or more lines
// than it has, at the three sizes users actually run.
func TestLayoutFitsTheTerminal(t *testing.T) {
	states := []struct {
		name string
		keys []string
	}{
		{"initial", nil},
		{"expanded all", []string{"e"}},
		{"deep in the list", []string{"G", "up", "up"}},
		{"sections hidden", []string{"S"}},
		{"help pane", []string{"H"}},
		{"key bindings overlay", []string{"?"}},
		{"filter open", []string{"/", "s", "v"}},
		{"search modal", []string{"ctrl+f"}},
		{"flag picker", []string{"down", "f"}},
		{"compact style", []string{"T", "T"}},
		{"graph style", []string{"T", "T", "T"}},
	}
	shapes := map[string]func() *models.Node{
		"aws-shaped": func() *models.Node { return scaleTree(60, 6, 4) },
		"long-text":  longTextTree, // the tui-layout probe's shape: long descriptions and flag names
	}
	for shape, build := range shapes {
		for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 50}} {
			w, h := size[0], size[1]
			for _, st := range states {
				tree := build()
				t.Run(fmt.Sprintf("%s/%dx%d/%s", shape, w, h, st.name), func(t *testing.T) {
					m := tui.NewModel(tree, config.DefaultConfig())
					m.Update(tea.WindowSizeMsg{Width: w, Height: h})
					for _, k := range st.keys {
						m.Update(key(k))
					}
					lines := strings.Split(strings.TrimRight(m.View(), "\n"), "\n")
					if len(lines) > h {
						t.Errorf("view has %d lines, terminal has %d", len(lines), h)
					}
					for i, l := range lines {
						if lw := lipgloss.Width(l); lw > w {
							t.Errorf("line %d is %d columns wide, terminal has %d: %q", i+1, lw, w, l)
						}
					}
				})
			}
		}
	}
}

// The key bindings overlay scrolls; whatever part of it is on screen must fit
// the terminal (long lines used to wrap inside the box and push it past the
// bottom of the screen).
func TestKeybindOverlayFitsAtEveryScrollOffset(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 50}} {
		w, h := size[0], size[1]
		m := tui.NewModel(scaleTree(60, 6, 4), config.DefaultConfig())
		m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		m.Update(key("?"))
		for offset := 0; offset < 60; offset++ {
			lines := strings.Split(strings.TrimRight(m.View(), "\n"), "\n")
			if len(lines) > h {
				t.Fatalf("%dx%d offset %d: overlay has %d lines, terminal has %d", w, h, offset, len(lines), h)
			}
			for i, l := range lines {
				if lw := lipgloss.Width(l); lw > w {
					t.Fatalf("%dx%d offset %d: line %d is %d columns wide", w, h, offset, i+1, lw)
				}
			}
			m.Update(key("down"))
		}
	}
}

// selectedChild reads which child the status bar (the last line) names, or -1
// when the cursor is on the root or a section row.
var childInStatus = regexp.MustCompile(`aws (c\d{3})`)

func selectedChild(t *testing.T, m *tui.Model) int {
	t.Helper()
	lines := strings.Split(strings.TrimRight(m.View(), "\n"), "\n")
	mm := childInStatus.FindStringSubmatch(lines[len(lines)-1])
	if mm == nil {
		return -1 // the cursor is on the root or a section row
	}
	n, _ := strconv.Atoi(mm[1][1:])
	return n
}

// On a 200-command CLI the tree must be navigable by page, not only one row
// at a time: PgDn/PgUp (and Ctrl+D/Ctrl+U) move by about a screenful.
func TestPageKeysMoveByAScreenful(t *testing.T) {
	for _, pair := range [][2]string{{"pgdown", "pgup"}, {"ctrl+d", "ctrl+u"}} {
		t.Run(pair[0], func(t *testing.T) {
			root := &models.Node{Name: "aws", FullPath: []string{"aws"}, Discovered: true}
			for i := 0; i < 200; i++ {
				n := fmt.Sprintf("c%03d", i)
				root.Children = append(root.Children, &models.Node{Name: n, FullPath: []string{"aws", n}, Discovered: true})
			}
			m := tui.NewModel(root, config.DefaultConfig())
			m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			m.Update(key("e"))
			m.Update(key("down"))
			m.Update(key("down")) // onto c000
			start := selectedChild(t, m)

			m.Update(key(pair[0]))
			afterOne := selectedChild(t, m)
			if step := afterOne - start; step < 8 || step > 30 {
				t.Errorf("one %s moved %d rows (c%03d -> c%03d), want about a screenful", pair[0], step, start, afterOne)
			}
			m.Update(key(pair[0]))
			if second := selectedChild(t, m); second <= afterOne {
				t.Errorf("a second %s did not advance (c%03d -> c%03d)", pair[0], afterOne, second)
			}
			m.Update(key(pair[1]))
			if back := selectedChild(t, m); back > afterOne {
				t.Errorf("%s did not go back (now c%03d, was c%03d)", pair[1], back, afterOne)
			}
			for i := 0; i < 40; i++ {
				m.Update(key(pair[0]))
			}
			if last := selectedChild(t, m); last != 199 {
				t.Errorf("paging down past the end ended on c%03d, want c199", last)
			}
			for i := 0; i < 40; i++ {
				m.Update(key(pair[1]))
			}
			if first := selectedChild(t, m); first > 1 {
				t.Errorf("paging up past the start ended on c%03d, want the first rows", first)
			}
		})
	}
}

// longTextTree has long descriptions and long flag names, the shape that made
// frames taller than the terminal (#212). Mirrors tools/tuiprobe's tree.
func longTextTree() *models.Node {
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

package tui_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

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

// Every key press on an aws-sized tree must feel instant: Update plus View
// under 50ms. The best of three runs is taken so a busy machine does not fail it.
func TestKeyPressLatencyOnAwsSizedTree(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 50}} {
		m := tui.NewModel(scaleTree(422, 0, 0), config.DefaultConfig())
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.View()
		steps := []struct {
			name string
			msg  tea.Msg
		}{
			{"down", key("down")}, {"up", key("up")}, {"pgdown", key("pgdown")}, {"pgup", key("pgup")},
			{"G", key("G")}, {"right", key("right")}, {"left", key("left")},
			{"e expand all", key("e")}, {"down after e", key("down")}, {"E collapse all", key("E")},
			{"S sections", key("S")}, {"S again", key("S")}, {"T style", key("T")},
			{"H help pane", key("H")}, {"H again", key("H")}, {"? keys", key("?")}, {"esc", key("esc")},
			{"/ filter", key("/")}, {"type s", key("s")}, {"type v", key("v")}, {"enter", key("enter")},
			{"esc clears", key("esc")},
			{"resize", tea.WindowSizeMsg{Width: size[0] + 9, Height: size[1] + 4}},
			{"resize back", tea.WindowSizeMsg{Width: size[0], Height: size[1]}},
			{"wheel", tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, X: 5, Y: 5}},
			{"click", tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 5, Y: 6}},
		}
		for _, st := range steps {
			best := time.Hour
			for i := 0; i < 3; i++ {
				start := time.Now()
				m.Update(st.msg)
				m.View()
				if d := time.Since(start); d < best {
					best = d
				}
			}
			if best > 50*time.Millisecond {
				t.Errorf("%dx%d: %q took %v on a 422-node tree, want under 50ms", size[0], size[1], st.name, best)
			}
		}
	}
}

// Expanding everything on a fully expanded aws snapshot (about 9,000 commands
// and 42,000 flags) used to take about 300ms; row building now reuses its
// buffers. The bound is generous so slow CI machines do not flake; it exists to
// catch a return to quadratic or allocation-heavy rebuilds.
func TestExpandAllOnAFullyExpandedAwsTree(t *testing.T) {
	m := tui.NewModel(scaleTree(422, 20, 5), config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.View()
	best := time.Hour
	for i := 0; i < 3; i++ {
		m.Update(key("E")) // collapse so the next e does real work
		start := time.Now()
		m.Update(key("e"))
		m.View()
		if d := time.Since(start); d < best {
			best = d
		}
	}
	if best > 250*time.Millisecond {
		t.Errorf("expand-all on ~60,000 rows took %v, want under 250ms", best)
	}
	m.Update(key("down"))
	start := time.Now()
	m.Update(key("down"))
	m.View()
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Errorf("a key press after expand-all took %v, want under 50ms", d)
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
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 50}} {
		w, h := size[0], size[1]
		for _, st := range states {
			t.Run(fmt.Sprintf("%dx%d/%s", w, h, st.name), func(t *testing.T) {
				m := tui.NewModel(scaleTree(60, 6, 4), config.DefaultConfig())
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

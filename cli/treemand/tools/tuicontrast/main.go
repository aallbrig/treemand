// Command tuicontrast renders the TUI in the states a user sees and reports
// the WCAG 2.x contrast of every foreground and background pair it draws, as
// JSON, for the tui-contrast probe (scripts/metrics/probes/tui-contrast.sh).
//
// It reads the colours from the rendered frames themselves (truecolour SGR),
// so hard-coded styles count as much as the configurable scheme. Wherever the
// TUI leaves a colour to the terminal it is judged on a dark terminal
// (#D4D4D4 on #1E1E1E) and a light one (#000000 on #FFFFFF); faint text is
// drawn at half intensity toward its background, as terminals render SGR 2.
//
// Text needs 4.5:1 (WCAG 1.4.3). Bold text and box-drawing UI (borders) need
// 3:1: a terminal has one font size, so bold is the only "large" text it can
// draw, and borders are non-text UI (WCAG 1.4.11).
//
// It also checks NO_COLOR: with NO_COLOR set, or with no_color in the config
// (the --no-color flag), no state may emit a colour.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/aallbrig/treemand/cli/treemand/config"
	"github.com/aallbrig/treemand/cli/treemand/models"
	"github.com/aallbrig/treemand/cli/treemand/tui"
)

var terminals = []terminal{
	{name: "dark", fg: mustHex("#D4D4D4"), bg: mustHex("#1E1E1E")},
	{name: "light", fg: mustHex("#000000"), bg: mustHex("#FFFFFF")},
}

// themes are the colour schemes treemand ships. Only the default exists today;
// a new built-in theme belongs here so the probe judges it too.
var themes = map[string]config.ColorScheme{"default": config.DefaultColors()}

type state struct {
	name string
	keys []tea.Msg
}

func runes(s string) []tea.Msg {
	var out []tea.Msg
	for _, r := range s {
		out = append(out, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return out
}

func keys(parts ...any) []tea.Msg {
	var out []tea.Msg
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			out = append(out, runes(v)...)
		case tea.KeyType:
			out = append(out, tea.KeyMsg{Type: v})
		}
	}
	return out
}

// states cover every surface: tree rows (selected, normal, descriptions, flag
// pills, error marker), each display style, the status bar and its messages,
// the key overlay, filter, search, flag picker and exec modal, and pane focus.
var states = []state{
	{"initial", nil},
	{"expanded_all", keys("e")},
	{"selected_child", keys("e", tea.KeyDown, tea.KeyDown)},
	{"style_columns", keys("t")},
	{"style_compact", keys("t", "t")},
	{"style_graph", keys("t", "t", "t")},
	{"status_message", keys("S")},
	{"key_overlay", keys("?")},
	{"filter_open", keys("/", "sub")},
	{"search_modal", keys(tea.KeyCtrlF, "sub")},
	{"flag_modal", keys("f")},
	{"exec_modal", keys(tea.KeyCtrlE)},
	{"help_focused", keys(tea.KeyTab, tea.KeyTab)},
	{"preview_focused", keys(tea.KeyTab)},
}

func sampleTree() *models.Node {
	root := &models.Node{Name: "tool", FullPath: []string{"tool"}, Discovered: true,
		Description: "a sample command-line tool",
		Positionals: []models.Positional{{Name: "target", Description: "what to act on", Required: true}}}
	for i, vt := range []string{"bool", "string", "int", "duration"} {
		root.Flags = append(root.Flags, models.Flag{Name: fmt.Sprintf("--opt-%d", i), ValueType: vt, Description: "a global " + vt + " option"})
	}
	for s := 0; s < 12; s++ {
		n := fmt.Sprintf("subcommand-%02d", s)
		c := &models.Node{Name: n, FullPath: []string{"tool", n}, Discovered: true, Description: "does the " + n + " thing"}
		if s == 2 {
			c.DiscoveryErr = "help timed out"
		}
		for f, vt := range []string{"bool", "string", "int", "float"} {
			c.Flags = append(c.Flags, models.Flag{Name: fmt.Sprintf("--flag-%d", f), ValueType: vt, Description: "flag description"})
		}
		c.Positionals = []models.Positional{{Name: "name", Description: "the resource name"}}
		for k := 0; k < 2; k++ {
			kn := fmt.Sprintf("child-%d", k)
			c.Children = append(c.Children, &models.Node{Name: kn, FullPath: []string{"tool", n, kn}, Discovered: true, Description: "nested"})
		}
		root.Children = append(root.Children, c)
	}
	return root
}

func render(cfg *config.Config, st state) string {
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	for _, k := range st.keys {
		m.Update(k)
	}
	return m.View()
}

// classify returns the minimum ratio a run needs, and its class, or ok=false
// for runs that draw nothing (whitespace).
func classify(r run) (min float64, class string, ok bool) {
	visible, ui := false, true
	for _, c := range r.text {
		if unicode.IsSpace(c) {
			continue
		}
		visible = true
		if c < 0x2500 || c > 0x257F {
			ui = false
		}
	}
	switch {
	case !visible:
		return 0, "", false
	case ui:
		return 3, "ui", true
	case r.bold:
		return 3, "bold", true
	}
	return 4.5, "text", true
}

type example struct {
	State string `json:"state"`
	Where string `json:"where"`
	Text  string `json:"text"`
}

type pair struct {
	Kind     string    `json:"kind"`
	Theme    string    `json:"theme"`
	Terminal string    `json:"terminal"`
	Class    string    `json:"class"`
	Fg       string    `json:"fg"`
	Bg       string    `json:"bg"`
	Source   string    `json:"source"`
	Ratio    float64   `json:"ratio"`
	Min      float64   `json:"min"`
	Suggest  string    `json:"suggest"`
	Examples []example `json:"examples"`
}

// source names where the pair's colours come from, so a fix knows what to edit.
func source(r run) string {
	var s []string
	if r.fg == nil {
		s = append(s, "terminal fg")
	} else {
		s = append(s, "fg "+r.fg.hex())
	}
	if r.bg == nil {
		s = append(s, "terminal bg")
	} else {
		s = append(s, "bg "+r.bg.hex())
	}
	if r.faint {
		s = append(s, "faint")
	}
	if r.bold {
		s = append(s, "bold")
	}
	if r.reverse {
		s = append(s, "reverse")
	}
	return strings.Join(s, ", ")
}

func sample(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}

// envProfile is the profile lipgloss picks on a truecolour TTY with environ.
func envProfile(environ map[string]string) termenv.Profile {
	return termenv.NewOutput(os.Stdout, termenv.WithTTY(true), termenv.WithEnvironment(env(environ))).EnvColorProfile()
}

type env map[string]string

func (e env) Getenv(k string) string { return e[k] }
func (e env) Environ() []string {
	var out []string
	for k, v := range e {
		out = append(out, k+"="+v)
	}
	return out
}

// pairSet collects distinct pairs, keyed by theme, terminal, class and colours.
type pairSet map[string]*pair

func (ps pairSet) add(theme, state string, r run, last int) {
	min, class, ok := classify(r)
	if !ok {
		return
	}
	where := "body"
	if r.line == last {
		where = "status_bar"
	}
	for _, term := range terminals {
		fg, bg := effective(r, term)
		key := strings.Join([]string{theme, term.name, class, fg.hex(), bg.hex()}, "|")
		p := ps[key]
		if p == nil {
			p = &pair{Kind: "contrast", Theme: theme, Terminal: term.name, Class: class, Fg: fg.hex(), Bg: bg.hex(),
				Source: source(r), Ratio: round2(ratio(fg, bg)), Min: min}
			if p.Ratio < min {
				p.Suggest = suggest(fg, bg, min).hex()
			}
			ps[key] = p
		}
		if len(p.Examples) < 3 {
			p.Examples = append(p.Examples, example{state, where, sample(r.text)})
		}
	}
}

// collect renders every state of every theme in truecolour and gathers its pairs.
func collect() pairSet {
	ps := pairSet{}
	lipgloss.SetColorProfile(termenv.TrueColor)
	for theme, colors := range themes {
		for _, st := range states {
			cfg := config.DefaultConfig()
			cfg.NoColor = false
			cfg.Colors = colors
			frame := render(cfg, st)
			last := strings.Count(strings.TrimRight(frame, "\n"), "\n")
			for _, r := range scan(frame) {
				ps.add(theme, st.name, r, last)
			}
		}
	}
	return ps
}

// judge counts the pairs per terminal and returns the failing ones, worst first.
func judge(ps pairSet, metrics map[string]float64) []*pair {
	var failing []*pair
	for _, term := range terminals {
		metrics["pairs_below_4_5."+term.name] = 0
		metrics["pairs_below_3."+term.name] = 0
		metrics["pairs_total."+term.name] = 0
	}
	for _, p := range ps {
		metrics["pairs_total."+p.Terminal]++
		if p.Ratio >= p.Min {
			continue
		}
		failing = append(failing, p)
		family := "pairs_below_4_5."
		if p.Min == 3 {
			family = "pairs_below_3."
		}
		metrics[family+p.Terminal]++
	}
	sort.Slice(failing, func(i, j int) bool {
		a, b := failing[i], failing[j]
		if a.Ratio != b.Ratio {
			return a.Ratio < b.Ratio
		}
		return a.Terminal+a.Class+a.Fg+a.Bg < b.Terminal+b.Class+b.Fg+b.Bg
	})
	return failing
}

// colouredStates lists the states that emit colour under profile with cfg.NoColor set to noColor.
func colouredStates(profile termenv.Profile, noColor bool) []string {
	lipgloss.SetColorProfile(profile)
	var coloured []string
	for _, st := range states {
		cfg := config.DefaultConfig()
		cfg.NoColor = noColor
		if hasColour(render(cfg, st)) {
			coloured = append(coloured, st.name)
		}
	}
	return coloured
}

// checkNoColor covers the NO_COLOR environment variable (lipgloss drops to
// the Ascii profile) and the config/flag (cfg.NoColor) on a colour terminal.
func checkNoColor(metrics map[string]float64) []any {
	checks := []struct {
		metric  string
		profile termenv.Profile
		cfg     bool
	}{
		{"no_color_env_coloured_states", envProfile(map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor", "NO_COLOR": "1"}), false},
		{"no_color_flag_coloured_states", termenv.TrueColor, true},
	}
	var breaches []any
	for _, c := range checks {
		coloured := colouredStates(c.profile, c.cfg)
		metrics[c.metric] = float64(len(coloured))
		if len(coloured) > 0 {
			breaches = append(breaches, map[string]any{"kind": "no_color", "check": c.metric, "coloured_states": coloured})
		}
	}
	return breaches
}

func main() {
	metrics := map[string]float64{}
	failing := judge(collect(), metrics)
	noColor := checkNoColor(metrics)
	// details: the setup first, then every failing pair worst first, then
	// any NO_COLOR breach.
	details := []any{map[string]any{"kind": "setup", "states": len(states), "terminals": terminals,
		"themes": len(themes), "size": "120x40"}}
	for _, p := range failing {
		details = append(details, p)
	}
	details = append(details, noColor...)
	out, _ := json.Marshal(map[string]any{"probe": "tui-contrast", "metrics": metrics, "details": details})
	fmt.Println(string(out))
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

// MarshalJSON keeps the terminals readable in details.
func (t terminal) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"name": t.name, "fg": t.fg.hex(), "bg": t.bg.hex()})
}

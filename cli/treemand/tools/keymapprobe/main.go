// Command keymapprobe compares the keys the TUI binds with the keys it
// documents, for the tui-keymap probe (scripts/metrics/probes/tui-keymap.sh).
//
// It is static and deterministic: it parses the tui package's Go source (the
// `case "k":` labels of every key switch) and the documentation that names
// keys (the ? overlay text, the footer and dialog hints in the render code,
// and the website's key binding tables), then reports:
//
//   - conflicts: a key listed by a later switch in a mode's dispatch chain
//     that an earlier switch already consumes unconditionally (dead code, or
//     a key that does something other than what that pane intends)
//   - unreachable_bindings: a bound string bubbletea never reports, such as
//     "shift+l" (the terminal sends "L")
//   - undocumented_bindings.<surface>: bound keys missing from the overlay,
//     the website reference page, or a dialog's own hint
//   - documented_unbound.<surface>: keys the overlay, a hint or a website
//     table names that nothing binds in that mode
//
// The tree pane's keys, its ? overlay and its footers come from the TUI's key
// map (tui.Keymap, tui.KeyOverlay, tui.Footer), which the probe reads
// directly. The dialogs still bind keys in switches, so the mode table
// (DefaultConfig) says which switches make up each dialog mode. Any key
// switch the table does not name, and any table entry the code no longer
// has, fails the probe: drift is loud.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/aallbrig/treemand/tui"
)

// SwitchRef names the Index-th switch with string-literal cases in Func.
type SwitchRef struct {
	Func  string
	Index int
}

// HintRef selects hint text: Text when set (a footer the key map renders);
// otherwise the string literals of Func, only those inside the case clause
// whose condition contains Clause ("default" for the default clause) when
// Clause is set, and only those containing Contains when set.
type HintRef struct {
	Func, Clause, Contains string
	Text                   string
}

// Mode is one input context of the TUI.
type Mode struct {
	Name string
	// Scheme ties a tree mode to a website table column ("Keys (vim)").
	Scheme string
	// Chain is the dispatch order: an earlier case that always returns
	// consumes the key, so a later switch listing it is shadowed.
	Chain []SwitchRef
	// Layers are the key map modes of a mode that binds from the key map,
	// top first: an upper layer overrides a lower one on purpose.
	Layers []string
	// Hints are what the TUI shows while in this mode.
	Hints []HintRef
	// Overlay and Reference: the ? overlay and the website reference page
	// must document every key of this mode. Other modes are judged against
	// their own hints.
	Overlay, Reference bool
}

// Config is the mode table plus where documentation lives.
type Config struct {
	Modes        []Mode
	OverlayConst string // the const holding the ? overlay text, when OverlayText is ""
	OverlayText  string // the ? overlay text
	Table        []tui.Binding
	Reference    string   // the website page that must list every key
	Pages        []string // website pages whose key tables are checked
}

// DefaultConfig is treemand's TUI as of this probe.
func DefaultConfig() Config {
	tree := func(name, scheme string) Mode {
		return Mode{Name: name, Scheme: scheme, Overlay: true, Reference: true,
			Layers: []string{name, tui.ModeTree}, Hints: []HintRef{{Text: tui.Footer(name)}}}
	}
	table := func(name string, layers ...string) Mode {
		return Mode{Name: name, Layers: layers, Hints: []HintRef{{Text: tui.Footer(name)}}}
	}
	dialog := func(name, update string, idx int, render, contains string) Mode {
		return Mode{Name: name, Chain: []SwitchRef{{update, idx}},
			Hints: []HintRef{{Func: render, Contains: contains}}}
	}
	return Config{
		OverlayText: tui.KeyOverlay(),
		Table:       tui.Keymap(),
		Reference:   "reference.md",
		Pages:       []string{"reference.md", "commands/interactive.md", "getting-started.md"},
		Modes: []Mode{
			tree(tui.ModeArrows, "arrows"),
			tree(tui.ModeVim, "vim"),
			tree(tui.ModeWASD, "wasd"),
			func() Mode {
				md := table(tui.ModeHelpPane, tui.ModeHelpPane, tui.ModeTree)
				md.Overlay, md.Reference = true, true
				return md
			}(),
			table(tui.ModePreview, tui.ModePreview),
			table(tui.ModeFilter, tui.ModeFilter),
			dialog("keys-overlay", "updateKeybindModal", 0, "renderKeybindModal", "Esc"),
			dialog("value-input", "updateValueModal", 0, "renderValueInputModal", "Esc"),
			dialog("exec", "updateModal", 0, "renderModal", "Esc"),
			dialog("recents", "updateRecentsModal", 0, "renderRecentsModal", "Esc"),
			dialog("presets", "updatePresetsModal", 0, "renderPresetsModal", "Esc"),
			dialog("preset-name", "updatePresetName", 0, "renderPresetsModal", "Enter save"),
			dialog("flag-value", "updateFlagModal", 0, "renderFlagModal", "Type value"),
			dialog("flag-picker", "updateFlagModal", 1, "renderFlagModal", "navigate"),
			dialog("search", "updateSearchModal", 0, "renderSearchModal", "Esc"),
		},
	}
}

// Detail is one finding, with where it is.
type Detail struct {
	Metric string `json:"metric"`
	Key    string `json:"key"`
	Mode   string `json:"mode,omitempty"`
	Where  string `json:"where"`
	Note   string `json:"note,omitempty"`
}

// Report is the probe's output.
type Report struct {
	Probe   string             `json:"probe"`
	Metrics map[string]float64 `json:"metrics"`
	Details []Detail           `json:"details"`
	Bound   int                `json:"-"`
}

func main() {
	tuiDir, siteDir := "tui", "../../www/treemand/content"
	if len(os.Args) == 3 {
		tuiDir, siteDir = os.Args[1], os.Args[2]
	}
	rep, err := Scan(tuiDir, siteDir, DefaultConfig())
	if err != nil {
		fmt.Fprintln(os.Stderr, "keymapprobe:", err)
		os.Exit(1)
	}
	sort.SliceStable(rep.Details, func(i, j int) bool {
		a, b := rep.Details[i], rep.Details[j]
		if a.Metric != b.Metric {
			return a.Metric < b.Metric
		}
		if a.Mode != b.Mode {
			return a.Mode < b.Mode
		}
		return a.Key < b.Key
	})
	out, _ := json.Marshal(rep)
	fmt.Println(string(out))
}

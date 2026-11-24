package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// action is what a key does. The key map binds keys to actions per mode; the
// handlers in model_keys.go run them.
type action int

const (
	actNone action = iota
	actQuit
	actBack
	actFocusNext
	actFocusPrev
	actCycleScheme
	actCycleStyle
	actToggleSections
	actToggleHelpPane
	actFilter
	actSearch
	actKeys
	actExec
	actExecPreview
	actRecents
	actPresets
	actClear
	actRemoveToken
	actRediscover
	actFlags
	actDocs
	actExpandAll
	actCollapseAll
	actExpandSubtree
	actCollapseSubtree
	actPendingTop
	actBottom
	actPageDown
	actPageUp
	actNextMatch
	actPrevMatch
	actUp
	actDown
	actLeft
	actRight
	actToggle
	actPick
	actScrollUp
	actScrollDown
	actScrollPageUp
	actScrollPageDown
	actScrollTop
	actScrollBottom
	actFilterKeep
	actFilterClear
)

// The key map's modes. A tree scheme mode sits under ModeTree: the tree's
// keys are ModeTree's, then the scheme's. ModeHelpPane sits over ModeTree.
const (
	ModeTree     = "tree"
	ModeArrows   = "tree:arrows"
	ModeVim      = "tree:vim"
	ModeWASD     = "tree:wasd"
	ModeHelpPane = "help-pane"
	ModePreview  = "preview"
	ModeFilter   = "filter"
	// ModeMouse documents the mouse in the ? overlay; nothing dispatches it.
	ModeMouse = "mouse"
)

// Binding is one row of the key map: keys of a mode, what they do, and how
// the ? overlay and the footer name them.
type Binding struct {
	Mode string
	// Keys as bubbletea reports them: "L", "pgdown", "ctrl+e", " ".
	Keys   []string
	action action
	// Section, Label and Help make the binding's ? overlay row; a binding
	// with no Section is documented by another row's Label.
	Section, Label, Help string
	// Hint is the binding's footer token, "" when no footer names it.
	Hint string
}

// overlaySections is the ? overlay's section order.
var overlaySections = []string{"Navigation", "Tree", "Building Commands", "View", "Mouse", "Quit"}

// keymap is every key binding of the tree pane, the help pane, the preview
// line and the filter line. Within a section, rows show in table order.
var keymap = []Binding{
	// Navigation: the arrows rows document every scheme's keys.
	{Mode: ModeArrows, Keys: []string{"up"}, action: actUp, Hint: "↑↓:nav",
		Section: "Navigation", Label: "↑ / ↓  or  k / j  or  w / s", Help: "Move up / down"},
	{Mode: ModeArrows, Keys: []string{"down"}, action: actDown},
	{Mode: ModeArrows, Keys: []string{"right"}, action: actRight,
		Section: "Navigation", Label: "→ / l / d", Help: "Expand node; enter children (2nd press)"},
	{Mode: ModeArrows, Keys: []string{"left"}, action: actLeft, Hint: "←→:expand/collapse",
		Section: "Navigation", Label: "← / h / a", Help: "Collapse node; go to parent (2nd press)"},
	{Mode: ModeTree, Keys: []string{"shift+right"}, action: actExpandSubtree, Hint: "Shift+←→:subtree",
		Section: "Navigation", Label: "Shift+→", Help: "Expand entire subtree"},
	{Mode: ModeTree, Keys: []string{"shift+left"}, action: actCollapseSubtree,
		Section: "Navigation", Label: "Shift+←", Help: "Collapse entire subtree"},
	{Mode: ModeTree, Keys: []string{"pgdown", "ctrl+d"}, action: actPageDown,
		Section: "Navigation", Label: "PgUp / PgDn  or  Ctrl+U / Ctrl+D", Help: "Move up / down a screenful"},
	{Mode: ModeTree, Keys: []string{"pgup", "ctrl+u"}, action: actPageUp},
	{Mode: ModeTree, Keys: []string{"g"}, action: actPendingTop,
		Section: "Navigation", Label: "gg", Help: "Jump to top"},
	{Mode: ModeTree, Keys: []string{"G"}, action: actBottom, Hint: "gg/G:top/bottom",
		Section: "Navigation", Label: "G", Help: "Jump to bottom"},
	{Mode: ModeArrows, Keys: []string{" "}, action: actToggle,
		Section: "Navigation", Label: "Space", Help: "Toggle expand / collapse"},
	{Mode: ModeArrows, Keys: []string{"enter"}, action: actPick, Hint: "Enter:pick"},

	{Mode: ModeVim, Keys: []string{"k"}, action: actUp, Hint: "j/k:nav"},
	{Mode: ModeVim, Keys: []string{"j"}, action: actDown},
	{Mode: ModeVim, Keys: []string{"h"}, action: actLeft, Hint: "h/l:expand/collapse"},
	{Mode: ModeVim, Keys: []string{"l"}, action: actRight},
	{Mode: ModeVim, Keys: []string{" "}, action: actToggle},
	{Mode: ModeVim, Keys: []string{"enter"}, action: actPick, Hint: "Enter:pick"},
	{Mode: ModeVim, Keys: []string{"d"}, action: actDocs},

	{Mode: ModeWASD, Keys: []string{"w"}, action: actUp, Hint: "w/s:nav"},
	{Mode: ModeWASD, Keys: []string{"s"}, action: actDown},
	{Mode: ModeWASD, Keys: []string{"a"}, action: actLeft, Hint: "a/d:expand/collapse"},
	{Mode: ModeWASD, Keys: []string{"d"}, action: actRight},
	{Mode: ModeWASD, Keys: []string{" "}, action: actToggle},
	{Mode: ModeWASD, Keys: []string{"enter"}, action: actPick, Hint: "Enter:pick"},

	// Tree
	{Mode: ModeTree, Keys: []string{"/"}, action: actFilter, Hint: "/:filter",
		Section: "Tree", Label: "/", Help: "Fuzzy filter tree (Esc clears, Enter keeps)"},
	{Mode: ModeTree, Keys: []string{"ctrl+f"}, action: actSearch,
		Section: "Tree", Label: "Ctrl+F",
		Help: "Global fuzzy find: commands, flags, args, help text (Tab inside: index all undiscovered nodes in background)"},
	{Mode: ModeTree, Keys: []string{"n"}, action: actNextMatch, Hint: "n/N:search",
		Section: "Tree", Label: "n / N", Help: "Next / previous search match"},
	{Mode: ModeTree, Keys: []string{"N"}, action: actPrevMatch},
	{Mode: ModeTree, Keys: []string{"e"}, action: actExpandAll, Hint: "e/E:expand/collapse all",
		Section: "Tree", Label: "e / E", Help: "Expand all / collapse all"},
	{Mode: ModeTree, Keys: []string{"E"}, action: actCollapseAll},
	{Mode: ModeTree, Keys: []string{"S"}, action: actToggleSections, Hint: "S:sections",
		Section: "Tree", Label: "S", Help: "Toggle section headers"},
	{Mode: ModeTree, Keys: []string{"t", "T"}, action: actCycleStyle,
		Section: "Tree", Label: "t / T", Help: "Cycle display style (default → columns → compact → graph)"},
	{Mode: ModeTree, Keys: []string{"r", "R"}, action: actRediscover,
		Section: "Tree", Label: "r / R", Help: "Re-discover selected node (refresh children)"},

	// Building Commands
	{Mode: ModeArrows, Keys: nil, action: actPick,
		Section: "Building Commands", Label: "Enter", Help: "Set command / add flag / fill positional"},
	{Mode: ModeTree, Keys: []string{"f", "F"}, action: actFlags, Hint: "f:flags",
		Section: "Building Commands", Label: "f / F", Help: "Open flag picker modal"},
	{Mode: ModeTree, Keys: []string{"backspace", "delete"}, action: actRemoveToken,
		Section: "Building Commands", Label: "Backspace / Delete", Help: "Remove last token from preview"},
	{Mode: ModeTree, Keys: []string{"ctrl+k"}, action: actClear,
		Section: "Building Commands", Label: "Ctrl+K", Help: "Clear entire preview bar"},
	{Mode: ModeTree, Keys: []string{"ctrl+e"}, action: actExec, Hint: "Ctrl+E:exec",
		Section: "Building Commands", Label: "Ctrl+E", Help: "Copy or execute the assembled command"},
	{Mode: ModeTree, Keys: []string{"ctrl+r"}, action: actRecents,
		Section: "Building Commands", Label: "Ctrl+R", Help: "Recent commands (Enter loads one into the preview)"},
	{Mode: ModeTree, Keys: []string{"ctrl+o"}, action: actPresets,
		Section: "Building Commands", Label: "Ctrl+O", Help: "Saved presets: load, save (a) or delete (x) named commands"},

	// View
	{Mode: ModeTree, Keys: []string{"H", "ctrl+p"}, action: actToggleHelpPane, Hint: "H:help",
		Section: "View", Label: "H / Ctrl+P", Help: "Toggle help pane"},
	{Mode: ModeTree, Keys: []string{"tab"}, action: actFocusNext, Hint: "Tab:switch",
		Section: "View", Label: "Tab / Shift+Tab", Help: "Cycle pane focus"},
	{Mode: ModeTree, Keys: []string{"shift+tab"}, action: actFocusPrev},
	{Mode: ModeTree, Keys: []string{"D"}, action: actDocs,
		Section: "View", Label: "d / D", Help: "Open docs URL in browser"},
	{Mode: ModeArrows, Keys: []string{"d"}, action: actDocs},
	{Mode: ModeTree, Keys: []string{"ctrl+s"}, action: actCycleScheme,
		Section: "View", Label: "Ctrl+S", Help: "Cycle navigation scheme (arrows → vim → WASD)"},
	{Mode: ModeTree, Keys: []string{"?"}, action: actKeys, Hint: "?:keys",
		Section: "View", Label: "?", Help: "Show this help"},

	{Mode: ModeMouse, Section: "Mouse", Label: "Click", Help: "Select a node or flag"},
	{Mode: ModeMouse, Section: "Mouse", Label: "Click ▶ / ▼", Help: "Expand / collapse"},
	{Mode: ModeMouse, Section: "Mouse", Label: "Wheel", Help: "Scroll the tree"},

	// Quit: Esc goes back one level and quits at the root.
	{Mode: ModeTree, Keys: []string{"ctrl+c", "q"}, action: actQuit, Hint: "q:quit",
		Section: "Quit", Label: "q / Esc / Ctrl+C", Help: "Quit"},
	{Mode: ModeTree, Keys: []string{"esc"}, action: actBack},

	// The focused help pane's scroll keys, over the tree's keys.
	{Mode: ModeHelpPane, Keys: []string{"up", "k"}, action: actScrollUp, Hint: "↑↓:scroll"},
	{Mode: ModeHelpPane, Keys: []string{"down", "j"}, action: actScrollDown},
	{Mode: ModeHelpPane, Keys: []string{"pgup", "ctrl+u", "b"}, action: actScrollPageUp, Hint: "PgUp/PgDn"},
	{Mode: ModeHelpPane, Keys: []string{"pgdown", "ctrl+d"}, action: actScrollPageDown},
	{Mode: ModeHelpPane, Keys: []string{"g"}, action: actScrollTop, Hint: "g/G:top/bottom"},
	{Mode: ModeHelpPane, Keys: []string{"G"}, action: actScrollBottom},

	// The focused preview line; other keys edit it.
	{Mode: ModePreview, Keys: []string{"ctrl+c"}, action: actQuit},
	{Mode: ModePreview, Keys: []string{"esc"}, action: actBack, Hint: "Esc:back"},
	{Mode: ModePreview, Keys: []string{"ctrl+e"}, action: actExecPreview, Hint: "Ctrl+E:exec/copy"},
	{Mode: ModePreview, Keys: []string{"tab"}, action: actFocusNext, Hint: "Tab:switch"},
	{Mode: ModePreview, Keys: []string{"shift+tab"}, action: actFocusPrev},

	// The filter line; other keys edit it.
	{Mode: ModeFilter, Keys: []string{"enter"}, action: actFilterKeep, Hint: "Enter:keep"},
	{Mode: ModeFilter, Keys: []string{"esc"}, action: actFilterClear, Hint: "Esc:clear"},
}

// footerOrder is the footer's token order, by action, for each footer.
var footerOrder = map[string][]action{
	ModeTree: {actUp, actLeft, actPick, actExpandAll, actExpandSubtree, actToggleSections, actFlags,
		actFilter, actToggleHelpPane, actExec, actBottom, actNextMatch, actKeys, actQuit},
	ModeHelpPane: {actScrollUp, actScrollPageUp, actScrollTop, actFocusNext},
	ModePreview:  {actBack, actExecPreview, actFocusNext},
	ModeFilter:   {actFilterKeep, actFilterClear},
}

// hintDropOrder lists the positions in footerOrder[ModeTree] of the tokens
// dropped when the footer is too narrow (least important first).
var hintDropOrder = []int{11, 10, 9, 4, 3, 5, 6, 1}

// keyIndex maps mode -> key -> action.
var keyIndex = indexKeymap(keymap)

func indexKeymap(table []Binding) map[string]map[string]action {
	idx := map[string]map[string]action{}
	for _, b := range table {
		if idx[b.Mode] == nil {
			idx[b.Mode] = map[string]action{}
		}
		for _, k := range b.Keys {
			if _, dup := idx[b.Mode][k]; !dup {
				idx[b.Mode][k] = b.action
			}
		}
	}
	return idx
}

// lookupKey returns what key does in mode, or actNone.
func lookupKey(mode, key string) action {
	return keyIndex[mode][key]
}

// schemeMode is the key map mode of a navigation scheme.
func schemeMode(s NavScheme) string {
	switch s {
	case SchemeVim:
		return ModeVim
	case SchemeWASD:
		return ModeWASD
	default:
		return ModeArrows
	}
}

// layers is the modes whose keys apply in mode, top first.
func layers(mode string) []string {
	switch mode {
	case ModeArrows, ModeVim, ModeWASD:
		return []string{mode, ModeTree}
	case ModeHelpPane:
		return []string{ModeHelpPane, ModeTree}
	default:
		return []string{mode}
	}
}

// Keymap returns a copy of the key map, for tools that check the
// documentation against it.
func Keymap() []Binding {
	out := make([]Binding, len(keymap))
	copy(out, keymap)
	return out
}

// Footer returns the key hints the footer shows in mode (a tree scheme mode,
// ModeHelpPane, ModePreview or ModeFilter), before narrowing to fit.
func Footer(mode string) string {
	order := footerOrder[mode]
	if strings.HasPrefix(mode, ModeTree+":") {
		order = footerOrder[ModeTree]
	}
	var tokens []string
	for _, act := range order {
		if h := hintFor(layers(mode), act); h != "" {
			tokens = append(tokens, h)
		}
	}
	return strings.Join(tokens, "  ")
}

// hintFor is the first footer hint for act among the bindings of modes.
func hintFor(modes []string, act action) string {
	for _, mode := range modes {
		for _, b := range keymap {
			if b.Mode == mode && b.action == act && b.Hint != "" {
				return b.Hint
			}
		}
	}
	return ""
}

// KeyOverlay returns the ? overlay's text: one section per heading, each row
// a key column and what it does.
func KeyOverlay() string {
	var sb strings.Builder
	for i, section := range overlaySections {
		var rows []Binding
		width := 0
		for _, b := range keymap {
			if b.Section == section {
				rows = append(rows, b)
				width = max(width, lipgloss.Width(b.Label))
			}
		}
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(section)
		for _, b := range rows {
			pad := strings.Repeat(" ", width-lipgloss.Width(b.Label)+3)
			fmt.Fprintf(&sb, "\n  %s%s%s", b.Label, pad, b.Help)
		}
	}
	return sb.String()
}

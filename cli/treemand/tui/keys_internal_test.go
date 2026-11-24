package tui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// docTokens splits the key bindings overlay into the tokens a user reads
// ("Ctrl+F", "PgUp", "k", ...).
func docTokens() map[string]bool {
	set := map[string]bool{}
	for _, tok := range regexp.MustCompile(`[\s/·,;:()]+`).Split(KeyOverlay(), -1) {
		if tok != "" {
			set[tok] = true
		}
	}
	// "/" is also the separator above, so a line that starts with it is the
	// filter binding itself.
	if regexp.MustCompile(`(?m)^\s+/\s`).MatchString(KeyOverlay()) {
		set["/"] = true
	}
	return set
}

// docForms are the spellings under which the overlay may document a key as
// reported by bubbletea; any one of them counts.
func docForms(k string) []string {
	arrows := map[string]string{"up": "↑", "down": "↓", "left": "←", "right": "→"}
	words := map[string]string{
		"pgup": "PgUp", "pgdown": "PgDn", "esc": "Esc", "tab": "Tab", "enter": "Enter",
		" ": "Space", "backspace": "Backspace", "delete": "Delete",
	}
	switch {
	case arrows[k] != "":
		return []string{arrows[k]}
	case words[k] != "":
		return []string{words[k]}
	case strings.HasPrefix(k, "shift+") && arrows[k[6:]] != "":
		return []string{"Shift+" + arrows[k[6:]]}
	case strings.HasPrefix(k, "shift+") || strings.HasPrefix(k, "ctrl+"):
		parts := strings.SplitN(k, "+", 2)
		name := strings.ToUpper(parts[0][:1]) + parts[0][1:]
		rest := parts[1]
		if w := words[rest]; w != "" {
			return []string{name + "+" + w}
		}
		return []string{name + "+" + strings.ToUpper(rest)}
	case k == "g":
		return []string{"gg"} // the overlay documents the two-key sequence
	}
	return []string{k} // a plain letter, case-sensitive
}

// tableKeys returns every key the key map binds in modes.
func tableKeys(modes ...string) []string {
	var keys []string
	for _, b := range keymap {
		for _, mode := range modes {
			if b.Mode == mode {
				keys = append(keys, b.Keys...)
			}
		}
	}
	return keys
}

// A key bound twice in one mode would silently lose one binding.
func TestKeymapBindsEachKeyOnceInAMode(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range keymap {
		for _, k := range b.Keys {
			id := b.Mode + " " + strconv.Quote(k)
			if seen[id] {
				t.Errorf("key %s is bound twice", id)
			}
			seen[id] = true
		}
	}
}

// Every bound action runs something, and every footer token is there in
// each scheme, and every action hintDropOrder drops is in the tree footer.
func TestKeymapActionsRunAndFootersAreComplete(t *testing.T) {
	for _, b := range keymap {
		if _, ok := actionFuncs[b.action]; !ok && len(b.Keys) > 0 {
			t.Errorf("%s %q: action %d has no function", b.Mode, b.Keys, b.action)
		}
	}
	if got := len(hintDropPositions()); got != len(hintDropOrder) {
		t.Errorf("hintDropOrder names %d actions, %d of them in the tree footer", len(hintDropOrder), got)
	}
	for mode, order := range footerOrder {
		modes := []string{mode}
		if mode == ModeTree {
			modes = []string{ModeArrows, ModeVim, ModeWASD}
		}
		for _, md := range modes {
			if got := len(strings.Split(Footer(md), "  ")); got != len(order) {
				t.Errorf("footer of %s has %d tokens, want %d: %q", md, got, len(order), Footer(md))
			}
		}
	}
}

// Shift plus a letter arrives as the capital letter, so the subtree keys are
// Shift+arrows in every scheme, and the footers say so (#312).
func TestSubtreeKeysAreShiftArrowsInEveryScheme(t *testing.T) {
	for _, mode := range []string{ModeArrows, ModeVim, ModeWASD} {
		if f := Footer(mode); !strings.Contains(f, "Shift+←→:subtree") {
			t.Errorf("%s footer lacks Shift+←→:subtree: %s", mode, f)
		}
	}
	for _, k := range tableKeys(ModeTree, ModeArrows, ModeVim, ModeWASD) {
		if strings.HasPrefix(k, "shift+") && len(k) == len("shift+x") {
			t.Errorf("%q is bound, but bubbletea reports Shift+letter as the capital letter", k)
		}
	}
}

// Every key the tree pane reacts to must be in the in-app key bindings
// overlay (the `?` screen), in whichever navigation scheme it belongs to. A
// new binding without documentation fails here.
func TestEveryTreeKeyIsDocumentedInTheOverlay(t *testing.T) {
	tokens := docTokens()
	keys := tableKeys(ModeTree, ModeArrows, ModeVim, ModeWASD)
	if len(keys) < 40 {
		t.Fatalf("the key map binds only %d tree keys", len(keys))
	}
	var missing []string
	for _, k := range keys {
		documented := false
		for _, form := range docForms(k) {
			if tokens[form] {
				documented = true
			}
		}
		if !documented {
			missing = append(missing, strconv.Quote(k))
		}
	}
	if len(missing) > 0 {
		t.Errorf("keys in the key map but missing from the ? overlay: %s", strings.Join(missing, ", "))
	}
}

// The overlay is only useful if people can find it: the footer names the key,
// in every navigation scheme and at 80 columns.
func TestFooterAdvertisesTheKeyBindingsOverlay(t *testing.T) {
	for _, scheme := range []NavScheme{SchemeArrows, SchemeVim, SchemeWASD} {
		hints := Footer(schemeMode(scheme))
		if !strings.Contains(hints, "?:keys") {
			t.Errorf("scheme %v footer hints lack ?:keys: %s", scheme, hints)
		}
		// 80 columns minus a short selection on the left.
		if fitted := fitHints(hints, 80-len("git")-3-len("[arrows] ")); !strings.Contains(fitted, "?:keys") {
			t.Errorf("scheme %v: ?:keys dropped when the footer is narrow: %s", scheme, fitted)
		}
	}
}

// A wrapped overlay entry keeps its description in its column, so
// "Expand node; enter" and "children (2nd press)" read as one entry.
func TestWrapHangingKeepsTheDescriptionColumn(t *testing.T) {
	line := "  → / l / d                          Expand node; enter children (2nd press)"
	idx := strings.Index(line, "Expand")
	if idx < 0 {
		t.Fatal("fixture lost its description")
	}
	col := lipgloss.Width(line[:idx]) // display columns, not bytes
	got := wrapHanging(line, 52)
	if len(got) < 2 {
		t.Fatalf("expected the line to wrap at width 52, got %q", got)
	}
	for i, l := range got {
		if lipgloss.Width(l) > 52 {
			t.Errorf("row %d is %d wide, want at most 52: %q", i, lipgloss.Width(l), l)
		}
		if i > 0 && !strings.HasPrefix(l, strings.Repeat(" ", col)) {
			t.Errorf("continuation %q does not start under the description (column %d)", l, col)
		}
	}
	if joined := strings.Join(strings.Fields(strings.Join(got, " ")), " "); joined != strings.Join(strings.Fields(line), " ") {
		t.Errorf("wrapping changed the words: %q", joined)
	}
	if short := wrapHanging("  q / Esc    Quit", 52); len(short) != 1 || short[0] != "  q / Esc    Quit" {
		t.Errorf("a short line must be left alone, got %q", short)
	}
	if head := wrapHanging("Navigation", 52); len(head) != 1 || head[0] != "Navigation" {
		t.Errorf("a heading must be left alone, got %q", head)
	}
}

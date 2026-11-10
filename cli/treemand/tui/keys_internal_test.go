package tui

import (
	"os"
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
	for _, tok := range regexp.MustCompile(`[\s/·,;:()]+`).Split(keybindText, -1) {
		if tok != "" {
			set[tok] = true
		}
	}
	// "/" is also the separator above, so a line that starts with it is the
	// filter binding itself.
	if regexp.MustCompile(`(?m)^\s+/\s`).MatchString(keybindText) {
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

// keyCases returns every key literal in the `case "a", "b":` labels of the
// named functions of model_keys.go.
func keyCases(t *testing.T, funcs ...string) []string {
	t.Helper()
	src, err := os.ReadFile("model_keys.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	caseRe := regexp.MustCompile(`(?m)^\s*case ((?:"(?:[^"\\]|\\.)*"(?:,\s*)?)+):`)
	quoted := regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	var keys []string
	for _, name := range funcs {
		start := strings.Index(text, "func (m *Model) "+name+"(")
		if start < 0 {
			t.Fatalf("function %s not found in model_keys.go", name)
		}
		body := text[start+1:]
		if end := strings.Index(body, "\nfunc "); end >= 0 {
			body = body[:end]
		}
		for _, m := range caseRe.FindAllStringSubmatch(body, -1) {
			for _, q := range quoted.FindAllString(m[1], -1) {
				k, err := strconv.Unquote(q)
				if err != nil {
					t.Fatalf("bad key literal %s: %v", q, err)
				}
				keys = append(keys, k)
			}
		}
	}
	return keys
}

// Every key the tree pane reacts to must be in the in-app key bindings
// overlay (the `?` screen), in whichever navigation scheme it belongs to. A
// new binding without documentation fails here.
func TestEveryTreeKeyIsDocumentedInTheOverlay(t *testing.T) {
	tokens := docTokens()
	keys := keyCases(t, "updateKeys", "handleArrows", "handleVim", "handleWASD")
	if len(keys) < 40 {
		t.Fatalf("found only %d key cases; the scan is probably broken", len(keys))
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
		t.Errorf("keys handled in model_keys.go but missing from keybindText: %s", strings.Join(missing, ", "))
	}
}

// The overlay is only useful if people can find it: the footer names the key,
// in every navigation scheme and at 80 columns.
func TestFooterAdvertisesTheKeyBindingsOverlay(t *testing.T) {
	for _, scheme := range []NavScheme{SchemeArrows, SchemeVim, SchemeWASD} {
		m := &Model{scheme: scheme}
		hints := m.schemeHints()
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
	col := lipgloss.Width(line[:strings.Index(line, "Expand")]) // display columns, not bytes
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

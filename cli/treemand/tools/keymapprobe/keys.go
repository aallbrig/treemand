package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// ---- keys as bubbletea reports them ----

var namedKeys = func() map[string]bool {
	out := map[string]bool{}
	for k := -200; k < 256; k++ {
		if s := tea.KeyType(k).String(); s != "" {
			out[s] = true
		}
	}
	return out
}()

// reachable reports whether bubbletea can report key as msg.String().
func reachable(key string) bool {
	if namedKeys[key] {
		return true
	}
	if rest, ok := strings.CutPrefix(key, "alt+"); ok {
		return reachable(rest)
	}
	r, n := utf8.DecodeRuneInString(key)
	return n == len(key) && n > 0 && r >= 0x20 && r != 0x7f
}

func unreachableNote(key string) string {
	if rest, ok := strings.CutPrefix(key, "shift+"); ok && utf8.RuneCountInString(rest) == 1 {
		up := strings.ToUpper(rest)
		return fmt.Sprintf("bubbletea never reports %q: Shift+%s arrives as %q", key, up, up)
	}
	return fmt.Sprintf("bubbletea never reports %q", key)
}

// ---- reading written keys ----

var arrowGlyph = map[rune]string{'↑': "up", '↓': "down", '←': "left", '→': "right"}

var wordKeys = map[string]string{
	"esc": "esc", "enter": "enter", "tab": "tab", "space": " ", "backspace": "backspace",
	"delete": "delete", "del": "delete", "pgup": "pgup", "pgdn": "pgdown", "pgdown": "pgdown",
	"home": "home", "end": "end", "insert": "insert",
}

// splitMods splits "Ctrl+Shift+x" into its lower-case modifiers and the rest.
func splitMods(s string) ([]string, string) {
	var mods []string
	for {
		i := strings.Index(s, "+")
		if i <= 0 || i == len(s)-1 {
			return mods, s
		}
		m := strings.ToLower(s[:i])
		if m != "ctrl" && m != "shift" && m != "alt" {
			return mods, s
		}
		mods = append(mods, m)
		s = s[i+1:]
	}
}

// baseKey turns one documented key name (no modifiers) into bubbletea's name.
func baseKey(b string) (string, bool) {
	if w, ok := wordKeys[strings.ToLower(b)]; ok && utf8.RuneCountInString(b) > 1 {
		return w, true
	}
	if b == "gg" {
		return "g", true // a two-key sequence bound as g
	}
	r, n := utf8.DecodeRuneInString(b)
	if n != len(b) || n == 0 {
		return "", false
	}
	if a, ok := arrowGlyph[r]; ok {
		return a, true
	}
	return b, true
}

func applyMods(mods []string, k string) string {
	single := utf8.RuneCountInString(k) == 1
	var prefix []string
	for _, m := range mods {
		switch {
		case m == "shift" && single:
			k = strings.ToUpper(k) // the terminal sends the capital
		case m == "ctrl" && single:
			k = strings.ToLower(k)
			prefix = append(prefix, m)
		default:
			prefix = append(prefix, m)
		}
	}
	if len(prefix) == 0 {
		return k
	}
	return strings.Join(prefix, "+") + "+" + k
}

func isArrow(r rune) bool   { _, ok := arrowGlyph[r]; return ok }
func isVimMove(r rune) bool { return strings.ContainsRune("hjkl", r) }

// expandBase splits runs written as one token: "↑↓" and "jk".
func expandBase(base string, modified bool) []string {
	switch {
	case utf8.RuneCountInString(base) > 1 && allRunes(base, isArrow):
		var out []string
		for _, r := range base {
			out = append(out, string(r))
		}
		return out
	case !modified && len(base) == 2 && allRunes(base, isVimMove):
		return []string{base[:1], base[1:]}
	}
	return []string{base}
}

// docKeys reads a written key group such as "Shift+h/l", "↑↓/jk" or "PgUp/PgDn"
// as the keys bubbletea reports. Text that is not a key gives nil.
func docKeys(s string) []string {
	s = strings.TrimSpace(s)
	parts := []string{s}
	if s != "/" {
		parts = strings.Split(s, "/")
	}
	var out []string
	var prevMods []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		mods, base := splitMods(p)
		if len(mods) == 0 && utf8.RuneCountInString(base) == 1 {
			mods = prevMods // "Shift+h/l": the modifier distributes
		}
		for _, b := range expandBase(base, len(mods) > 0) {
			k, ok := baseKey(b)
			if !ok {
				return nil // one part is not a key: the group is prose
			}
			out = append(out, applyMods(mods, k))
		}
		prevMods = mods
	}
	return out
}

func allRunes(s string, f func(rune) bool) bool {
	for _, r := range s {
		if !f(r) {
			return false
		}
	}
	return s != ""
}

// ---- hints ----

var hintSep = regexp.MustCompile(`\s*·\s*|\s{2,}`)
var bracket = regexp.MustCompile(`\[([^\]]+)\]`)

// hintKeys reads the keys a one-line hint names: "j/k:nav  ?:keys",
// "↑↓/jk scroll · Esc close" or "[Enter] confirm  [Esc] cancel".
func hintKeys(hint string) []string {
	var out []string
	for _, e := range hintSep.Split(strings.TrimSpace(hint), -1) {
		if e != "" {
			out = append(out, entryKeys(e)...)
		}
	}
	return out
}

// entryKeys reads one hint entry: "[keys] text", "keys:text" or "keys text".
func entryKeys(e string) []string {
	if m := bracket.FindAllStringSubmatch(e, -1); m != nil {
		var out []string
		for _, g := range m {
			out = append(out, docKeys(g[1])...)
		}
		return out
	}
	if i := strings.Index(e[1:], ":"); i >= 0 {
		return docKeys(e[:i+1])
	}
	return docKeys(strings.Fields(e)[0])
}

// ---- the ? overlay ----

// overlayKeyColumn reads the key column of each overlay entry
// ("  keys   description"), skipping the Mouse section.
func overlayKeyColumn(text string) map[string]bool {
	out := map[string]bool{}
	section := ""
	for _, line := range strings.Split(text, "\n") {
		lead := len(line) - len(strings.TrimLeft(line, " "))
		switch {
		case strings.TrimSpace(line) == "":
		case lead == 0:
			section = strings.TrimSpace(line)
		case lead == 2 && section != "Mouse": // not continuations or pointer actions
			for _, k := range keyColumn(line) {
				out[k] = true
			}
		}
	}
	return out
}

// keyColumn reads keys from the start of an overlay entry until the
// description begins.
func keyColumn(line string) []string {
	var out []string
	for i, tok := range strings.Fields(line) {
		if i > 0 && (tok == "/" || tok == "or") {
			continue
		}
		ks := docKeys(tok)
		if ks == nil {
			break
		}
		out = append(out, ks...)
	}
	return out
}

// ---- website tables ----

type tableCell struct {
	scheme string
	keys   []string
	line   int
}

var codeSpan = regexp.MustCompile("`([^`]+)`")
var heading = regexp.MustCompile(`^(#+)\s+(.*)$`)
var keySection = regexp.MustCompile(`(?i)key|keyboard`)
var schemeCol = regexp.MustCompile(`(?i)\((arrows|vim|wasd)\)`)

// mdReader tracks the heading path and the current table header.
type mdReader struct {
	stack  []string // heading text by level-1
	header []string
}

func (r *mdReader) heading(level int, text string) {
	if len(r.stack) >= level {
		r.stack = r.stack[:level-1]
	}
	for len(r.stack) < level-1 {
		r.stack = append(r.stack, "")
	}
	r.stack = append(r.stack, text)
	r.header = nil
}

func (r *mdReader) inKeySection() bool {
	for _, h := range r.stack {
		if keySection.MatchString(h) {
			return true
		}
	}
	return false
}

// row reads one data row's key columns (headed "Key…").
func (r *mdReader) row(cells []string, line int) []tableCell {
	var out []tableCell
	for c, cell := range cells {
		if c >= len(r.header) || !strings.HasPrefix(strings.TrimSpace(r.header[c]), "Key") {
			continue
		}
		tc := tableCell{line: line}
		if m := schemeCol.FindStringSubmatch(r.header[c]); m != nil {
			tc.scheme = strings.ToLower(m[1])
		}
		for _, span := range codeSpan.FindAllStringSubmatch(cell, -1) {
			tc.keys = append(tc.keys, docKeys(span[1])...)
		}
		if len(tc.keys) > 0 {
			out = append(out, tc)
		}
	}
	return out
}

// keyTableCells reads the key columns of the markdown tables under a heading
// about keys.
func keyTableCells(md string) []tableCell {
	var out []tableCell
	r := &mdReader{}
	for i, line := range strings.Split(md, "\n") {
		t := strings.TrimSpace(line)
		m := heading.FindStringSubmatch(line)
		switch {
		case m != nil:
			r.heading(len(m[1]), m[2])
		case !strings.HasPrefix(t, "|"):
			r.header = nil
		case r.header == nil:
			r.header = strings.Split(strings.Trim(t, "|"), "|")
		case strings.Trim(t, "|-: ") == "": // the separator row
		case r.inKeySection():
			out = append(out, r.row(strings.Split(strings.Trim(t, "|"), "|"), i+1)...)
		}
	}
	return out
}

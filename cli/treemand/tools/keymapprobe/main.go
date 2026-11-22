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
// The TUI has no single key map, so the mode table (DefaultConfig) says which
// switches make up each mode. Any key switch the table does not name, and any
// table entry the code no longer has, fails the probe: drift is loud.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// SwitchRef names the Index-th switch with string-literal cases in Func.
type SwitchRef struct {
	Func  string
	Index int
}

// HintRef selects hint text: the string literals of Func, only those inside
// the case clause whose condition contains Clause ("default" for the default
// clause) when Clause is set, and only those containing Contains when set.
type HintRef struct {
	Func, Clause, Contains string
}

// Mode is one input context of the TUI.
type Mode struct {
	Name string
	// Scheme ties a tree mode to a website table column ("Keys (vim)").
	Scheme string
	// Chain is the dispatch order: an earlier case that always returns
	// consumes the key, so a later switch listing it is shadowed.
	Chain []SwitchRef
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
	OverlayConst string   // the const holding the ? overlay text
	Reference    string   // the website page that must list every key
	Pages        []string // website pages whose key tables are checked
}

// DefaultConfig is treemand's TUI as of this probe.
func DefaultConfig() Config {
	tree := func(name, scheme, handler, clause string) Mode {
		return Mode{Name: name, Scheme: scheme, Overlay: true, Reference: true,
			Chain: []SwitchRef{{"updateKeys", 0}, {handler, 0}},
			Hints: []HintRef{{Func: "schemeHints", Clause: clause}}}
	}
	dialog := func(name, update string, idx int, render, contains string) Mode {
		return Mode{Name: name, Chain: []SwitchRef{{update, idx}},
			Hints: []HintRef{{Func: render, Contains: contains}}}
	}
	return Config{
		OverlayConst: "keybindText",
		Reference:    "reference.md",
		Pages:        []string{"reference.md", "commands/interactive.md", "getting-started.md"},
		Modes: []Mode{
			tree("tree:arrows", "arrows", "handleArrows", "default"),
			tree("tree:vim", "vim", "handleVim", "SchemeVim"),
			tree("tree:wasd", "wasd", "handleWASD", "SchemeWASD"),
			{Name: "help-pane", Overlay: true, Reference: true,
				Chain: []SwitchRef{{"updateKeys", 0}, {"updateHelpPaneKeys", 0}},
				Hints: []HintRef{{Func: "renderStatusBar", Clause: "paneHelp"}}},
			{Name: "preview", Chain: []SwitchRef{{"updatePreviewInput", 0}},
				Hints: []HintRef{{Func: "renderStatusBar", Clause: "panePreview"}}},
			{Name: "filter", Chain: []SwitchRef{{"updateFilter", 0}},
				Hints: []HintRef{{Func: "renderStatusBar", Clause: "m.filtering"}}},
			dialog("keys-overlay", "updateKeybindModal", 0, "renderKeybindModal", "Esc"),
			dialog("value-input", "updateValueModal", 0, "renderValueInputModal", "Esc"),
			dialog("exec", "updateModal", 0, "renderModal", "Esc"),
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

type binding struct {
	key      string
	consumes bool // the case always returns
	where    string
}

type keySwitch struct {
	fn       string
	idx      int
	bindings []binding
}

type source struct {
	fset     *token.FileSet
	files    map[*ast.File][]byte
	funcs    map[string][]*ast.FuncDecl
	switches map[SwitchRef]*keySwitch
	keySw    map[SwitchRef]bool // switches on msg.String() or key
	consts   map[string]string
}

// Scan reads the tui package in tuiDir and the website content in siteDir.
func Scan(tuiDir, siteDir string, cfg Config) (*Report, error) {
	src, err := load(tuiDir)
	if err != nil {
		return nil, err
	}
	rep := &Report{Probe: "tui-keymap", Metrics: map[string]float64{}}
	add := func(d Detail) {
		rep.Details = append(rep.Details, d)
		rep.Metrics[d.Metric]++
	}
	for _, m := range []string{"conflicts", "unreachable_bindings", "undocumented_bindings.overlay",
		"undocumented_bindings.reference", "undocumented_bindings.dialogs", "documented_unbound.overlay",
		"documented_unbound.hints", "documented_unbound.website"} {
		rep.Metrics[m] = 0
	}

	// Every key switch must belong to a mode, and every mode's switches exist.
	used := map[SwitchRef]bool{}
	for _, md := range cfg.Modes {
		for _, ref := range md.Chain {
			if len(src.funcs[ref.Func]) == 0 {
				return nil, fmt.Errorf("mode %s: function %s not found in %s", md.Name, ref.Func, tuiDir)
			}
			if src.switches[ref] == nil {
				return nil, fmt.Errorf("mode %s: %s has no key switch #%d", md.Name, ref.Func, ref.Index)
			}
			used[ref] = true
		}
	}
	var unmapped []string
	for ref := range src.keySw {
		if !used[ref] {
			unmapped = append(unmapped, fmt.Sprintf("%s#%d", ref.Func, ref.Index))
		}
	}
	if len(unmapped) > 0 {
		sort.Strings(unmapped)
		return nil, fmt.Errorf("key switches not in the mode table (add them to DefaultConfig): %s", strings.Join(unmapped, ", "))
	}

	// Bindings, reachability and conflicts.
	bound := map[string]map[string]string{} // mode -> key -> where
	seenUnreach := map[string]bool{}
	var refs []SwitchRef
	for ref := range used {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Func != refs[j].Func {
			return refs[i].Func < refs[j].Func
		}
		return refs[i].Index < refs[j].Index
	})
	for _, ref := range refs {
		for _, b := range src.switches[ref].bindings {
			rep.Bound++
			if !reachable(b.key) && !seenUnreach[ref.Func+"\x00"+b.key] {
				seenUnreach[ref.Func+"\x00"+b.key] = true
				add(Detail{Metric: "unreachable_bindings", Key: b.key, Where: b.where, Note: unreachableNote(b.key)})
			}
		}
	}
	for _, md := range cfg.Modes {
		bound[md.Name] = map[string]string{}
		consumed := map[string]string{}
		for i, ref := range md.Chain {
			for _, b := range src.switches[ref].bindings {
				if !reachable(b.key) {
					continue
				}
				if by, ok := consumed[b.key]; ok && i > 0 {
					add(Detail{Metric: "conflicts", Key: b.key, Mode: md.Name, Where: b.where,
						Note: "shadowed by " + by + ", which handles it first"})
				}
				if _, ok := bound[md.Name][b.key]; !ok {
					bound[md.Name][b.key] = b.where
				}
				if b.consumes {
					if _, ok := consumed[b.key]; !ok {
						consumed[b.key] = ref.Func + " (" + b.where + ")"
					}
				}
			}
		}
	}

	// The overlay.
	overlay, ok := src.consts[cfg.OverlayConst]
	if !ok {
		return nil, fmt.Errorf("overlay const %s not found", cfg.OverlayConst)
	}
	overlayKeys := overlayKeyColumn(overlay)
	overlayWhere := cfg.OverlayConst
	unionBound := func(pick func(Mode) bool) map[string]string {
		out := map[string]string{}
		for _, md := range cfg.Modes {
			if pick(md) {
				for k, w := range bound[md.Name] {
					if _, ok := out[k]; !ok || w < out[k] {
						out[k] = w
					}
				}
			}
		}
		return out
	}
	overlayBound := unionBound(func(m Mode) bool { return m.Overlay })
	for _, k := range sortedKeys(overlayBound) {
		if !overlayKeys[k] {
			add(Detail{Metric: "undocumented_bindings.overlay", Key: k, Where: overlayBound[k],
				Note: "bound but not in the ? overlay"})
		}
	}
	for _, k := range sortedKeys(overlayKeys) {
		if _, ok := overlayBound[k]; !ok {
			add(Detail{Metric: "documented_unbound.overlay", Key: k, Where: overlayWhere,
				Note: "the ? overlay names it; no tree or help-pane switch binds it"})
		}
	}

	// In-app hints.
	for _, md := range cfg.Modes {
		documented := map[string]string{} // key -> where the hint is
		for _, h := range md.Hints {
			lits, where, err := src.hintLiterals(h)
			if err != nil {
				return nil, fmt.Errorf("mode %s: %w", md.Name, err)
			}
			for _, lit := range lits {
				for _, k := range hintKeys(lit) {
					documented[k] = where
				}
			}
		}
		for _, k := range sortedKeys(documented) {
			if _, ok := bound[md.Name][k]; !ok {
				add(Detail{Metric: "documented_unbound.hints", Key: k, Mode: md.Name, Where: documented[k],
					Note: "the hint names it; this mode does not bind it"})
			}
		}
		if md.Overlay {
			continue // tree footers are a curated subset; the overlay is complete
		}
		for _, k := range sortedKeys(bound[md.Name]) {
			// Ctrl+C cancels or quits everywhere, and the overlay says so.
			if _, ok := documented[k]; !ok && (k != "ctrl+c" || !overlayKeys[k]) {
				add(Detail{Metric: "undocumented_bindings.dialogs", Key: k, Mode: md.Name, Where: bound[md.Name][k],
					Note: "bound but not in this mode's hint"})
			}
		}
	}

	// The website.
	refBound := unionBound(func(m Mode) bool { return m.Reference })
	for _, page := range cfg.Pages {
		data, err := os.ReadFile(filepath.Join(siteDir, page)) //nolint:gosec // docs paths from the mode table
		if err != nil {
			return nil, err
		}
		cells := keyTableCells(string(data))
		if len(cells) == 0 {
			return nil, fmt.Errorf("%s: no key binding table found", page)
		}
		pageKeys := map[string]bool{}
		for _, c := range cells {
			want := refBound
			if c.scheme != "" {
				want = unionBound(func(m Mode) bool { return m.Reference && m.Scheme == c.scheme })
				if len(want) == 0 {
					return nil, fmt.Errorf("%s: column for scheme %q matches no mode", page, c.scheme)
				}
			}
			for _, k := range c.keys {
				pageKeys[k] = true
				if _, ok := want[k]; !ok {
					note := "the website names it; nothing binds it"
					if c.scheme != "" {
						note = "the website names it for the " + c.scheme + " scheme; that scheme does not bind it"
					}
					add(Detail{Metric: "documented_unbound.website", Key: k, Mode: c.scheme,
						Where: fmt.Sprintf("%s:%d", page, c.line), Note: note})
				}
			}
		}
		if page == cfg.Reference {
			for _, k := range sortedKeys(refBound) {
				if !pageKeys[k] {
					add(Detail{Metric: "undocumented_bindings.reference", Key: k, Where: refBound[k],
						Note: "bound but not in the website reference (" + page + ")"})
				}
			}
		}
	}
	rep.Metrics["bindings"] = float64(rep.Bound)
	return rep, nil
}

func load(dir string) (*source, error) {
	src := &source{fset: token.NewFileSet(), files: map[*ast.File][]byte{}, funcs: map[string][]*ast.FuncDecl{},
		switches: map[SwitchRef]*keySwitch{}, keySw: map[SwitchRef]bool{}, consts: map[string]string{}}
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		data, err := os.ReadFile(p) //nolint:gosec // source files of the directory the caller names
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(src.fset, filepath.Base(p), data, 0)
		if err != nil {
			return nil, err
		}
		src.files[f] = data
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				src.funcs[d.Name.Name] = append(src.funcs[d.Name.Name], d)
				src.collectSwitches(d)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || d.Tok != token.CONST {
						continue
					}
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								if s, err := strconv.Unquote(lit.Value); err == nil {
									src.consts[name.Name] = s
								}
							}
						}
					}
				}
			}
		}
	}
	if len(src.funcs) == 0 {
		return nil, fmt.Errorf("no Go source in %s", dir)
	}
	return src, nil
}

func (s *source) collectSwitches(fn *ast.FuncDecl) {
	if fn.Body == nil {
		return
	}
	idx := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		ks := &keySwitch{fn: fn.Name.Name, idx: idx}
		for _, st := range sw.Body.List {
			cc := st.(*ast.CaseClause)
			consumes := false
			if len(cc.Body) > 0 {
				_, consumes = cc.Body[len(cc.Body)-1].(*ast.ReturnStmt)
			}
			for _, e := range cc.List {
				lit, ok := e.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				k, err := strconv.Unquote(lit.Value)
				if err != nil {
					continue
				}
				pos := s.fset.Position(lit.Pos())
				ks.bindings = append(ks.bindings, binding{key: k, consumes: consumes,
					where: fmt.Sprintf("%s:%d", pos.Filename, pos.Line)})
			}
		}
		if len(ks.bindings) == 0 {
			return true
		}
		ref := SwitchRef{fn.Name.Name, idx}
		s.switches[ref] = ks
		if isKeyTag(sw.Tag) {
			s.keySw[ref] = true
		}
		idx++
		return true
	})
}

// isKeyTag reports whether a switch tag is a key: msg.String() or key.
func isKeyTag(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == "key"
	case *ast.CallExpr:
		sel, ok := t.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "String" || len(t.Args) != 0 {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == "msg"
	}
	return false
}

func (s *source) text(n ast.Node) string {
	pos := s.fset.Position(n.Pos())
	end := s.fset.Position(n.End())
	for f, data := range s.files {
		if s.fset.Position(f.Pos()).Filename == pos.Filename {
			return string(data[pos.Offset:end.Offset])
		}
	}
	return ""
}

func (s *source) hintLiterals(h HintRef) ([]string, string, error) {
	fns := s.funcs[h.Func]
	if len(fns) != 1 {
		return nil, "", fmt.Errorf("hint function %s: found %d definitions, want 1", h.Func, len(fns))
	}
	var scope []ast.Node
	if h.Clause == "" {
		scope = []ast.Node{fns[0].Body}
	} else {
		ast.Inspect(fns[0].Body, func(n ast.Node) bool {
			cc, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			match := h.Clause == "default" && cc.List == nil
			for _, e := range cc.List {
				if strings.Contains(s.text(e), h.Clause) {
					match = true
				}
			}
			if match {
				for _, st := range cc.Body {
					scope = append(scope, st)
				}
			}
			return true
		})
	}
	var lits []string
	where := ""
	for _, node := range scope {
		ast.Inspect(node, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, err := strconv.Unquote(lit.Value)
			if err != nil || (h.Contains != "" && !strings.Contains(v, h.Contains)) {
				return true
			}
			if where == "" {
				p := s.fset.Position(lit.Pos())
				where = fmt.Sprintf("%s:%d", p.Filename, p.Line)
			}
			lits = append(lits, v)
			return true
		})
	}
	if len(lits) == 0 {
		return nil, "", fmt.Errorf("hint %+v matches no string literal", h)
	}
	return lits, where, nil
}

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
	if strings.HasPrefix(key, "alt+") {
		return reachable(key[4:])
	}
	r, n := utf8.DecodeRuneInString(key)
	return n == len(key) && n > 0 && r >= 0x20 && r != 0x7f
}

func unreachableNote(key string) string {
	if rest, ok := strings.CutPrefix(key, "shift+"); ok && utf8.RuneCountInString(rest) == 1 {
		return fmt.Sprintf("bubbletea never reports %q: Shift+%s arrives as %q", key, strings.ToUpper(rest), strings.ToUpper(rest))
	}
	return fmt.Sprintf("bubbletea never reports %q", key)
}

// ---- reading documentation ----

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

// docKeys reads a written key group such as "Shift+h/l", "↑↓/jk" or "PgUp/PgDn"
// as the keys bubbletea reports. Text that is not a key gives nil.
func docKeys(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
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
		if len(mods) == 0 && len(prevMods) > 0 && utf8.RuneCountInString(base) == 1 {
			mods = prevMods // "Shift+h/l": the modifier distributes
		}
		var bases []string
		switch {
		case allRunes(base, func(r rune) bool { _, ok := arrowGlyph[r]; return ok }) && utf8.RuneCountInString(base) > 1:
			for _, r := range base {
				bases = append(bases, string(r))
			}
		case len(mods) == 0 && len(base) == 2 && allRunes(base, func(r rune) bool { return strings.ContainsRune("hjkl", r) }):
			bases = []string{base[:1], base[1:]} // "jk"
		default:
			bases = []string{base}
		}
		for _, b := range bases {
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

var hintSep = regexp.MustCompile(`\s*·\s*|\s{2,}`)
var bracket = regexp.MustCompile(`\[([^\]]+)\]`)

// hintKeys reads the keys a one-line hint names: "j/k:nav  ?:keys",
// "↑↓/jk scroll · Esc close" or "[Enter] confirm  [Esc] cancel".
func hintKeys(hint string) []string {
	var out []string
	for _, e := range hintSep.Split(strings.TrimSpace(hint), -1) {
		if e == "" {
			continue
		}
		if m := bracket.FindAllStringSubmatch(e, -1); m != nil {
			for _, g := range m {
				out = append(out, docKeys(g[1])...)
			}
			continue
		}
		if i := strings.Index(e[1:], ":"); i >= 0 {
			out = append(out, docKeys(e[:i+1])...)
			continue
		}
		out = append(out, docKeys(strings.Fields(e)[0])...)
	}
	return out
}

// overlayKeyColumn reads the key column of each overlay entry
// ("  keys   description"), skipping the Mouse section.
func overlayKeyColumn(text string) map[string]bool {
	out := map[string]bool{}
	section := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lead := len(line) - len(strings.TrimLeft(line, " "))
		if lead == 0 {
			section = strings.TrimSpace(line)
			continue
		}
		if lead != 2 || section == "Mouse" {
			continue // continuation lines and pointer actions
		}
		for i, tok := range strings.Fields(line) {
			if i > 0 && (tok == "/" || tok == "or") {
				continue
			}
			ks := docKeys(tok)
			if ks == nil {
				break // the description starts
			}
			for _, k := range ks {
				out[k] = true
			}
		}
	}
	return out
}

type tableCell struct {
	scheme string
	keys   []string
	line   int
}

var codeSpan = regexp.MustCompile("`([^`]+)`")
var heading = regexp.MustCompile(`^(#+)\s+(.*)$`)
var keySection = regexp.MustCompile(`(?i)key|keyboard`)
var schemeCol = regexp.MustCompile(`(?i)\((arrows|vim|wasd)\)`)

// keyTableCells reads the key columns (headed "Key…") of the markdown tables
// under a heading about keys.
func keyTableCells(md string) []tableCell {
	var out []tableCell
	var stack []string // heading text by level-1
	var header []string
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if m := heading.FindStringSubmatch(line); m != nil {
			lvl := len(m[1])
			if len(stack) >= lvl {
				stack = stack[:lvl-1]
			}
			for len(stack) < lvl-1 {
				stack = append(stack, "")
			}
			stack = append(stack, m[2])
			header = nil
			continue
		}
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			header = nil
			continue
		}
		cells := strings.Split(strings.Trim(t, "|"), "|")
		if header == nil {
			header = cells
			continue
		}
		if strings.Trim(t, "|-: ") == "" {
			continue // the separator row
		}
		inKeys := false
		for _, h := range stack {
			if keySection.MatchString(h) {
				inKeys = true
			}
		}
		if !inKeys {
			continue
		}
		for c, cell := range cells {
			if c >= len(header) || !strings.HasPrefix(strings.TrimSpace(header[c]), "Key") {
				continue
			}
			tc := tableCell{line: i + 1}
			if m := schemeCol.FindStringSubmatch(header[c]); m != nil {
				tc.scheme = strings.ToLower(m[1])
			}
			for _, span := range codeSpan.FindAllStringSubmatch(cell, -1) {
				tc.keys = append(tc.keys, docKeys(span[1])...)
			}
			if len(tc.keys) > 0 {
				out = append(out, tc)
			}
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/aallbrig/treemand/tui"
)

var metricNames = []string{"conflicts", "unreachable_bindings", "undocumented_bindings.overlay",
	"undocumented_bindings.reference", "undocumented_bindings.dialogs", "documented_unbound.overlay",
	"documented_unbound.hints", "documented_unbound.website"}

type scanner struct {
	src          *source
	cfg          Config
	rep          *Report
	bound        map[string]map[string]string // mode -> key -> where
	overlayKeys  map[string]bool
	overlayWhere string
}

// Scan reads the tui package in tuiDir and the website content in siteDir.
func Scan(tuiDir, siteDir string, cfg Config) (*Report, error) {
	src, err := load(tuiDir)
	if err != nil {
		return nil, err
	}
	s := &scanner{src: src, cfg: cfg, rep: &Report{Probe: "tui-keymap", Metrics: map[string]float64{}},
		bound: map[string]map[string]string{}}
	for _, m := range metricNames {
		s.rep.Metrics[m] = 0
	}
	if err := s.checkMapping(tuiDir); err != nil {
		return nil, err
	}
	s.checkUnreachable()
	for _, md := range cfg.Modes {
		s.bindMode(md)
	}
	overlay, where := cfg.OverlayText, "tui.KeyOverlay"
	if overlay == "" {
		var ok bool
		if overlay, ok = src.consts[cfg.OverlayConst]; !ok {
			return nil, fmt.Errorf("overlay const %s not found", cfg.OverlayConst)
		}
		where = cfg.OverlayConst
	}
	s.overlayKeys, s.overlayWhere = overlayKeyColumn(overlay), where
	s.checkOverlay()
	for _, md := range cfg.Modes {
		if err := s.checkHints(md); err != nil {
			return nil, err
		}
	}
	for _, page := range cfg.Pages {
		if err := s.checkPage(siteDir, page); err != nil {
			return nil, err
		}
	}
	s.rep.Metrics["bindings"] = float64(s.rep.Bound)
	return s.rep, nil
}

func (s *scanner) add(d Detail) {
	s.rep.Details = append(s.rep.Details, d)
	s.rep.Metrics[d.Metric]++
}

// usedSwitches lists the switches the mode table names, in a stable order.
func (s *scanner) usedSwitches() []SwitchRef {
	seen := map[SwitchRef]bool{}
	var refs []SwitchRef
	for _, md := range s.cfg.Modes {
		for _, ref := range md.Chain {
			if !seen[ref] {
				seen[ref] = true
				refs = append(refs, ref)
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Func != refs[j].Func {
			return refs[i].Func < refs[j].Func
		}
		return refs[i].Index < refs[j].Index
	})
	return refs
}

// checkMapping fails when the mode table and the key switches disagree.
func (s *scanner) checkMapping(tuiDir string) error {
	used := map[SwitchRef]bool{}
	for _, ref := range s.usedSwitches() {
		if len(s.src.funcs[ref.Func]) == 0 {
			return fmt.Errorf("mode table: function %s not found in %s", ref.Func, tuiDir)
		}
		if s.src.switches[ref] == nil {
			return fmt.Errorf("mode table: %s has no key switch #%d", ref.Func, ref.Index)
		}
		used[ref] = true
	}
	var unmapped []string
	for ref, isKey := range s.src.keySw {
		if isKey && !used[ref] {
			unmapped = append(unmapped, fmt.Sprintf("%s#%d", ref.Func, ref.Index))
		}
	}
	if len(unmapped) > 0 {
		sort.Strings(unmapped)
		return fmt.Errorf("key switches not in the mode table (add them to DefaultConfig): %s", strings.Join(unmapped, ", "))
	}
	return nil
}

func (s *scanner) checkUnreachable() {
	seen := map[string]bool{}
	for _, b := range s.cfg.Table {
		for _, k := range b.Keys {
			s.rep.Bound++
			if !reachable(k) && !seen[k] {
				seen[k] = true
				s.add(Detail{Metric: "unreachable_bindings", Key: k, Where: tableWhere(b), Note: unreachableNote(k)})
			}
		}
	}
	for _, ref := range s.usedSwitches() {
		seen := map[string]bool{}
		for _, b := range s.src.switches[ref].bindings {
			s.rep.Bound++
			if !reachable(b.key) && !seen[b.key] {
				seen[b.key] = true
				s.add(Detail{Metric: "unreachable_bindings", Key: b.key, Where: b.where, Note: unreachableNote(b.key)})
			}
		}
	}
}

// bindMode records the mode's reachable keys and reports the ones a later
// switch lists after an earlier one has consumed them.
func (s *scanner) bindMode(md Mode) {
	if len(md.Layers) > 0 {
		s.bindLayers(md)
		return
	}
	bound := map[string]string{}
	consumed := map[string]string{}
	for _, ref := range md.Chain {
		for _, b := range s.src.switches[ref].bindings {
			if !reachable(b.key) {
				continue
			}
			if by, ok := consumed[b.key]; ok && !strings.HasPrefix(by, ref.Func+" ") {
				s.add(Detail{Metric: "conflicts", Key: b.key, Mode: md.Name, Where: b.where,
					Note: "shadowed by " + by + ", which handles it first"})
			}
			setFirst(bound, b.key, b.where)
			if b.consumes {
				setFirst(consumed, b.key, ref.Func+" ("+b.where+")")
			}
		}
	}
	s.bound[md.Name] = bound
}

// bindLayers records a key map mode's reachable keys and reports a key bound
// twice in one layer. An upper layer overriding a lower one is deliberate.
func (s *scanner) bindLayers(md Mode) {
	bound := map[string]string{}
	for _, layer := range md.Layers {
		inLayer := map[string]bool{}
		for _, b := range s.cfg.Table {
			if b.Mode != layer {
				continue
			}
			for _, k := range b.Keys {
				if !reachable(k) {
					continue
				}
				if inLayer[k] {
					s.add(Detail{Metric: "conflicts", Key: k, Mode: md.Name, Where: tableWhere(b),
						Note: "bound twice in key map mode " + layer})
				}
				inLayer[k] = true
				setFirst(bound, k, tableWhere(b))
			}
		}
	}
	s.bound[md.Name] = bound
}

// tableWhere names a key map binding for a finding.
func tableWhere(b tui.Binding) string {
	name := b.Label
	if name == "" && len(b.Keys) > 0 {
		name = strconv.Quote(b.Keys[0])
	}
	return fmt.Sprintf("keymap.go (%s %s)", b.Mode, name)
}

func setFirst(m map[string]string, k, v string) {
	if _, ok := m[k]; !ok {
		m[k] = v
	}
}

// unionBound merges the bound keys of the modes pick selects.
func (s *scanner) unionBound(pick func(Mode) bool) map[string]string {
	out := map[string]string{}
	for _, md := range s.cfg.Modes {
		if !pick(md) {
			continue
		}
		for k, w := range s.bound[md.Name] {
			if old, ok := out[k]; !ok || w < old {
				out[k] = w
			}
		}
	}
	return out
}

func (s *scanner) checkOverlay() {
	bound := s.unionBound(func(m Mode) bool { return m.Overlay })
	for _, k := range sortedKeys(bound) {
		if !s.overlayKeys[k] {
			s.add(Detail{Metric: "undocumented_bindings.overlay", Key: k, Where: bound[k],
				Note: "bound but not in the ? overlay"})
		}
	}
	for _, k := range sortedKeys(s.overlayKeys) {
		if _, ok := bound[k]; !ok {
			s.add(Detail{Metric: "documented_unbound.overlay", Key: k, Where: s.overlayWhere,
				Note: "the ? overlay names it; no tree or help-pane switch binds it"})
		}
	}
}

// hintDocs maps each key the mode's hints name to where the hint is.
func (s *scanner) hintDocs(md Mode) (map[string]string, error) {
	documented := map[string]string{}
	for _, h := range md.Hints {
		lits, where, err := s.hintText(md, h)
		if err != nil {
			return nil, fmt.Errorf("mode %s: %w", md.Name, err)
		}
		for _, lit := range lits {
			for _, k := range hintKeys(lit) {
				documented[k] = where
			}
		}
	}
	return documented, nil
}

// hintText is the hint text h selects and where it is.
func (s *scanner) hintText(md Mode, h HintRef) ([]string, string, error) {
	if h.Text != "" {
		return []string{h.Text}, "tui.Footer(" + md.Name + ")", nil
	}
	return s.src.hintLiterals(h)
}

func (s *scanner) checkHints(md Mode) error {
	documented, err := s.hintDocs(md)
	if err != nil {
		return err
	}
	bound := s.bound[md.Name]
	for _, k := range sortedKeys(documented) {
		if _, ok := bound[k]; !ok {
			s.add(Detail{Metric: "documented_unbound.hints", Key: k, Mode: md.Name, Where: documented[k],
				Note: "the hint names it; this mode does not bind it"})
		}
	}
	if md.Overlay {
		return nil // tree footers are a curated subset; the overlay is complete
	}
	for _, k := range sortedKeys(bound) {
		_, inHint := documented[k]
		// Ctrl+C cancels or quits everywhere, and the overlay says so.
		universal := k == "ctrl+c" && s.overlayKeys[k]
		if !inHint && !universal {
			s.add(Detail{Metric: "undocumented_bindings.dialogs", Key: k, Mode: md.Name, Where: bound[k],
				Note: "bound but not in this mode's hint"})
		}
	}
	return nil
}

func (s *scanner) checkPage(siteDir, page string) error {
	data, err := os.ReadFile(filepath.Join(siteDir, page)) //nolint:gosec // docs paths from the mode table
	if err != nil {
		return err
	}
	cells := keyTableCells(string(data))
	if len(cells) == 0 {
		return fmt.Errorf("%s: no key binding table found", page)
	}
	pageKeys := map[string]bool{}
	for _, c := range cells {
		if err := s.checkCell(page, c); err != nil {
			return err
		}
		for _, k := range c.keys {
			pageKeys[k] = true
		}
	}
	if page != s.cfg.Reference {
		return nil
	}
	refBound := s.unionBound(func(m Mode) bool { return m.Reference })
	for _, k := range sortedKeys(refBound) {
		if !pageKeys[k] {
			s.add(Detail{Metric: "undocumented_bindings.reference", Key: k, Where: refBound[k],
				Note: "bound but not in the website reference (" + page + ")"})
		}
	}
	return nil
}

// checkCell reports keys a website table cell names that its scheme (or,
// for a plain "Key" column, any reference mode) does not bind.
func (s *scanner) checkCell(page string, c tableCell) error {
	want := s.unionBound(func(m Mode) bool { return m.Reference && (c.scheme == "" || m.Scheme == c.scheme) })
	if len(want) == 0 {
		return fmt.Errorf("%s: column for scheme %q matches no mode", page, c.scheme)
	}
	note := "the website names it; nothing binds it"
	if c.scheme != "" {
		note = "the website names it for the " + c.scheme + " scheme; that scheme does not bind it"
	}
	for _, k := range c.keys {
		if _, ok := want[k]; !ok {
			s.add(Detail{Metric: "documented_unbound.website", Key: k, Mode: c.scheme,
				Where: fmt.Sprintf("%s:%d", page, c.line), Note: note})
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

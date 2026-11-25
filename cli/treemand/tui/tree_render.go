package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

// treeRenderer draws a treeRows in the configured display style. It reads
// rows and expansion state and never changes them, apart from clamping the
// cursor to the last row.
type treeRenderer struct {
	cfg       *config.Config
	cmdTokens []string // the command line being built; matching names and flags are highlighted
	focused   bool
}

// commandRowStyles draws a command row, one renderer per display style.
// A style missing from the map draws as StyleDefault.
var commandRowStyles = map[config.DisplayStyle]func(r *treeRenderer, row treeRow, expanded bool, maxW int) string{
	config.StyleDefault: (*treeRenderer).commandRowDefault,
	config.StyleColumns: (*treeRenderer).commandRowColumns,
	config.StyleCompact: (*treeRenderer).commandRowCompact,
	config.StyleGraph:   (*treeRenderer).commandRowGraph,
}

// view draws the visible rows of t inside a rounded border of w by h cells.
func (r *treeRenderer) view(t *treeRows, w, h int) string {
	borderColor := lipgloss.Color("#555555")
	if r.focused {
		borderColor = lipgloss.Color("#5EA4F5")
	}
	innerW := max(w-4, 1)
	innerH := max(h-2, 1)

	lines := make([]string, 0, innerH)
	for i, row := range t.visible(innerH) {
		// One row is one line: a row wider than the pane would wrap inside the
		// box and push the frame past the terminal's height (#212).
		line := r.row(t, row, t.offset+i == t.cursor, innerW)
		lines = append(lines, ansi.Truncate(line, innerW, "…"))
	}
	for len(lines) < innerH {
		lines = append(lines, "")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(w - 2).
		Height(h - 2).
		Render(strings.Join(lines, "\n"))
}

// row draws one row, highlighted when selected.
func (r *treeRenderer) row(t *treeRows, row treeRow, selected bool, maxW int) string {
	var line string
	switch row.kind {
	case rowKindCommand:
		draw, ok := commandRowStyles[r.cfg.TreeStyle]
		if !ok {
			draw = (*treeRenderer).commandRowDefault
		}
		line = draw(r, row, t.isNodeExpanded(row), maxW)
	case rowKindSection:
		line = r.sectionRow(row, t.isSectionExpanded(row.sectionKey, row.sectionDefault))
	case rowKindFlag:
		line = r.flagRow(row)
	case rowKindPositional:
		line = r.positionalRow(row)
	}
	return r.applySelection(line, selected, maxW)
}

// commandRowDefault is the baseline: icon, name, a description while
// collapsed and inline flag pills.
func (r *treeRenderer) commandRowDefault(row treeRow, expanded bool, maxW int) string {
	head := indent(row.depth) + r.expandIcon(row.node, expanded) + r.discoveryErrIndicator(row.node) + r.name(row)
	if expanded {
		return head
	}
	summary := r.flagSummary(row.node)
	return head + r.description(row.node.Description, maxW, lipgloss.Width(head)+lipgloss.Width(summary)) + summary
}

// commandRowColumns shows the name on the left and the description after a
// · separator, expanded or not.
func (r *treeRenderer) commandRowColumns(row treeRow, expanded bool, maxW int) string {
	head := indent(row.depth) + r.expandIcon(row.node, expanded) + r.discoveryErrIndicator(row.node) + r.name(row)
	return head + r.description(row.node.Description, maxW, lipgloss.Width(head))
}

// commandRowCompact draws the name only: no icons, no inline flags.
func (r *treeRenderer) commandRowCompact(row treeRow, _ bool, _ int) string {
	return indent(row.depth) + r.discoveryErrIndicator(row.node) + r.name(row)
}

// commandRowGraph draws classic tree connectors (├── / └──) and a count of
// the node's own flags.
func (r *treeRenderer) commandRowGraph(row treeRow, _ bool, _ int) string {
	prefix := ""
	switch {
	case row.depth == 0:
	case row.isLast:
		prefix = row.graphPrefix + "└── "
	default:
		prefix = row.graphPrefix + "├── "
	}
	hint := ""
	if n := len(ownFlags(row.node)); n > 0 {
		hint = faint.Render(fmt.Sprintf("  [%d flags]", n))
	}
	return faint.Render(prefix) + r.discoveryErrIndicator(row.node) + r.name(row) + hint
}

func (r *treeRenderer) sectionRow(row treeRow, expanded bool) string {
	icon := r.cfg.Icons.SectionCollapsed
	if expanded {
		icon = r.cfg.Icons.SectionExpanded
	}
	return lipgloss.NewStyle().Faint(true).Italic(true).Render(indent(row.depth) + icon + row.label())
}

func (r *treeRenderer) flagRow(row treeRow) string {
	f := row.flag
	nameStyle := r.flagColorStyle(f.ValueType)
	if isFlagActive(*f, r.cmdTokens) {
		nameStyle = nameStyle.Underline(true).Bold(true)
	}
	line := r.itemIndent(row.depth) + nameStyle.Render(f.Name)
	if r.compact() {
		return line
	}
	if f.ValueType != "" && f.ValueType != "bool" {
		line += faint.Render(" <" + f.ValueType + ">")
	}
	return line + itemDescription(f.Description)
}

func (r *treeRenderer) positionalRow(row treeRow) string {
	p := row.positional
	name := "<" + p.Name + ">"
	if !p.Required {
		name = "[" + p.Name + "]"
	}
	line := r.itemIndent(row.depth) + lipgloss.NewStyle().Foreground(lipgloss.Color(r.cfg.Colors.Pos)).Render(name)
	if r.compact() {
		return line
	}
	return line + itemDescription(p.Description)
}

// ---------- pieces shared by the styles ----------

var faint = lipgloss.NewStyle().Faint(true)

func indent(depth int) string { return strings.Repeat("  ", depth) }

func (r *treeRenderer) compact() bool { return r.cfg.TreeStyle == config.StyleCompact }

// itemIndent indents a flag or positional row; graph style indents wider to
// clear the connectors.
func (r *treeRenderer) itemIndent(depth int) string {
	if r.cfg.TreeStyle == config.StyleGraph {
		return strings.Repeat("    ", depth)
	}
	return indent(depth)
}

// expandIcon is the expanded or collapsed marker of a node that has anything
// to expand, or "" for a leaf.
func (r *treeRenderer) expandIcon(node *models.Node, expanded bool) string {
	if !hasContent(node) {
		return ""
	}
	if expanded {
		return r.cfg.Icons.Branch
	}
	return r.cfg.Icons.Collapsed
}

func hasContent(node *models.Node) bool {
	if len(node.Flags) > 0 || len(node.Positionals) > 0 {
		return true
	}
	for _, c := range node.Children {
		if !c.Virtual {
			return true
		}
	}
	return false
}

// name draws a command's name: bold at the root, and highlighted when it is
// on the command line being built.
func (r *treeRenderer) name(row treeRow) string {
	color := r.cfg.Colors.Base
	if row.depth > 0 {
		color = r.cfg.Colors.Subcmd
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(row.depth == 0)
	if r.matchesTokenPrefix(row.node) {
		style = style.Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	}
	return style.Render(row.node.Name)
}

// description is " · desc" cut to the width left after used cells, or ""
// when there is no description or too little room for one.
func (r *treeRenderer) description(desc string, maxW, used int) string {
	if desc == "" {
		return ""
	}
	sep := faint.Render("  ·  ")
	room := maxW - used - lipgloss.Width(sep) - 2
	if room <= 8 {
		return ""
	}
	if runes := []rune(desc); len(runes) > room {
		desc = string(runes[:room-1]) + "…"
	}
	return sep + faint.Render(desc)
}

// itemDescription is a flag's or positional's description, cut to 45 runes.
func itemDescription(desc string) string {
	if desc == "" {
		return ""
	}
	const maxDescLen = 45
	if runes := []rune(desc); len(runes) > maxDescLen {
		desc = string(runes[:maxDescLen-1]) + "…"
	}
	return "  " + faint.Render(desc)
}

// discoveryErrIndicator returns a styled ⚠ prefix when the node has a
// non-empty DiscoveryErr, or "" when the node is healthy.
func (r *treeRenderer) discoveryErrIndicator(node *models.Node) string {
	if node.DiscoveryErr == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(r.cfg.Colors.Invalid)).Render("⚠ ")
}

func ownFlags(node *models.Node) []models.Flag {
	var own []models.Flag
	for _, f := range node.Flags {
		if !f.Inherited {
			own = append(own, f)
		}
	}
	return own
}

// flagSpec is a flag as a pill shows it: the name, plus =<type> for a
// flag that takes a value.
func flagSpec(f models.Flag) string {
	if f.ValueType != "" && f.ValueType != "bool" {
		return f.Name + "=<" + f.ValueType + ">"
	}
	return f.Name
}

// flagSummary builds the inline flag pills of the default style: every own
// flag when there are a few, otherwise the active ones and a count.
func (r *treeRenderer) flagSummary(node *models.Node) string {
	own := ownFlags(node)
	if len(own) == 0 {
		return ""
	}
	const maxInlineFlags = 5
	active := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C")).Bold(true)
	var parts []string
	for _, f := range own {
		switch {
		case isFlagActive(f, r.cmdTokens):
			parts = append(parts, active.Render(flagSpec(f)))
		case len(own) <= maxInlineFlags:
			parts = append(parts, r.flagColorStyle(f.ValueType).Faint(true).Render(flagSpec(f)))
		}
	}
	if len(own) <= maxInlineFlags {
		return " " + faint.Render("[") + strings.Join(parts, faint.Render(",")) + faint.Render("]")
	}
	if len(parts) == 0 {
		return " " + faint.Render(fmt.Sprintf("[%d flags]", len(own)))
	}
	return " " + faint.Render("[") +
		strings.Join(parts, faint.Render(",")) + faint.Render(",") +
		faint.Render(fmt.Sprintf("…+%d flags", len(own)-len(parts))) +
		faint.Render("]")
}

// applySelection highlights a line if it is the selected row.
func (r *treeRenderer) applySelection(line string, selected bool, maxW int) string {
	if !selected {
		return line
	}
	selStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(r.cfg.Colors.Selected)).
		Foreground(lipgloss.Color(r.cfg.Colors.SelectedText)).
		Bold(true)
	if lineW := lipgloss.Width(line); lineW < maxW {
		line += strings.Repeat(" ", maxW-lineW)
	}
	return selStyle.Render(line)
}

func isFlagActive(f models.Flag, tokens []string) bool {
	longName := strings.TrimPrefix(f.Name, "--")
	for _, tok := range tokens {
		if strings.HasPrefix(tok, "--") {
			name := strings.TrimPrefix(tok, "--")
			if idx := strings.Index(name, "="); idx >= 0 {
				name = name[:idx]
			}
			if strings.EqualFold(name, longName) {
				return true
			}
		} else if strings.HasPrefix(tok, "-") && len(tok) == 2 && f.ShortName != "" {
			if strings.EqualFold(tok[1:], f.ShortName) {
				return true
			}
		}
	}
	return false
}

func (r *treeRenderer) matchesTokenPrefix(node *models.Node) bool {
	fp := node.FullPath
	if len(r.cmdTokens) == 0 || len(fp) > len(r.cmdTokens) {
		return false
	}
	for i, part := range fp {
		if !strings.EqualFold(part, r.cmdTokens[i]) {
			return false
		}
	}
	return true
}

func (r *treeRenderer) flagColorStyle(valueType string) lipgloss.Style {
	var hex string
	switch valueType {
	case "bool", "":
		hex = r.cfg.Colors.FlagBool
	case "string", "stringArray", "[]string":
		hex = r.cfg.Colors.FlagString
	case "int", "int64", "uint", "uint64", "count":
		hex = r.cfg.Colors.FlagInt
	default:
		hex = r.cfg.Colors.FlagOther
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
}

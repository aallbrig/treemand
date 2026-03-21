package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

// rowKind identifies the type of a tree row.
type rowKind int

const (
	rowKindCommand rowKind = iota
	rowKindSection
	rowKindFlag
	rowKindPositional
)

// treeRow is a flattened row for rendering.
type treeRow struct {
	kind  rowKind
	depth int

	// rowKindCommand
	node *models.Node

	// Graph-style rendering: set by flattenNode for StyleGraph.
	graphPrefix string // continuation prefix inherited from parent (spaces / "│ ")
	isLast      bool   // true when this is the last sibling at its level

	// rowKindSection
	sectionKey     string
	sectionLabel   string
	sectionDefault bool // default expanded state

	// rowKindFlag / rowKindPositional
	flag       *models.Flag
	positional *models.Positional
	owner      *models.Node
	ownerDepth int
	sectionRef string // key of the containing section
}

// SelectionKind identifies what is currently selected.
type SelectionKind int

const (
	SelCommand SelectionKind = iota
	SelFlag
	SelPositional
)

// Selection describes the currently selected tree item.
type Selection struct {
	Kind       SelectionKind
	Node       *models.Node
	Flag       *models.Flag
	Positional *models.Positional
	Owner      *models.Node // for SelFlag / SelPositional
}

// TreeModel manages the scrollable, filterable tree pane.
type TreeModel struct {
	root            *models.Node
	rows            []treeRow
	cursor          int
	offset          int
	filter          string
	nodeExpanded    map[string]bool
	sectionExpanded map[string]bool
	hideSections    bool // when true, section headers are hidden and all items shown flat
	cmdTokens       []string
	focused         bool
	cfg             *config.Config
	width           int
	height          int
}

func NewTreeModel(root *models.Node, cfg *config.Config) *TreeModel {
	t := &TreeModel{
		root:            root,
		nodeExpanded:    make(map[string]bool),
		sectionExpanded: make(map[string]bool),
		cfg:             cfg,
	}
	t.nodeExpanded[nodeKey(root, 0)] = true
	t.rebuild()
	return t
}

func (t *TreeModel) SetSize(w, h int) { t.width = w; t.height = h }

func (t *TreeModel) SetFilter(f string) {
	t.filter = f
	t.cursor = 0
	t.offset = 0
	t.rebuild()
}

func (t *TreeModel) SetCmdTokens(tokens []string) { t.cmdTokens = tokens }
func (t *TreeModel) SetFocused(f bool)            { t.focused = f }

// SetDisplayStyle changes the presentation variant and triggers a rebuild.
func (t *TreeModel) SetDisplayStyle(s config.DisplayStyle) {
	t.cfg.TreeStyle = s
	t.rebuild()
}

// SelectedItem returns the full Selection for the current cursor position.
func (t *TreeModel) SelectedItem() *Selection {
	if t.cursor >= len(t.rows) || len(t.rows) == 0 {
		return nil
	}
	row := t.rows[t.cursor]
	switch row.kind {
	case rowKindCommand:
		return &Selection{Kind: SelCommand, Node: row.node}
	case rowKindFlag:
		return &Selection{Kind: SelFlag, Flag: row.flag, Owner: row.owner}
	case rowKindPositional:
		return &Selection{Kind: SelPositional, Positional: row.positional, Owner: row.owner}
	}
	return nil
}

// Selected returns the *models.Node for the current selection (backwards compat).
// For flag/positional rows it returns the owning command node.
func (t *TreeModel) Selected() *models.Node {
	sel := t.SelectedItem()
	if sel == nil {
		return nil
	}
	switch sel.Kind {
	case SelCommand:
		return sel.Node
	case SelFlag, SelPositional:
		return sel.Owner
	}
	return nil
}

// SelectedDepth returns the depth of the currently selected command row.
func (t *TreeModel) SelectedDepth() int {
	if t.cursor >= len(t.rows) {
		return 0
	}
	return t.rows[t.cursor].depth
}

// Rebuild is a public alias for rebuild, used when callers mutate state externally.
func (t *TreeModel) Rebuild() { t.rebuild() }

func (t *TreeModel) Up() {
	pos := t.cursor - 1
	for pos >= 0 && t.rows[pos].kind == rowKindSection {
		pos--
	}
	if pos >= 0 {
		t.cursor = pos
		t.scrollIntoView()
	}
}

func (t *TreeModel) Down() {
	// Simply advance to the next non-section row. Never auto-expand anything.
	pos := t.cursor + 1
	for pos < len(t.rows) && t.rows[pos].kind == rowKindSection {
		pos++
	}
	if pos < len(t.rows) {
		t.cursor = pos
		t.scrollIntoView()
	}
}

// Right implements VS Code-style tree navigation:
//   - collapsed command node → expand it and stay on the node
//   - expanded command node → jump to the first command child (skip flags/positionals)
//   - flag / positional row → no-op
func (t *TreeModel) Right() {
	if t.cursor >= len(t.rows) {
		return
	}
	row := t.rows[t.cursor]
	if row.kind != rowKindCommand {
		return
	}
	key := nodeKey(row.node, row.depth)
	if !t.nodeExpanded[key] {
		// Step 1: expand and stay on this node.
		t.nodeExpanded[key] = true
		t.rebuild()
		for i, r := range t.rows {
			if r.kind == rowKindCommand && r.node == row.node && r.depth == row.depth {
				t.cursor = i
				break
			}
		}
		t.scrollIntoView()
		return
	}
	// Step 2: already expanded — jump to first child row.
	// Prefer a command child; fall back to first flag/positional (leaf nodes).
	firstChild := -1
	for pos := t.cursor + 1; pos < len(t.rows); pos++ {
		r := t.rows[pos]
		if r.depth <= row.depth {
			break // walked past children
		}
		if r.kind == rowKindCommand {
			t.cursor = pos
			t.scrollIntoView()
			return
		}
		if firstChild == -1 && r.kind != rowKindSection {
			firstChild = pos
		}
	}
	if firstChild >= 0 {
		t.cursor = firstChild
		t.scrollIntoView()
	}
}

// Left implements VS Code-style tree navigation:
//   - expanded command node → collapse it and stay on the node
//   - collapsed command node (or leaf) → jump to the parent command row
//   - flag / positional row → jump to the owner command row
func (t *TreeModel) Left() {
	if t.cursor >= len(t.rows) {
		return
	}
	row := t.rows[t.cursor]
	switch row.kind {
	case rowKindFlag, rowKindPositional:
		for i, r := range t.rows {
			if r.kind == rowKindCommand && r.node == row.owner && r.depth == row.ownerDepth {
				t.cursor = i
				t.scrollIntoView()
				return
			}
		}
	case rowKindCommand:
		key := nodeKey(row.node, row.depth)
		if t.nodeExpanded[key] {
			// Collapse and stay on this node.
			delete(t.nodeExpanded, key)
			t.rebuild()
			for i, r := range t.rows {
				if r.kind == rowKindCommand && r.node == row.node && r.depth == row.depth {
					t.cursor = i
					break
				}
			}
			t.scrollIntoView()
		} else {
			// Already collapsed: go to parent.
			// Try FullPath-based lookup first (exact), then fall back to
			// scanning backward for the nearest shallower command row.
			if row.depth == 0 {
				return // at root, nowhere to go
			}
			if len(row.node.FullPath) > 1 {
				parentPath := row.node.FullPath[:len(row.node.FullPath)-1]
				for i, r := range t.rows {
					if r.kind == rowKindCommand && pathsEqual(r.node.FullPath, parentPath) {
						t.cursor = i
						t.scrollIntoView()
						return
					}
				}
			}
			// Depth-based fallback: nearest command row above cursor at shallower depth.
			for i := t.cursor - 1; i >= 0; i-- {
				r := t.rows[i]
				if r.kind == rowKindCommand && r.depth < row.depth {
					t.cursor = i
					t.scrollIntoView()
					return
				}
			}
		}
	}
}

// Expand is kept as an alias for Right for backward compatibility.
func (t *TreeModel) Expand() { t.Right() }

// Collapse is kept as an alias for Left for backward compatibility.
func (t *TreeModel) Collapse() { t.Left() }

// ToggleExpand toggles the current command node's expansion (Space key).
func (t *TreeModel) ToggleExpand() {
	if t.cursor >= len(t.rows) {
		return
	}
	row := t.rows[t.cursor]
	if row.kind != rowKindCommand {
		return
	}
	key := nodeKey(row.node, row.depth)
	if t.nodeExpanded[key] {
		delete(t.nodeExpanded, key)
	} else {
		t.nodeExpanded[key] = true
	}
	t.rebuild()
}

// ExpandAllFrom recursively expands the given node and all its descendants.
func (t *TreeModel) ExpandAllFrom(node *models.Node, depth int) {
	t.nodeExpanded[nodeKey(node, depth)] = true
	for _, c := range node.Children {
		if !c.Virtual {
			t.ExpandAllFrom(c, depth+1)
		}
	}
}

// ExpandAll expands every node in the tree starting from the root.
func (t *TreeModel) ExpandAll() {
	t.ExpandAllFrom(t.root, 0)
	t.rebuild()
}

// CollapseAll collapses every node in the tree, leaving only the root row visible.
func (t *TreeModel) CollapseAll() {
	t.nodeExpanded = make(map[string]bool)
	t.sectionExpanded = make(map[string]bool)
	t.cursor = 0
	t.offset = 0
	t.rebuild()
}

// ToggleSections shows or hides section header rows (Sub commands, Flags, Inherited flags).
// When hidden, all child items are shown flat without grouping headers.
func (t *TreeModel) ToggleSections() {
	t.hideSections = !t.hideSections
	t.rebuild()
}

// SectionsHidden reports whether section headers are currently hidden.
func (t *TreeModel) SectionsHidden() bool { return t.hideSections }

// IsAtRoot reports whether the cursor is currently on the root command row.
func (t *TreeModel) IsAtRoot() bool {
	if t.cursor >= len(t.rows) {
		return true
	}
	row := t.rows[t.cursor]
	return row.kind == rowKindCommand && row.depth == 0
}

// CollapseSubtree recursively collapses a node and all its descendants.
// Call Rebuild() afterward to refresh the visible rows.
func (t *TreeModel) CollapseSubtree(node *models.Node, depth int) {
	delete(t.nodeExpanded, nodeKey(node, depth))
	for _, c := range node.Children {
		if !c.Virtual {
			t.CollapseSubtree(c, depth+1)
		}
	}
}

// ToggleSectionAtY toggles the section row at content y-coordinate y (0-based inside content area).
func (t *TreeModel) ToggleSectionAtY(y int) {
	contentIdx := t.offset + y
	if contentIdx < 0 || contentIdx >= len(t.rows) {
		return
	}
	row := t.rows[contentIdx]
	if row.kind != rowKindSection {
		return
	}
	current := t.isSectionExpanded(row.sectionKey, row.sectionDefault)
	t.sectionExpanded[row.sectionKey] = !current
	t.rebuild()
}

// PatchNode replaces a stub node's children with discovered children and clears
// the Stub flag. The node is matched by pointer identity.
func (t *TreeModel) PatchNode(stub *models.Node, discovered *models.Node) {
	if stub == nil || discovered == nil {
		return
	}
	stub.Stub = false
	stub.Children = discovered.Children
	if stub.Description == "" {
		stub.Description = discovered.Description
	}
	if len(stub.Flags) == 0 {
		stub.Flags = discovered.Flags
	}
	// Auto-expand the freshly-discovered node.
	key := t.findNodeKey(stub)
	if key != "" {
		t.nodeExpanded[key] = true
	}
	t.rebuild()
}

// findNodeKey locates the nodeKey for a given node pointer within the current rows.
func (t *TreeModel) findNodeKey(target *models.Node) string {
	for _, row := range t.rows {
		if row.kind == rowKindCommand && row.node == target {
			return nodeKey(row.node, row.depth)
		}
	}
	return ""
}

func (t *TreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return t, nil }
func (t *TreeModel) Init() tea.Cmd                           { return nil }
func (t *TreeModel) View() string                            { return t.ViewSized(t.width, t.height) }

// RowCount returns the number of tree rows currently visible (after filtering).
// Exported for testing.
func (t *TreeModel) RowCount() int { return len(t.rows) }

func (t *TreeModel) ViewSized(w, h int) string {
	t.width = w
	t.height = h

	borderColor := lipgloss.Color("#555555")
	if t.focused {
		borderColor = lipgloss.Color("#5EA4F5")
	}

	innerW := w - 4
	if innerW < 1 {
		innerW = 1
	}
	innerH := h - 2
	if innerH < 1 {
		innerH = 1
	}

	// Clamp cursor
	if t.cursor >= len(t.rows) && len(t.rows) > 0 {
		t.cursor = len(t.rows) - 1
	}

	var lines []string
	end := t.offset + innerH
	if end > len(t.rows) {
		end = len(t.rows)
	}
	for i := t.offset; i < end; i++ {
		line := t.renderRow(t.rows[i], i == t.cursor, innerW)
		lines = append(lines, line)
	}
	for len(lines) < innerH {
		lines = append(lines, "")
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(w - 2).
		Height(h - 2)

	return boxStyle.Render(strings.Join(lines, "\n"))
}

// ---------- rendering ----------

func (t *TreeModel) renderRow(row treeRow, selected bool, maxW int) string {
	switch row.kind {
	case rowKindCommand:
		return t.renderCommandRow(row, selected, maxW)
	case rowKindSection:
		return t.renderSectionRow(row)
	case rowKindFlag:
		return t.renderFlagRow(row, selected, maxW)
	case rowKindPositional:
		return t.renderPositionalRow(row, selected, maxW)
	}
	return ""
}

func (t *TreeModel) renderCommandRow(row treeRow, selected bool, maxW int) string {
	switch t.cfg.TreeStyle {
	case config.StyleColumns:
		return t.renderCommandRowColumns(row, selected, maxW)
	case config.StyleCompact:
		return t.renderCommandRowCompact(row, selected, maxW)
	case config.StyleGraph:
		return t.renderCommandRowGraph(row, selected, maxW)
	default:
		return t.renderCommandRowDefault(row, selected, maxW)
	}
}

// renderCommandRowDefault is the baseline: icon + name + inline flag pills.
func (t *TreeModel) renderCommandRowDefault(row treeRow, selected bool, maxW int) string {
	indent := strings.Repeat("  ", row.depth)
	key := nodeKey(row.node, row.depth)
	isExpanded := t.nodeExpanded[key]

	hasContent := len(row.node.Flags) > 0 || len(row.node.Positionals) > 0
	for _, c := range row.node.Children {
		if !c.Virtual {
			hasContent = true
			break
		}
	}

	icon := ""
	if hasContent {
		if isExpanded {
			icon = t.cfg.Icons.Branch
		} else {
			icon = t.cfg.Icons.Collapsed
		}
	}

	nameColor := lipgloss.Color(t.cfg.Colors.Base)
	if row.depth > 0 {
		nameColor = lipgloss.Color(t.cfg.Colors.Subcmd)
	}
	nameStyle := lipgloss.NewStyle().Foreground(nameColor)
	if row.depth == 0 {
		nameStyle = nameStyle.Bold(true)
	}
	if t.matchesTokenPrefix(row.node) {
		nameStyle = nameStyle.Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	}
	name := nameStyle.Render(row.node.Name)
	summary := t.buildFlagSummary(row, isExpanded)
	line := indent + icon + name + summary
	return t.applySelection(line, selected, maxW)
}

// renderCommandRowColumns shows name on the left and description after a · separator.
func (t *TreeModel) renderCommandRowColumns(row treeRow, selected bool, maxW int) string {
	indent := strings.Repeat("  ", row.depth)
	key := nodeKey(row.node, row.depth)
	isExpanded := t.nodeExpanded[key]

	hasContent := len(row.node.Flags) > 0 || len(row.node.Positionals) > 0
	for _, c := range row.node.Children {
		if !c.Virtual {
			hasContent = true
			break
		}
	}
	icon := ""
	if hasContent {
		if isExpanded {
			icon = t.cfg.Icons.Branch
		} else {
			icon = t.cfg.Icons.Collapsed
		}
	}

	nameColor := lipgloss.Color(t.cfg.Colors.Base)
	if row.depth > 0 {
		nameColor = lipgloss.Color(t.cfg.Colors.Subcmd)
	}
	nameStyle := lipgloss.NewStyle().Foreground(nameColor)
	if row.depth == 0 {
		nameStyle = nameStyle.Bold(true)
	}
	if t.matchesTokenPrefix(row.node) {
		nameStyle = nameStyle.Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	}
	name := nameStyle.Render(row.node.Name)

	// Build description part: truncate to fit available space.
	descPart := ""
	if row.node.Description != "" {
		sep := lipgloss.NewStyle().Faint(true).Render("  ·  ")
		maxDesc := maxW - lipgloss.Width(indent+icon+name) - lipgloss.Width(sep) - 2
		desc := row.node.Description
		if maxDesc > 8 {
			runes := []rune(desc)
			if len(runes) > maxDesc {
				desc = string(runes[:maxDesc-1]) + "…"
			}
			descPart = sep + lipgloss.NewStyle().Faint(true).Render(desc)
		}
	}

	line := indent + icon + name + descPart
	return t.applySelection(line, selected, maxW)
}

// renderCommandRowCompact renders name only — no icons, no inline flags.
func (t *TreeModel) renderCommandRowCompact(row treeRow, selected bool, maxW int) string {
	indent := strings.Repeat("  ", row.depth)

	nameColor := lipgloss.Color(t.cfg.Colors.Base)
	if row.depth > 0 {
		nameColor = lipgloss.Color(t.cfg.Colors.Subcmd)
	}
	nameStyle := lipgloss.NewStyle().Foreground(nameColor)
	if row.depth == 0 {
		nameStyle = nameStyle.Bold(true)
	}
	if t.matchesTokenPrefix(row.node) {
		nameStyle = nameStyle.Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	}
	line := indent + nameStyle.Render(row.node.Name)
	return t.applySelection(line, selected, maxW)
}

// renderCommandRowGraph renders classic tree connectors (├── / └──).
func (t *TreeModel) renderCommandRowGraph(row treeRow, selected bool, maxW int) string {
	var prefix string
	if row.depth == 0 {
		prefix = ""
	} else if row.isLast {
		prefix = row.graphPrefix + "└── "
	} else {
		prefix = row.graphPrefix + "├── "
	}
	prefix = lipgloss.NewStyle().Faint(true).Render(prefix)

	nameColor := lipgloss.Color(t.cfg.Colors.Base)
	if row.depth > 0 {
		nameColor = lipgloss.Color(t.cfg.Colors.Subcmd)
	}
	nameStyle := lipgloss.NewStyle().Foreground(nameColor)
	if row.depth == 0 {
		nameStyle = nameStyle.Bold(true)
	}
	if t.matchesTokenPrefix(row.node) {
		nameStyle = nameStyle.Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	}
	name := nameStyle.Render(row.node.Name)

	// Show flag count hint when node has own flags.
	hint := ""
	var ownFlags []models.Flag
	for _, f := range row.node.Flags {
		if !f.Inherited {
			ownFlags = append(ownFlags, f)
		}
	}
	if len(ownFlags) > 0 {
		hint = lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("  [%d flags]", len(ownFlags)))
	}

	line := prefix + name + hint
	return t.applySelection(line, selected, maxW)
}

// buildFlagSummary builds the inline flag pill string for the default style.
func (t *TreeModel) buildFlagSummary(row treeRow, isExpanded bool) string {
	if isExpanded {
		return ""
	}
	var ownFlags []models.Flag
	for _, f := range row.node.Flags {
		if !f.Inherited {
			ownFlags = append(ownFlags, f)
		}
	}
	if len(ownFlags) == 0 {
		return ""
	}
	const maxInlineFlags = 5
	bracketStyle := lipgloss.NewStyle().Faint(true)
	dimStyle := lipgloss.NewStyle().Faint(true)
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C")).Bold(true)
	if len(ownFlags) > maxInlineFlags {
		var activeParts []string
		for _, f := range ownFlags {
			if isFlagActive(f, t.cmdTokens) {
				fs := f.Name
				if f.ValueType != "" && f.ValueType != "bool" {
					fs += "=<" + f.ValueType + ">"
				}
				activeParts = append(activeParts, activeStyle.Render(fs))
			}
		}
		if len(activeParts) > 0 {
			return " " + bracketStyle.Render("[") +
				strings.Join(activeParts, bracketStyle.Render(",")) + bracketStyle.Render(",") +
				dimStyle.Render(fmt.Sprintf("…+%d flags", len(ownFlags)-len(activeParts))) +
				bracketStyle.Render("]")
		}
		return " " + dimStyle.Render(fmt.Sprintf("[%d flags]", len(ownFlags)))
	}
	var flagParts []string
	for _, f := range ownFlags {
		fs := f.Name
		if f.ValueType != "" && f.ValueType != "bool" {
			fs += "=<" + f.ValueType + ">"
		}
		if isFlagActive(f, t.cmdTokens) {
			flagParts = append(flagParts, activeStyle.Render(fs))
		} else {
			flagParts = append(flagParts, t.flagColorStyle(f.ValueType).Faint(true).Render(fs))
		}
	}
	return " " + bracketStyle.Render("[") +
		strings.Join(flagParts, bracketStyle.Render(",")) +
		bracketStyle.Render("]")
}

// applySelection highlights a line if it is the selected row.
func (t *TreeModel) applySelection(line string, selected bool, maxW int) string {
	if !selected {
		return line
	}
	selStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(t.cfg.Colors.Selected)).
		Foreground(lipgloss.Color(t.cfg.Colors.SelectedText)).
		Bold(true)
	lineW := lipgloss.Width(line)
	if lineW < maxW {
		line += strings.Repeat(" ", maxW-lineW)
	}
	return selStyle.Render(line)
}

func (t *TreeModel) renderSectionRow(row treeRow) string {
	indent := strings.Repeat("  ", row.depth)
	expanded := t.isSectionExpanded(row.sectionKey, row.sectionDefault)
	icon := t.cfg.Icons.SectionCollapsed
	if expanded {
		icon = t.cfg.Icons.SectionExpanded
	}
	dimStyle := lipgloss.NewStyle().Faint(true).Italic(true)
	return dimStyle.Render(indent + icon + row.sectionLabel)
}

func (t *TreeModel) renderFlagRow(row treeRow, selected bool, maxW int) string {
	f := row.flag
	compact := t.cfg.TreeStyle == config.StyleCompact

	var indent string
	if t.cfg.TreeStyle == config.StyleGraph {
		indent = strings.Repeat("    ", row.depth)
	} else {
		indent = strings.Repeat("  ", row.depth)
	}

	typeHint := ""
	if !compact && f.ValueType != "" && f.ValueType != "bool" {
		typeHint = " <" + f.ValueType + ">"
	}

	nameStyle := t.flagColorStyle(f.ValueType)
	if isFlagActive(*f, t.cmdTokens) {
		nameStyle = nameStyle.Underline(true).Bold(true)
	}

	namePart := nameStyle.Render(f.Name)
	typePart := ""
	if typeHint != "" {
		typePart = lipgloss.NewStyle().Faint(true).Render(typeHint)
	}
	descPart := ""
	if !compact && f.Description != "" {
		const maxDescLen = 45
		desc := f.Description
		if len([]rune(desc)) > maxDescLen {
			desc = string([]rune(desc)[:maxDescLen-1]) + "…"
		}
		descPart = "  " + lipgloss.NewStyle().Faint(true).Render(desc)
	}

	line := indent + namePart + typePart + descPart
	return t.applySelection(line, selected, maxW)
}

func (t *TreeModel) renderPositionalRow(row treeRow, selected bool, maxW int) string {
	compact := t.cfg.TreeStyle == config.StyleCompact
	var indent string
	if t.cfg.TreeStyle == config.StyleGraph {
		indent = strings.Repeat("    ", row.depth)
	} else {
		indent = strings.Repeat("  ", row.depth)
	}
	p := row.positional

	posStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.cfg.Colors.Pos))

	nameStr := "<" + p.Name + ">"
	if !p.Required {
		nameStr = "[" + p.Name + "]"
	}

	namePart := posStyle.Render(nameStr)
	descPart := ""
	if !compact && p.Description != "" {
		const maxDescLen = 45
		desc := p.Description
		if len([]rune(desc)) > maxDescLen {
			desc = string([]rune(desc)[:maxDescLen-1]) + "…"
		}
		descPart = "  " + lipgloss.NewStyle().Faint(true).Render(desc)
	}

	line := indent + namePart + descPart
	return t.applySelection(line, selected, maxW)
}

// ---------- rebuild ----------

func (t *TreeModel) rebuild() {
	t.rows = nil
	t.flattenNode(t.root, 0, "", true)
	t.adjustCursorOffSection()
}

func (t *TreeModel) flattenNode(node *models.Node, depth int, graphPrefix string, isLast bool) {
	if node.Virtual {
		return
	}

	key := nodeKey(node, depth)
	expanded := t.nodeExpanded[key]

	// Collect visible (non-virtual) children up front — needed by filter logic.
	var visChildren []*models.Node
	for _, c := range node.Children {
		if !c.Virtual {
			visChildren = append(visChildren, c)
		}
	}

	filtering := t.filter != ""

	// When filtering: show this node if it directly matches OR any descendant
	// matches (so ancestors act as context breadcrumbs). Always recurse into
	// children regardless of expanded state so the full tree is searched.
	if filtering {
		if matchesFilter(node, t.filter) || nodeOrDescendantMatchesFilter(node, t.filter) {
			t.rows = append(t.rows, treeRow{
				kind:  rowKindCommand,
				depth: depth,
				node:  node,
			})
		}
		for _, c := range visChildren {
			t.flattenNode(c, depth+1, "", true)
		}
		return
	}

	// Normal (non-filtered) path: add row then stop if not expanded.
	t.rows = append(t.rows, treeRow{
		kind:        rowKindCommand,
		depth:       depth,
		node:        node,
		graphPrefix: graphPrefix,
		isLast:      isLast,
	})

	if !expanded {
		return
	}

	// Sub commands section.
	if len(visChildren) > 0 {
		sKey := key + "/subcommands"
		subDefault := len(visChildren) <= 6
		subExpanded := t.hideSections || t.isSectionExpanded(sKey, subDefault)
		if !t.hideSections {
			t.rows = append(t.rows, treeRow{
				kind:           rowKindSection,
				depth:          depth + 1,
				sectionKey:     sKey,
				sectionLabel:   fmt.Sprintf("Sub commands (%d)", len(visChildren)),
				sectionDefault: subDefault,
			})
		}
		if subExpanded {
			childGraphPrefix := graphPrefix
			if depth == 0 {
				childGraphPrefix = ""
			} else if isLast {
				childGraphPrefix = graphPrefix + "    "
			} else {
				childGraphPrefix = graphPrefix + "│   "
			}
			for i, c := range visChildren {
				t.flattenNode(c, depth+1, childGraphPrefix, i == len(visChildren)-1)
			}
		}
	}

	// Flags section — own (non-inherited) flags only.
	var ownFlags, inheritedFlags []int // indices into node.Flags
	for i := range node.Flags {
		if node.Flags[i].Inherited {
			inheritedFlags = append(inheritedFlags, i)
		} else {
			ownFlags = append(ownFlags, i)
		}
	}
	if len(ownFlags) > 0 {
		sKey := key + "/flags"
		flagExpanded := t.hideSections || t.isSectionExpanded(sKey, false)
		if !t.hideSections {
			t.rows = append(t.rows, treeRow{
				kind:           rowKindSection,
				depth:          depth + 1,
				sectionKey:     sKey,
				sectionLabel:   fmt.Sprintf("Flags (%d)", len(ownFlags)),
				sectionDefault: false,
			})
		}
		if flagExpanded {
			for _, i := range ownFlags {
				t.rows = append(t.rows, treeRow{
					kind:       rowKindFlag,
					depth:      depth + 2,
					flag:       &node.Flags[i],
					owner:      node,
					ownerDepth: depth,
					sectionRef: sKey,
				})
			}
		}
	}
	if len(inheritedFlags) > 0 {
		sKey := key + "/inherited"
		inhExpanded := t.hideSections || t.isSectionExpanded(sKey, false)
		if !t.hideSections {
			t.rows = append(t.rows, treeRow{
				kind:           rowKindSection,
				depth:          depth + 1,
				sectionKey:     sKey,
				sectionLabel:   fmt.Sprintf("Inherited flags (%d)", len(inheritedFlags)),
				sectionDefault: false,
			})
		}
		if inhExpanded {
			for _, i := range inheritedFlags {
				t.rows = append(t.rows, treeRow{
					kind:       rowKindFlag,
					depth:      depth + 2,
					flag:       &node.Flags[i],
					owner:      node,
					ownerDepth: depth,
					sectionRef: sKey,
				})
			}
		}
	}

	// Positionals section.
	if len(node.Positionals) > 0 {
		sKey := key + "/positionals"
		posExpanded := t.hideSections || t.isSectionExpanded(sKey, false)
		if !t.hideSections {
			t.rows = append(t.rows, treeRow{
				kind:           rowKindSection,
				depth:          depth + 1,
				sectionKey:     sKey,
				sectionLabel:   fmt.Sprintf("Positional arguments (%d)", len(node.Positionals)),
				sectionDefault: false,
			})
		}
		if posExpanded {
			for i := range node.Positionals {
				t.rows = append(t.rows, treeRow{
					kind:       rowKindPositional,
					depth:      depth + 2,
					positional: &node.Positionals[i],
					owner:      node,
					ownerDepth: depth,
					sectionRef: sKey,
				})
			}
		}
	}
}

// adjustCursorOffSection ensures the cursor is never resting on a section row.
func (t *TreeModel) adjustCursorOffSection() {
	if len(t.rows) == 0 {
		t.cursor = 0
		return
	}
	if t.cursor >= len(t.rows) {
		t.cursor = len(t.rows) - 1
	}
	// Scan forward.
	for t.cursor < len(t.rows) && t.rows[t.cursor].kind == rowKindSection {
		t.cursor++
	}
	// If past end, scan backward.
	if t.cursor >= len(t.rows) {
		t.cursor = len(t.rows) - 1
		for t.cursor > 0 && t.rows[t.cursor].kind == rowKindSection {
			t.cursor--
		}
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

func (t *TreeModel) isSectionExpanded(key string, defaultVal bool) bool {
	if v, ok := t.sectionExpanded[key]; ok {
		return v
	}
	return defaultVal
}

func (t *TreeModel) scrollIntoView() {
	innerH := t.height - 2
	if innerH < 1 {
		innerH = 1
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+innerH {
		t.offset = t.cursor - innerH + 1
	}
}

// ---------- helpers ----------

func nodeKey(n *models.Node, depth int) string {
	return fmt.Sprintf("%s@%d", strings.Join(n.FullPath, "/"), depth)
}

func pathsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

func matchesFilter(node *models.Node, filter string) bool {
	return strings.Contains(strings.ToLower(node.Name), strings.ToLower(filter))
}

// nodeOrDescendantMatchesFilter returns true if any non-virtual descendant of
// node matches the filter. Used to keep ancestor rows visible as breadcrumbs.
func nodeOrDescendantMatchesFilter(node *models.Node, filter string) bool {
	for _, c := range node.Children {
		if c.Virtual {
			continue
		}
		if matchesFilter(c, filter) || nodeOrDescendantMatchesFilter(c, filter) {
			return true
		}
	}
	return false
}

func (t *TreeModel) matchesTokenPrefix(node *models.Node) bool {
	if len(t.cmdTokens) == 0 {
		return false
	}
	fp := node.FullPath
	if len(fp) > len(t.cmdTokens) {
		return false
	}
	for i, part := range fp {
		if !strings.EqualFold(part, t.cmdTokens[i]) {
			return false
		}
	}
	return true
}

func (t *TreeModel) flagColorStyle(valueType string) lipgloss.Style {
	var hex string
	switch valueType {
	case "bool", "":
		hex = t.cfg.Colors.FlagBool
	case "string", "stringArray", "[]string":
		hex = t.cfg.Colors.FlagString
	case "int", "int64", "uint", "uint64", "count":
		hex = t.cfg.Colors.FlagInt
	default:
		hex = t.cfg.Colors.FlagOther
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
}

package tui

import (
	"strconv"
	"strings"

	"github.com/aallbrig/treemand/fuzzy"
	"github.com/aallbrig/treemand/models"
)

// treeRows is the tree pane's row model: it flattens the command tree into
// rows and owns expansion, the filter, hidden sections, the cursor and
// scrolling. It knows nothing about how a row looks; treeRenderer draws it.
type treeRows struct {
	root            *models.Node
	rows            []treeRow
	cursor          int
	offset          int
	filter          string
	nodeExpanded    map[string]bool
	sectionExpanded map[string]bool
	// keys memoizes nodeKey strings: a rebuild of a large tree asks for every
	// row's keys again, and building them was most of expand-all's allocations.
	keys         map[keyID]string
	hideSections bool // when true, section headers are hidden and all items shown flat
	height       int  // pane height, border included; scrolling keeps the cursor inside it
	patches      int  // nodes filled in by discovery since the tree was built
}

// newTreeRows returns the rows of root with only the root expanded.
func newTreeRows(root *models.Node) *treeRows {
	t := &treeRows{
		root:            root,
		nodeExpanded:    make(map[string]bool),
		sectionExpanded: make(map[string]bool),
	}
	t.nodeExpanded[nodeKey(root, 0)] = true
	t.rebuild()
	return t
}

// isNodeExpanded reports whether a command row is expanded.
func (t *treeRows) isNodeExpanded(row treeRow) bool {
	return t.nodeExpanded[nodeKey(row.node, row.depth)]
}

// visible returns the rows the viewport shows for n content lines, clamping
// the cursor to the last row first.
func (t *treeRows) visible(n int) []treeRow {
	if t.cursor >= len(t.rows) && len(t.rows) > 0 {
		t.cursor = len(t.rows) - 1
	}
	end := min(t.offset+n, len(t.rows))
	if t.offset >= end {
		return nil
	}
	return t.rows[t.offset:end]
}

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
	sectionName    string // "Flags"; label() adds the count
	sectionCount   int
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

func (t *treeRows) SetFilter(f string) {
	t.filter = f
	t.cursor = 0
	t.offset = 0
	t.rebuild()
}

// SelectedItem returns the full Selection for the current cursor position.
func (t *treeRows) SelectedItem() *Selection {
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
func (t *treeRows) Selected() *models.Node {
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

// SelectedOrOwner returns the command node at or owning the current cursor
// position. Unlike Selected(), this also works when the cursor is on a
// section row by scanning backward to find the nearest parent command.
func (t *treeRows) SelectedOrOwner() *models.Node {
	if n := t.Selected(); n != nil {
		return n
	}
	if t.cursor >= len(t.rows) {
		return nil
	}
	row := t.rows[t.cursor]
	// Section rows: scan backward for the nearest command at a shallower depth.
	if row.kind == rowKindSection {
		for i := t.cursor - 1; i >= 0; i-- {
			if t.rows[i].kind == rowKindCommand && t.rows[i].depth < row.depth {
				return t.rows[i].node
			}
		}
	}
	return nil
}

// SelectedCommandDepth returns the depth of the command node that owns the
// current cursor position. For command rows this is the row's own depth;
// for section/flag/positional rows it's the owning command's depth.
func (t *treeRows) SelectedCommandDepth() int {
	if t.cursor >= len(t.rows) {
		return 0
	}
	row := t.rows[t.cursor]
	if row.kind == rowKindCommand {
		return row.depth
	}
	if row.kind == rowKindFlag || row.kind == rowKindPositional {
		return row.ownerDepth
	}
	// Section row: scan backward.
	for i := t.cursor - 1; i >= 0; i-- {
		if t.rows[i].kind == rowKindCommand && t.rows[i].depth < row.depth {
			return t.rows[i].depth
		}
	}
	return 0
}

// Rebuild is a public alias for rebuild, used when callers mutate state externally.
func (t *treeRows) Rebuild() { t.rebuild() }

func (t *treeRows) Up() {
	if t.cursor > 0 {
		t.cursor--
		t.scrollIntoView()
	}
}

func (t *treeRows) Down() {
	if t.cursor < len(t.rows)-1 {
		t.cursor++
		t.scrollIntoView()
	}
}

// Top jumps the cursor to the first row.
func (t *treeRows) Top() {
	t.cursor = 0
	t.scrollIntoView()
}

// Bottom jumps the cursor to the last row.
// pageStep is how many rows a page key moves: a screenful less one, so the row
// you were on stays in view as context.
func (t *treeRows) pageStep() int {
	if step := t.height - 2 - 1; step > 1 {
		return step
	}
	return 1
}

// PageDown moves the cursor down by a screenful, stopping at the last row.
func (t *treeRows) PageDown() {
	if len(t.rows) == 0 {
		return
	}
	t.cursor = min(t.cursor+t.pageStep(), len(t.rows)-1)
	t.scrollIntoView()
}

// PageUp moves the cursor up by a screenful, stopping at the first row.
func (t *treeRows) PageUp() {
	t.cursor = max(t.cursor-t.pageStep(), 0)
	t.scrollIntoView()
}

func (t *treeRows) Bottom() {
	if len(t.rows) > 0 {
		t.cursor = len(t.rows) - 1
		t.scrollIntoView()
	}
}

// NextMatch moves the cursor to the next row whose name contains the search
// string (case-insensitive). Wraps around to the beginning if needed.
// Returns true if a match was found.
func (t *treeRows) NextMatch(search string) bool {
	if search == "" || len(t.rows) == 0 {
		return false
	}
	s := strings.ToLower(search)
	for i := 1; i <= len(t.rows); i++ {
		idx := (t.cursor + i) % len(t.rows)
		if t.rowMatchesSearch(idx, s) {
			t.cursor = idx
			t.scrollIntoView()
			return true
		}
	}
	return false
}

// PrevMatch moves the cursor to the previous row whose name contains the
// search string (case-insensitive). Wraps around to the end if needed.
// Returns true if a match was found.
func (t *treeRows) PrevMatch(search string) bool {
	if search == "" || len(t.rows) == 0 {
		return false
	}
	s := strings.ToLower(search)
	for i := 1; i <= len(t.rows); i++ {
		idx := (t.cursor - i + len(t.rows)) % len(t.rows)
		if t.rowMatchesSearch(idx, s) {
			t.cursor = idx
			t.scrollIntoView()
			return true
		}
	}
	return false
}

func (t *treeRows) rowMatchesSearch(idx int, lowerSearch string) bool {
	row := t.rows[idx]
	switch row.kind {
	case rowKindCommand:
		return strings.Contains(strings.ToLower(row.node.Name), lowerSearch)
	case rowKindFlag:
		return strings.Contains(strings.ToLower(row.flag.Name), lowerSearch)
	case rowKindPositional:
		return strings.Contains(strings.ToLower(row.positional.Name), lowerSearch)
	case rowKindSection:
		return strings.Contains(strings.ToLower(row.label()), lowerSearch)
	}
	return false
}

// Right implements VS Code-style tree navigation:
//   - section header → expand it
//   - collapsed command node → expand it and stay on the node
//   - expanded command node → jump to its first child (see enterChildren)
//   - flag / positional row → no-op
func (t *treeRows) Right() {
	if t.cursor >= len(t.rows) {
		return
	}
	row := t.rows[t.cursor]
	switch row.kind {
	case rowKindSection:
		t.expandSection(row)
	case rowKindCommand:
		key := nodeKey(row.node, row.depth)
		if t.nodeExpanded[key] {
			t.enterChildren(row)
			return
		}
		t.nodeExpanded[key] = true
		t.rebuild()
		t.moveToCommandRow(row.node, row.depth)
	}
}

// enterChildren moves the cursor from an expanded command row into its
// children. Priority: the first command child, then the first flag or
// positional, then expanding the first collapsed section header.
func (t *treeRows) enterChildren(row treeRow) {
	firstCommand, firstChild, firstSection := t.scanChildren(row)
	switch {
	case firstCommand >= 0:
		t.moveCursor(firstCommand)
	case firstChild >= 0:
		t.moveCursor(firstChild)
	case firstSection >= 0:
		t.openFirstSection(row, firstSection)
	}
}

// scanChildren walks the rows below the cursor that belong to row and
// returns the position of the first command, the first flag or positional,
// and the first section header (-1 where there is none). The walk stops at
// the first command, since a command child wins over everything else.
func (t *treeRows) scanChildren(row treeRow) (firstCommand, firstChild, firstSection int) {
	firstChild, firstSection = -1, -1
	for pos := t.cursor + 1; pos < len(t.rows) && t.rows[pos].depth > row.depth; pos++ {
		switch r := t.rows[pos]; {
		case r.kind == rowKindCommand:
			return pos, firstChild, firstSection
		case r.kind == rowKindSection:
			if firstSection == -1 {
				firstSection = pos
			}
		case firstChild == -1:
			firstChild = pos
		}
	}
	return -1, firstChild, firstSection
}

// openFirstSection expands the section header at firstSection, the only
// children of row on screen, and lands on the first non-section row deeper
// than row, or on the header itself when there is none.
func (t *treeRows) openFirstSection(row treeRow, firstSection int) {
	t.sectionExpanded[t.rows[firstSection].sectionKey] = true
	t.rebuild()
	for pos, r := range t.rows {
		if r.kind != rowKindSection && r.depth > row.depth {
			t.moveCursor(pos)
			return
		}
	}
	t.moveCursor(firstSection)
}

// moveCursor puts the cursor on pos and scrolls it into view.
func (t *treeRows) moveCursor(pos int) {
	t.cursor = pos
	t.scrollIntoView()
}

// Left implements VS Code-style tree navigation:
//   - expanded command node → collapse it and stay on the node
//   - collapsed command node (or leaf) → jump to the parent command row
//   - flag / positional row → jump to the owner command row
func (t *treeRows) Left() {
	if t.cursor >= len(t.rows) {
		return
	}
	row := t.rows[t.cursor]
	switch row.kind {
	case rowKindSection:
		t.collapseSection(row)
	case rowKindFlag, rowKindPositional:
		t.moveToCommandRow(row.owner, row.ownerDepth)
	case rowKindCommand:
		key := nodeKey(row.node, row.depth)
		if t.nodeExpanded[key] {
			// Collapse and stay on this node.
			delete(t.nodeExpanded, key)
			t.rebuild()
			t.moveToCommandRow(row.node, row.depth)
		} else if row.depth > 0 {
			t.moveToParentRow(row)
		}
	}
}

// moveToCommandRow puts the cursor on node's command row at depth, if shown.
func (t *treeRows) moveToCommandRow(node *models.Node, depth int) {
	for i, r := range t.rows {
		if r.kind == rowKindCommand && r.node == node && r.depth == depth {
			t.cursor = i
			t.scrollIntoView()
			return
		}
	}
}

// moveToParentRow puts the cursor on the parent command of a collapsed
// command row: by FullPath first (exact), then the nearest shallower command
// row above the cursor.
func (t *treeRows) moveToParentRow(row treeRow) {
	if pos := t.commandRowByPath(parentPath(row.node)); pos >= 0 {
		t.cursor = pos
		t.scrollIntoView()
		return
	}
	for i := t.cursor - 1; i >= 0; i-- {
		r := t.rows[i]
		if r.kind == rowKindCommand && r.depth < row.depth {
			t.cursor = i
			t.scrollIntoView()
			return
		}
	}
}

// parentPath is node's FullPath without its last element, or nil when the
// node has no parent in its path.
func parentPath(node *models.Node) []string {
	if len(node.FullPath) < 2 {
		return nil
	}
	return node.FullPath[:len(node.FullPath)-1]
}

// commandRowByPath returns the index of the first command row whose node has
// the given FullPath, or -1 (always -1 for a nil path).
func (t *treeRows) commandRowByPath(path []string) int {
	if path == nil {
		return -1
	}
	for i, r := range t.rows {
		if r.kind == rowKindCommand && pathsEqual(r.node.FullPath, path) {
			return i
		}
	}
	return -1
}

// Expand is kept as an alias for Right for backward compatibility.
func (t *treeRows) Expand() { t.Right() }

// Collapse is kept as an alias for Left for backward compatibility.
func (t *treeRows) Collapse() { t.Left() }

// ToggleExpand toggles the current node's or section's expansion (Space key).
func (t *treeRows) ToggleExpand() {
	if t.cursor >= len(t.rows) {
		return
	}
	row := t.rows[t.cursor]
	switch row.kind {
	case rowKindSection:
		t.toggleSection(row)
	case rowKindCommand:
		key := nodeKey(row.node, row.depth)
		if t.nodeExpanded[key] {
			delete(t.nodeExpanded, key)
		} else {
			t.nodeExpanded[key] = true
		}
		t.rebuild()
	}
}

// ToggleSelectedSection toggles expansion of the section at the current cursor.
// Returns true if the cursor was on a section row (and it was toggled), false otherwise.
func (t *treeRows) ToggleSelectedSection() bool {
	if t.cursor >= len(t.rows) {
		return false
	}
	row := t.rows[t.cursor]
	if row.kind != rowKindSection {
		return false
	}
	t.toggleSection(row)
	return true
}

// toggleSection toggles a section row's expansion and rebuilds. Keeps the
// cursor on the section header after rebuild.
func (t *treeRows) toggleSection(row treeRow) {
	current := t.isSectionExpanded(row.sectionKey, row.sectionDefault)
	t.sectionExpanded[row.sectionKey] = !current
	t.rebuildKeepingSection(row.sectionKey)
}

// expandSection expands a section row (no-op if already expanded).
func (t *treeRows) expandSection(row treeRow) {
	if t.isSectionExpanded(row.sectionKey, row.sectionDefault) {
		// Already expanded — jump to the first content row inside this section.
		if pos := t.firstSectionChild(row); pos >= 0 {
			t.cursor = pos
			t.scrollIntoView()
		}
		return
	}
	t.sectionExpanded[row.sectionKey] = true
	t.rebuildKeepingSection(row.sectionKey)
}

// collapseSection collapses a section row (no-op if already collapsed).
func (t *treeRows) collapseSection(row treeRow) {
	if !t.isSectionExpanded(row.sectionKey, row.sectionDefault) {
		return
	}
	t.sectionExpanded[row.sectionKey] = false
	t.rebuildKeepingSection(row.sectionKey)
}

// rebuildKeepingSection rebuilds the row list and restores the cursor to
// the section row identified by key.
func (t *treeRows) rebuildKeepingSection(key string) {
	t.rebuild()
	for i, r := range t.rows {
		if r.kind == rowKindSection && r.sectionKey == key {
			t.cursor = i
			break
		}
	}
	t.scrollIntoView()
}

// firstSectionChild returns the index of the first content row (non-section)
// inside the given section, or -1 if none found.
func (t *treeRows) firstSectionChild(section treeRow) int {
	pos := t.cursor + 1
	if pos >= len(t.rows) {
		return -1
	}
	r := t.rows[pos]
	if r.kind == rowKindSection || r.depth < section.depth {
		return -1
	}
	return pos
}

// ExpandAllFrom recursively expands the given node and all its descendants.
func (t *treeRows) ExpandAllFrom(node *models.Node, depth int) {
	t.nodeExpanded[t.keyOf(node, depth, keyNode)] = true
	for _, c := range node.Children {
		if !c.Virtual {
			t.ExpandAllFrom(c, depth+1)
		}
	}
}

// ExpandAll expands every node and every section in the tree.
func (t *treeRows) ExpandAll() {
	t.ExpandAllFrom(t.root, 0)
	// Also expand all sections so flags/positionals are visible.
	t.expandAllSections(t.root, 0)
	t.reserveRows(estimateRows(t.root))
	t.rebuild()
}

// estimateRows is an upper bound on the rows a fully expanded tree needs: the
// node, one row per flag and positional, and a section header for each kind.
func estimateRows(n *models.Node) int {
	c := 1 + len(n.Flags) + len(n.Positionals) + 3
	for _, ch := range n.Children {
		if !ch.Virtual {
			c += estimateRows(ch)
		}
	}
	return c
}

// reserveRows makes room for n rows so a big rebuild appends without growing.
func (t *treeRows) reserveRows(n int) {
	if cap(t.rows) < n {
		t.rows = make([]treeRow, 0, n)
	}
}

// expandAllSections recursively marks every section under node as expanded.
func (t *treeRows) expandAllSections(node *models.Node, depth int) {
	if len(node.Children) > 0 {
		t.sectionExpanded[t.keyOf(node, depth, keySubcommands)] = true
	}
	var ownFlags, inheritedFlags int
	for _, f := range node.Flags {
		if f.Inherited {
			inheritedFlags++
		} else {
			ownFlags++
		}
	}
	if ownFlags > 0 {
		t.sectionExpanded[t.keyOf(node, depth, keyFlags)] = true
	}
	if inheritedFlags > 0 {
		t.sectionExpanded[t.keyOf(node, depth, keyInherited)] = true
	}
	if len(node.Positionals) > 0 {
		t.sectionExpanded[t.keyOf(node, depth, keyPositionals)] = true
	}
	for _, c := range node.Children {
		if !c.Virtual {
			t.expandAllSections(c, depth+1)
		}
	}
}

// CollapseAll collapses every node in the tree, leaving only the root row visible.
func (t *treeRows) CollapseAll() {
	t.nodeExpanded = make(map[string]bool)
	t.sectionExpanded = make(map[string]bool)
	t.cursor = 0
	t.offset = 0
	t.rebuild()
}

// ToggleSections shows or hides section header rows (Subcommands, Flags, Inherited flags).
// When hidden, all child items are shown flat without grouping headers.
func (t *treeRows) ToggleSections() {
	t.hideSections = !t.hideSections
	t.rebuild()
}

// SectionsHidden reports whether section headers are currently hidden.
func (t *treeRows) SectionsHidden() bool { return t.hideSections }

// CollapseSubtree recursively collapses a node and all its descendants.
// Call Rebuild() afterward to refresh the visible rows.
func (t *treeRows) CollapseSubtree(node *models.Node, depth int) {
	delete(t.nodeExpanded, nodeKey(node, depth))
	for _, c := range node.Children {
		if !c.Virtual {
			t.CollapseSubtree(c, depth+1)
		}
	}
}

// ToggleSectionAtY toggles the section row at content y-coordinate y (0-based inside content area).
// SelectAtY moves the cursor to the row at visual position y (relative to the
// tree content area, not including the border). Returns true if the cursor was
// moved to a selectable (non-section) row; returns false if the row is a
// section header or out of bounds.
func (t *treeRows) SelectAtY(y int) bool {
	contentIdx := t.offset + y
	if contentIdx < 0 || contentIdx >= len(t.rows) {
		return false
	}
	if t.rows[contentIdx].kind == rowKindSection {
		return false
	}
	t.cursor = contentIdx
	return true
}

func (t *treeRows) ToggleSectionAtY(y int) {
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

// NodeChanged records that discovery filled or refreshed node (merged by
// discovery.NodeExpander), expands it and rebuilds the rows.
func (t *treeRows) NodeChanged(node *models.Node) {
	if node == nil {
		return
	}
	t.patches++
	if key := t.findNodeKey(node); key != "" {
		t.nodeExpanded[key] = true
	}
	t.rebuild()
}

// CountChange records a node filled in by background discovery without
// expanding it or rebuilding rows. Call Rebuild afterwards.
func (t *treeRows) CountChange() { t.patches++ }

// Patches reports how many nodes were filled in by discovery since the tree
// was built; a non-zero count means the tree differs from what was loaded.
func (t *treeRows) Patches() int { return t.patches }

// findNodeKey locates the nodeKey for a given node pointer within the current rows.
func (t *treeRows) findNodeKey(target *models.Node) string {
	for _, row := range t.rows {
		if row.kind == rowKindCommand && row.node == target {
			return nodeKey(row.node, row.depth)
		}
	}
	return ""
}

// RowCount returns the number of tree rows currently visible (after filtering).
// Exported for testing.
func (t *treeRows) RowCount() int { return len(t.rows) }

func (t *treeRows) rebuild() {
	// Keep the backing array: rebuilding after every expand/collapse must not
	// reallocate and copy tens of thousands of rows on a big tree.
	t.rows = t.rows[:0]
	t.flattenNode(t.root, 0, "", true)
	if t.cursor >= len(t.rows) && len(t.rows) > 0 {
		t.cursor = len(t.rows) - 1
	}
}

func (t *treeRows) flattenNode(node *models.Node, depth int, graphPrefix string, isLast bool) {
	if node.Virtual {
		return
	}
	visChildren := visibleChildren(node)
	if t.filter != "" {
		t.flattenFiltered(node, depth, visChildren)
		return
	}

	t.rows = append(t.rows, treeRow{
		kind:        rowKindCommand,
		depth:       depth,
		node:        node,
		graphPrefix: graphPrefix,
		isLast:      isLast,
	})
	if !t.nodeExpanded[t.keyOf(node, depth, keyNode)] {
		return
	}

	// Sections in display order: inherited (global) flags, own flags,
	// subcommands, positionals.
	ownFlags, inheritedFlags := partitionFlags(node.Flags)
	t.appendFlagSection(node, depth, keyInherited, "Inherited flags", inheritedFlags, false)
	t.appendFlagSection(node, depth, keyFlags, "Flags", ownFlags, len(ownFlags) <= 5)
	t.appendSubcommands(node, depth, graphPrefix, isLast, visChildren)
	t.appendPositionals(node, depth)
}

// flattenFiltered shows node if it matches the filter or any descendant does
// (so ancestors act as context breadcrumbs), and always recurses so the whole
// tree is searched regardless of expanded state.
func (t *treeRows) flattenFiltered(node *models.Node, depth int, visChildren []*models.Node) {
	if matchesFilter(node, t.filter) || nodeOrDescendantMatchesFilter(node, t.filter) {
		t.rows = append(t.rows, treeRow{kind: rowKindCommand, depth: depth, node: node})
	}
	for _, c := range visChildren {
		t.flattenNode(c, depth+1, "", true)
	}
}

// visibleChildren returns node's non-virtual children.
func visibleChildren(node *models.Node) []*models.Node {
	var vis []*models.Node
	for _, c := range node.Children {
		if !c.Virtual {
			vis = append(vis, c)
		}
	}
	return vis
}

// partitionFlags splits flags into indices of own (local) and inherited
// (global) flags.
func partitionFlags(flags []models.Flag) (own, inherited []int) {
	for i := range flags {
		if flags[i].Inherited {
			inherited = append(inherited, i)
		} else {
			own = append(own, i)
		}
	}
	return own, inherited
}

// appendSection adds a section header row (unless sections are hidden) and
// reports the section's key and whether its items should be shown.
func (t *treeRows) appendSection(node *models.Node, depth int, section keySection, name string, count int, def bool) (string, bool) {
	sKey := t.keyOf(node, depth, section)
	if t.hideSections {
		return sKey, true
	}
	t.rows = append(t.rows, treeRow{
		kind:           rowKindSection,
		depth:          depth + 1,
		sectionKey:     sKey,
		sectionName:    name,
		sectionCount:   count,
		sectionDefault: def,
	})
	return sKey, t.isSectionExpanded(sKey, def)
}

func (t *treeRows) appendFlagSection(node *models.Node, depth int, section keySection, name string, idx []int, def bool) {
	if len(idx) == 0 {
		return
	}
	sKey, expanded := t.appendSection(node, depth, section, name, len(idx), def)
	if !expanded {
		return
	}
	for _, i := range idx {
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

func (t *treeRows) appendSubcommands(node *models.Node, depth int, graphPrefix string, isLast bool, visChildren []*models.Node) {
	if len(visChildren) == 0 {
		return
	}
	if _, expanded := t.appendSection(node, depth, keySubcommands, "Subcommands", len(visChildren), len(visChildren) <= 6); !expanded {
		return
	}
	childGraphPrefix := childPrefix(depth, graphPrefix, isLast)
	for i, c := range visChildren {
		t.flattenNode(c, depth+1, childGraphPrefix, i == len(visChildren)-1)
	}
}

// childPrefix is the graph-style continuation prefix for a node's children.
func childPrefix(depth int, graphPrefix string, isLast bool) string {
	switch {
	case depth == 0:
		return ""
	case isLast:
		return graphPrefix + "    "
	default:
		return graphPrefix + "│   "
	}
}

func (t *treeRows) appendPositionals(node *models.Node, depth int) {
	n := len(node.Positionals)
	if n == 0 {
		return
	}
	sKey, expanded := t.appendSection(node, depth, keyPositionals, "Positional arguments", n, n <= 5)
	if !expanded {
		return
	}
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

func (t *treeRows) isSectionExpanded(key string, defaultVal bool) bool {
	if v, ok := t.sectionExpanded[key]; ok {
		return v
	}
	return defaultVal
}

func (t *treeRows) scrollIntoView() {
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

// keySection names a node's own key or one of its sections. A small integer
// keeps keyID fixed-size, so memo lookups hash no strings.
type keySection uint8

const (
	keyNode keySection = iota
	keyInherited
	keyFlags
	keySubcommands
	keyPositionals
)

var keySectionSuffix = [...]string{"", "/inherited", "/flags", "/subcommands", "/positionals"}

type keyID struct {
	n       *models.Node
	depth   int
	section keySection
}

// keyOf is nodeKey (plus "/section" for a section), memoized.
func (t *treeRows) keyOf(n *models.Node, depth int, section keySection) string {
	id := keyID{n, depth, section}
	if k, ok := t.keys[id]; ok {
		return k
	}
	k := nodeKey(n, depth) + keySectionSuffix[section]
	if t.keys == nil {
		t.keys = map[keyID]string{}
	}
	t.keys[id] = k
	return k
}

// label is a section row's text, "Flags (3)". Built when drawn, not on every
// rebuild: a big tree has thousands of sections and a frame shows a few.
func (r treeRow) label() string {
	return r.sectionName + " (" + strconv.Itoa(r.sectionCount) + ")"
}

func nodeKey(n *models.Node, depth int) string {
	return strings.Join(n.FullPath, "/") + "@" + strconv.Itoa(depth)
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

// matchesFilter reports whether node fuzzy-matches filter on its name or
// one-line description. Space-separated terms must all match.
func matchesFilter(node *models.Node, filter string) bool {
	for _, term := range strings.Fields(filter) {
		if _, _, ok := fuzzy.Match(term, node.Name); ok {
			continue
		}
		if strings.Contains(strings.ToLower(node.Description), strings.ToLower(term)) {
			continue
		}
		return false
	}
	return true
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

// pathTo returns the chain of nodes from root down to target (inclusive), or
// nil when target is not reachable from root.
func pathTo(root, target *models.Node) []*models.Node {
	if root == target {
		return []*models.Node{root}
	}
	for _, c := range root.Children {
		if p := pathTo(c, target); p != nil {
			return append([]*models.Node{root}, p...)
		}
	}
	return nil
}

// Reveal clears any filter, expands every ancestor of node (and node itself
// when a flag or positional is requested) and moves the cursor to the
// requested row. flagName/posName select a flag or positional row on node;
// leave both empty to land on the command row. It reports whether a row was
// found.
func (t *treeRows) Reveal(node *models.Node, flagName, posName string) bool {
	chain := pathTo(t.root, node)
	if chain == nil {
		return false
	}
	t.filter = ""
	for i, n := range chain {
		key := nodeKey(n, i)
		if i < len(chain)-1 {
			t.nodeExpanded[key] = true
			t.sectionExpanded[key+"/subcommands"] = true
		} else if flagName != "" || posName != "" {
			t.nodeExpanded[key] = true
			t.sectionExpanded[key+"/flags"] = true
			t.sectionExpanded[key+"/inherited"] = true
			t.sectionExpanded[key+"/positionals"] = true
		}
	}
	t.rebuild()
	for i, r := range t.rows {
		var hit bool
		switch {
		case flagName != "":
			hit = r.kind == rowKindFlag && r.owner == node && "--"+strings.TrimLeft(r.flag.Name, "-") == flagName
		case posName != "":
			hit = r.kind == rowKindPositional && r.owner == node && r.positional.Name == posName
		default:
			hit = r.kind == rowKindCommand && r.node == node
		}
		if hit {
			t.cursor = i
			t.scrollIntoView()
			return true
		}
	}
	return false
}

// rebuildKeepingSelection rebuilds the rows and moves the cursor back to the
// row it was on (same node, flag or positional), so rows inserted above the
// selection by background discovery do not shift it.
func (t *treeRows) rebuildKeepingSelection() {
	var prev treeRow
	had := t.cursor >= 0 && t.cursor < len(t.rows)
	if had {
		prev = t.rows[t.cursor]
	}
	t.rebuild()
	if !had {
		return
	}
	for i, r := range t.rows {
		if r.kind == prev.kind && r.node == prev.node && r.flag == prev.flag &&
			r.positional == prev.positional && r.sectionKey == prev.sectionKey && r.owner == prev.owner {
			t.cursor = i
			t.scrollIntoView()
			return
		}
	}
}

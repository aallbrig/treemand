package tui

import (
	"strconv"
	"strings"

	"github.com/aallbrig/treemand/cli/treemand/fuzzy"
	"github.com/aallbrig/treemand/cli/treemand/models"
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

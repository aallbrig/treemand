package tui

import (
	"strings"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// Cursor movement over treeRows: line, page and jump moves, search matches,
// and the VS Code-style Left and Right that expand, collapse and walk the tree.

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

// Bottom jumps the cursor to the last row.
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

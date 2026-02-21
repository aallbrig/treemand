package tui

import (
"strings"

tea "github.com/charmbracelet/bubbletea"
"github.com/charmbracelet/lipgloss"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
)

// treeItem is a flattened view of a tree node for display.
type treeItem struct {
node     *models.Node
depth    int
expanded bool
visible  bool
}

// TreeModel manages the tree pane.
type TreeModel struct {
root     *models.Node
items    []treeItem
cursor   int
offset   int
filter   string
expanded map[string]bool
cfg      *config.Config
width    int
height   int

selectedStyle lipgloss.Style
normalStyle   lipgloss.Style
dimStyle      lipgloss.Style
}

func NewTreeModel(root *models.Node, cfg *config.Config) *TreeModel {
selBg := lipgloss.Color(cfg.Colors.Selected)
t := &TreeModel{
root:          root,
expanded:      make(map[string]bool),
cfg:           cfg,
selectedStyle: lipgloss.NewStyle().Background(selBg).Bold(true),
normalStyle:   lipgloss.NewStyle(),
dimStyle:      lipgloss.NewStyle().Faint(true),
}
// Expand root by default
t.expanded[nodeKey(root, 0)] = true
t.rebuild()
return t
}

func (t *TreeModel) SetSize(w, h int) {
t.width = w
t.height = h
}

func (t *TreeModel) SetFilter(f string) {
t.filter = f
t.rebuild()
}

func (t *TreeModel) Selected() *models.Node {
visible := t.visibleItems()
if t.cursor < len(visible) {
return visible[t.cursor].node
}
return nil
}

func (t *TreeModel) Up() {
if t.cursor > 0 {
t.cursor--
}
t.scrollIntoView()
}

func (t *TreeModel) Down() {
visible := t.visibleItems()
if t.cursor < len(visible)-1 {
t.cursor++
}
t.scrollIntoView()
}

func (t *TreeModel) Expand() {
visible := t.visibleItems()
if t.cursor >= len(visible) {
return
}
item := visible[t.cursor]
key := nodeKey(item.node, item.depth)
if !t.expanded[key] {
t.expanded[key] = true
t.rebuild()
}
}

func (t *TreeModel) Collapse() {
visible := t.visibleItems()
if t.cursor >= len(visible) {
return
}
item := visible[t.cursor]
key := nodeKey(item.node, item.depth)
if t.expanded[key] {
delete(t.expanded, key)
t.rebuild()
}
}

func (t *TreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
return t, nil
}

func (t *TreeModel) Init() tea.Cmd { return nil }

func (t *TreeModel) View() string { return t.ViewSized(t.width, t.height) }

func (t *TreeModel) ViewSized(w, h int) string {
t.width = w
t.height = h
visible := t.visibleItems()

// Ensure cursor is in range
if t.cursor >= len(visible) && len(visible) > 0 {
t.cursor = len(visible) - 1
}

var sb strings.Builder
end := t.offset + h
if end > len(visible) {
end = len(visible)
}
for i := t.offset; i < end; i++ {
item := visible[i]
line := t.renderItem(item, i == t.cursor)
// Pad/truncate to width
lineW := lipgloss.Width(line)
if lineW < w {
line += strings.Repeat(" ", w-lineW)
}
sb.WriteString(line)
sb.WriteByte('\n')
}

boxStyle := lipgloss.NewStyle().
Border(lipgloss.RoundedBorder()).
BorderForeground(lipgloss.Color("#555555")).
Width(w - 2).
Height(h)

title := lipgloss.NewStyle().Bold(true).Render("Tree: " + t.root.Name)
content := title + "\n" + sb.String()
return boxStyle.Render(content)
}

func (t *TreeModel) renderItem(item treeItem, selected bool) string {
indent := strings.Repeat("  ", item.depth)
icon := "• "
if len(item.node.Children) > 0 {
if t.expanded[nodeKey(item.node, item.depth)] {
icon = "▼ "
} else {
icon = "▶ "
}
}

name := item.node.Name
meta := ""
if len(item.node.Positionals) > 0 {
var parts []string
for _, p := range item.node.Positionals {
if p.Required {
parts = append(parts, "<"+p.Name+">")
} else {
parts = append(parts, "["+p.Name+"]")
}
}
meta = " " + strings.Join(parts, " ")
}

line := indent + icon + name + meta
if selected {
return t.selectedStyle.Render(line)
}
if item.depth == 0 {
return lipgloss.NewStyle().Bold(true).Render(line)
}
return t.normalStyle.Render(line)
}

func (t *TreeModel) rebuild() {
t.items = nil
t.flatten(t.root, 0)
}

func (t *TreeModel) flatten(node *models.Node, depth int) {
item := treeItem{node: node, depth: depth}
item.expanded = t.expanded[nodeKey(node, depth)]

if t.filter == "" || matchesFilter(node, t.filter) {
t.items = append(t.items, item)
}

if item.expanded {
for _, child := range node.Children {
t.flatten(child, depth+1)
}
}
}

func (t *TreeModel) visibleItems() []treeItem {
return t.items
}

func (t *TreeModel) scrollIntoView() {
if t.cursor < t.offset {
t.offset = t.cursor
}
if t.cursor >= t.offset+t.height {
t.offset = t.cursor - t.height + 1
}
}

func nodeKey(n *models.Node, depth int) string {
return strings.Join(n.FullPath, "/") + "@" + string(rune('0'+depth))
}

func matchesFilter(node *models.Node, filter string) bool {
return strings.Contains(strings.ToLower(node.Name), strings.ToLower(filter))
}

package tui

import (
"strings"

tea "github.com/charmbracelet/bubbletea"
"github.com/charmbracelet/lipgloss"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
)

// treeItem is a flattened node for rendering.
type treeItem struct {
node     *models.Node
depth    int
expanded bool
}

// TreeModel manages the scrollable, filterable tree pane.
type TreeModel struct {
root      *models.Node
items     []treeItem
cursor    int
offset    int
filter    string
expanded  map[string]bool
cmdTokens []string // from preview textinput
focused   bool
cfg       *config.Config
width     int
height    int
}

func NewTreeModel(root *models.Node, cfg *config.Config) *TreeModel {
t := &TreeModel{
root:     root,
expanded: make(map[string]bool),
cfg:      cfg,
}
t.expanded[nodeKey(root, 0)] = true
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
func (t *TreeModel) SetFocused(f bool)             { t.focused = f }

func (t *TreeModel) Selected() *models.Node {
vis := t.visibleItems()
if t.cursor < len(vis) {
return vis[t.cursor].node
}
return nil
}

func (t *TreeModel) Up() {
if t.cursor > 0 {
t.cursor--
t.scrollIntoView()
}
}

func (t *TreeModel) Down() {
if t.cursor < len(t.visibleItems())-1 {
t.cursor++
t.scrollIntoView()
}
}

func (t *TreeModel) Expand() {
vis := t.visibleItems()
if t.cursor >= len(vis) {
return
}
item := vis[t.cursor]
key := nodeKey(item.node, item.depth)
if !t.expanded[key] {
t.expanded[key] = true
t.rebuild()
}
}

func (t *TreeModel) Collapse() {
vis := t.visibleItems()
if t.cursor >= len(vis) {
return
}
item := vis[t.cursor]
key := nodeKey(item.node, item.depth)
if t.expanded[key] {
delete(t.expanded, key)
t.rebuild()
}
}

func (t *TreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return t, nil }
func (t *TreeModel) Init() tea.Cmd                           { return nil }
func (t *TreeModel) View() string                            { return t.ViewSized(t.width, t.height) }

func (t *TreeModel) ViewSized(w, h int) string {
t.width = w
t.height = h
vis := t.visibleItems()
if t.cursor >= len(vis) && len(vis) > 0 {
t.cursor = len(vis) - 1
}

borderColor := lipgloss.Color("#555555")
if t.focused {
borderColor = lipgloss.Color("#5EA4F5")
}

// Inner width available for content (box subtracts 2 for borders + 1 pad each side)
innerW := w - 4
if innerW < 1 {
innerW = 1
}

// Visible height inside box: total h minus top and bottom border rows.
innerH := h - 2
if innerH < 1 {
innerH = 1
}

var lines []string
end := t.offset + innerH
if end > len(vis) {
end = len(vis)
}
for i := t.offset; i < end; i++ {
line := t.renderItem(vis[i], i == t.cursor, innerW)
lines = append(lines, line)
}
// Pad to fill box height.
for len(lines) < innerH {
lines = append(lines, "")
}

boxStyle := lipgloss.NewStyle().
Border(lipgloss.RoundedBorder()).
BorderForeground(borderColor).
Width(w - 2).
Height(h - 2)

content := strings.Join(lines, "\n")
return boxStyle.Render(content)
}

func (t *TreeModel) renderItem(item treeItem, selected bool, maxW int) string {
indent := strings.Repeat("  ", item.depth)
hasChildren := len(item.node.Children) > 0

icon := ""
if hasChildren {
if t.expanded[nodeKey(item.node, item.depth)] {
icon = "▼ "
} else {
icon = "▶ "
}
}

// Determine text colors
nameColor := lipgloss.Color(t.cfg.Colors.Base)
if item.depth > 0 {
nameColor = lipgloss.Color(t.cfg.Colors.Subcmd)
}
nameStyle := lipgloss.NewStyle().Foreground(nameColor)
if item.depth == 0 {
nameStyle = nameStyle.Bold(true)
}

// Check if this node is matched by current cmd tokens (highlighted in preview).
if t.matchesTokenPrefix(item.node) {
nameStyle = nameStyle.Foreground(lipgloss.Color("#50FA7B")).Bold(true) // bright green
}

name := nameStyle.Render(item.node.Name)

// Inline positionals
var metaParts []string
posStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.cfg.Colors.Pos)).Faint(true)
for _, p := range item.node.Positionals {
if p.Required {
metaParts = append(metaParts, posStyle.Render("<"+p.Name+">"))
} else {
metaParts = append(metaParts, posStyle.Render("["+p.Name+"]"))
}
}

// Inline flags (abbreviated). Active flags are highlighted orange;
// inactive flags use per-type colors (bool=green, string=cyan, int=orange, other=purple).
if len(item.node.Flags) > 0 {
bracketStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.cfg.Colors.Flag)).Faint(true)
sepStyle := bracketStyle
activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C")).Bold(true)
const maxInline = 3
var flagParts []string
for i, f := range item.node.Flags {
if i >= maxInline {
flagParts = append(flagParts, bracketStyle.Render("…"))
break
}
n := f.Name
if f.ShortName != "" && !strings.HasPrefix(f.ShortName, "-") && len(f.ShortName) == 1 {
n += "|-" + f.ShortName
}
if isFlagActive(f, t.cmdTokens) {
flagParts = append(flagParts, activeStyle.Render(n))
} else {
flagParts = append(flagParts, t.flagColorStyle(f.ValueType).Faint(true).Render(n))
}
}
sep := sepStyle.Render(", ")
combined := ""
for i, p := range flagParts {
if i > 0 {
combined += sep
}
combined += p
}
metaParts = append(metaParts, bracketStyle.Render("[")+combined+bracketStyle.Render("]"))
}

meta := ""
if len(metaParts) > 0 {
meta = " " + strings.Join(metaParts, " ")
}

line := indent + icon + name + meta

if selected {
selStyle := lipgloss.NewStyle().
Background(lipgloss.Color(t.cfg.Colors.Selected)).
Bold(true)
// Pad to full width so the background covers the whole line.
lineW := lipgloss.Width(line)
if lineW < maxW {
line += strings.Repeat(" ", maxW-lineW)
}
return selStyle.Render(line)
}
return line
}

// matchesTokenPrefix returns true when the node's full path is a prefix-match
// of the current cmdTokens (case-insensitive). Used to highlight nodes that
// correspond to what is typed in the preview bar.
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

func (t *TreeModel) rebuild() {
t.items = nil
t.flatten(t.root, 0)
}

func (t *TreeModel) flatten(node *models.Node, depth int) {
item := treeItem{
node:     node,
depth:    depth,
expanded: t.expanded[nodeKey(node, depth)],
}
if t.filter == "" || matchesFilter(node, t.filter) {
t.items = append(t.items, item)
}
if item.expanded {
for _, child := range node.Children {
t.flatten(child, depth+1)
}
}
}

func (t *TreeModel) visibleItems() []treeItem { return t.items }

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

func nodeKey(n *models.Node, depth int) string {
return strings.Join(n.FullPath, "/") + "@" + string(rune('0'+depth))
}


// isFlagActive returns true when the flag's name or short name matches any
// flag-shaped token (--flag or -f) in the current cmdTokens.
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
// flagColorStyle returns a lipgloss.Style coloured for the given flag ValueType.
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



// ToggleExpand expands the selected node if collapsed, or collapses it if expanded.
func (t *TreeModel) ToggleExpand() {
vis := t.visibleItems()
if t.cursor >= len(vis) {
return
}
item := vis[t.cursor]
key := nodeKey(item.node, item.depth)
if t.expanded[key] {
delete(t.expanded, key)
} else {
t.expanded[key] = true
}
t.rebuild()
}

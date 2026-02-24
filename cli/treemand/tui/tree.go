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
rowKindCommand    rowKind = iota
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
SelCommand    SelectionKind = iota
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
func (t *TreeModel) SetFocused(f bool)             { t.focused = f }

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
pos := t.cursor + 1
for pos < len(t.rows) && t.rows[pos].kind == rowKindSection {
pos++
}
if pos < len(t.rows) {
t.cursor = pos
t.scrollIntoView()
// Auto-expand collapsed command nodes when navigating into them.
row := t.rows[t.cursor]
if row.kind == rowKindCommand {
key := nodeKey(row.node, row.depth)
if !t.nodeExpanded[key] {
t.nodeExpanded[key] = true
t.rebuild()
// cursor may have shifted after rebuild; re-find the same node
for i, r := range t.rows {
if r.kind == rowKindCommand && r.node == row.node && r.depth == row.depth {
t.cursor = i
break
}
}
t.scrollIntoView()
}
}
}
}

// Expand expands the current command node if collapsed.
// If the command is already expanded, expands its flags section (if any and collapsed).
// Does nothing on flag/positional rows.
func (t *TreeModel) Expand() {
if t.cursor >= len(t.rows) {
return
}
row := t.rows[t.cursor]
if row.kind != rowKindCommand {
return
}
key := nodeKey(row.node, row.depth)
if !t.nodeExpanded[key] {
t.nodeExpanded[key] = true
t.rebuild()
return
}
// Already expanded: expand flags section if available and collapsed.
if len(row.node.Flags) > 0 {
sKey := key + "/flags"
if !t.isSectionExpanded(sKey, false) {
t.sectionExpanded[sKey] = true
t.rebuild()
}
}
}

// Collapse collapses the current node.
// On a flag/positional row: collapses the containing section and moves cursor to the owner.
func (t *TreeModel) Collapse() {
if t.cursor >= len(t.rows) {
return
}
row := t.rows[t.cursor]
switch row.kind {
case rowKindCommand:
key := nodeKey(row.node, row.depth)
if t.nodeExpanded[key] {
delete(t.nodeExpanded, key)
t.rebuild()
}
case rowKindFlag, rowKindPositional:
t.sectionExpanded[row.sectionRef] = false
t.rebuild()
// Move cursor back to the owning command row.
for i, r := range t.rows {
if r.kind == rowKindCommand && r.node == row.owner && r.depth == row.ownerDepth {
t.cursor = i
break
}
}
t.scrollIntoView()
}
}

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

func (t *TreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return t, nil }
func (t *TreeModel) Init() tea.Cmd                           { return nil }
func (t *TreeModel) View() string                            { return t.ViewSized(t.width, t.height) }

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
indent := strings.Repeat("  ", row.depth)
key := nodeKey(row.node, row.depth)
isExpanded := t.nodeExpanded[key]

// Determine if node has any content worth showing a toggle for.
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
icon = "▼ "
} else {
icon = "▶ "
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

// Collapsed: show inline flag list like [--all,--clean,--config=<string>]
// Colors match the non-interactive output: per-type (bool=green, string=cyan,
// int=orange, other=purple); active flags are highlighted brighter.
summary := ""
if !isExpanded && len(row.node.Flags) > 0 {
bracketStyle := lipgloss.NewStyle().Faint(true)
activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C")).Bold(true)
var flagParts []string
for _, f := range row.node.Flags {
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
summary = " " + bracketStyle.Render("[") +
strings.Join(flagParts, bracketStyle.Render(",")) +
bracketStyle.Render("]")
}

line := indent + icon + name + summary
if selected {
selStyle := lipgloss.NewStyle().
Background(lipgloss.Color(t.cfg.Colors.Selected)).
Bold(true)
lineW := lipgloss.Width(line)
if lineW < maxW {
line += strings.Repeat(" ", maxW-lineW)
}
return selStyle.Render(line)
}
return line
}

func (t *TreeModel) renderSectionRow(row treeRow) string {
indent := strings.Repeat("  ", row.depth)
expanded := t.isSectionExpanded(row.sectionKey, row.sectionDefault)
icon := "▷ "
if expanded {
icon = "▽ "
}
dimStyle := lipgloss.NewStyle().Faint(true).Italic(true)
return dimStyle.Render(indent + icon + row.sectionLabel)
}

func (t *TreeModel) renderFlagRow(row treeRow, selected bool, maxW int) string {
indent := strings.Repeat("  ", row.depth)
f := row.flag

typeHint := ""
if f.ValueType != "" && f.ValueType != "bool" {
typeHint = " <" + f.ValueType + ">"
}

nameStyle := t.flagColorStyle(f.ValueType)
if isFlagActive(*f, t.cmdTokens) {
nameStyle = nameStyle.Underline(true).Bold(true)
}

const maxDescLen = 45
desc := f.Description
if len([]rune(desc)) > maxDescLen {
desc = string([]rune(desc)[:maxDescLen-1]) + "…"
}

namePart := nameStyle.Render(f.Name)
typePart := ""
if typeHint != "" {
typePart = lipgloss.NewStyle().Faint(true).Render(typeHint)
}
descPart := ""
if desc != "" {
descPart = "  " + lipgloss.NewStyle().Faint(true).Render(desc)
}

line := indent + namePart + typePart + descPart
if selected {
selStyle := lipgloss.NewStyle().
Background(lipgloss.Color(t.cfg.Colors.Selected)).
Bold(true)
lineW := lipgloss.Width(line)
if lineW < maxW {
line += strings.Repeat(" ", maxW-lineW)
}
return selStyle.Render(line)
}
return line
}

func (t *TreeModel) renderPositionalRow(row treeRow, selected bool, maxW int) string {
indent := strings.Repeat("  ", row.depth)
p := row.positional

posStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.cfg.Colors.Pos))

nameStr := "<" + p.Name + ">"
if !p.Required {
nameStr = "[" + p.Name + "]"
}

const maxDescLen = 45
desc := p.Description
if len([]rune(desc)) > maxDescLen {
desc = string([]rune(desc)[:maxDescLen-1]) + "…"
}

namePart := posStyle.Render(nameStr)
descPart := ""
if desc != "" {
descPart = "  " + lipgloss.NewStyle().Faint(true).Render(desc)
}

line := indent + namePart + descPart
if selected {
selStyle := lipgloss.NewStyle().
Background(lipgloss.Color(t.cfg.Colors.Selected)).
Bold(true)
lineW := lipgloss.Width(line)
if lineW < maxW {
line += strings.Repeat(" ", maxW-lineW)
}
return selStyle.Render(line)
}
return line
}

// ---------- rebuild ----------

func (t *TreeModel) rebuild() {
t.rows = nil
t.flattenNode(t.root, 0)
t.adjustCursorOffSection()
}

func (t *TreeModel) flattenNode(node *models.Node, depth int) {
if node.Virtual {
return
}

key := nodeKey(node, depth)
expanded := t.nodeExpanded[key]

// Add command row (filtered).
if t.filter == "" || matchesFilter(node, t.filter) {
t.rows = append(t.rows, treeRow{
kind:  rowKindCommand,
depth: depth,
node:  node,
})
}

if !expanded {
return
}

// Collect visible (non-virtual) children.
var visChildren []*models.Node
for _, c := range node.Children {
if !c.Virtual {
visChildren = append(visChildren, c)
}
}

// When a filter is active, skip section rows and just recurse into children.
if t.filter != "" {
for _, c := range visChildren {
t.flattenNode(c, depth+1)
}
return
}

// Sub commands section.
if len(visChildren) > 0 {
sKey := key + "/subcommands"
subDefault := len(visChildren) <= 6
subExpanded := t.isSectionExpanded(sKey, subDefault)
t.rows = append(t.rows, treeRow{
kind:           rowKindSection,
depth:          depth + 1,
sectionKey:     sKey,
sectionLabel:   fmt.Sprintf("Sub commands (%d)", len(visChildren)),
sectionDefault: subDefault,
})
if subExpanded {
for _, c := range visChildren {
t.flattenNode(c, depth+1)
}
}
}

// Flags section.
if len(node.Flags) > 0 {
sKey := key + "/flags"
flagExpanded := t.isSectionExpanded(sKey, false)
t.rows = append(t.rows, treeRow{
kind:           rowKindSection,
depth:          depth + 1,
sectionKey:     sKey,
sectionLabel:   fmt.Sprintf("Flags (%d)", len(node.Flags)),
sectionDefault: false,
})
if flagExpanded {
for i := range node.Flags {
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
posExpanded := t.isSectionExpanded(sKey, false)
t.rows = append(t.rows, treeRow{
kind:           rowKindSection,
depth:          depth + 1,
sectionKey:     sKey,
sectionLabel:   fmt.Sprintf("Positional arguments (%d)", len(node.Positionals)),
sectionDefault: false,
})
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

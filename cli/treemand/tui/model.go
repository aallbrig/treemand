// Package tui implements the interactive Bubble Tea TUI for treemand.
package tui

import (
"fmt"
"os"
"os/exec"
"strings"

"github.com/atotto/clipboard"
"github.com/charmbracelet/bubbles/textinput"
tea "github.com/charmbracelet/bubbletea"
"github.com/charmbracelet/lipgloss"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
"github.com/aallbrig/treemand/render"
)

// NavScheme is the keyboard navigation scheme.
type NavScheme int

const (
SchemeArrows NavScheme = iota
SchemeVim
SchemeWASD
)

// pane identifies which pane currently has focus.
type pane int

const (
paneTree    pane = 0
paneHelp    pane = 1
panePreview pane = 2
paneCount        = 3
)

// executeModal is the Ctrl+E dialog for running or copying the built command.
type executeModal struct {
active  bool
command string
}

// Model is the root Bubble Tea model.
type Model struct {
root         *models.Node
cfg          *config.Config
scheme       NavScheme
tree         *TreeModel
preview      *PreviewModel
helpPane     *HelpPaneModel
showHelpPane bool
filter       textinput.Model
filtering    bool
focusedPane  pane
width        int
height       int
statusMsg    string
quitting     bool
modal        *executeModal
commandToRun string // set when user picks "Run" in the modal
	fm           flagModal
}

// NewModel creates a new root TUI model.
func NewModel(root *models.Node, cfg *config.Config) *Model {
filter := textinput.New()
filter.Placeholder = "filter…"
filter.CharLimit = 64

m := &Model{
root:         root,
cfg:          cfg,
tree:         NewTreeModel(root, cfg),
preview:      NewPreviewModel(cfg),
helpPane:     NewHelpPaneModel(cfg),
filter:       filter,
showHelpPane: true,
focusedPane:  paneTree,
modal:        &executeModal{},
}
m.tree.SetFocused(true)
m.preview.SetNode(root)
m.helpPane.SetNode(root)
return m
}

func (m *Model) Init() tea.Cmd {
return tea.EnableMouseAllMotion
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
// Modal intercepts all input when active.
if m.modal.active {
if km, ok := msg.(tea.KeyMsg); ok {
return m.updateModal(km)
}
	if m.fm.active {
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.updateFlagModal(km)
		}
		return m, nil
	}
return m, nil
}

switch msg := msg.(type) {
case tea.WindowSizeMsg:
m.width = msg.Width
m.height = msg.Height
m.applyLayout()
return m, nil

case tea.KeyMsg:
if m.filtering {
return m.updateFilter(msg)
}
if m.focusedPane == panePreview {
return m.updatePreviewInput(msg)
}
return m.updateKeys(msg)

case tea.MouseMsg:
return m.updateMouse(msg)
}
return m, nil
}

// ---------- modal ----------

func (m *Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "ctrl+c", "esc", "q":
m.modal.active = false
m.statusMsg = "cancelled"
case "enter", "r", "R":
m.commandToRun = m.modal.command
m.modal.active = false
m.quitting = true
return m, tea.Quit
case "c", "C":
if err := clipboard.WriteAll(m.modal.command); err != nil {
m.statusMsg = "copy failed: " + err.Error()
} else {
m.statusMsg = "copied: " + m.modal.command
}
m.modal.active = false
}
return m, nil
}

func (m *Model) renderModal() string {
cmd := m.modal.command
if cmd == "" {
cmd = "(empty command)"
}

modalW := min(m.width-8, 72)
if modalW < 30 {
modalW = 30
}

titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.cfg.Colors.Base)).Bold(true)
hintStyle := lipgloss.NewStyle().Faint(true)

inner := titleStyle.Render("Execute Command") + "\n\n" +
cmdStyle.Render(cmd) + "\n\n" +
hintStyle.Render("[Enter/R] Run  [C] Copy  [Esc] Cancel")

box := lipgloss.NewStyle().
Border(lipgloss.RoundedBorder()).
BorderForeground(lipgloss.Color("#5EA4F5")).
Padding(1, 2).
Width(modalW - 2).
Render(inner)

// Center horizontally; place in the middle of the screen vertically.
padLeft := (m.width - lipgloss.Width(box)) / 2
if padLeft < 0 {
padLeft = 0
}
padTop := (m.height - lipgloss.Height(box)) / 2
if padTop < 0 {
padTop = 0
}

var sb strings.Builder
emptyLine := strings.Repeat(" ", m.width)
for i := 0; i < padTop; i++ {
sb.WriteString(emptyLine + "\n")
}
leftPad := strings.Repeat(" ", padLeft)
for _, line := range strings.Split(box, "\n") {
sb.WriteString(leftPad + line + "\n")
}
return sb.String()
}

// ---------- key routing ----------

func (m *Model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
key := msg.String()

switch key {
case "ctrl+c", "q", "esc":
m.quitting = true
return m, tea.Quit

case "tab":
m.cycleFocus(1)
return m, nil

case "shift+tab":
m.cycleFocus(-1)
return m, nil

case "ctrl+s":
m.scheme = (m.scheme + 1) % 3
m.statusMsg = "nav: " + schemeName(m.scheme)
return m, nil

case "h", "H", "ctrl+p":
m.showHelpPane = !m.showHelpPane
m.applyLayout()
return m, nil

case "/":
m.filtering = true
m.filter.Focus()
return m, textinput.Blink

case "r", "R":
m.statusMsg = "refreshed"
return m, nil

case "ctrl+e":
cmd := strings.Join(m.preview.Tokens(), " ")
if cmd == "" {
if node := m.tree.Selected(); node != nil {
cmd = node.FullCommand()
}
}
m.modal.command = cmd
m.modal.active = true
return m, nil

case "backspace", "delete":
m.preview.RemoveLastToken()
m.tree.SetCmdTokens(m.preview.Tokens())
m.statusMsg = "removed last token"
return m, nil

case "f", "F":
		m.openFlagModal()
		return m, nil
}

// Help pane specific keys.
if m.focusedPane == paneHelp {
return m.updateHelpPaneKeys(key)
}

// Tree navigation.
switch m.scheme {
case SchemeVim:
return m.handleVim(msg)
case SchemeWASD:
return m.handleWASD(msg)
default:
return m.handleArrows(msg)
}
}

// flagEntry is one row in the flag picker modal.
type flagEntry struct {
flag   models.Flag
global bool // true when sourced from the root node (global flag)
added  bool // true when already present in the preview
}

// flagModal is the f-key flag-picker overlay.
type flagModal struct {
active  bool
entries []flagEntry
cursor  int
offset  int
}

// openFlagModal builds and activates the flag picker for the selected node.
func (m *Model) openFlagModal() {
node := m.tree.Selected()
if node == nil {
m.statusMsg = "no node selected"
return
}

// Build set of flag names already in the preview.
addedSet := make(map[string]bool)
for _, tok := range m.preview.Tokens() {
if strings.HasPrefix(tok, "--") {
name := strings.TrimPrefix(tok, "--")
if idx := strings.Index(name, "="); idx >= 0 {
name = name[:idx]
}
addedSet["--"+name] = true
} else if strings.HasPrefix(tok, "-") && len(tok) == 2 {
addedSet[tok] = true
}
}

// Collect node-specific flags.
nodeFlags := make(map[string]bool)
var entries []flagEntry
for _, f := range node.Flags {
nodeFlags[f.Name] = true
entries = append(entries, flagEntry{
flag:   f,
global: false,
added:  addedSet[f.Name] || (f.ShortName != "" && addedSet["-"+f.ShortName]),
})
}

// Append global (root) flags not already listed above.
if node != m.root {
for _, f := range m.root.Flags {
if nodeFlags[f.Name] {
continue
}
entries = append(entries, flagEntry{
flag:   f,
global: true,
added:  addedSet[f.Name] || (f.ShortName != "" && addedSet["-"+f.ShortName]),
})
}
}

if len(entries) == 0 {
m.statusMsg = "no flags available"
return
}

m.fm = flagModal{active: true, entries: entries}
}

// updateFlagModal handles keys while the flag picker is open.
func (m *Model) updateFlagModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "ctrl+c", "esc", "q":
m.fm.active = false
case "up", "k":
if m.fm.cursor > 0 {
m.fm.cursor--
}
case "down", "j":
if m.fm.cursor < len(m.fm.entries)-1 {
m.fm.cursor++
}
case "enter", " ":
e := m.fm.entries[m.fm.cursor]
if !e.added {
m.preview.AppendToken(e.flag.Name)
m.tree.SetCmdTokens(m.preview.Tokens())
m.fm.entries[m.fm.cursor].added = true
m.statusMsg = "added: " + e.flag.Name
}
}
return m, nil
}

// renderFlagModal renders the flag picker as a centered overlay.
func (m *Model) renderFlagModal() string {
modalW := min(m.width-6, 68)
if modalW < 36 {
modalW = 36
}
const maxVisible = 18
vp := min(maxVisible, len(m.fm.entries))

if m.fm.cursor < m.fm.offset {
m.fm.offset = m.fm.cursor
}
if m.fm.cursor >= m.fm.offset+vp {
m.fm.offset = m.fm.cursor - vp + 1
}

titleStyle     := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
hintStyle      := lipgloss.NewStyle().Faint(true)
selStyle       := lipgloss.NewStyle().Background(lipgloss.Color("#264F78")).Bold(true)
addedStyle     := lipgloss.NewStyle().Faint(true).Strikethrough(true)
globalStyle    := lipgloss.NewStyle().Faint(true).Italic(true)
flagStyle      := lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
sepStyle       := lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#888888"))

inner := modalW - 6
var rows []string
prevWasLocal := true
for i := m.fm.offset; i < m.fm.offset+vp && i < len(m.fm.entries); i++ {
e := m.fm.entries[i]
if e.global && prevWasLocal {
rows = append(rows, sepStyle.Render(strings.Repeat("─", inner)))
rows = append(rows, sepStyle.Render("  global flags"))
prevWasLocal = false
}
if !e.global {
prevWasLocal = true
}

check := "  "
if e.added {
check = "✓ "
}
name := e.flag.Name
if e.flag.ShortName != "" {
name += ", -" + e.flag.ShortName
}
if e.flag.ValueType != "" && e.flag.ValueType != "bool" {
name += " <" + e.flag.ValueType + ">"
}
desc := e.flag.Description
maxDesc := inner - len(check) - len(name) - 2
if maxDesc < 0 {
maxDesc = 0
}
if len(desc) > maxDesc {
if maxDesc > 3 {
desc = desc[:maxDesc-1] + "…"
} else {
desc = ""
}
}
row := check + name
if desc != "" {
row += "  " + desc
}
if len(row) > inner {
row = row[:inner]
}

var rendered string
switch {
case i == m.fm.cursor:
padded := row + strings.Repeat(" ", max(0, inner-len(row)))
rendered = selStyle.Render(padded)
case e.added:
rendered = addedStyle.Render(row)
case e.global:
rendered = globalStyle.Render(row)
default:
rendered = flagStyle.Render(row)
}
rows = append(rows, rendered)
}

scrollHint := ""
if len(m.fm.entries) > vp {
scrollHint = fmt.Sprintf(" [%d/%d]", m.fm.cursor+1, len(m.fm.entries))
}

content := titleStyle.Render("Add Flag"+scrollHint) + "\n" +
hintStyle.Render("↑↓/jk navigate · Enter/Space add · Esc close") + "\n\n" +
strings.Join(rows, "\n")

box := lipgloss.NewStyle().
Border(lipgloss.RoundedBorder()).
BorderForeground(lipgloss.Color("#5EA4F5")).
Padding(0, 2).
Width(modalW - 2).
Render(content)

padLeft := (m.width - lipgloss.Width(box)) / 2
if padLeft < 0 {
padLeft = 0
}
padTop := (m.height - lipgloss.Height(box)) / 2
if padTop < 0 {
padTop = 0
}

var sb strings.Builder
blankLine := strings.Repeat(" ", m.width)
for i := 0; i < padTop; i++ {
sb.WriteString(blankLine + "\n")
}
leftPad := strings.Repeat(" ", padLeft)
for _, line := range strings.Split(box, "\n") {
sb.WriteString(leftPad + line + "\n")
}
return sb.String()
}


func (m *Model) updateHelpPaneKeys(key string) (tea.Model, tea.Cmd) {
switch key {
case "up", "k":
m.helpPane.ScrollUp(1)
case "down", "j":
m.helpPane.ScrollDown(1)
case "pgup", "ctrl+u", "b":
m.helpPane.PageUp()
case "pgdown", "ctrl+d":
m.helpPane.PageDown()
case "g":
m.helpPane.Top()
case "G":
m.helpPane.Bottom()
}
return m, nil
}

func (m *Model) updatePreviewInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
key := msg.String()
switch key {
case "ctrl+c":
m.quitting = true
return m, tea.Quit
case "esc":
m.setFocus(paneTree)
m.statusMsg = "focus: tree"
return m, nil
case "tab":
m.cycleFocus(1)
return m, nil
case "shift+tab":
m.cycleFocus(-1)
return m, nil
case "ctrl+e":
cmd := strings.Join(m.preview.Tokens(), " ")
m.modal.command = cmd
m.modal.active = true
return m, nil
}
cmd := m.preview.Update(msg)
m.tree.SetCmdTokens(m.preview.Tokens())
return m, cmd
}

func (m *Model) handleArrows(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "up":
m.tree.Up()
case "down":
m.tree.Down()
case "left":
m.tree.Collapse()
case "right":
m.tree.Expand()
case " ":
m.tree.ToggleExpand()
case "enter":
if node := m.tree.Selected(); node != nil {
m.preview.SetCommand(node.FullCommand())
m.tree.SetCmdTokens(m.preview.Tokens())
m.statusMsg = "set: " + node.FullCommand()
}
}
m.syncSelected()
return m, nil
}

func (m *Model) handleVim(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "k":
m.tree.Up()
case "j":
m.tree.Down()
case "l":
m.tree.Expand()
case " ":
m.tree.ToggleExpand()
case "enter":
if node := m.tree.Selected(); node != nil {
m.preview.SetCommand(node.FullCommand())
m.tree.SetCmdTokens(m.preview.Tokens())
m.statusMsg = "set: " + node.FullCommand()
}
}
m.syncSelected()
return m, nil
}

func (m *Model) handleWASD(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "w":
m.tree.Up()
case "s":
m.tree.Down()
case "a":
m.tree.Collapse()
case "d":
m.tree.Expand()
case " ":
m.tree.ToggleExpand()
case "enter":
if node := m.tree.Selected(); node != nil {
m.preview.SetCommand(node.FullCommand())
m.tree.SetCmdTokens(m.preview.Tokens())
m.statusMsg = "set: " + node.FullCommand()
}
}
m.syncSelected()
return m, nil
}

func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "esc", "enter":
m.filtering = false
m.filter.Blur()
m.tree.SetFilter(m.filter.Value())
return m, nil
}
var cmd tea.Cmd
m.filter, cmd = m.filter.Update(msg)
m.tree.SetFilter(m.filter.Value())
return m, cmd
}

func (m *Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
switch msg.Type {
case tea.MouseLeft:
m.handleMouseClick(msg.X, msg.Y)

case tea.MouseWheelUp:
if m.showHelpPane && m.width > 0 && msg.X >= m.treeWidth() {
m.helpPane.ScrollUp(3)
} else {
m.tree.Up()
m.syncSelected()
}

case tea.MouseWheelDown:
if m.showHelpPane && m.width > 0 && msg.X >= m.treeWidth() {
m.helpPane.ScrollDown(3)
} else {
m.tree.Down()
m.syncSelected()
}
}
return m, nil
}

func (m *Model) handleMouseClick(x, y int) {
if y < previewBarHeight {
m.setFocus(panePreview)
} else if m.showHelpPane && m.width > 0 && x >= m.treeWidth() {
m.setFocus(paneHelp)
} else {
m.setFocus(paneTree)
}
m.statusMsg = "focus: " + paneName(m.focusedPane)
}

// ---------- focus management ----------

func (m *Model) cycleFocus(delta int) {
next := (int(m.focusedPane) + delta + paneCount) % paneCount
if pane(next) == paneHelp && !m.showHelpPane {
next = (next + delta + paneCount) % paneCount
}
m.setFocus(pane(next))
m.statusMsg = "focus: " + paneName(pane(next))
}

func (m *Model) setFocus(p pane) {
m.focusedPane = p
m.tree.SetFocused(p == paneTree)
m.preview.SetFocused(p == panePreview)
m.helpPane.SetFocused(p == paneHelp)
}

func (m *Model) syncSelected() {
if node := m.tree.Selected(); node != nil {
m.preview.SetNode(node)
m.helpPane.SetNode(node)
}
}

// ---------- layout ----------

func (m *Model) applyLayout() {
if m.width == 0 || m.height == 0 {
return
}
cH := m.contentHeight()
m.tree.SetSize(m.treeWidth(), cH)
m.helpPane.SetSize(m.helpWidth(), cH)
}

const previewBarHeight = 2

func (m *Model) contentHeight() int {
h := m.height - previewBarHeight - 1
if h < 1 {
return 1
}
return h
}

func (m *Model) treeWidth() int {
if m.showHelpPane && m.width >= 80 {
tw := m.width * 55 / 100
if tw < 30 {
tw = 30
}
return tw
}
return m.width
}

func (m *Model) helpWidth() int {
return m.width - m.treeWidth()
}

// ---------- view ----------

func (m *Model) View() string {
if m.quitting {
return ""
}

if m.modal.active {
return m.renderModal()
}
	if m.fm.active {
		return m.renderFlagModal()
	}


previewBar := m.preview.View(m.width)
statusBar := m.renderStatusBar()

cH := m.contentHeight()
treeView := m.tree.ViewSized(m.treeWidth(), cH)

var body string
if m.showHelpPane && m.helpWidth() > 20 {
helpView := m.helpPane.View(m.helpWidth(), cH)
body = lipgloss.JoinHorizontal(lipgloss.Top, treeView, helpView)
} else {
body = treeView
}

return lipgloss.JoinVertical(lipgloss.Left, previewBar, body, statusBar)
}

func (m *Model) renderStatusBar() string {
selected := ""
if node := m.tree.Selected(); node != nil {
selected = node.FullCommand()
}
left := lipgloss.NewStyle().Bold(true).Render(selected)

var hint string
switch {
case m.statusMsg != "":
hint = m.statusMsg
m.statusMsg = ""
case m.filtering:
hint = "filter: " + m.filter.View() + "  (Enter/Esc)"
case m.focusedPane == panePreview:
hint = "editing · Esc:tree · Enter:flag · Ctrl+E:exec · Tab:switch"
case m.focusedPane == paneHelp:
hint = "↑↓/jk:scroll · PgUp/PgDn · g/G:top/bottom · Tab:switch"
default:
hint = fmt.Sprintf("Enter:set-cmd  f:add-flag  Backspace:remove  Ctrl+E:exec  h:help  q:quit  nav:%s",
schemeName(m.scheme))
}
right := lipgloss.NewStyle().Faint(true).Render(hint)

gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
if gap < 1 {
gap = 1
}
return left + strings.Repeat(" ", gap) + right
}

// ---------- helpers ----------

func schemeName(s NavScheme) string {
switch s {
case SchemeVim:
return "vim"
case SchemeWASD:
return "wasd"
default:
return "arrows"
}
}

func paneName(p pane) string {
switch p {
case panePreview:
return "preview"
case paneHelp:
return "help"
default:
return "tree"
}
}

func min(a, b int) int {
if a < b {
return a
}
return b
}

func max(a, b int) int {
if a > b {
return a
}
return b
}

// Run starts the interactive TUI. If the user chose "Run" in the Ctrl+E modal,
// it executes the command after the TUI exits.
func Run(root *models.Node, cfg *config.Config) error {
m := NewModel(root, cfg)
p := tea.NewProgram(m,
tea.WithAltScreen(),
tea.WithMouseAllMotion(),
)
finalModel, err := p.Run()
if err != nil {
return err
}
if fm, ok := finalModel.(*Model); ok && fm.commandToRun != "" {
parts := strings.Fields(fm.commandToRun)
if len(parts) > 0 {
c := exec.Command(parts[0], parts[1:]...) //nolint:gosec
c.Stdin = os.Stdin
c.Stdout = os.Stdout
c.Stderr = os.Stderr
return c.Run()
}
}
return nil
}

// NodePreview returns a color-coded command preview string.
func NodePreview(node *models.Node, cfg *config.Config) string {
opts := render.DefaultOptions()
opts.NoColor = cfg.NoColor
opts.Colors = cfg.Colors
opts.MaxDepth = 0
s, _ := render.RenderToString(node, opts)
return strings.TrimSpace(s)
}

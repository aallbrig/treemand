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
panePreview pane = 1
paneHelp    pane = 2
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
flagCycleIdx int    // index for cycling through flags with the f key
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
return m.addNextFlag()
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

// addNextFlag appends the next flag of the selected node (cycling) to the preview.
func (m *Model) addNextFlag() (tea.Model, tea.Cmd) {
node := m.tree.Selected()
if node == nil || len(node.Flags) == 0 {
m.statusMsg = "no flags available"
return m, nil
}
m.flagCycleIdx = m.flagCycleIdx % len(node.Flags)
f := node.Flags[m.flagCycleIdx]
m.preview.AppendToken(f.Name)
m.tree.SetCmdTokens(m.preview.Tokens())
m.statusMsg = "added: " + f.Name
m.flagCycleIdx++
return m, nil
}

func (m *Model) updateHelpPaneKeys(key string) (tea.Model, tea.Cmd) {
switch key {
case "up", "k":
m.helpPane.ScrollUp(1)
case "down", "j":
m.helpPane.ScrollDown(1)
case "pgup", "ctrl+u", "b":
m.helpPane.PageUp()
case "pgdown", "ctrl+d", "f":
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
m.flagCycleIdx = 0
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
m.flagCycleIdx = 0
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
m.flagCycleIdx = 0
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

// Package tui implements the interactive Bubble Tea TUI for treemand.
package tui

import (
"fmt"
"strings"

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

// Model is the root Bubble Tea model.
type Model struct {
root        *models.Node
cfg         *config.Config
scheme      NavScheme
tree        *TreeModel
preview     *PreviewModel
helpPane    *HelpPaneModel
showHelpPane bool
filter      textinput.Model
filtering   bool
focusedPane pane
width       int
height      int
statusMsg   string
quitting    bool
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
// Preview pane is focused: forward all typing to the textinput.
if m.focusedPane == panePreview {
return m.updatePreviewInput(msg)
}
return m.updateKeys(msg)

case tea.MouseMsg:
return m.updateMouse(msg)
}
return m, nil
}

// ---------- key routing ----------

func (m *Model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
key := msg.String()

// Global keys work in all non-preview panes.
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

case "H", "ctrl+p":
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
}

// Pane-specific key handling.
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
// Leave preview focus back to tree.
m.setFocus(paneTree)
m.statusMsg = "focus: tree"
return m, nil
case "tab":
m.cycleFocus(1)
return m, nil
case "shift+tab":
m.cycleFocus(-1)
return m, nil
}
// Forward to textinput.
cmd := m.preview.Update(msg)
// Update tree highlighting whenever the input changes.
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
case "right", " ", "enter":
m.tree.Expand()
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
case "h":
m.tree.Collapse()
case "l", " ", "enter":
m.tree.Expand()
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
case "d", " ", "enter":
m.tree.Expand()
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
case tea.MouseWheelUp:
if m.focusedPane == paneHelp {
m.helpPane.ScrollUp(3)
} else {
m.tree.Up()
m.syncSelected()
}
case tea.MouseWheelDown:
if m.focusedPane == paneHelp {
m.helpPane.ScrollDown(3)
} else {
m.tree.Down()
m.syncSelected()
}
}
return m, nil
}

// ---------- focus management ----------

func (m *Model) cycleFocus(delta int) {
next := (int(m.focusedPane) + delta + paneCount) % paneCount
// Skip help pane if it's hidden.
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

// previewHeight is how many terminal rows the preview bar occupies
// (content line + bottom border).
const previewBarHeight = 2

func (m *Model) contentHeight() int {
h := m.height - previewBarHeight - 1 // 1 for status bar
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
m.statusMsg = "" // clear after one render
case m.filtering:
hint = "filter: " + m.filter.View() + "  (Enter/Esc)"
case m.focusedPane == panePreview:
hint = "editing cmd · Esc: exit · Tab: switch pane"
case m.focusedPane == paneHelp:
hint = "↑↓/jk: scroll · PgUp/PgDn · g/G: top/bottom · Tab: switch"
default:
hint = fmt.Sprintf("Tab:focus  nav:%s  /:filter  H:help  q:quit",
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

// Run starts the interactive TUI.
func Run(root *models.Node, cfg *config.Config) error {
m := NewModel(root, cfg)
p := tea.NewProgram(m,
tea.WithAltScreen(),
tea.WithMouseAllMotion(),
)
_, err := p.Run()
return err
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

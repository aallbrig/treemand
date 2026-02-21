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

// Model is the root Bubble Tea model.
type Model struct {
root        *models.Node
cfg         *config.Config
scheme      NavScheme
tree        *TreeModel
preview     *PreviewModel
helpPane    *HelpPaneModel
showHelp    bool
showHelpPane bool
filter      textinput.Model
filtering   bool
width       int
height      int
statusMsg   string
quitting    bool
}

// NewModel creates a new root TUI model.
func NewModel(root *models.Node, cfg *config.Config) *Model {
filter := textinput.New()
filter.Placeholder = "filter..."
filter.CharLimit = 64

tm := &Model{
root:     root,
cfg:      cfg,
tree:     NewTreeModel(root, cfg),
preview:  NewPreviewModel(cfg),
helpPane: NewHelpPaneModel(cfg),
filter:   filter,
}
tm.preview.SetNode(root)
tm.helpPane.SetNode(root)
return tm
}

func (m *Model) Init() tea.Cmd {
return tea.EnableMouseAllMotion
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
var cmds []tea.Cmd

switch msg := msg.(type) {
case tea.WindowSizeMsg:
m.width = msg.Width
m.height = msg.Height
m.tree.SetSize(m.treeWidth(), m.contentHeight())
m.helpPane.SetSize(m.helpWidth(), m.contentHeight())
return m, nil

case tea.KeyMsg:
if m.filtering {
return m.updateFilter(msg)
}
return m.updateKeys(msg)

case tea.MouseMsg:
return m.updateMouse(msg)
}

// Propagate to sub-models
newTree, cmd := m.tree.Update(msg)
m.tree = newTree.(*TreeModel)
cmds = append(cmds, cmd)

return m, tea.Batch(cmds...)
}

func (m *Model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
switch msg.String() {
case "ctrl+c", "q", "esc":
m.quitting = true
return m, tea.Quit

case "ctrl+s":
m.scheme = (m.scheme + 1) % 3
m.statusMsg = fmt.Sprintf("nav: %s", schemeName(m.scheme))
return m, nil

case "?":
m.showHelp = !m.showHelp
return m, nil

case "h", "H":
if m.scheme == SchemeVim && msg.String() == "h" {
// vim: h = left (collapse)
m.tree.Collapse()
} else {
m.showHelpPane = !m.showHelpPane
}
return m, nil

case "/":
m.filtering = true
m.filter.Focus()
return m, textinput.Blink

case "r", "R":
m.statusMsg = "refreshing..."
return m, nil

case "ctrl+p":
m.showHelpPane = !m.showHelpPane
return m, nil
}

// Navigation delegation
switch m.scheme {
case SchemeVim:
return m.handleVim(msg)
case SchemeWASD:
return m.handleWASD(msg)
default:
return m.handleArrows(msg)
}
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
// Scroll tree pane
if msg.Type == tea.MouseWheelUp {
m.tree.Up()
m.syncSelected()
} else if msg.Type == tea.MouseWheelDown {
m.tree.Down()
m.syncSelected()
}
return m, nil
}

func (m *Model) syncSelected() {
if node := m.tree.Selected(); node != nil {
m.preview.SetNode(node)
m.helpPane.SetNode(node)
}
}

func (m *Model) View() string {
if m.quitting {
return ""
}
if m.showHelp {
return m.helpView()
}

previewBar := m.preview.View(m.width)
statusBar := m.statusBar()

contentH := m.contentHeight()
treeView := m.tree.ViewSized(m.treeWidth(), contentH)

var content string
if m.showHelpPane {
helpView := m.helpPane.View(m.helpWidth(), contentH)
content = lipgloss.JoinHorizontal(lipgloss.Top, treeView, helpView)
} else {
content = treeView
}

return lipgloss.JoinVertical(lipgloss.Left, previewBar, content, statusBar)
}

func (m *Model) helpView() string {
sb := strings.Builder{}
sb.WriteString("treemand key bindings\n\n")
sb.WriteString("  Navigation (toggle Ctrl+S):\n")
sb.WriteString("    Arrows / vim (hjkl) / WASD\n\n")
sb.WriteString("  Actions:\n")
sb.WriteString("    Space/Enter  expand/activate\n")
sb.WriteString("    /            fuzzy filter\n")
sb.WriteString("    H            toggle help pane\n")
sb.WriteString("    R            refresh node\n")
sb.WriteString("    Ctrl+P       toggle panes\n")
sb.WriteString("    Ctrl+S       cycle nav scheme\n")
sb.WriteString("    ?            toggle this help\n")
sb.WriteString("    q/Esc        quit\n")
return sb.String()
}

func (m *Model) statusBar() string {
selected := ""
if node := m.tree.Selected(); node != nil {
selected = node.FullCommand()
}
scheme := schemeName(m.scheme)
hint := fmt.Sprintf("nav:%s  ?:help  /:filter  H:help-pane  q:quit", scheme)
if m.statusMsg != "" {
hint = m.statusMsg
}
if m.filtering {
hint = "filter: " + m.filter.View() + "  (Enter/Esc to finish)"
}
left := lipgloss.NewStyle().Bold(true).Render(selected)
right := lipgloss.NewStyle().Faint(true).Render(hint)
gap := max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right)-2)
return left + strings.Repeat(" ", gap) + right
}

func (m *Model) treeWidth() int {
if m.showHelpPane && m.width > 80 {
return m.width * 6 / 10
}
return m.width
}

func (m *Model) helpWidth() int {
return m.width - m.treeWidth()
}

func (m *Model) contentHeight() int {
// subtract preview bar (3) + status bar (1)
h := m.height - 4
if h < 1 {
return 1
}
return h
}

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

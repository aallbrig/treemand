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

// valueInputModal is the inline value-entry dialog for flag/positional rows.
type valueInputModal struct {
	active bool
	label  string // e.g. "--flag-name <string>"
	prefix string // token prefix e.g. "--flag-name=" or ""
	input  textinput.Model
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
	vm           valueInputModal
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
	// Value input modal intercepts all input when active.
	if m.vm.active {
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.updateValueModal(km)
		}
		return m, nil
	}

	// Flag modal intercepts all input when active.
	if m.fm.active {
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.updateFlagModal(km)
		}
		return m, nil
	}

	// Execute modal intercepts all input when active.
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

// ---------- value input modal ----------

func (m *Model) updateValueModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := m.vm.prefix + m.vm.input.Value()
		m.preview.AppendToken(val)
		m.tree.SetCmdTokens(m.preview.Tokens())
		m.statusMsg = "added: " + val
		m.vm.active = false
		return m, nil
	case "esc", "ctrl+c":
		m.vm.active = false
		return m, nil
	}
	var cmd tea.Cmd
	m.vm.input, cmd = m.vm.input.Update(msg)
	return m, cmd
}

func (m *Model) renderValueInputModal() string {
	modalW := min(m.width-8, 60)
	if modalW < 30 {
		modalW = 30
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
	hintStyle := lipgloss.NewStyle().Faint(true)

	m.vm.input.Width = modalW - 8
	inner := titleStyle.Render(m.vm.label) + "\n\n" +
		m.vm.input.View() + "\n\n" +
		hintStyle.Render("[Enter] confirm  [Esc] cancel")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5EA4F5")).
		Padding(1, 2).
		Width(modalW - 2).
		Render(inner)

	padLeft := (m.width - lipgloss.Width(box)) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	padTop := (m.height - lipgloss.Height(box)) / 2
	if padTop < 0 {
		padTop = 0
	}
	blankLine := strings.Repeat(" ", m.width)
	leftPad := strings.Repeat(" ", padLeft)
	var sb strings.Builder
	for i := 0; i < padTop; i++ {
		sb.WriteString(blankLine + "\n")
	}
	for _, line := range strings.Split(box, "\n") {
		sb.WriteString(leftPad + line + "\n")
	}
	return sb.String()
}

func (m *Model) openValueModal(f *models.Flag) {
	vi := textinput.New()
	vi.Placeholder = "value…"
	vi.CharLimit = 256
	vi.Focus()
	m.vm = valueInputModal{
		active: true,
		label:  f.Name + " <" + f.ValueType + ">",
		prefix: f.Name + "=",
		input:  vi,
	}
}

func (m *Model) openPositionalModal(p *models.Positional) {
	name := "<" + p.Name + ">"
	if !p.Required {
		name = "[" + p.Name + "]"
	}
	vi := textinput.New()
	vi.Placeholder = p.Name
	vi.CharLimit = 256
	vi.Focus()
	m.vm = valueInputModal{
		active: true,
		label:  name,
		prefix: "",
		input:  vi,
	}
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
active        bool
entries       []flagEntry
cursor        int
offset        int
awaitingValue bool   // true when prompting the user to type a value
awaitingIdx   int    // index of the entry awaiting a value
valueInput    textinput.Model
}

// flagTypeColor returns a colour for a flag's value-type indicator in the modal.
func flagTypeColor(valueType string) lipgloss.Color {
switch strings.ToLower(valueType) {
case "", "bool":
return lipgloss.Color("#50FA7B") // green
case "string", "str":
return lipgloss.Color("#8BE9FD") // cyan
case "int", "int64", "uint", "uint64", "float", "float64", "duration":
return lipgloss.Color("#FFB86C") // orange
default:
return lipgloss.Color("#BD93F9") // purple
}
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

vi := textinput.New()
vi.CharLimit = 128

m.fm = flagModal{active: true, entries: entries, valueInput: vi}
}

// updateFlagModal handles keys while the flag picker is open.
func (m *Model) updateFlagModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
// When awaiting a value, route all keys into the text input.
if m.fm.awaitingValue {
switch msg.String() {
case "ctrl+c", "esc":
m.fm.awaitingValue = false
m.fm.valueInput.SetValue("")
return m, nil
case "enter":
e := m.fm.entries[m.fm.awaitingIdx]
val := strings.TrimSpace(m.fm.valueInput.Value())
token := e.flag.Name
if val != "" {
token += "=" + val
}
m.preview.AppendToken(token)
m.tree.SetCmdTokens(m.preview.Tokens())
m.fm.entries[m.fm.awaitingIdx].added = true
m.statusMsg = "added: " + token
m.fm.awaitingValue = false
m.fm.valueInput.SetValue("")
return m, nil
}
var cmd tea.Cmd
m.fm.valueInput, cmd = m.fm.valueInput.Update(msg)
return m, cmd
}

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
vt := strings.ToLower(e.flag.ValueType)
if vt != "" && vt != "bool" {
// Non-bool flag: prompt for a value before adding.
m.fm.awaitingValue = true
m.fm.awaitingIdx = m.fm.cursor
m.fm.valueInput.Placeholder = "value for " + e.flag.Name
m.fm.valueInput.SetValue("")
m.fm.valueInput.Focus()
return m, textinput.Blink
}
m.preview.AppendToken(e.flag.Name)
m.tree.SetCmdTokens(m.preview.Tokens())
m.fm.entries[m.fm.cursor].added = true
m.statusMsg = "added: " + e.flag.Name
}
}
return m, nil
}

// renderFlagModal renders the flag picker as a centered overlay that fills
// the full terminal height so Bubble Tea clears stale content from the
// previous frame.
func (m *Model) renderFlagModal() string {
modalW := min(m.width-6, 72)
if modalW < 36 {
modalW = 36
}

// Calculate how many global-separator rows will be inserted so we can
// keep the viewport from overflowing the modal box.
hasGlobals := false
for _, e := range m.fm.entries {
if e.global {
hasGlobals = true
break
}
}
sepRows := 0
if hasGlobals {
sepRows = 2 // separator line + "global flags" label
}
const maxVisible = 14
vp := min(maxVisible, len(m.fm.entries))
// Clamp vp so that entries + separator rows fit inside the box.
if sepRows > 0 && vp+sepRows > maxVisible {
vp = max(1, maxVisible-sepRows)
}

if m.fm.cursor < m.fm.offset {
m.fm.offset = m.fm.cursor
}
if m.fm.cursor >= m.fm.offset+vp {
m.fm.offset = m.fm.cursor - vp + 1
}

titleStyle  := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
hintStyle   := lipgloss.NewStyle().Faint(true)
selStyle    := lipgloss.NewStyle().Background(lipgloss.Color("#264F78")).Bold(true)
addedStyle  := lipgloss.NewStyle().Faint(true) // checkmark shown separately; no strikethrough
globalStyle := lipgloss.NewStyle().Faint(true).Italic(true)
descStyle   := lipgloss.NewStyle().Faint(true)
sepStyle    := lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#888888"))

inner := modalW - 6
var rows []string
prevWasLocal := true
for i := m.fm.offset; i < m.fm.offset+vp && i < len(m.fm.entries); i++ {
e := m.fm.entries[i]
// Insert the "global flags" separator once, before the first global entry.
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

// Flag name coloured by value type.
nameColor := flagTypeColor(e.flag.ValueType)
nameStr := e.flag.Name
if e.flag.ShortName != "" {
nameStr += ", -" + e.flag.ShortName
}
// Value-type badge for non-bool flags.
typeTag := ""
if vt := strings.ToLower(e.flag.ValueType); vt != "" && vt != "bool" {
typeTag = " <" + e.flag.ValueType + ">"
}

// Measure available space for description.
fullName := nameStr + typeTag
maxDesc := inner - lipgloss.Width(check) - lipgloss.Width(fullName) - 3
desc := e.flag.Description
if maxDesc < 4 {
desc = ""
} else if lipgloss.Width(desc) > maxDesc {
desc = desc[:maxDesc-1] + "…"
}

// Render this row using coloured sub-parts, then merge.
var rendered string
switch {
case i == m.fm.cursor:
// Selected: uniform highlight across full inner width.
plain := check + fullName
if desc != "" {
plain += "   " + desc
}
if lipgloss.Width(plain) > inner {
plain = plain[:inner]
}
pad := max(0, inner-lipgloss.Width(plain))
rendered = selStyle.Render(plain + strings.Repeat(" ", pad))
case e.added:
// Added: faint; the checkmark is the indicator, no strikethrough.
plain := check + fullName
if desc != "" {
plain += "   " + desc
}
rendered = addedStyle.Render(plain)
case e.global:
// Global flag (from root node): italic + faint.
plain := check + fullName
if desc != "" {
plain += "   " + desc
}
rendered = globalStyle.Render(plain)
default:
// Normal flag: type-coloured name, faint description.
namePart := lipgloss.NewStyle().Foreground(nameColor).Render(check + nameStr)
typePart := ""
if typeTag != "" {
typePart = lipgloss.NewStyle().
Foreground(nameColor).Faint(true).Render(typeTag)
}
descPart := ""
if desc != "" {
descPart = "   " + descStyle.Render(desc)
}
rendered = namePart + typePart + descPart
}
rows = append(rows, rendered)
}

// Hint line changes when awaiting a value input.
hint := "↑↓/jk navigate · Enter add · Esc close"
if m.fm.awaitingValue {
hint = "Type value · Enter confirm · Esc cancel"
}

scrollHint := ""
if len(m.fm.entries) > vp {
scrollHint = fmt.Sprintf(" [%d/%d]", m.fm.cursor+1, len(m.fm.entries))
}

listSection := strings.Join(rows, "\n")

// Value input prompt (shown when a non-bool flag is selected).
valueSection := ""
if m.fm.awaitingValue {
e := m.fm.entries[m.fm.awaitingIdx]
promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB86C"))
m.fm.valueInput.Width = inner - 4
valueSection = "\n" +
sepStyle.Render(strings.Repeat("─", inner)) + "\n" +
promptStyle.Render("Value for "+e.flag.Name+":") + "\n" +
m.fm.valueInput.View()
}

content := titleStyle.Render("Add Flag"+scrollHint) + "\n" +
hintStyle.Render(hint) + "\n\n" +
listSection + valueSection

box := lipgloss.NewStyle().
Border(lipgloss.RoundedBorder()).
BorderForeground(lipgloss.Color("#5EA4F5")).
Padding(0, 2).
Width(modalW - 2).
Render(content)

boxH := lipgloss.Height(box)
padLeft := (m.width - lipgloss.Width(box)) / 2
if padLeft < 0 {
padLeft = 0
}
padTop := (m.height - boxH) / 2
if padTop < 0 {
padTop = 0
}
padBottom := max(0, m.height-padTop-boxH)

// Build a full-height string so Bubble Tea clears old content beneath.
blankLine := strings.Repeat(" ", m.width)
leftPad := strings.Repeat(" ", padLeft)
var sb strings.Builder
for i := 0; i < padTop; i++ {
sb.WriteString(blankLine + "\n")
}
for _, line := range strings.Split(box, "\n") {
sb.WriteString(leftPad + line + "\n")
}
for i := 0; i < padBottom; i++ {
sb.WriteString(blankLine + "\n")
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
m.tree.Left()
case "right":
m.tree.Right()
case " ":
m.tree.ToggleExpand()
case "enter":
	sel := m.tree.SelectedItem()
	if sel == nil {
		break
	}
	switch sel.Kind {
	case SelCommand:
		if !sel.Node.Virtual {
			m.preview.SetCommand(sel.Node.FullCommand())
			m.tree.SetCmdTokens(m.preview.Tokens())
			m.statusMsg = "set: " + sel.Node.FullCommand()
		}
	case SelFlag:
		vt := strings.ToLower(sel.Flag.ValueType)
		if vt == "" || vt == "bool" {
			if !isFlagActive(*sel.Flag, m.preview.Tokens()) {
				m.preview.AppendToken(sel.Flag.Name)
				m.tree.SetCmdTokens(m.preview.Tokens())
				m.statusMsg = "added: " + sel.Flag.Name
			}
		} else {
			m.openValueModal(sel.Flag)
		}
	case SelPositional:
		m.openPositionalModal(sel.Positional)
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
m.tree.Right()
case " ":
m.tree.ToggleExpand()
case "enter":
	sel := m.tree.SelectedItem()
	if sel == nil {
		break
	}
	switch sel.Kind {
	case SelCommand:
		if !sel.Node.Virtual {
			m.preview.SetCommand(sel.Node.FullCommand())
			m.tree.SetCmdTokens(m.preview.Tokens())
			m.statusMsg = "set: " + sel.Node.FullCommand()
		}
	case SelFlag:
		vt := strings.ToLower(sel.Flag.ValueType)
		if vt == "" || vt == "bool" {
			if !isFlagActive(*sel.Flag, m.preview.Tokens()) {
				m.preview.AppendToken(sel.Flag.Name)
				m.tree.SetCmdTokens(m.preview.Tokens())
				m.statusMsg = "added: " + sel.Flag.Name
			}
		} else {
			m.openValueModal(sel.Flag)
		}
	case SelPositional:
		m.openPositionalModal(sel.Positional)
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
m.tree.Left()
case "d":
m.tree.Right()
case " ":
m.tree.ToggleExpand()
case "enter":
	sel := m.tree.SelectedItem()
	if sel == nil {
		break
	}
	switch sel.Kind {
	case SelCommand:
		if !sel.Node.Virtual {
			m.preview.SetCommand(sel.Node.FullCommand())
			m.tree.SetCmdTokens(m.preview.Tokens())
			m.statusMsg = "set: " + sel.Node.FullCommand()
		}
	case SelFlag:
		vt := strings.ToLower(sel.Flag.ValueType)
		if vt == "" || vt == "bool" {
			if !isFlagActive(*sel.Flag, m.preview.Tokens()) {
				m.preview.AppendToken(sel.Flag.Name)
				m.tree.SetCmdTokens(m.preview.Tokens())
				m.statusMsg = "added: " + sel.Flag.Name
			}
		} else {
			m.openValueModal(sel.Flag)
		}
	case SelPositional:
		m.openPositionalModal(sel.Positional)
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
		m.tree.ToggleSectionAtY(y - previewBarHeight - 1)
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
	sel := m.tree.SelectedItem()
	if sel == nil {
		return
	}
	switch sel.Kind {
	case SelCommand:
		m.helpPane.SetNode(sel.Node)
	case SelFlag:
		m.helpPane.SetFlagContext(sel.Flag, sel.Owner)
	case SelPositional:
		m.helpPane.SetPositionalContext(sel.Positional, sel.Owner)
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
	if m.vm.active {
		return m.renderValueInputModal()
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
	if sel := m.tree.SelectedItem(); sel != nil {
		switch sel.Kind {
		case SelFlag:
			selected = sel.Flag.Name
			if sel.Flag.ValueType != "" {
				selected += " (" + sel.Flag.ValueType + ")"
			}
		case SelPositional:
			selected = "<" + sel.Positional.Name + ">"
		default:
			if sel.Node != nil {
				selected = sel.Node.FullCommand()
			}
		}
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

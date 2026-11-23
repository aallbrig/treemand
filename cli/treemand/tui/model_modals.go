package tui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aallbrig/treemand/models"
)

// ---------- key-bindings help modal (??) ----------

// keybindModal is the ? key overlay showing all keyboard shortcuts.
type keybindModal struct {
	active bool
	offset int
}

const keybindText = `Navigation
  ↑ / ↓    or  k / j   or  w / s    Move up / down
  → / l / d                          Expand node; enter children (2nd press)
  ← / h / a                          Collapse node; go to parent (2nd press)
  Shift+→ / Shift+L / Shift+D        Expand entire subtree
  Shift+← / Shift+H / Shift+A        Collapse entire subtree
  PgUp / PgDn  or  Ctrl+U / Ctrl+D  Move up / down a screenful
  gg                                  Jump to top
  G                                   Jump to bottom
  Space                               Toggle expand / collapse

Tree
  /        Fuzzy filter tree (Esc clears, Enter keeps)
  Ctrl+F   Global fuzzy find: commands, flags, args, help text
             (Tab inside: index all undiscovered nodes in background)
  n / N    Next / previous search match
  e / E    Expand all / collapse all
  S        Toggle section headers
  t / T    Cycle display style (default → columns → compact → graph)
  r / R    Re-discover selected node (refresh children)

Building Commands
  Enter    Set command / add flag / fill positional
  f / F    Open flag picker modal
  Backspace / Delete  Remove last token from preview
  Ctrl+K   Clear entire preview bar
  Ctrl+E   Copy or execute the assembled command
  Ctrl+R   Recent commands (Enter loads one into the preview)
  Ctrl+O   Saved presets: load, save (a) or delete (x) named commands

View
  H / Ctrl+P   Toggle help pane
  Tab / Shift+Tab  Cycle pane focus
  d / D    Open docs URL in browser
  Ctrl+S   Cycle navigation scheme (arrows → vim → WASD)
  ?        Show this help

Mouse
  Click        Select a node or flag
  Click ▶ / ▼  Expand / collapse
  Wheel        Scroll the tree

Quit
  q / Esc / Ctrl+C    Quit`

func (m *Model) updateKeybindModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "q", "?":
		m.kb.active = false
	case "up", "k":
		if m.kb.offset > 0 {
			m.kb.offset--
		}
	case "down", "j":
		m.kb.offset++
	case "pgup", "b", "ctrl+u":
		m.kb.offset -= 10
		if m.kb.offset < 0 {
			m.kb.offset = 0
		}
	case "pgdown", "ctrl+d":
		m.kb.offset += 10
	}
	return m, nil
}

// wrapHanging wraps one line of the key bindings text to width columns. An
// entry "  keys      description" continues under its description column, so a
// wrapped description stays readable; other lines wrap plainly.
func wrapHanging(line string, width int) []string {
	if lipgloss.Width(line) <= width {
		return []string{line}
	}
	lead := len(line) - len(strings.TrimLeft(line, " "))
	gap := strings.Index(line[lead:], "  ")
	if gap < 0 {
		return strings.Split(lipgloss.NewStyle().Width(width).Render(line), "\n")
	}
	start := lead + gap
	for start < len(line) && line[start] == ' ' {
		start++
	}
	prefix := line[:start]
	col := lipgloss.Width(prefix)
	if col >= width-10 {
		return strings.Split(lipgloss.NewStyle().Width(width).Render(line), "\n")
	}
	indent := strings.Repeat(" ", col)
	var rows []string
	row, empty := prefix, true
	for _, word := range strings.Fields(line[start:]) {
		if !empty && lipgloss.Width(row)+1+lipgloss.Width(word) > width {
			rows = append(rows, row)
			row, empty = indent, true
		}
		if !empty {
			row += " "
		}
		row += word
		empty = false
	}
	return append(rows, row)
}

func (m *Model) renderKeybindModal() string {
	modalW := min(m.width-6, 76)
	if modalW < 40 {
		modalW = 40
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
	hintStyle := lipgloss.NewStyle().Faint(true)

	// Wrap each source line to the box's inner width first, so one entry is one
	// screen row and the scroll window below counts rows, not source lines.
	innerW := modalW - 6 // border (2) + horizontal padding (4)
	var lines []string
	for _, src := range strings.Split(keybindText, "\n") {
		lines = append(lines, wrapHanging(src, innerW)...)
	}
	maxVisible := m.height - 10
	if maxVisible < 5 {
		maxVisible = 5
	}
	if m.kb.offset > len(lines)-maxVisible {
		m.kb.offset = max(0, len(lines)-maxVisible)
	}
	end := m.kb.offset + maxVisible
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[m.kb.offset:end]

	scrollHint := ""
	if len(lines) > maxVisible {
		scrollHint = fmt.Sprintf(" [%d/%d]", m.kb.offset+1, len(lines))
	}

	content := titleStyle.Render("Key Bindings"+scrollHint) + "\n" +
		hintStyle.Render("↑↓/jk scroll · Esc close") + "\n\n" +
		strings.Join(visible, "\n")

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

// ---------- value input modal ----------

func (m *Model) updateValueModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.ensureCommandBase(m.vm.owner)
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

func (m *Model) openValueModal(f *models.Flag, owner *models.Node) {
	vi := textinput.New()
	vi.Placeholder = "value…"
	vi.CharLimit = 256
	vi.Focus()
	m.vm = valueInputModal{
		active: true,
		label:  f.Name + " <" + f.ValueType + ">",
		prefix: f.Name + "=",
		input:  vi,
		owner:  owner,
	}
}

func (m *Model) openPositionalModal(p *models.Positional, owner *models.Node) {
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
		owner:  owner,
	}
}

// ---------- ensureCommandBase ----------

// ensureCommandBase ensures the draft command in the preview starts with the full
// subcommand chain that owns the flag/positional being added. This lets users add
// flags without first pressing Enter on every ancestor subcommand.
func (m *Model) ensureCommandBase(owner *models.Node) {
	if owner == nil {
		return
	}
	full := owner.FullCommand()
	ownerToks := strings.Fields(full)
	current := m.preview.Tokens()
	if len(current) >= len(ownerToks) {
		match := true
		for i, t := range ownerToks {
			if current[i] != t {
				match = false
				break
			}
		}
		if match {
			return
		}
	}
	m.preview.SetCommand(full)
	m.tree.SetCmdTokens(m.preview.Tokens())
}

// ---------- execute modal ----------

func (m *Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		m.modal.active = false
		m.statusMsg = "cancelled"
	case "enter", "r", "R":
		m.commandToRun = m.modal.command
		m.recordHistory(m.modal.command)
		m.modal.active = false
		m.quitting = true
		return m, tea.Quit
	case "c", "C":
		if err := clipboard.WriteAll(m.modal.command); err != nil {
			m.statusMsg = "copy failed: " + err.Error()
		} else {
			m.statusMsg = "copied: " + m.modal.command
			m.recordHistory(m.modal.command)
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

	return m.centerBox(box)
}

// centerBox places a dialog box in the middle of the screen.
func (m *Model) centerBox(box string) string {
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

// ---------- flag picker modal ----------

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
	awaitingValue bool // true when prompting the user to type a value
	awaitingIdx   int  // index of the entry awaiting a value
	valueInput    textinput.Model
	owner         *models.Node // node whose flags are shown
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

	m.fm = flagModal{active: true, entries: entries, valueInput: vi, owner: node}
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
			m.ensureCommandBase(m.fm.owner)
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
			m.ensureCommandBase(m.fm.owner)
			m.preview.AppendToken(e.flag.Name)
			m.tree.SetCmdTokens(m.preview.Tokens())
			m.fm.entries[m.fm.cursor].added = true
			m.statusMsg = "added: " + e.flag.Name
		}
	}
	return m, nil
}

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
  gg                                  Jump to top
  G                                   Jump to bottom

Tree
  /        Fuzzy filter
  n / N    Next / previous search match
  e / E    Expand all / collapse all
  S        Toggle section headers
  T        Cycle display style (default → columns → compact → graph)
  R        Re-discover selected node (refresh children)

Building Commands
  Enter    Set command / add flag / fill positional
  f / F    Open flag picker modal
  Backspace  Remove last token from preview
  Ctrl+K   Clear entire preview bar
  Ctrl+E   Copy or execute the assembled command

View
  H / Ctrl+P   Toggle help pane
  Tab / Shift+Tab  Cycle pane focus
  d / D    Open docs URL in browser
  Ctrl+S   Cycle navigation scheme (arrows → vim → WASD)
  ?        Show this help

Quit
  q / Esc    Quit`

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

func (m *Model) renderKeybindModal() string {
	modalW := min(m.width-6, 64)
	if modalW < 40 {
		modalW = 40
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
	hintStyle := lipgloss.NewStyle().Faint(true)

	lines := strings.Split(keybindText, "\n")
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

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
	hintStyle := lipgloss.NewStyle().Faint(true)
	selStyle := lipgloss.NewStyle().Background(lipgloss.Color("#264F78")).Bold(true)
	addedStyle := lipgloss.NewStyle().Faint(true) // checkmark shown separately; no strikethrough
	globalStyle := lipgloss.NewStyle().Faint(true).Italic(true)
	descStyle := lipgloss.NewStyle().Faint(true)
	sepStyle := lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#888888"))

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

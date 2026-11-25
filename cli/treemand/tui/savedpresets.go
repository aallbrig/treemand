package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aallbrig/treemand/cli/treemand/savedpresets"
)

// presetsModal is the Ctrl+O list of saved presets: named assembled
// commands kept for this CLI.
type presetsModal struct {
	active  bool
	list    []savedpresets.Preset
	cursor  int
	naming  bool            // the name prompt for saving is open
	input   textinput.Model // the name being typed
	err     string          // why the last action failed
	note    string          // what the last action did
	command string          // the assembled command being saved
}

// SetSavedPresets turns on saved presets: Ctrl+O lists, loads, saves and
// deletes the presets in s. nil turns them off.
func (m *Model) SetSavedPresets(s *savedpresets.Store) { m.saved = s }

// loadCommand puts cmd into the assembled command, as loading the preset
// called name does.
func (m *Model) loadCommand(cmd, name string) {
	m.preview.SetCommand(cmd)
	m.tree.SetCmdTokens(m.preview.Tokens())
	m.statusMsg = "loaded preset " + name + ": " + cmd
}

// loadStartPreset loads p when the TUI opens (--preset). It says so with a
// timed message, which outlives the renders before the first resize.
func (m *Model) loadStartPreset(p savedpresets.Preset) {
	m.preview.SetCommand(p.Command)
	m.tree.SetCmdTokens(m.preview.Tokens())
	m.setTimedMsg("loaded preset " + p.Name + ": " + p.Command)
}

// openPresets shows the saved presets.
func (m *Model) openPresets() {
	if m.saved == nil {
		m.statusMsg = "presets are unavailable (no config directory)"
		return
	}
	list, err := m.saved.List()
	if err != nil {
		m.statusMsg = "presets unreadable: " + err.Error()
		return
	}
	m.pm = presetsModal{active: true, list: list}
}

func (m *Model) updatePresetsModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pm.naming {
		return m.updatePresetName(msg)
	}
	m.pm.err, m.pm.note = "", ""
	switch msg.String() {
	case "ctrl+c", "esc", "q", "ctrl+o":
		m.pm.active = false
	case "up", "k":
		if m.pm.cursor > 0 {
			m.pm.cursor--
		}
	case "down", "j":
		if m.pm.cursor < len(m.pm.list)-1 {
			m.pm.cursor++
		}
	case "enter":
		if m.pm.cursor < len(m.pm.list) {
			p := m.pm.list[m.pm.cursor]
			m.pm.active = false
			m.loadCommand(p.Command, p.Name)
		}
	case "a":
		return m, m.startPresetName()
	case "x", "delete":
		m.deleteSelectedPreset()
	}
	return m, nil
}

// startPresetName opens the name prompt for saving the assembled command.
func (m *Model) startPresetName() tea.Cmd {
	cmd := strings.Join(m.preview.Tokens(), " ")
	if cmd == "" {
		m.pm.err = "nothing to save: build a command first"
		return nil
	}
	in := textinput.New()
	in.Placeholder = "name"
	in.CharLimit = savedpresets.MaxNameLen
	in.Focus()
	m.pm.input = in
	m.pm.naming = true
	m.pm.command = cmd
	return textinput.Blink
}

func (m *Model) updatePresetName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.pm.naming, m.pm.err = false, ""
		return m, nil
	case "ctrl+c":
		m.pm.active, m.pm.naming = false, false
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.pm.input.Value())
		if err := savedpresets.ValidName(name); err != nil {
			m.pm.err = err.Error()
			return m, nil
		}
		if err := m.saved.Save(name, m.pm.command); err != nil {
			m.pm.err = "not saved: " + err.Error()
			return m, nil
		}
		m.pm.active, m.pm.naming = false, false
		m.statusMsg = "saved preset " + name + ": " + m.pm.command
		return m, nil
	}
	var cmd tea.Cmd
	m.pm.input, cmd = m.pm.input.Update(msg)
	return m, cmd
}

// deleteSelectedPreset removes the preset under the cursor.
func (m *Model) deleteSelectedPreset() {
	if m.pm.cursor >= len(m.pm.list) {
		return
	}
	name := m.pm.list[m.pm.cursor].Name
	if _, err := m.saved.Delete(name); err != nil {
		m.pm.err = "not deleted: " + err.Error()
		return
	}
	m.pm.list = append(m.pm.list[:m.pm.cursor:m.pm.cursor], m.pm.list[m.pm.cursor+1:]...)
	m.pm.cursor = max(0, min(m.pm.cursor, len(m.pm.list)-1))
	m.pm.note = "deleted preset " + name
}

func (m *Model) renderPresetsModal() string {
	modalW := max(30, min(m.width-8, 90))
	inner := modalW - 6

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
	selStyle := lipgloss.NewStyle().Bold(true).
		Background(lipgloss.Color(m.cfg.Colors.Selected)).
		Foreground(lipgloss.Color(m.cfg.Colors.SelectedText))
	hintStyle := lipgloss.NewStyle().Faint(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.cfg.Colors.Invalid))

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Saved Presets") + "\n\n")
	if m.pm.naming {
		sb.WriteString("Save " + truncate(m.pm.command, inner-5) + "\n")
		sb.WriteString("as: " + m.pm.input.View() + "\n")
	} else {
		m.renderPresetRows(&sb, inner, selStyle)
	}
	if m.pm.err != "" {
		sb.WriteString("\n" + errStyle.Render(truncate(m.pm.err, inner)) + "\n")
	} else if m.pm.note != "" {
		sb.WriteString("\n" + truncate(m.pm.note, inner) + "\n")
	}
	hint := "↑↓/j/k navigate  Enter load  a save current  x/Delete delete  Esc/q/Ctrl+O close"
	if m.pm.naming {
		hint = "Enter save  Esc back  (letters, digits, . _ -)"
	}
	sb.WriteString("\n" + hintStyle.Render(hint))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5EA4F5")).
		Padding(1, 2).
		Width(modalW - 2).
		Render(sb.String())
	return m.centerBox(box)
}

// renderPresetRows lists the presets that fit, keeping the cursor visible.
func (m *Model) renderPresetRows(sb *strings.Builder, inner int, selStyle lipgloss.Style) {
	if len(m.pm.list) == 0 {
		sb.WriteString("No presets saved. Build a command, then press a to save it.\n")
		return
	}
	nameW := 0
	for _, p := range m.pm.list {
		nameW = max(nameW, len(p.Name))
	}
	nameW = min(nameW, inner/3)
	rows := m.recentsRows()
	first := max(0, m.pm.cursor-rows+1)
	end := min(len(m.pm.list), first+rows)
	for i := first; i < end; i++ {
		p := m.pm.list[i]
		name := truncate(p.Name, nameW)
		line := name + strings.Repeat(" ", nameW-lipgloss.Width(name)) + "  " +
			truncate(p.Command, inner-4-nameW)
		if i == m.pm.cursor {
			sb.WriteString(selStyle.Render("▶ "+line) + "\n")
		} else {
			sb.WriteString("  " + line + "\n")
		}
	}
}

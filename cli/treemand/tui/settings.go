package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aallbrig/treemand/cli/treemand/config"
)

// settingsModal is the , settings screen: every key `treemand config`
// knows, edited in place and written to the same config file.
type settingsModal struct {
	active  bool
	list    []config.Setting
	cursor  int
	editing bool            // the value prompt is open
	input   textinput.Model // the value being typed
	err     string          // why the last edit was refused
	note    string          // what the last edit did
}

// SetSettings turns on the settings screen for the config file s. nil
// turns it off.
func (m *Model) SetSettings(s *config.Settings) { m.settings = s }

// openSettings shows the settings screen.
func (m *Model) openSettings() {
	if m.settings == nil {
		m.statusMsg = "settings are unavailable"
		return
	}
	list, err := m.settings.List()
	if err != nil {
		m.statusMsg = "config unreadable: " + err.Error()
		return
	}
	m.st = settingsModal{active: true, list: list}
}

func (m *Model) updateSettingsModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.st.editing {
		return m.updateSettingValue(msg)
	}
	m.st.err = ""
	switch msg.String() {
	case "ctrl+c", "esc", "q", ",":
		m.st.active = false
	case "up", "k":
		m.st.cursor = max(0, m.st.cursor-1)
	case "down", "j":
		m.st.cursor = min(len(m.st.list)-1, m.st.cursor+1)
	case "enter":
		return m, m.startSettingEdit()
	}
	return m, nil
}

// startSettingEdit opens the value prompt on the selected key.
func (m *Model) startSettingEdit() tea.Cmd {
	if m.st.cursor >= len(m.st.list) {
		return nil
	}
	in := textinput.New()
	in.Prompt = "value: "
	in.CharLimit = 256
	in.SetValue(m.st.list[m.st.cursor].Value)
	in.Focus()
	m.st.input, m.st.editing, m.st.note = in, true, ""
	return textinput.Blink
}

func (m *Model) updateSettingValue(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.st.editing, m.st.err = false, ""
		return m, nil
	case "ctrl+c":
		m.st.active, m.st.editing = false, false
		return m, nil
	case "enter":
		m.saveSetting(strings.TrimSpace(m.st.input.Value()))
		return m, nil
	}
	var cmd tea.Cmd
	m.st.input, cmd = m.st.input.Update(msg)
	return m, cmd
}

// saveSetting writes value for the selected key, applying it to the running
// TUI when the key is part of the theme or the layout.
func (m *Model) saveSetting(value string) {
	key := m.st.list[m.st.cursor].Key
	if err := m.settings.Set(key, value); err != nil {
		m.st.err = err.Error()
		return
	}
	list, err := m.settings.List()
	if err != nil {
		m.st.err = "saved, but the config is unreadable: " + err.Error()
		return
	}
	m.st.list, m.st.editing, m.st.err = list, false, ""
	m.st.note = "saved " + key + " = " + list[m.st.cursor].Value
	switch {
	case list[m.st.cursor].Source == config.SourceFlag || list[m.st.cursor].Source == config.SourceEnv:
		m.st.note += "; this run's " + string(list[m.st.cursor].Source) + " still wins"
	case config.ApplyLive(m.cfg, key, value):
		m.st.note += "; applied"
	default:
		m.st.note += "; applies on next run"
	}
}

func (m *Model) renderSettingsModal() string {
	modalW := max(30, min(m.width-8, 90))
	inner := modalW - 6

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
	selStyle := lipgloss.NewStyle().Bold(true).
		Background(lipgloss.Color(m.cfg.Colors.Selected)).
		Foreground(lipgloss.Color(m.cfg.Colors.SelectedText))
	hintStyle := lipgloss.NewStyle().Faint(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.cfg.Colors.Invalid))

	var sb strings.Builder
	pos := fmt.Sprintf("%d/%d", m.st.cursor+1, len(m.st.list))
	sb.WriteString(titleStyle.Render("Settings") + " " + pos + "  " +
		hintStyle.Render(truncate(m.settings.Path, max(1, inner-12-len(pos)))) + "\n\n")
	m.renderSettingRows(&sb, inner, selStyle)
	if m.st.cursor < len(m.st.list) {
		what, values := settingDetail(m.st.list[m.st.cursor])
		sb.WriteString("\n" + truncate(what, inner) + "\n" + hintStyle.Render(truncate(values, inner)) + "\n")
	}
	if m.st.editing {
		sb.WriteString(m.st.input.View() + "\n")
	}
	if m.st.err != "" {
		sb.WriteString(errStyle.Render(truncate(m.st.err, inner)) + "\n")
	} else if m.st.note != "" {
		sb.WriteString(truncate(m.st.note, inner) + "\n")
	}
	hint := "↑↓/j/k navigate  Enter edit  Esc/q/, close"
	if m.st.editing {
		hint = "Enter save  Esc back"
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

// settingDetail describes a key in two lines: what it does, then its
// default and the values it takes.
func settingDetail(s config.Setting) (what, values string) {
	values = "default: " + s.Default
	if len(s.AllowedValues) > 0 {
		values += "  ·  one of: " + strings.Join(s.AllowedValues, ", ")
	}
	return s.Description, values
}

// renderSettingRows lists the keys that fit, keeping the cursor visible:
// key, value and where the value comes from.
func (m *Model) renderSettingRows(sb *strings.Builder, inner int, selStyle lipgloss.Style) {
	keyW := 0
	for _, s := range m.st.list {
		keyW = max(keyW, len(s.Key))
	}
	keyW = min(keyW, inner/2)
	rows := max(1, min(15, m.height-16))
	first := max(0, m.st.cursor-rows+1)
	end := min(len(m.st.list), first+rows)
	for i := first; i < end; i++ {
		s := m.st.list[i]
		key := truncate(s.Key, keyW)
		source := "(" + string(s.Source) + ")"
		valW := max(1, inner-4-keyW-len(source)-2)
		val := truncate(s.Value, valW)
		line := key + strings.Repeat(" ", keyW-lipgloss.Width(key)) + "  " +
			val + strings.Repeat(" ", valW-lipgloss.Width(val)) + "  " + source
		if i == m.st.cursor {
			sb.WriteString(selStyle.Render("▶ "+line) + "\n")
		} else {
			sb.WriteString("  " + line + "\n")
		}
	}
}

package tui

// A miniature TUI for the keymap probe's tests. Every finding the probe can
// report appears here exactly once; see main_test.go.

const keybindText = `Navigation
  ↑ / ↓  or  k / j    Move up / down
  Shift+L             Expand subtree
  x                   Documented but bound nowhere

Mouse
  Click               Select`

func (m *Model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "q":
		return m, tea.Quit
	case "j":
		m.down()
		return m, nil
	case "d":
		if m.wasd {
			return m, nil
		}
	case "shift+l":
		return m, nil
	case "z":
		return m, nil
	}
	return m.handleVim(msg)
}

func (m *Model) handleVim(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "k", "up":
		m.up()
	case "j", "down":
		m.down()
	case "d":
		m.right()
	}
	return m, nil
}

func (m *Model) schemeHints() string {
	switch m.scheme {
	case SchemeVim:
		return "j/k:nav  y:yank  q:quit"
	default:
		return "↑↓:nav  q:quit"
	}
}

func (m *Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "c":
		m.close()
	}
	return m, nil
}

func (m *Model) renderModal() string {
	return hintStyle.Render("[Esc] close") + "#5EA4F5"
}

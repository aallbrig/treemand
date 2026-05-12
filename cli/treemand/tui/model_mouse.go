package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		m.handleMouseClick(msg.X, msg.Y)

	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelUp:
		if m.showHelpPane && m.width > 0 && msg.X >= m.treeWidth() {
			m.helpPane.ScrollUp(3)
		} else {
			m.tree.Up()
			m.syncSelected()
		}

	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelDown:
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
		// Account for the tree pane's border (1 row).
		treeY := y - previewBarHeight - 1
		if m.tree.SelectAtY(treeY) {
			m.syncSelected()
		} else {
			m.tree.ToggleSectionAtY(treeY)
		}
	}
	m.statusMsg = "focus: " + paneName(m.focusedPane)
}

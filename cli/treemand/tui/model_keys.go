package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/config"
)

// updateKeys is the main key dispatcher when no modal is active.
func (m *Model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending 'g' for gg (jump to top) sequence.
	if m.pendingG {
		m.pendingG = false
		if key == "g" {
			m.tree.Top()
			m.syncSelected()
			return m, nil
		}
		// Not 'g' — cancel pending and process this key normally.
	}

	switch key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		return m.handleEsc()

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

	case "t", "T":
		next := config.DisplayStyle((int(m.cfg.TreeStyle) + 1) % len(config.DisplayStyleNames))
		m.cfg.TreeStyle = next
		m.tree.SetDisplayStyle(next)
		m.setTimedMsg("style: " + config.DisplayStyleNames[next])
		return m, m.timedMsgCmd()

	case "S":
		m.tree.ToggleSections()
		if m.tree.SectionsHidden() {
			m.statusMsg = "sections: hidden"
		} else {
			m.statusMsg = "sections: visible"
		}
		return m, nil

	// Help pane toggle: H (uppercase) and ctrl+p only.
	// Lowercase h is reserved for Left navigation in vim mode.
	case "H", "ctrl+p":
		m.showHelpPane = !m.showHelpPane
		m.applyLayout()
		return m, nil

	case "/":
		m.filtering = true
		m.filter.Focus()
		return m, textinput.Blink

	case "?":
		m.kb.active = true
		m.kb.offset = 0
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

	case "ctrl+k":
		m.preview.ClearAll()
		m.tree.SetCmdTokens(nil)
		m.statusMsg = "cleared command"
		return m, nil

	case "backspace", "delete":
		m.preview.RemoveLastToken()
		m.tree.SetCmdTokens(m.preview.Tokens())
		m.statusMsg = "removed last token"
		return m, nil

	case "r", "R":
		return m, m.forceExpandSelected()

	case "f", "F":
		m.openFlagModal()
		return m, nil

	case "d":
		if m.scheme != SchemeWASD {
			return m, m.openDocsURL()
		}
		// In WASD mode, lowercase 'd' is Right navigation — fall through to scheme handler.
	case "D":
		return m, m.openDocsURL()

	// e/E: expand all / collapse all (global).
	case "e":
		m.tree.ExpandAll()
		m.statusMsg = "expanded all"
		return m, nil
	case "E":
		m.tree.CollapseAll()
		m.statusMsg = "collapsed all"
		return m, nil

	// Shift+Right/Left: expand/collapse subtree under the current node.
	case "shift+right", "shift+l", "shift+d":
		if node := m.tree.SelectedOrOwner(); node != nil {
			m.tree.ExpandAllFrom(node, m.tree.SelectedCommandDepth())
			m.tree.Rebuild()
			m.statusMsg = "expanded: " + node.Name
		}
		return m, nil

	case "shift+left", "shift+h", "shift+a":
		if node := m.tree.SelectedOrOwner(); node != nil {
			m.tree.CollapseSubtree(node, m.tree.SelectedCommandDepth())
			m.tree.Rebuild()
			m.statusMsg = "collapsed: " + node.Name
		}
		return m, nil

	// G: jump to last row.
	case "G":
		m.tree.Bottom()
		m.syncSelected()
		return m, nil

	// g: first press sets pendingG; second 'g' triggers jump to top.
	case "g":
		m.pendingG = true
		return m, nil

	// n/N: cycle through search matches.
	case "n":
		if m.lastSearch != "" {
			if m.tree.NextMatch(m.lastSearch) {
				m.syncSelected()
			}
		}
		return m, nil
	case "N":
		if m.lastSearch != "" {
			if m.tree.PrevMatch(m.lastSearch) {
				m.syncSelected()
			}
		}
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

// handlePick handles the Enter/pick action shared across all nav schemes.
func (m *Model) handlePick() {
	// If cursor is on a section header, toggle it.
	if m.tree.ToggleSelectedSection() {
		return
	}
	sel := m.tree.SelectedItem()
	if sel == nil {
		return
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
				m.ensureCommandBase(sel.Owner)
				m.preview.AppendToken(sel.Flag.Name)
				m.tree.SetCmdTokens(m.preview.Tokens())
				m.statusMsg = "added: " + sel.Flag.Name
			}
		} else {
			m.openValueModal(sel.Flag, sel.Owner)
		}
	case SelPositional:
		m.openPositionalModal(sel.Positional, sel.Owner)
	}
}

// handleEsc implements Esc as "back one level" in the tree pane:
//   - On an expanded node: collapse it (same as Left)
//   - On a collapsed child node: jump to parent (same as Left)
//   - On root (nowhere to go back): quit
func (m *Model) handleEsc() (tea.Model, tea.Cmd) {
	if m.focusedPane != paneTree {
		// From other panes, Esc returns to tree.
		m.setFocus(paneTree)
		m.statusMsg = "focus: tree"
		return m, nil
	}
	sel := m.tree.SelectedItem()
	if sel != nil && sel.Kind == SelCommand && sel.Node != nil && len(sel.Node.FullPath) <= 1 {
		// On root node — quit.
		m.quitting = true
		return m, tea.Quit
	}
	// Otherwise, behave like Left (collapse or jump to parent).
	m.tree.Left()
	m.syncSelected()
	return m, nil
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
		m.handlePick()
	}
	m.syncSelected()
	return m, m.lazyExpandIfStub()
}

func (m *Model) handleVim(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "k":
		m.tree.Up()
	case "j":
		m.tree.Down()
	case "h":
		m.tree.Left()
	case "l":
		m.tree.Right()
	case " ":
		m.tree.ToggleExpand()
	case "enter":
		m.handlePick()
	}
	m.syncSelected()
	return m, m.lazyExpandIfStub()
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
		m.handlePick()
	}
	m.syncSelected()
	return m, m.lazyExpandIfStub()
}

func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.filtering = false
		m.filter.Blur()
		if v := m.filter.Value(); v != "" {
			m.lastSearch = v
		}
		m.tree.SetFilter(m.filter.Value())
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.tree.SetFilter(m.filter.Value())
	return m, cmd
}

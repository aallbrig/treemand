package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/cli/treemand/config"
)

// actionFuncs runs each action of the key map (keymap.go).
var actionFuncs = map[action]func(*Model) tea.Cmd{
	actQuit:            (*Model).quit,
	actBack:            func(m *Model) tea.Cmd { _, cmd := m.handleEsc(); return cmd },
	actFocusNext:       func(m *Model) tea.Cmd { m.cycleFocus(1); return nil },
	actFocusPrev:       func(m *Model) tea.Cmd { m.cycleFocus(-1); return nil },
	actCycleScheme:     (*Model).cycleScheme,
	actCycleStyle:      (*Model).cycleStyle,
	actToggleSections:  (*Model).toggleSections,
	actToggleHelpPane:  (*Model).toggleHelpPane,
	actFilter:          (*Model).openFilter,
	actSearch:          func(m *Model) tea.Cmd { m.openSearchModal(); return textinput.Blink },
	actKeys:            func(m *Model) tea.Cmd { m.kb.active = true; m.kb.offset = 0; return nil },
	actExec:            func(m *Model) tea.Cmd { m.openExec(true); return nil },
	actExecPreview:     func(m *Model) tea.Cmd { m.openExec(false); return nil },
	actRecents:         func(m *Model) tea.Cmd { m.openRecents(); return nil },
	actPresets:         func(m *Model) tea.Cmd { m.openPresets(); return nil },
	actSettings:        func(m *Model) tea.Cmd { m.openSettings(); return nil },
	actClear:           (*Model).clearPreview,
	actRemoveToken:     (*Model).removeLastToken,
	actRediscover:      (*Model).forceExpandSelected,
	actFlags:           func(m *Model) tea.Cmd { m.openFlagModal(); return nil },
	actDocs:            (*Model).openDocsURL,
	actExpandAll:       (*Model).expandAll,
	actCollapseAll:     (*Model).collapseAll,
	actExpandSubtree:   (*Model).expandSubtree,
	actCollapseSubtree: (*Model).collapseSubtree,
	actPendingTop:      func(m *Model) tea.Cmd { m.pendingG = true; return nil },
	actBottom:          func(m *Model) tea.Cmd { m.tree.Bottom(); m.syncSelected(); return nil },
	actPageDown:        func(m *Model) tea.Cmd { m.tree.PageDown(); return m.afterNav() },
	actPageUp:          func(m *Model) tea.Cmd { m.tree.PageUp(); return m.afterNav() },
	actNextMatch:       func(m *Model) tea.Cmd { m.jumpMatch(m.tree.NextMatch); return nil },
	actPrevMatch:       func(m *Model) tea.Cmd { m.jumpMatch(m.tree.PrevMatch); return nil },
	actUp:              func(m *Model) tea.Cmd { m.tree.Up(); return m.afterNav() },
	actDown:            func(m *Model) tea.Cmd { m.tree.Down(); return m.afterNav() },
	actLeft:            func(m *Model) tea.Cmd { m.tree.Left(); return m.afterNav() },
	actRight:           func(m *Model) tea.Cmd { m.tree.Right(); return m.afterNav() },
	actToggle:          func(m *Model) tea.Cmd { m.tree.ToggleExpand(); return m.afterNav() },
	actPick:            func(m *Model) tea.Cmd { m.handlePick(); return m.afterNav() },
	actScrollUp:        func(m *Model) tea.Cmd { m.helpPane.ScrollUp(1); return nil },
	actScrollDown:      func(m *Model) tea.Cmd { m.helpPane.ScrollDown(1); return nil },
	actScrollPageUp:    func(m *Model) tea.Cmd { m.helpPane.PageUp(); return nil },
	actScrollPageDown:  func(m *Model) tea.Cmd { m.helpPane.PageDown(); return nil },
	actScrollTop:       func(m *Model) tea.Cmd { m.helpPane.Top(); return nil },
	actScrollBottom:    func(m *Model) tea.Cmd { m.helpPane.Bottom(); return nil },
	actFilterKeep:      (*Model).keepFilter,
	actFilterClear:     (*Model).clearFilter,
}

// run runs act and reports whether there was one to run.
func (m *Model) run(act action) (tea.Cmd, bool) {
	fn, ok := actionFuncs[act]
	if !ok {
		return nil, false
	}
	return fn(m), true
}

// updateKeys is the main key dispatcher when no modal is active: the
// navigation scheme's keys, then the tree's.
func (m *Model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.takeKeyBeforeGlobals(key) {
		return m, nil
	}
	// The scheme's keys sit over the tree's, as layers says; the scheme's
	// navigation keys do nothing while the help pane has focus.
	modes := layers(schemeMode(m.scheme))
	if m.focusedPane == paneHelp {
		modes = []string{ModeTree}
	}
	for _, mode := range modes {
		if cmd, ok := m.run(lookupKey(mode, key)); ok {
			return m, cmd
		}
	}
	if m.focusedPane == paneHelp {
		return m, nil
	}
	return m, m.afterNav()
}

// takeKeyBeforeGlobals handles the keys that must run before the global ones
// and reports whether key was used: the second g of gg, and the focused help
// pane's scroll keys, which the global keys would use to move the tree.
func (m *Model) takeKeyBeforeGlobals(key string) bool {
	if m.pendingG {
		m.pendingG = false
		if key == "g" {
			m.tree.Top()
			m.syncSelected()
			return true
		}
		// Not g: cancel the pending g and process this key normally.
	}
	if m.focusedPane != paneHelp {
		return false
	}
	_, ok := m.run(lookupKey(ModeHelpPane, key))
	return ok
}

// updatePreviewInput handles keys while the preview line has focus: the
// preview mode's keys, and typing for the rest.
func (m *Model) updatePreviewInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, ok := m.run(lookupKey(ModePreview, msg.String())); ok {
		return m, cmd
	}
	cmd := m.preview.Update(msg)
	m.tree.SetCmdTokens(m.preview.Tokens())
	return m, cmd
}

// updateFilter handles keys while the filter line is open: Enter and Esc
// close it, the rest edit the filter.
func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, ok := m.run(lookupKey(ModeFilter, msg.String())); ok {
		return m, cmd
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.tree.SetFilter(m.filter.Value())
	return m, cmd
}

// afterNav follows a move in the tree: the selection, then discovery of a
// stub the cursor landed on.
func (m *Model) afterNav() tea.Cmd {
	m.syncSelected()
	return m.lazyExpandIfStub()
}

func (m *Model) quit() tea.Cmd {
	m.quitting = true
	return tea.Quit
}

func (m *Model) cycleScheme() tea.Cmd {
	m.scheme = (m.scheme + 1) % 3
	m.statusMsg = "nav: " + schemeName(m.scheme)
	return nil
}

func (m *Model) cycleStyle() tea.Cmd {
	next := config.DisplayStyle((int(m.cfg.TreeStyle) + 1) % len(config.DisplayStyleNames))
	m.cfg.TreeStyle = next
	m.tree.SetDisplayStyle(next)
	m.setTimedMsg("style: " + config.DisplayStyleNames[next])
	return m.timedMsgCmd()
}

func (m *Model) toggleSections() tea.Cmd {
	m.tree.ToggleSections()
	if m.tree.SectionsHidden() {
		m.statusMsg = "sections: hidden"
	} else {
		m.statusMsg = "sections: visible"
	}
	return nil
}

func (m *Model) toggleHelpPane() tea.Cmd {
	m.showHelpPane = !m.showHelpPane
	m.applyLayout()
	return nil
}

func (m *Model) openFilter() tea.Cmd {
	m.filtering = true
	m.filter.Focus()
	return textinput.Blink
}

// openExec opens the copy-or-run dialog on the preview's command; from the
// tree, an empty preview falls back to the selected node's command.
func (m *Model) openExec(fallbackToSelection bool) {
	cmd := strings.Join(m.preview.Tokens(), " ")
	if cmd == "" && fallbackToSelection {
		if node := m.tree.Selected(); node != nil {
			cmd = node.FullCommand()
		}
	}
	m.modal.command = cmd
	m.modal.active = true
}

func (m *Model) clearPreview() tea.Cmd {
	m.preview.ClearAll()
	m.tree.SetCmdTokens(nil)
	m.statusMsg = "cleared command"
	return nil
}

func (m *Model) removeLastToken() tea.Cmd {
	m.preview.RemoveLastToken()
	m.tree.SetCmdTokens(m.preview.Tokens())
	m.statusMsg = "removed last token"
	return nil
}

func (m *Model) expandAll() tea.Cmd {
	m.tree.ExpandAll()
	m.statusMsg = "expanded all"
	return nil
}

func (m *Model) collapseAll() tea.Cmd {
	m.tree.CollapseAll()
	m.statusMsg = "collapsed all"
	return nil
}

func (m *Model) expandSubtree() tea.Cmd {
	if node := m.tree.SelectedOrOwner(); node != nil {
		m.tree.ExpandAllFrom(node, m.tree.SelectedCommandDepth())
		m.tree.Rebuild()
		m.statusMsg = "expanded: " + node.Name
	}
	return nil
}

func (m *Model) collapseSubtree() tea.Cmd {
	if node := m.tree.SelectedOrOwner(); node != nil {
		m.tree.CollapseSubtree(node, m.tree.SelectedCommandDepth())
		m.tree.Rebuild()
		m.statusMsg = "collapsed: " + node.Name
	}
	return nil
}

// jumpMatch moves to the next or previous match of the last search.
func (m *Model) jumpMatch(step func(string) bool) {
	if m.lastSearch != "" && step(m.lastSearch) {
		m.syncSelected()
	}
}

func (m *Model) keepFilter() tea.Cmd {
	m.filtering = false
	m.filter.Blur()
	if v := m.filter.Value(); v != "" {
		m.lastSearch = v
	}
	m.tree.SetFilter(m.filter.Value())
	return nil
}

func (m *Model) clearFilter() tea.Cmd {
	m.filtering = false
	m.filter.Blur()
	m.filter.SetValue("")
	m.tree.SetFilter("")
	return nil
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

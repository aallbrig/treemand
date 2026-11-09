package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const previewBarHeight = 2

func (m *Model) applyLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	cH := m.contentHeight()
	m.tree.SetSize(m.treeWidth(), cH)
	m.helpPane.SetSize(m.helpWidth(), cH)
}

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

func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	if m.sm.active {
		return m.renderSearchModal()
	}
	if m.kb.active {
		return m.renderKeybindModal()
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
	// Left side: what is currently selected / focused item context.
	selected := ""
	if sel := m.tree.SelectedItem(); sel != nil {
		switch sel.Kind {
		case SelFlag:
			selected = sel.Flag.Name
			if sel.Flag.ValueType != "" && sel.Flag.ValueType != "bool" {
				selected += " <" + sel.Flag.ValueType + ">"
			}
			if sel.Owner != nil {
				selected = sel.Owner.Name + " " + selected
			}
		case SelPositional:
			selected = "<" + sel.Positional.Name + ">"
			if sel.Owner != nil {
				selected = sel.Owner.Name + " " + selected
			}
		default:
			if sel.Node != nil {
				selected = sel.Node.FullCommand()
			}
		}
	}
	leftStyle := lipgloss.NewStyle().Bold(true)
	if m.statusMsg != "" {
		leftStyle = leftStyle.Foreground(lipgloss.Color("#FFB86C"))
	}
	left := leftStyle.Render(selected)

	// Right side: context-sensitive key hints.
	// Priority: one-shot statusMsg > timed message (e.g. style name) > contextual hints.
	var hint string
	var hintStyle lipgloss.Style
	schemeIndicator := "[" + schemeName(m.scheme) + "] "
	switch {
	case m.statusMsg != "":
		hint = m.statusMsg
		m.statusMsg = ""
		hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C"))
	case m.timedMsg != "":
		hint = m.timedMsg
		hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9"))
	case m.filtering:
		hint = m.filter.View() + "  Enter:keep  Esc:clear"
		hintStyle = lipgloss.NewStyle().Faint(true)
	case m.focusedPane == panePreview:
		hint = "Esc:back  Ctrl+E:exec/copy  Tab:switch"
		hintStyle = lipgloss.NewStyle().Faint(true)
	case m.focusedPane == paneHelp:
		hint = "↑↓:scroll  PgUp/PgDn  g/G:top/bottom  Tab:switch"
		hintStyle = lipgloss.NewStyle().Faint(true)
	default:
		hint = schemeIndicator + fitHints(m.schemeHints(), m.width-lipgloss.Width(left)-3-len(schemeIndicator))
		hintStyle = lipgloss.NewStyle().Faint(true)
	}
	right := hintStyle.Render(hint)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// schemeHints returns the key-hint text adapted to the active navigation scheme.
func (m *Model) schemeHints() string {
	switch m.scheme {
	case SchemeVim:
		return "j/k:nav  h/l:expand/collapse  Enter:pick  e/E:expand/collapse all  Shift+h/l:subtree  S:sections  f:flags  /:filter  Ctrl+P:help  Ctrl+E:exec  gg/G:top/bottom  n/N:search  q:quit"
	case SchemeWASD:
		return "w/s:nav  a/d:expand/collapse  Enter:pick  e/E:expand/collapse all  Shift+a/d:subtree  S:sections  f:flags  /:filter  H:help  Ctrl+E:exec  gg/G:top/bottom  n/N:search  q:quit"
	default:
		return "↑↓:nav  ←→:expand/collapse  Enter:pick  e/E:expand/collapse all  Shift+←→:subtree  S:sections  f:flags  /:filter  H:help  Ctrl+E:exec  gg/G:top/bottom  n/N:search  q:quit"
	}
}

// hintDropOrder lists the indices of schemeHints tokens in the order they are
// dropped when the footer is too narrow (least important first).
var hintDropOrder = []int{11, 10, 9, 4, 3, 5, 6, 1}

// fitHints removes whole hint tokens (double-space separated) until the text
// fits in avail columns, so the footer never truncates mid-token.
func fitHints(hints string, avail int) string {
	tokens := strings.Split(hints, "  ")
	dropped := map[int]bool{}
	join := func() string {
		var kept []string
		for i, t := range tokens {
			if !dropped[i] {
				kept = append(kept, t)
			}
		}
		return strings.Join(kept, "  ")
	}
	out := join()
	for _, idx := range hintDropOrder {
		if lipgloss.Width(out) <= avail || idx >= len(tokens) {
			break
		}
		dropped[idx] = true
		out = join()
	}
	return out
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

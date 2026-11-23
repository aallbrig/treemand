package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aallbrig/treemand/history"
)

// recentsModal is the Ctrl+R list of commands copied or run before.
type recentsModal struct {
	active  bool
	entries []history.Entry
	cursor  int
}

// SetHistory turns on command history: commands copied or run from the
// execute dialog are recorded in s, and Ctrl+R lists them. nil turns it off.
func (m *Model) SetHistory(s *history.Store) { m.hist = s }

// recordHistory remembers cmd; a failure only shows in the status bar.
func (m *Model) recordHistory(cmd string) {
	if m.hist == nil {
		return
	}
	if err := m.hist.Add(cmd, time.Now()); err != nil {
		m.statusMsg = "history not saved: " + err.Error()
	}
}

// openRecents shows the recent commands, newest first.
func (m *Model) openRecents() {
	if m.hist == nil {
		m.statusMsg = "history is off (no_history)"
		return
	}
	entries, err := m.hist.Recent()
	if err != nil {
		m.statusMsg = "history unreadable: " + err.Error()
		return
	}
	m.rm = recentsModal{active: true, entries: entries}
}

func (m *Model) updateRecentsModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "q", "ctrl+r":
		m.rm.active = false
	case "up", "k":
		if m.rm.cursor > 0 {
			m.rm.cursor--
		}
	case "down", "j":
		if m.rm.cursor < len(m.rm.entries)-1 {
			m.rm.cursor++
		}
	case "enter":
		m.rm.active = false
		if m.rm.cursor < len(m.rm.entries) {
			cmd := m.rm.entries[m.rm.cursor].Command
			m.preview.SetCommand(cmd)
			m.tree.SetCmdTokens(m.preview.Tokens())
			m.statusMsg = "loaded: " + cmd
		}
	}
	return m, nil
}

// recentsRows is how many commands the dialog lists at once.
func (m *Model) recentsRows() int {
	return max(3, min(15, m.height-10))
}

func (m *Model) renderRecentsModal() string {
	modalW := max(30, min(m.width-8, 90))
	inner := modalW - 6

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
	selStyle := lipgloss.NewStyle().Bold(true).
		Background(lipgloss.Color(m.cfg.Colors.Selected)).
		Foreground(lipgloss.Color(m.cfg.Colors.SelectedText))
	hintStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Recent Commands") + "\n\n")
	if len(m.rm.entries) == 0 {
		sb.WriteString("No commands yet. Copy or run one with Ctrl+E.\n")
	}
	rows := m.recentsRows()
	first := max(0, m.rm.cursor-rows+1)
	end := min(len(m.rm.entries), first+rows)
	for i := first; i < end; i++ {
		e := m.rm.entries[i]
		count := ""
		if e.Count > 1 {
			count = fmt.Sprintf(" ×%d", e.Count)
		}
		line := truncate(e.Command, inner-2-lipgloss.Width(count)) + count
		if i == m.rm.cursor {
			sb.WriteString(selStyle.Render("▶ "+line) + "\n")
		} else {
			sb.WriteString("  " + line + "\n")
		}
	}
	sb.WriteString("\n" + hintStyle.Render("↑↓/j/k navigate  Enter load into preview  Esc/q/Ctrl+R close"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5EA4F5")).
		Padding(1, 2).
		Width(modalW - 2).
		Render(sb.String())
	return m.centerBox(box)
}

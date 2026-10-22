package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aallbrig/treemand/fuzzy"
	"github.com/aallbrig/treemand/models"
)

const searchMaxResults = 200

// searchModal is the Ctrl+F global fuzzy finder over commands, flags,
// positionals, descriptions and help text.
type searchModal struct {
	active  bool
	input   textinput.Model
	entries []fuzzy.Entry
	results []fuzzy.Result
	cursor  int
	offset  int
	stubs   int // undiscovered (stub) nodes that cannot be searched yet

	indexing   bool // background "index all" run in progress
	indexed    int
	indexTotal int
}

// openSearchModal indexes the current tree and activates the finder.
func (m *Model) openSearchModal() {
	in := textinput.New()
	in.Prompt = "🔍 "
	in.Placeholder = "search commands, flags, help text…"
	in.CharLimit = 128
	in.Focus()

	entries := fuzzy.Build(m.root)
	prev := m.sm
	m.sm = searchModal{active: true, input: in, entries: entries, stubs: countStubs(m.root),
		indexing: prev.indexing, indexed: prev.indexed, indexTotal: prev.indexTotal}
}

func (m *Model) updateSearchModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.sm.active = false
		return m, nil
	case "up", "ctrl+p", "ctrl+k":
		if m.sm.cursor > 0 {
			m.sm.cursor--
		}
		return m, nil
	case "down", "ctrl+n", "ctrl+j":
		if m.sm.cursor < len(m.sm.results)-1 {
			m.sm.cursor++
		}
		return m, nil
	case "pgup":
		m.sm.cursor = max(0, m.sm.cursor-10)
		return m, nil
	case "pgdown":
		m.sm.cursor = min(max(0, len(m.sm.results)-1), m.sm.cursor+10)
		return m, nil
	case "tab":
		if m.sm.indexing || m.sm.stubs == 0 {
			return m, nil
		}
		return m, m.indexAllStubs()
	case "enter":
		if len(m.sm.results) == 0 {
			return m, nil
		}
		e := m.sm.results[m.sm.cursor].Entry
		m.sm.active = false
		return m.jumpToEntry(e)
	}
	var cmd tea.Cmd
	before := m.sm.input.Value()
	m.sm.input, cmd = m.sm.input.Update(msg)
	if m.sm.input.Value() != before {
		m.refreshSearch()
		m.sm.cursor, m.sm.offset = 0, 0
	}
	return m, cmd
}

// refreshSearch re-indexes the tree and re-runs the current query, keeping
// the cursor where it was when possible.
func (m *Model) refreshSearch() {
	if !m.sm.active {
		return
	}
	m.sm.entries = fuzzy.Build(m.root)
	m.sm.stubs = countStubs(m.root)
	m.sm.results = fuzzy.Search(m.sm.entries, m.sm.input.Value(), searchMaxResults)
	if m.sm.cursor >= len(m.sm.results) {
		m.sm.cursor = max(0, len(m.sm.results)-1)
	}
}

func countStubs(root *models.Node) int {
	n := 0
	root.Walk(func(x *models.Node) {
		if x.Stub {
			n++
		}
	})
	return n
}

// jumpToEntry reveals a search hit in the tree and focuses it.
func (m *Model) jumpToEntry(e fuzzy.Entry) (tea.Model, tea.Cmd) {
	m.filtering = false
	m.filter.SetValue("")
	var flagName, posName string
	switch e.Kind {
	case fuzzy.KindFlag:
		flagName = e.Name
	case fuzzy.KindPositional:
		posName = e.Name
	}
	if !m.tree.Reveal(e.Node, flagName, posName) {
		m.statusMsg = "not found in tree: " + e.Label()
		return m, nil
	}
	m.focusedPane = paneTree
	m.syncSelected()
	return m, m.lazyExpandIfStub()
}

var (
	searchHit   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#50FA7B"))
	searchBadge = map[fuzzy.Kind]string{
		fuzzy.KindCommand:    lipgloss.NewStyle().Foreground(lipgloss.Color("#5EA4F5")).Render("cmd "),
		fuzzy.KindFlag:       lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Render("flag"),
		fuzzy.KindPositional: lipgloss.NewStyle().Foreground(lipgloss.Color("#F1FA8C")).Render("arg "),
	}
)

// highlight renders s with the rune indices in idx emphasised.
func highlight(s string, idx []int) string {
	if len(idx) == 0 {
		return s
	}
	set := make(map[int]bool, len(idx))
	for _, i := range idx {
		set[i] = true
	}
	var sb strings.Builder
	for i, r := range []rune(s) {
		if set[i] {
			sb.WriteString(searchHit.Render(string(r)))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func truncate(s string, w int) string {
	r := []rune(s)
	if w <= 0 || len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

func (m *Model) renderSearchModal() string {
	modalW := min(m.width-4, 100)
	if modalW < 40 {
		modalW = 40
	}
	inner := modalW - 6
	vp := max(3, min(m.height-12, 18))

	if m.sm.cursor < m.sm.offset {
		m.sm.offset = m.sm.cursor
	}
	if m.sm.cursor >= m.sm.offset+vp {
		m.sm.offset = m.sm.cursor - vp + 1
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5"))
	faint := lipgloss.NewStyle().Faint(true)
	sel := lipgloss.NewStyle().Background(lipgloss.Color("#264F78"))

	var rows []string
	for i := m.sm.offset; i < m.sm.offset+vp && i < len(m.sm.results); i++ {
		r := m.sm.results[i]
		label := truncate(r.Entry.Label(), inner/2)
		idx := r.NameIdx
		line := searchBadge[r.Entry.Kind] + " " + highlight(label, idx)
		if d := r.Entry.Desc; d != "" {
			room := inner - 7 - len([]rune(label))
			if room > 8 {
				line += "  " + faint.Render(truncate(d, room))
			}
		} else if r.MatchedIn == "help" {
			line += "  " + faint.Render("(help text)")
		}
		if r.Entry.Node.Stub && r.Entry.Kind == fuzzy.KindCommand {
			line += faint.Render(" …")
		}
		if i == m.sm.cursor {
			line = sel.Render(line)
		}
		rows = append(rows, line)
	}
	if len(rows) == 0 {
		msg := "type to search"
		if m.sm.input.Value() != "" {
			msg = "no matches"
		}
		rows = append(rows, faint.Render(msg))
	}

	count := ""
	if len(m.sm.results) > 0 {
		count = fmt.Sprintf(" [%d/%d]", m.sm.cursor+1, len(m.sm.results))
	}
	footer := "↑↓ select · Enter jump · Esc close"
	switch {
	case m.sm.indexing:
		footer = fmt.Sprintf("indexing %d/%d… · ", m.sm.indexed, m.sm.indexTotal) + footer
	case m.sm.stubs > 0:
		footer += fmt.Sprintf(" · Tab: index %d undiscovered", m.sm.stubs)
	}
	content := titleStyle.Render("Find"+count) + "\n" +
		m.sm.input.View() + "\n\n" +
		strings.Join(rows, "\n") + "\n\n" +
		faint.Render(truncate(footer, inner))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5EA4F5")).
		Padding(0, 2).
		Width(modalW - 2).
		Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// flagModalMaxVisible is the most list rows (entries plus the global
// separator) the flag picker shows at once.
const flagModalMaxVisible = 14

// flagModalStyles holds the styles the flag picker draws with.
type flagModalStyles struct {
	title, hint, sel, added, global, desc, sep lipgloss.Style
}

func newFlagModalStyles() flagModalStyles {
	return flagModalStyles{
		title:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA4F5")),
		hint:   lipgloss.NewStyle().Faint(true),
		sel:    lipgloss.NewStyle().Background(lipgloss.Color("#264F78")).Bold(true),
		added:  lipgloss.NewStyle().Faint(true), // checkmark shown separately; no strikethrough
		global: lipgloss.NewStyle().Faint(true).Italic(true),
		desc:   lipgloss.NewStyle().Faint(true),
		sep:    lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#888888")),
	}
}

// renderFlagModal renders the flag picker as a centered overlay that fills
// the full terminal height so Bubble Tea clears stale content from the
// previous frame.
func (m *Model) renderFlagModal() string {
	modalW := max(min(m.width-6, 72), 36)
	inner := modalW - 6
	st := newFlagModalStyles()

	vp := m.fm.visibleEntries()
	m.fm.scrollTo(vp)

	// Hint line changes when awaiting a value input.
	hint := "↑↓/jk navigate · Enter add · Esc close"
	if m.fm.awaitingValue {
		hint = "Type value · Enter confirm · Esc cancel"
	}
	scrollHint := ""
	if len(m.fm.entries) > vp {
		scrollHint = fmt.Sprintf(" [%d/%d]", m.fm.cursor+1, len(m.fm.entries))
	}

	content := st.title.Render("Add Flag"+scrollHint) + "\n" +
		st.hint.Render(hint) + "\n\n" +
		strings.Join(m.fm.renderRows(st, vp, inner), "\n") +
		m.fm.renderValuePrompt(st, inner)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5EA4F5")).
		Padding(0, 2).
		Width(modalW - 2).
		Render(content)
	return centerFullScreen(box, m.width, m.height)
}

// visibleEntries returns how many entries fit in the list, leaving room for
// the two global-separator rows when any entry is global.
func (fm *flagModal) visibleEntries() int {
	vp := min(flagModalMaxVisible, len(fm.entries))
	if fm.hasGlobals() && vp+2 > flagModalMaxVisible {
		vp = max(1, flagModalMaxVisible-2)
	}
	return vp
}

func (fm *flagModal) hasGlobals() bool {
	for _, e := range fm.entries {
		if e.global {
			return true
		}
	}
	return false
}

// scrollTo moves the offset so the cursor sits inside a viewport of vp rows.
func (fm *flagModal) scrollTo(vp int) {
	if fm.cursor < fm.offset {
		fm.offset = fm.cursor
	}
	if fm.cursor >= fm.offset+vp {
		fm.offset = fm.cursor - vp + 1
	}
}

// renderRows draws the visible entries, inserting the "global flags"
// separator before the first global entry that follows a local one (or that
// opens the viewport).
func (fm *flagModal) renderRows(st flagModalStyles, vp, inner int) []string {
	var rows []string
	prevWasLocal := true
	for i := fm.offset; i < fm.offset+vp && i < len(fm.entries); i++ {
		e := fm.entries[i]
		if e.global && prevWasLocal {
			rows = append(rows,
				st.sep.Render(strings.Repeat("─", inner)),
				st.sep.Render("  global flags"))
		}
		prevWasLocal = !e.global
		rows = append(rows, renderFlagRow(st, e, i == fm.cursor, inner))
	}
	return rows
}

// flagRowText is the plain text of one picker row before styling.
type flagRowText struct {
	check, name, typeTag, desc string
}

func newFlagRowText(e flagEntry, inner int) flagRowText {
	t := flagRowText{check: "  ", name: e.flag.Name}
	if e.added {
		t.check = "✓ "
	}
	if e.flag.ShortName != "" {
		t.name += ", -" + e.flag.ShortName
	}
	// Value-type badge for non-bool flags.
	if vt := strings.ToLower(e.flag.ValueType); vt != "" && vt != "bool" {
		t.typeTag = " <" + e.flag.ValueType + ">"
	}
	maxDesc := inner - lipgloss.Width(t.check) - lipgloss.Width(t.name+t.typeTag) - 3
	t.desc = e.flag.Description
	if maxDesc < 4 {
		t.desc = ""
	} else if lipgloss.Width(t.desc) > maxDesc {
		t.desc = t.desc[:maxDesc-1] + "…"
	}
	return t
}

// plain joins the row's parts without per-part colours.
func (t flagRowText) plain() string {
	s := t.check + t.name + t.typeTag
	if t.desc != "" {
		s += "   " + t.desc
	}
	return s
}

// renderFlagRow styles one entry: the cursor row is a uniform highlight,
// added and global rows are faint, and other rows colour the name by type.
func renderFlagRow(st flagModalStyles, e flagEntry, selected bool, inner int) string {
	t := newFlagRowText(e, inner)
	switch {
	case selected:
		plain := t.plain()
		if lipgloss.Width(plain) > inner {
			plain = plain[:inner]
		}
		pad := max(0, inner-lipgloss.Width(plain))
		return st.sel.Render(plain + strings.Repeat(" ", pad))
	case e.added:
		return st.added.Render(t.plain())
	case e.global:
		return st.global.Render(t.plain())
	}
	nameColor := flagTypeColor(e.flag.ValueType)
	out := lipgloss.NewStyle().Foreground(nameColor).Render(t.check + t.name)
	if t.typeTag != "" {
		out += lipgloss.NewStyle().Foreground(nameColor).Faint(true).Render(t.typeTag)
	}
	if t.desc != "" {
		out += "   " + st.desc.Render(t.desc)
	}
	return out
}

// renderValuePrompt draws the value input shown while a non-bool flag waits
// for its value, or nothing.
func (fm *flagModal) renderValuePrompt(st flagModalStyles, inner int) string {
	if !fm.awaitingValue {
		return ""
	}
	e := fm.entries[fm.awaitingIdx]
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB86C"))
	fm.valueInput.Width = inner - 4
	return "\n" +
		st.sep.Render(strings.Repeat("─", inner)) + "\n" +
		promptStyle.Render("Value for "+e.flag.Name+":") + "\n" +
		fm.valueInput.View()
}

// centerFullScreen centres box in a width x height screen and pads it with
// blank lines to the full height so Bubble Tea clears old content beneath.
func centerFullScreen(box string, width, height int) string {
	boxH := lipgloss.Height(box)
	padLeft := max(0, (width-lipgloss.Width(box))/2)
	padTop := max(0, (height-boxH)/2)
	padBottom := max(0, height-padTop-boxH)

	blankLine := strings.Repeat(" ", width)
	leftPad := strings.Repeat(" ", padLeft)
	var sb strings.Builder
	for range padTop {
		sb.WriteString(blankLine)
		sb.WriteByte('\n')
	}
	for _, line := range strings.Split(box, "\n") {
		sb.WriteString(leftPad)
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	for range padBottom {
		sb.WriteString(blankLine)
		sb.WriteByte('\n')
	}
	return sb.String()
}

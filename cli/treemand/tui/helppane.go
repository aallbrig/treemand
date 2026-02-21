package tui

import (
"fmt"
"strings"

"github.com/charmbracelet/lipgloss"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
)

// HelpPaneModel shows structured --help content for the selected node.
// The content is scrollable when the pane has focus.
type HelpPaneModel struct {
node         *models.Node
cfg          *config.Config
width        int
height       int
scrollOffset int
focused      bool
lines        []string // pre-rendered content lines
}

func NewHelpPaneModel(cfg *config.Config) *HelpPaneModel {
return &HelpPaneModel{cfg: cfg}
}

func (h *HelpPaneModel) SetNode(node *models.Node) {
if h.node == node {
return
}
h.node = node
h.scrollOffset = 0
h.rebuildLines()
}

func (h *HelpPaneModel) SetSize(w, hi int) {
h.width = w
h.height = hi
}

func (h *HelpPaneModel) SetFocused(f bool) { h.focused = f }

// ScrollUp scrolls the content up by n lines.
func (h *HelpPaneModel) ScrollUp(n int) {
h.scrollOffset -= n
if h.scrollOffset < 0 {
h.scrollOffset = 0
}
}

// ScrollDown scrolls the content down by n lines.
func (h *HelpPaneModel) ScrollDown(n int) {
maxOff := len(h.lines) - h.viewportLines()
if maxOff < 0 {
maxOff = 0
}
h.scrollOffset += n
if h.scrollOffset > maxOff {
h.scrollOffset = maxOff
}
}

func (h *HelpPaneModel) PageUp()   { h.ScrollUp(h.viewportLines()) }
func (h *HelpPaneModel) PageDown() { h.ScrollDown(h.viewportLines()) }
func (h *HelpPaneModel) Top()      { h.scrollOffset = 0 }
func (h *HelpPaneModel) Bottom() {
maxOff := len(h.lines) - h.viewportLines()
if maxOff < 0 {
maxOff = 0
}
h.scrollOffset = maxOff
}

func (h *HelpPaneModel) viewportLines() int {
// border top + title line + border bottom = 3 overhead
v := h.height - 3
if v < 1 {
return 1
}
return v
}

func (h *HelpPaneModel) View(w, hi int) string {
h.width = w
h.height = hi
if len(h.lines) == 0 {
h.rebuildLines()
}

vp := h.viewportLines()
end := h.scrollOffset + vp
if end > len(h.lines) {
end = len(h.lines)
}
slice := h.lines[h.scrollOffset:end]

// Pad to fill viewport so the box is a fixed height.
padded := make([]string, vp)
copy(padded, slice)
for i := len(slice); i < vp; i++ {
padded[i] = ""
}

// Scroll indicator shown in title when content overflows.
scrollSuffix := ""
if len(h.lines) > vp {
pct := 0
if len(h.lines) > 0 {
pct = (h.scrollOffset + vp) * 100 / len(h.lines)
if pct > 100 {
pct = 100
}
}
scrollSuffix = fmt.Sprintf(" [%d%%]", pct)
}

title := "Help"
if h.node != nil {
title += ": " + h.node.Name
}
title += scrollSuffix

borderColor := lipgloss.Color("#555555")
if h.focused {
borderColor = lipgloss.Color("#5EA4F5")
}

titleStyle := lipgloss.NewStyle().Bold(true)
if h.focused {
titleStyle = titleStyle.Foreground(lipgloss.Color("#5EA4F5"))
}

boxStyle := lipgloss.NewStyle().
Border(lipgloss.RoundedBorder()).
BorderForeground(borderColor).
Width(w - 2).
Height(hi - 2)

innerW := w - 4 // account for borders + 1 pad each side
var rendered []string
for _, line := range padded {
// Hard-wrap long lines to inner width so lipgloss doesn't overflow.
rendered = append(rendered, hardWrap(line, innerW))
}

content := titleStyle.Render(title) + "\n" + strings.Join(rendered, "\n")
return boxStyle.Render(content)
}

// hardWrap truncates a line to maxW visible characters (no lipgloss width needed here).
func hardWrap(s string, maxW int) string {
if maxW <= 0 || len(s) <= maxW {
return s
}
return s[:maxW]
}

func (h *HelpPaneModel) rebuildLines() {
if h.node == nil {
h.lines = nil
return
}

var sb strings.Builder

if h.node.Description != "" {
sb.WriteString(h.node.Description + "\n\n")
}

if len(h.node.Flags) > 0 {
sb.WriteString("Flags:\n")
for _, f := range h.node.Flags {
name := f.Name
if f.ShortName != "" && !strings.HasPrefix(f.ShortName, "-") {
name += ", -" + f.ShortName
} else if f.ShortName != "" {
name += ", " + f.ShortName
}
if f.ValueType != "" && f.ValueType != "bool" {
name += " <" + f.ValueType + ">"
}
line := "  " + name
if f.Description != "" {
line += "\n      " + f.Description
}
sb.WriteString(line + "\n")
}
sb.WriteString("\n")
}

if len(h.node.Positionals) > 0 {
sb.WriteString("Positionals:\n")
for _, p := range h.node.Positionals {
if p.Required {
sb.WriteString("  <" + p.Name + ">\n")
} else {
sb.WriteString("  [" + p.Name + "]\n")
}
}
sb.WriteString("\n")
}

if len(h.node.Children) > 0 {
sb.WriteString("Subcommands:\n")
for _, child := range h.node.Children {
line := "  " + child.Name
if child.Description != "" {
line += "  " + child.Description
}
sb.WriteString(line + "\n")
}
sb.WriteString("\n")
}

if h.node.HelpText != "" {
sb.WriteString("Raw help:\n")
sb.WriteString(h.node.HelpText)
}

raw := sb.String()
h.lines = strings.Split(strings.TrimRight(raw, "\n"), "\n")
}

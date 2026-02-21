package tui

import (
"strings"

"github.com/charmbracelet/lipgloss"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
)

// HelpPaneModel shows --help output for the selected node.
type HelpPaneModel struct {
node   *models.Node
cfg    *config.Config
width  int
height int
}

func NewHelpPaneModel(cfg *config.Config) *HelpPaneModel {
return &HelpPaneModel{cfg: cfg}
}

func (h *HelpPaneModel) SetNode(node *models.Node) {
h.node = node
}

func (h *HelpPaneModel) SetSize(w, hi int) {
h.width = w
h.height = hi
}

func (h *HelpPaneModel) View(w, hi int) string {
h.width = w
h.height = hi
content := h.buildContent()

titleStyle := lipgloss.NewStyle().Bold(true)
title := "Help: "
if h.node != nil {
title += h.node.Name
}

boxStyle := lipgloss.NewStyle().
Border(lipgloss.RoundedBorder()).
BorderForeground(lipgloss.Color("#555555")).
Width(w - 2).
Height(hi)

return boxStyle.Render(titleStyle.Render(title) + "\n" + content)
}

func (h *HelpPaneModel) buildContent() string {
if h.node == nil {
return ""
}

var sb strings.Builder

if h.node.Description != "" {
sb.WriteString(h.node.Description + "\n\n")
}

if len(h.node.Flags) > 0 {
sb.WriteString("Flags:\n")
for _, f := range h.node.Flags {
line := "  " + f.Name
if f.ShortName != "" {
line += ", -" + f.ShortName
}
if f.ValueType != "" && f.ValueType != "bool" {
line += " <" + f.ValueType + ">"
}
if f.Description != "" {
line += "\n      " + f.Description
}
sb.WriteString(line + "\n")
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
}

if h.node.HelpText != "" {
// Show truncated raw help text
lines := strings.Split(h.node.HelpText, "\n")
limit := 20
if len(lines) < limit {
limit = len(lines)
}
sb.WriteString("\nRaw help:\n")
sb.WriteString(strings.Join(lines[:limit], "\n"))
}

return sb.String()
}

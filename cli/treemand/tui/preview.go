package tui

import (
"strings"

"github.com/charmbracelet/lipgloss"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
)

// PreviewModel shows a live color-coded command preview at the top.
type PreviewModel struct {
node *models.Node
cfg  *config.Config
}

func NewPreviewModel(cfg *config.Config) *PreviewModel {
return &PreviewModel{cfg: cfg}
}

func (p *PreviewModel) SetNode(node *models.Node) {
p.node = node
}

func (p *PreviewModel) View(width int) string {
if p.node == nil {
return ""
}
content := p.buildPreview()
style := lipgloss.NewStyle().
Border(lipgloss.NormalBorder(), false, false, true, false).
BorderForeground(lipgloss.Color("#555555")).
Width(width - 2).
Padding(0, 1)
return style.Render(content)
}

func (p *PreviewModel) buildPreview() string {
if p.node == nil {
return ""
}
var parts []string

// Command path
cmdStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.cfg.Colors.Base))
subcmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.cfg.Colors.Subcmd))

for i, part := range p.node.FullPath {
if i == 0 {
parts = append(parts, cmdStyle.Render(part))
} else {
parts = append(parts, subcmdStyle.Render(part))
}
}

// Positionals
posStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.cfg.Colors.Pos))
for _, pos := range p.node.Positionals {
if pos.Required {
parts = append(parts, posStyle.Render("<"+pos.Name+">"))
} else {
parts = append(parts, posStyle.Render("["+pos.Name+"]"))
}
}

// Flags summary
flagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.cfg.Colors.Flag))
if len(p.node.Flags) > 0 {
var flags []string
limit := 4
if len(p.node.Flags) < limit {
limit = len(p.node.Flags)
}
for _, f := range p.node.Flags[:limit] {
flags = append(flags, f.Name)
}
if len(p.node.Flags) > 4 {
flags = append(flags, "...")
}
parts = append(parts, flagStyle.Render("["+strings.Join(flags, " ")+"]"))
}

return strings.Join(parts, " ")
}

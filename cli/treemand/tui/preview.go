package tui

import (
"strings"

"github.com/charmbracelet/bubbles/textinput"
tea "github.com/charmbracelet/bubbletea"
"github.com/charmbracelet/lipgloss"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
)

// PreviewModel shows the currently-built command at the top of the screen.
// When focused the user can edit the command text directly; the tree will
// highlight nodes that match the typed tokens.
type PreviewModel struct {
node    *models.Node
cfg     *config.Config
focused bool
ti      textinput.Model
}

func NewPreviewModel(cfg *config.Config) *PreviewModel {
ti := textinput.New()
ti.Placeholder = "type a command…"
ti.CharLimit = 256
return &PreviewModel{cfg: cfg, ti: ti}
}

// SetNode updates the preview to reflect the given node. If the pane is not
// currently focused the textinput value is replaced with the node's full path.
func (p *PreviewModel) SetNode(node *models.Node) {
p.node = node
if !p.focused && node != nil {
p.ti.SetValue(strings.Join(node.FullPath, " "))
}
}

func (p *PreviewModel) SetFocused(focused bool) {
p.focused = focused
if focused {
p.ti.Focus()
p.ti.CursorEnd()
} else {
p.ti.Blur()
}
}

// Tokens returns whitespace-split tokens of the current textinput value.
func (p *PreviewModel) Tokens() []string {
v := strings.TrimSpace(p.ti.Value())
if v == "" {
return nil
}
return strings.Fields(v)
}

// Update forwards tea messages to the textinput when focused.
func (p *PreviewModel) Update(msg tea.Msg) tea.Cmd {
var cmd tea.Cmd
p.ti, cmd = p.ti.Update(msg)
return cmd
}

// View renders the preview bar.
func (p *PreviewModel) View(width int) string {
borderColor := lipgloss.Color("#555555")
if p.focused {
borderColor = lipgloss.Color("#5EA4F5")
}
style := lipgloss.NewStyle().
Border(lipgloss.NormalBorder(), false, false, true, false).
BorderForeground(borderColor).
Width(width - 2).
Padding(0, 1)

var content string
if p.focused {
p.ti.Width = width - 6
label := lipgloss.NewStyle().Faint(true).Render("cmd: ")
content = label + p.ti.View()
} else {
content = p.buildColoredPreview()
}
return style.Render(content)
}

// buildColoredPreview renders the node's full path with lipgloss colors.
func (p *PreviewModel) buildColoredPreview() string {
if p.node == nil {
return ""
}
baseStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.cfg.Colors.Base))
subcmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.cfg.Colors.Subcmd))
flagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.cfg.Colors.Flag))
posStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.cfg.Colors.Pos))

var parts []string
for i, part := range p.node.FullPath {
if i == 0 {
parts = append(parts, baseStyle.Render(part))
} else {
parts = append(parts, subcmdStyle.Render(part))
}
}
for _, pos := range p.node.Positionals {
if pos.Required {
parts = append(parts, posStyle.Render("<"+pos.Name+">"))
} else {
parts = append(parts, posStyle.Render("["+pos.Name+"]"))
}
}
const maxFlags = 4
for i, f := range p.node.Flags {
if i >= maxFlags {
parts = append(parts, flagStyle.Render("…"))
break
}
parts = append(parts, flagStyle.Render(f.Name))
}
return strings.Join(parts, " ")
}

// buildColoredFromTokens renders a manually-typed command with color coding
// by classifying each token (base CLI, subcommands, flags, values).
func buildColoredFromTokens(tokens []string, cfg *config.Config) string {
if len(tokens) == 0 {
return ""
}
baseStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.Colors.Base))
subcmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.Subcmd))
flagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.Flag))
valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.Value))

var parts []string
flagNext := false
for i, tok := range tokens {
switch {
case i == 0:
parts = append(parts, baseStyle.Render(tok))
case flagNext:
parts = append(parts, valueStyle.Render(tok))
flagNext = false
case strings.HasPrefix(tok, "--") || (strings.HasPrefix(tok, "-") && len(tok) == 2):
parts = append(parts, flagStyle.Render(tok))
if strings.Contains(tok, "=") {
flagNext = false
} else {
flagNext = true // next token may be the value
}
default:
parts = append(parts, subcmdStyle.Render(tok))
flagNext = false
}
}
return strings.Join(parts, " ")
}

// BuildPreviewFromNode is exported for render package use.
func BuildPreviewFromNode(node *models.Node, cfg *config.Config) string {
p := NewPreviewModel(cfg)
p.SetNode(node)
return p.buildColoredPreview()
}

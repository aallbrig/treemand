package tui

import (
"fmt"
"strings"

"github.com/charmbracelet/lipgloss"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
)

type helpMode int

const (
helpModeNode       helpMode = iota
helpModeFlag
helpModePositional
)

// HelpPaneModel shows structured --help content for the selected node.
// The content is scrollable when the pane has focus.
type HelpPaneModel struct {
node          *models.Node
cfg           *config.Config
width         int
height        int
scrollOffset  int
focused       bool
lines         []string // pre-rendered content lines
mode          helpMode
selFlag       *models.Flag
selPositional *models.Positional
selOwner      *models.Node
}

func NewHelpPaneModel(cfg *config.Config) *HelpPaneModel {
return &HelpPaneModel{cfg: cfg}
}

// SetNode clears flag/positional context and sets node context.
func (h *HelpPaneModel) SetNode(node *models.Node) {
if h.mode == helpModeNode && h.node == node {
return
}
h.mode = helpModeNode
h.selFlag = nil
h.selPositional = nil
h.selOwner = nil
h.node = node
h.scrollOffset = 0
h.rebuildLines()
}

// SetFlagContext sets content to the given flag's info.
func (h *HelpPaneModel) SetFlagContext(f *models.Flag, owner *models.Node) {
h.mode = helpModeFlag
h.selFlag = f
h.selOwner = owner
h.scrollOffset = 0
h.rebuildLines()
}

// SetPositionalContext sets content to the given positional's info.
func (h *HelpPaneModel) SetPositionalContext(p *models.Positional, owner *models.Node) {
h.mode = helpModePositional
h.selPositional = p
h.selOwner = owner
h.scrollOffset = 0
h.rebuildLines()
}

func (h *HelpPaneModel) SetSize(w, hi int) {
h.width = w
h.height = hi
}

func (h *HelpPaneModel) SetFocused(f bool) { h.focused = f }

func (h *HelpPaneModel) ScrollUp(n int) {
h.scrollOffset -= n
if h.scrollOffset < 0 {
h.scrollOffset = 0
}
}

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

padded := make([]string, vp)
copy(padded, slice)
for i := len(slice); i < vp; i++ {
padded[i] = ""
}

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
switch h.mode {
case helpModeFlag:
if h.selFlag != nil {
title += ": " + h.selFlag.Name
}
case helpModePositional:
if h.selPositional != nil {
title += ": <" + h.selPositional.Name + ">"
}
default:
if h.node != nil {
title += ": " + h.node.Name
}
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

innerW := w - 4
var rendered []string
for _, line := range padded {
rendered = append(rendered, hardWrap(line, innerW))
}

content := titleStyle.Render(title) + "\n" + strings.Join(rendered, "\n")
return boxStyle.Render(content)
}

func hardWrap(s string, maxW int) string {
if maxW <= 0 || len(s) <= maxW {
return s
}
return s[:maxW]
}

func (h *HelpPaneModel) rebuildLines() {
switch h.mode {
case helpModeFlag:
h.rebuildFlagLines()
case helpModePositional:
h.rebuildPositionalLines()
default:
h.rebuildNodeLines()
}
}

func (h *HelpPaneModel) rebuildFlagLines() {
if h.selFlag == nil {
h.lines = nil
return
}
f := h.selFlag
var sb strings.Builder
name := f.Name
if f.ShortName != "" {
name += " [-" + f.ShortName + "]"
}
sb.WriteString("Name: " + name + "\n")
vt := f.ValueType
if vt == "" {
vt = "bool"
}
sb.WriteString("Type: " + vt + "\n")
if f.Description != "" {
sb.WriteString("Description: " + f.Description + "\n")
}
if h.selOwner != nil {
sb.WriteString("\nCommand: " + h.selOwner.FullCommand() + "\n")
}
raw := sb.String()
h.lines = strings.Split(strings.TrimRight(raw, "\n"), "\n")
}

func (h *HelpPaneModel) rebuildPositionalLines() {
if h.selPositional == nil {
h.lines = nil
return
}
p := h.selPositional
var sb strings.Builder
sb.WriteString("Argument: <" + p.Name + ">\n")
req := "no"
if p.Required {
req = "yes"
}
sb.WriteString("Required: " + req + "\n")
if p.Description != "" {
sb.WriteString("Description: " + p.Description + "\n")
}
if h.selOwner != nil {
sb.WriteString("\nCommand: " + h.selOwner.FullCommand() + "\n")
}
raw := sb.String()
h.lines = strings.Split(strings.TrimRight(raw, "\n"), "\n")
}

func (h *HelpPaneModel) rebuildNodeLines() {
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

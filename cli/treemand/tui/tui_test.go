package tui_test

import (
"strings"
"testing"

tea "github.com/charmbracelet/bubbletea"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
"github.com/aallbrig/treemand/tui"
)

func sampleTree() *models.Node {
return &models.Node{
Name:     "git",
FullPath: []string{"git"},
Flags: []models.Flag{
{Name: "--version"},
{Name: "--help", ShortName: "h"},
{Name: "--paginate", ShortName: "p"},
{Name: "--no-pager"},
},
Children: []*models.Node{
{
Name: "commit", FullPath: []string{"git", "commit"},
Flags: []models.Flag{
{Name: "--message", ShortName: "m", ValueType: "string"},
{Name: "--all", ShortName: "a"},
{Name: "--amend"},
},
Positionals: []models.Positional{{Name: "msg", Required: true}},
},
{
Name: "remote", FullPath: []string{"git", "remote"},
Children: []*models.Node{
{Name: "add", FullPath: []string{"git", "remote", "add"}},
},
},
},
}
}

// --- Model ---

func TestNewModel(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
if m == nil {
t.Fatal("NewModel returned nil")
}
}

func TestModel_WindowSize(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
if updated == nil {
t.Fatal("Update returned nil after WindowSizeMsg")
}
}

func TestModel_TabCyclesFocus(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

// Tab should cycle focus without panicking.
for i := 0; i < 6; i++ {
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
if updated == nil {
t.Fatalf("Update returned nil on tab %d", i)
}
}
}

func TestModel_QuitOnQ(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
if cmd == nil {
t.Error("expected quit command after 'q'")
}
}

func TestModel_View_nonempty(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
v := m.View()
if v == "" {
t.Error("View() returned empty string")
}
}

// --- TreeModel ---

func TestNewTreeModel_selected(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
sel := tree.Selected()
if sel == nil {
t.Fatal("expected selected node")
}
if sel.Name != "git" {
t.Errorf("initial selected = %q, want %q", sel.Name, "git")
}
}

func TestTreeModel_navigation(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)
tree.Expand()
tree.Down()
sel := tree.Selected()
if sel == nil {
t.Fatal("expected selected node after Down()")
}
if sel.Name != "commit" {
t.Errorf("after Down, selected = %q, want %q", sel.Name, "commit")
}
tree.Up()
if tree.Selected().Name != "git" {
t.Errorf("after Up, want git, got %q", tree.Selected().Name)
}
}

func TestTreeModel_filter(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)
tree.Expand()
tree.SetFilter("commit")
if tree.Selected() == nil {
t.Fatal("expected selected node after filter")
}
}

func TestTreeModel_cmdTokens_highlight(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)
tree.Expand()

// Set tokens matching root and first child.
tree.SetCmdTokens([]string{"git", "commit"})
v := tree.ViewSized(80, 24)
// Both "git" and "commit" should appear in the view.
if !strings.Contains(v, "git") {
t.Error("expected 'git' in tree view")
}
if !strings.Contains(v, "commit") {
t.Error("expected 'commit' in tree view")
}
}

func TestTreeModel_inlineFlags(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(120, 40)
v := tree.ViewSized(120, 40)
// Root node has flags; at least one flag name should appear inline.
if !strings.Contains(v, "--version") && !strings.Contains(v, "--help") {
t.Error("expected inline flags in tree view")
}
}

func TestTreeModel_view(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)
v := tree.ViewSized(80, 24)
if v == "" {
t.Error("expected non-empty view")
}
}

// --- PreviewModel ---

func TestPreviewModel_view(t *testing.T) {
cfg := config.DefaultConfig()
p := tui.NewPreviewModel(cfg)
p.SetNode(sampleTree())
v := p.View(80)
if v == "" {
t.Error("expected non-empty preview view")
}
}

func TestPreviewModel_tokens(t *testing.T) {
cfg := config.DefaultConfig()
p := tui.NewPreviewModel(cfg)
node := &models.Node{Name: "git", FullPath: []string{"git", "commit"}}
p.SetNode(node)
tokens := p.Tokens()
if len(tokens) != 2 {
t.Errorf("expected 2 tokens, got %v", tokens)
}
}

func TestPreviewModel_focused(t *testing.T) {
cfg := config.DefaultConfig()
p := tui.NewPreviewModel(cfg)
p.SetNode(sampleTree())
p.SetFocused(true)
v := p.View(80)
if v == "" {
t.Error("expected non-empty view when focused")
}
// Should show "cmd:" label when focused.
if !strings.Contains(v, "cmd:") {
t.Errorf("expected 'cmd:' label when focused, got: %q", v)
}
}

func TestNodePreview(t *testing.T) {
cfg := config.DefaultConfig()
cfg.NoColor = true
node := &models.Node{Name: "git", FullPath: []string{"git"}}
preview := tui.NodePreview(node, cfg)
if preview == "" {
t.Error("expected non-empty preview")
}
}

// --- HelpPaneModel ---

func TestHelpPaneModel_view(t *testing.T) {
cfg := config.DefaultConfig()
h := tui.NewHelpPaneModel(cfg)
h.SetNode(sampleTree())
h.SetSize(40, 20)
v := h.View(40, 20)
if v == "" {
t.Error("expected non-empty help pane view")
}
}

func TestHelpPaneModel_scroll(t *testing.T) {
cfg := config.DefaultConfig()
h := tui.NewHelpPaneModel(cfg)
h.SetNode(sampleTree())
h.SetSize(40, 10)

// Scrolling should not panic.
h.ScrollDown(5)
h.ScrollDown(100) // clamps
h.ScrollUp(3)
h.ScrollUp(100) // clamps to 0
h.PageDown()
h.PageUp()
h.Bottom()
h.Top()

v := h.View(40, 10)
if v == "" {
t.Error("expected non-empty help pane view after scrolling")
}
}

func TestHelpPaneModel_focused_border(t *testing.T) {
cfg := config.DefaultConfig()
h := tui.NewHelpPaneModel(cfg)
h.SetNode(sampleTree())
h.SetSize(40, 20)
// Toggling focus must not panic and must still return non-empty content.
h.SetFocused(false)
if v := h.View(40, 20); v == "" {
t.Error("expected non-empty view when unfocused")
}
h.SetFocused(true)
if v := h.View(40, 20); v == "" {
t.Error("expected non-empty view when focused")
}
}

func TestHelpPaneModel_showsFlags(t *testing.T) {
cfg := config.DefaultConfig()
h := tui.NewHelpPaneModel(cfg)
h.SetNode(sampleTree())
h.SetSize(60, 30)
v := h.View(60, 30)
if !strings.Contains(v, "--version") {
t.Error("expected flag '--version' in help pane")
}
}

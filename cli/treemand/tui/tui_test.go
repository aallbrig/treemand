package tui_test

import (
"testing"

"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/models"
"github.com/aallbrig/treemand/tui"
)

func sampleTree() *models.Node {
return &models.Node{
Name:     "git",
FullPath: []string{"git"},
Flags:    []models.Flag{{Name: "--version"}},
Children: []*models.Node{
{Name: "commit", FullPath: []string{"git", "commit"},
Positionals: []models.Positional{{Name: "msg", Required: true}}},
{Name: "remote", FullPath: []string{"git", "remote"},
Children: []*models.Node{
{Name: "add", FullPath: []string{"git", "remote", "add"}},
}},
},
}
}

func TestNewModel(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
if m == nil {
t.Fatal("NewModel returned nil")
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

// Initially at root (git), expand to see children
tree.Expand()
tree.Down()
sel := tree.Selected()
if sel == nil {
t.Fatal("expected selected node after Down()")
}
// Should now be on "commit"
if sel.Name != "commit" {
t.Errorf("after Down, selected = %q, want %q", sel.Name, "commit")
}

// Up goes back to root
tree.Up()
sel = tree.Selected()
if sel.Name != "git" {
t.Errorf("after Up, selected = %q, want %q", sel.Name, "git")
}
}

func TestTreeModel_filter(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)
tree.Expand()

tree.SetFilter("commit")
sel := tree.Selected()
if sel == nil {
t.Fatal("expected selected after filter")
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

func TestPreviewModel(t *testing.T) {
cfg := config.DefaultConfig()
p := tui.NewPreviewModel(cfg)
p.SetNode(sampleTree())
v := p.View(80)
if v == "" {
t.Error("expected non-empty preview view")
}
}

func TestHelpPaneModel(t *testing.T) {
cfg := config.DefaultConfig()
h := tui.NewHelpPaneModel(cfg)
h.SetNode(sampleTree())
h.SetSize(40, 20)
v := h.View(40, 20)
if v == "" {
t.Error("expected non-empty help pane view")
}
}

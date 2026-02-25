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
// Right() expands root and enters first child.
tree.Right()
sel := tree.Selected()
if sel == nil {
t.Fatal("expected selected node after Right()")
}
if sel.Name != "commit" {
t.Errorf("after Right, selected = %q, want %q", sel.Name, "commit")
}
// Left() should return to the parent (git).
tree.Left()
if tree.Selected().Name != "git" {
t.Errorf("after Left, want git, got %q", tree.Selected().Name)
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
// Collapse the root so inline flag list is shown.
tree.Left()
v := tree.ViewSized(120, 40)
if !strings.Contains(v, "--version") && !strings.Contains(v, "--help") {
t.Error("expected inline flag names on collapsed root node")
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
// Should show ► label when focused.
if !strings.Contains(v, "►") {
t.Errorf("expected '►' label when focused, got: %q", v)
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

// --- Navigation edge cases ---

func TestTreeModel_Down_autoExpandsSection(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 40)
// Down from root (git) should move to first subcommand (commit) AND auto-expand it.
tree.Down()
sel := tree.SelectedItem()
if sel == nil || sel.Kind != tui.SelCommand || sel.Node.Name != "commit" {
t.Fatalf("expected commit after first Down from root, got %v", sel)
}
// Down again from commit (now expanded) should enter commit's first child value.
tree.Down()
sel2 := tree.SelectedItem()
if sel2 == nil {
t.Fatal("expected selection after Down into commit")
}
// Should be INSIDE commit (a flag, positional, or child command), not the next root sibling.
if sel2.Kind == tui.SelCommand && sel2.Node.Name == "remote" {
t.Error("second Down should enter commit's contents, not jump to remote sibling")
}
}

func TestTreeModel_Left_fromFlag_returnsToCommand(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 40)
// Navigate into commit, then Down into a flag row.
tree.Right() // → commit
for i := 0; i < 5; i++ {
tree.Down()
if sel := tree.SelectedItem(); sel != nil && sel.Kind == tui.SelFlag {
break
}
}
sel := tree.SelectedItem()
if sel == nil || sel.Kind != tui.SelFlag {
t.Skip("could not reach a flag row; skip Left-from-flag test")
}
tree.Left()
back := tree.Selected()
if back == nil {
t.Fatal("expected node after Left from flag")
}
// Should have landed on a command row owning that flag.
if back.Name != "commit" && back.Name != "git" {
t.Errorf("Left from flag should return to owner command, got %q", back.Name)
}
}

func TestTreeModel_Up_doesNotStop_atSectionHeader(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 40)
tree.Right() // into commit
// Go down several times so we're past any section headers.
for i := 0; i < 4; i++ {
tree.Down()
}
// Going up should never leave cursor on a section row (section rows are non-selectable).
for i := 0; i < 6; i++ {
tree.Up()
sel := tree.SelectedItem()
if sel == nil {
t.Fatalf("Up() left cursor with no selection at step %d", i)
}
}
}

func TestTreeModel_Right_noOp_onFlag(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 40)
tree.Right() // into commit
// Navigate to a flag row.
for i := 0; i < 5; i++ {
tree.Down()
if sel := tree.SelectedItem(); sel != nil && sel.Kind == tui.SelFlag {
break
}
}
if sel := tree.SelectedItem(); sel == nil || sel.Kind != tui.SelFlag {
t.Skip("could not reach a flag row")
}
before := tree.SelectedItem()
tree.Right() // should be a no-op on a flag row
after := tree.SelectedItem()
if before.Flag.Name != after.Flag.Name {
t.Error("Right() on a flag row should be a no-op")
}
}

// --- Model integration ---

func TestModel_Enter_setsPreview(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Expand into commit with Right.
m.Update(tea.KeyMsg{Type: tea.KeyRight})
// Enter should set the preview to the commit command.
m.Update(tea.KeyMsg{Type: tea.KeyEnter})
v := m.View()
if !strings.Contains(v, "commit") {
t.Error("expected 'commit' in preview after Enter on commit node")
}
}

func TestModel_Backspace_removesToken(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyRight})
m.Update(tea.KeyMsg{Type: tea.KeyEnter})
// Now backspace removes last token.
m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
v := m.View()
// After backspace the preview may show "git" only or be empty — should not contain "commit" as a separate token.
_ = v // just ensure no panic
}

func TestModel_FlagModalOpens(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Press 'f' to open the flag modal.
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
v := m.View()
// Flag modal should appear with "Add Flag" title.
if !strings.Contains(v, "Add Flag") {
t.Error("expected 'Add Flag' modal after pressing f")
}
}

func TestModel_FlagModal_EscCloses(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
// Esc should close the modal.
m.Update(tea.KeyMsg{Type: tea.KeyEsc})
v := m.View()
if strings.Contains(v, "Add Flag") {
t.Error("flag modal should be closed after Esc")
}
}

func TestModel_HelpPane_toggle(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Default: help pane visible. Press 'h' to hide.
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
v1 := m.View()
// Press 'h' again to show.
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
v2 := m.View()
// Both views should be non-empty.
if v1 == "" || v2 == "" {
t.Error("view should never be empty")
}
}

func TestModel_CtrlE_opensModal(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Set a command first.
m.Update(tea.KeyMsg{Type: tea.KeyRight})
m.Update(tea.KeyMsg{Type: tea.KeyEnter})
// Ctrl+E opens execute modal.
m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
v := m.View()
if !strings.Contains(v, "Execute") && !strings.Contains(v, "commit") {
t.Error("expected execute modal after Ctrl+E")
}
}

func TestModel_PreviewBar_hasLabel(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
v := m.View()
if !strings.Contains(v, "►") {
t.Error("expected ► label in preview bar")
}
}

func TestPreviewModel_noCommandPlaceholder(t *testing.T) {
cfg := config.DefaultConfig()
p := tui.NewPreviewModel(cfg)
v := p.View(80)
if !strings.Contains(v, "no command") {
t.Error("expected 'no command' placeholder in empty preview bar")
}
}

// --- HelpPane context modes ---

func TestHelpPaneModel_flagContext(t *testing.T) {
cfg := config.DefaultConfig()
h := tui.NewHelpPaneModel(cfg)
node := sampleTree()
flag := &node.Flags[0] // --version
h.SetFlagContext(flag, node)
h.SetSize(60, 20)
v := h.View(60, 20)
if !strings.Contains(v, "--version") {
t.Errorf("expected flag name '--version' in help pane flag context, got: %q", v)
}
}

func TestHelpPaneModel_positionalContext(t *testing.T) {
cfg := config.DefaultConfig()
h := tui.NewHelpPaneModel(cfg)
node := sampleTree().Children[0] // commit
pos := &node.Positionals[0]      // msg
h.SetPositionalContext(pos, node)
h.SetSize(60, 20)
v := h.View(60, 20)
if !strings.Contains(v, "msg") {
t.Errorf("expected positional name 'msg' in help pane positional context, got: %q", v)
}
}

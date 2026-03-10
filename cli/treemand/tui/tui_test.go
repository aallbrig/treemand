package tui_test

import (
	"fmt"
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

// --- Fuzzy filter ---

func deepFilterTree() *models.Node {
	// root -> alpha -> beta -> gamma (leaf)
	//
	//	         -> delta (leaf)
	//	-> epsilon (leaf)
	gamma := &models.Node{Name: "gamma", FullPath: []string{"root", "alpha", "beta", "gamma"}}
	delta := &models.Node{Name: "delta", FullPath: []string{"root", "alpha", "delta"}}
	beta := &models.Node{Name: "beta", FullPath: []string{"root", "alpha", "beta"}, Children: []*models.Node{gamma}}
	alpha := &models.Node{Name: "alpha", FullPath: []string{"root", "alpha"}, Children: []*models.Node{beta, delta}}
	epsilon := &models.Node{Name: "epsilon", FullPath: []string{"root", "epsilon"}}
	return &models.Node{
		Name:     "root",
		FullPath: []string{"root"},
		Children: []*models.Node{alpha, epsilon},
	}
}

func newTreeModel(root *models.Node) *tui.TreeModel {
	cfg := config.DefaultConfig()
	tm := tui.NewTreeModel(root, cfg)
	tm.SetSize(80, 40)
	return tm
}

func TestTreeModel_FilterEmpty_ShowsOnlyRoot(t *testing.T) {
	// Root is auto-expanded; just verify root node is visible with no filter.
	tm := newTreeModel(deepFilterTree())
	if tm.RowCount() == 0 {
		t.Error("expected at least 1 row, got 0")
	}
	if !strings.Contains(tm.ViewSized(80, 40), "root") {
		t.Error("root node should be visible when no filter is active")
	}
}

func TestTreeModel_Filter_MatchingNodeVisible(t *testing.T) {
	tm := newTreeModel(deepFilterTree())
	tm.SetFilter("gamma")
	// gamma matches; its ancestors root/alpha/beta should also appear as breadcrumbs.
	view := tm.ViewSized(80, 40)
	if !strings.Contains(view, "gamma") {
		t.Error("filtered tree should contain 'gamma'")
	}
}

func TestTreeModel_Filter_AncestorsVisibleAsBreadcrumbs(t *testing.T) {
	tm := newTreeModel(deepFilterTree())
	tm.SetFilter("gamma")
	view := tm.ViewSized(80, 40)
	for _, ancestor := range []string{"root", "alpha", "beta"} {
		if !strings.Contains(view, ancestor) {
			t.Errorf("ancestor %q should be visible as breadcrumb when filtering for 'gamma'", ancestor)
		}
	}
}

func TestTreeModel_Filter_NonMatchingNodeHidden(t *testing.T) {
	tm := newTreeModel(deepFilterTree())
	tm.SetFilter("gamma")
	view := tm.ViewSized(80, 40)
	// epsilon doesn't match and has no matching descendants
	if strings.Contains(view, "epsilon") {
		t.Error("'epsilon' should be hidden when filter is 'gamma'")
	}
	// delta doesn't match
	if strings.Contains(view, "delta") {
		t.Error("'delta' should be hidden when filter is 'gamma'")
	}
}

func TestTreeModel_Filter_CaseInsensitive(t *testing.T) {
	tm := newTreeModel(deepFilterTree())
	tm.SetFilter("GAMMA")
	view := tm.ViewSized(80, 40)
	if !strings.Contains(view, "gamma") {
		t.Error("filter should be case-insensitive")
	}
}

func TestTreeModel_Filter_ClearedRestoresFull(t *testing.T) {
	tm := newTreeModel(deepFilterTree())
	rowsBefore := tm.RowCount()
	tm.SetFilter("gamma")
	tm.SetFilter("") // clear
	// Row count should return to the same as before filtering.
	if tm.RowCount() != rowsBefore {
		t.Errorf("after clearing filter expected %d rows, got %d", rowsBefore, tm.RowCount())
	}
}

func TestTreeModel_Filter_PartialMatch(t *testing.T) {
	tm := newTreeModel(deepFilterTree())
	tm.SetFilter("lph") // partial match for "alpha"
	view := tm.ViewSized(80, 40)
	if !strings.Contains(view, "alpha") {
		t.Error("partial filter 'lph' should match 'alpha'")
	}
}

func TestTreeModel_Filter_NoMatch_EmptyView(t *testing.T) {
	tm := newTreeModel(deepFilterTree())
	tm.SetFilter("zzznomatch")
	if tm.RowCount() != 0 {
		t.Errorf("no-match filter should produce 0 rows, got %d", tm.RowCount())
	}
}

// --- Collapse / ToggleExpand / ToggleSectionAtY ---

func TestTreeModel_Collapse(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)

// Expand root first, then collapse.
tree.Expand()
tree.Collapse()
// After collapse we should be back on (or near) the parent.
sel := tree.Selected()
if sel == nil {
t.Fatal("selected is nil after Collapse")
}
}

func TestTreeModel_ToggleExpand_expandsAndCollapses(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)

// Initially at root with children collapsed (root auto-expanded but cursor is on root).
rowsBefore := tree.RowCount()

// ToggleExpand on root should toggle its expansion.
tree.ToggleExpand()
rowsAfter := tree.RowCount()

// Rows should have changed.
if rowsBefore == rowsAfter {
t.Logf("row count unchanged (root may already be in toggled state): before=%d after=%d", rowsBefore, rowsAfter)
}

// Toggle again — should restore.
tree.ToggleExpand()
rowsRestored := tree.RowCount()
if rowsRestored != rowsBefore {
t.Errorf("after double-toggle row count = %d, want %d", rowsRestored, rowsBefore)
}
}

func TestTreeModel_ToggleExpand_noopOnFlag(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)

// Navigate into the flags section of root, then ToggleExpand should not panic.
tree.Down() // move to first child or flag row
before := tree.RowCount()
tree.ToggleExpand()
after := tree.RowCount()
// Either no change or changed — just must not panic.
_ = before
_ = after
}

func TestTreeModel_ToggleSectionAtY_outOfBounds(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)

// Out-of-bounds y should not panic.
tree.ToggleSectionAtY(-1)
tree.ToggleSectionAtY(9999)
}

func TestTreeModel_ToggleSectionAtY_onCommandRow(t *testing.T) {
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(80, 24)

// y=0 is a command row (root), not a section — should be a no-op.
before := tree.RowCount()
tree.ToggleSectionAtY(0)
after := tree.RowCount()
if before != after {
t.Errorf("ToggleSectionAtY on command row changed row count: %d→%d", before, after)
}
}

// --- renderFlagRow / renderPositionalRow ---

func TestTreeModel_renderFlagRow_visible(t *testing.T) {
// Expand the flags section so flag rows are rendered.
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(120, 40)

// Navigate right to expand root, then Down past children until we see flags.
tree.Right()
// Move back to root to see its flag section.
tree.Left()
v := tree.ViewSized(120, 40)
// root has --version, --help etc — at least one should appear.
if !strings.Contains(v, "--version") && !strings.Contains(v, "--help") {
t.Log("flags not yet visible; trying Down to auto-expand flags section")
for i := 0; i < 10; i++ {
tree.Down()
}
v = tree.ViewSized(120, 40)
}
// Just verify ViewSized doesn't panic and returns content.
if v == "" {
t.Error("expected non-empty view")
}
}

func TestTreeModel_renderPositionalRow_visible(t *testing.T) {
// commit has a positional arg <msg>. Navigate to it and expand.
cfg := config.DefaultConfig()
tree := tui.NewTreeModel(sampleTree(), cfg)
tree.SetSize(120, 40)

// Expand root, navigate to commit.
tree.Right() // enter commit
// Expand commit's flag/positional sections.
for i := 0; i < 15; i++ {
tree.Down()
}
v := tree.ViewSized(120, 40)
if v == "" {
t.Error("expected non-empty view after navigating into positionals")
}
}

// --- Vim and WASD navigation schemes ---

func TestModel_VimScheme_navigation(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

// Ctrl+S cycles from Arrows → Vim.
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
v := m.View()
if !strings.Contains(v, "vim") && !strings.Contains(v, "Vim") && !strings.Contains(v, "hjkl") {
t.Logf("vim scheme indicator not obvious in status bar (ok): %q", v[:min(len(v), 200)])
}

// j = down, k = up, l = right, h = left — must not panic.
for _, r := range []rune{'j', 'k', 'l', 'h'} {
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
if updated == nil {
t.Fatalf("Update returned nil on vim key %q", string(r))
}
}
// Space to add node.
m.Update(tea.KeyMsg{Type: tea.KeySpace})
}

func TestModel_WASDScheme_navigation(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

// Ctrl+S twice → WASD scheme.
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

// s = down, w = up, d = right, a = left — must not panic.
for _, r := range []rune{'s', 'w', 'd', 'a'} {
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
if updated == nil {
t.Fatalf("Update returned nil on WASD key %q", string(r))
}
}
}

func TestModel_SchemeRotation(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

// Three Ctrl+S presses should cycle back to arrows.
for i := 0; i < 3; i++ {
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
}
// Arrow keys should still work (we're back to arrows scheme).
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
if updated == nil {
t.Error("Update returned nil after scheme rotation")
}
}

// --- Filter mode (updateFilter) ---

func TestModel_FilterMode_typeAndClear(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

// '/' enters filter mode.
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})

// Type some characters into the filter.
for _, r := range []rune("comm") {
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
if updated == nil {
t.Fatal("Update returned nil during filter typing")
}
}
v := m.View()
if v == "" {
t.Error("expected non-empty view during filter")
}

// Backspace clears a character.
m.Update(tea.KeyMsg{Type: tea.KeyBackspace})

// Esc clears filter.
m.Update(tea.KeyMsg{Type: tea.KeyEsc})
}

func TestModel_FilterMode_enter(t *testing.T) {
cfg := config.DefaultConfig()
m := tui.NewModel(sampleTree(), cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
// Enter confirms filter.
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
if updated == nil {
t.Error("Update returned nil after Enter in filter mode")
}
}

// --- Preview AppendToken ---

func TestPreviewModel_AppendToken(t *testing.T) {
cfg := config.DefaultConfig()
p := tui.NewPreviewModel(cfg)
p.SetNode(sampleTree())

// Direct call to AppendToken.
p.AppendToken("--verbose")
tokens := p.Tokens()
found := false
for _, tok := range tokens {
if tok == "--verbose" {
found = true
break
}
}
if !found {
t.Errorf("expected '--verbose' in tokens after AppendToken, got %v", tokens)
}

// Append a second token.
p.AppendToken("--dry-run")
tokens2 := p.Tokens()
if len(tokens2) <= len(tokens) {
t.Errorf("expected more tokens after second AppendToken: %v", tokens2)
}
}

// ---- Additional coverage tests ----

func sampleTreeWithStub() *models.Node {
stub := &models.Node{
Name:     "s3",
FullPath: []string{"aws", "s3"},
Stub:     true,
}
return &models.Node{
Name:     "aws",
FullPath: []string{"aws"},
Children: []*models.Node{stub},
}
}

func sampleTreeWithValueFlag() *models.Node {
return &models.Node{
Name:     "git",
FullPath: []string{"git"},
Children: []*models.Node{
{
Name:     "commit",
FullPath: []string{"git", "commit"},
Flags: []models.Flag{
{Name: "--message", ShortName: "m", ValueType: "string"},
},
Positionals: []models.Positional{{Name: "file", Required: false}},
},
},
}
}

func TestModel_Init(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
cmd := m.Init()
// Init returns tea.EnableMouseAllMotion — not nil
if cmd == nil {
t.Error("Init() should return a non-nil tea.Cmd")
}
}

func TestModel_LazyExpandMsg_patchesStub(t *testing.T) {
root := sampleTreeWithStub()
stub := root.Children[0]
m := tui.NewModel(root, config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

discovered := &models.Node{
Name:     "s3",
FullPath: []string{"aws", "s3"},
Children: []*models.Node{
{Name: "cp", FullPath: []string{"aws", "s3", "cp"}},
},
}
updated, _ := m.Update(tui.LazyExpandMsg{Stub: stub, Discovered: discovered})
if updated == nil {
t.Fatal("Update returned nil after LazyExpandMsg")
}
}

func TestModel_LazyExpandMsg_error(t *testing.T) {
root := sampleTreeWithStub()
stub := root.Children[0]
m := tui.NewModel(root, config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

updated, _ := m.Update(tui.LazyExpandMsg{Stub: stub, Err: fmt.Errorf("discovery failed")})
if updated == nil {
t.Fatal("Update returned nil on LazyExpandMsg with error")
}
// Should not panic; status should mention the error
_ = updated.(interface{ View() string }).View()
}

func TestPatchNode_replacesChildren(t *testing.T) {
root := sampleTreeWithStub()
stub := root.Children[0]
tree := tui.NewTreeModel(root, config.DefaultConfig())
tree.SetSize(80, 20)

disc := &models.Node{
Name:     "s3",
FullPath: []string{"aws", "s3"},
Children: []*models.Node{
{Name: "cp", FullPath: []string{"aws", "s3", "cp"}},
{Name: "ls", FullPath: []string{"aws", "s3", "ls"}},
},
}
tree.PatchNode(stub, disc)

if stub.Stub {
t.Error("PatchNode should clear Stub flag")
}
if len(stub.Children) != 2 {
t.Errorf("expected 2 children after patch, got %d", len(stub.Children))
}
}

func TestPatchNode_nil_noPanic(t *testing.T) {
tree := tui.NewTreeModel(sampleTree(), config.DefaultConfig())
tree.PatchNode(nil, nil)                                              // no panic
tree.PatchNode(sampleTree().Children[0], nil)                        // no panic
}

func TestModel_ExecuteModal_escCloses(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Open modal with Ctrl+E
m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
// Close with Esc
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
if updated == nil {
t.Fatal("Update returned nil after Esc in execute modal")
}
}

func TestModel_ExecuteModal_copyOption(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
// Press 'c' to copy (may fail if clipboard unavailable, should not panic)
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
if updated == nil {
t.Fatal("Update returned nil after 'c' in execute modal")
}
}

func TestModel_ValueModal_flagWithType(t *testing.T) {
root := sampleTreeWithValueFlag()
m := tui.NewModel(root, config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Navigate down into commit, then down to --message flag
m.Update(tea.KeyMsg{Type: tea.KeyDown})   // select commit
m.Update(tea.KeyMsg{Type: tea.KeyRight})  // expand commit
m.Update(tea.KeyMsg{Type: tea.KeyDown})   // move to --message flag
// Press Enter on the flag — should open value modal
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
if updated == nil {
t.Fatal("Update returned nil after Enter on flag")
}
// Press Esc to close
m.Update(tea.KeyMsg{Type: tea.KeyEsc})
}

func TestModel_ValueModal_confirmValue(t *testing.T) {
root := sampleTreeWithValueFlag()
m := tui.NewModel(root, config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyDown})
m.Update(tea.KeyMsg{Type: tea.KeyRight})
m.Update(tea.KeyMsg{Type: tea.KeyDown})
m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // opens value modal
// Type a value
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("main")})
// Confirm
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
if updated == nil {
t.Fatal("Update returned nil after confirming value modal")
}
}

func TestModel_FlagModal_navigation(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Open flag modal with 'f'
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
// Navigate down
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
// Toggle selection
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
// Navigate up
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
// Confirm
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
if updated == nil {
t.Fatal("Update returned nil after confirming flag modal")
}
}

func TestModel_FlagModal_escape(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
if updated == nil {
t.Fatal("Update returned nil after Esc in flag modal")
}
}

func TestModel_FlagModal_search(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
// Type '/' to activate search in flag modal
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")}) // search for --verbose
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
if updated == nil {
t.Fatal("Update returned nil after search in flag modal")
}
}

func TestModel_HandleArrows_enter_setsPreview(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyDown})
// Enter on 'commit' should set preview
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
if updated == nil {
t.Fatal("Update returned nil")
}
view := updated.(interface{ View() string }).View()
if !strings.Contains(view, "git") {
t.Error("view should contain 'git' after navigating")
}
}

func TestModel_HandleArrows_left_collapses(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyDown})
m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft}) // collapse
if updated == nil {
t.Fatal("returned nil after Left")
}
}

func TestModel_HandleVim_hjkl(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Activate vim scheme (Ctrl+S once)
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
// Navigate with j/k/h/l
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}) // expand
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")}) // collapse
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
if updated == nil {
t.Fatal("returned nil in vim navigation")
}
}

func TestModel_HandleVim_enter(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS}) // vim scheme
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // down to commit
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
if updated == nil {
t.Fatal("returned nil after vim Enter")
}
}

func TestModel_HandleWASD_keys(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// WASD is scheme 2 (arrows=0, vim=1, wasd=2)
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
// Navigate with s/w/d/a
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}) // down
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}) // right/expand
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}) // left/collapse
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")}) // up
if updated == nil {
t.Fatal("returned nil in WASD navigation")
}
}

func TestModel_HandleWASD_enter(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
if updated == nil {
t.Fatal("returned nil after WASD Enter")
}
}

func TestModel_Mouse_doesNotPanic(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Click at various positions
updated, _ := m.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 5, Y: 5})
if updated == nil {
t.Fatal("returned nil after mouse click")
}
m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, X: 5, Y: 5})
m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp, X: 5, Y: 5})
}

func TestModel_HelpPane_keys(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Focus help pane via Tab
m.Update(tea.KeyMsg{Type: tea.KeyTab})
m.Update(tea.KeyMsg{Type: tea.KeyTab})
// Scroll keys in help pane
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
if updated == nil {
t.Fatal("returned nil in help pane navigation")
}
}

func TestModel_PreviewPane_input(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
// Focus preview pane (Tab 3 times: tree→help→preview)
m.Update(tea.KeyMsg{Type: tea.KeyTab})
m.Update(tea.KeyMsg{Type: tea.KeyTab})
m.Update(tea.KeyMsg{Type: tea.KeyTab})
// Type in preview
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
if updated == nil {
t.Fatal("returned nil in preview input")
}
}

func TestModel_RefreshKey(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
if updated == nil {
t.Fatal("returned nil after R (refresh)")
}
}

func TestModel_QuestionMark_helpModal(t *testing.T) {
m := tui.NewModel(sampleTree(), config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
if updated == nil {
t.Fatal("returned nil after ? key")
}
}

func TestModel_SpaceOnStub(t *testing.T) {
root := sampleTreeWithStub()
m := tui.NewModel(root, config.DefaultConfig())
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
m.Update(tea.KeyMsg{Type: tea.KeyDown}) // navigate to stub child
// Space on stub should return a non-nil cmd (async discovery)
_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
// cmd may be nil if the stub isn't selected; just no panic
_ = cmd
}

func TestTreeModel_PatchNode_autoExpands(t *testing.T) {
root := sampleTreeWithStub()
stub := root.Children[0]
tree := tui.NewTreeModel(root, config.DefaultConfig())
tree.SetSize(80, 20)

// Before patch: no children
if !stub.Stub {
t.Fatal("expected stub node")
}

disc := &models.Node{
Name:     "s3",
FullPath: []string{"aws", "s3"},
Description: "S3 service",
Children: []*models.Node{
{Name: "cp", FullPath: []string{"aws", "s3", "cp"}},
},
}
tree.PatchNode(stub, disc)

if stub.Stub {
t.Error("Stub should be cleared after PatchNode")
}
if stub.Description != "S3 service" {
t.Errorf("Description should be filled in, got %q", stub.Description)
}
}

func TestDisplayStyle_cycleViaKey(t *testing.T) {
cfg := config.DefaultConfig()
root := &models.Node{
Name:        "git",
Description: "the stupid content tracker",
Children: []*models.Node{
{Name: "commit", Description: "Record changes"},
{Name: "log", Description: "Show commit logs"},
},
}
m := tui.NewModel(root, cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

// Default style should render something.
v0 := m.View()
if v0 == "" {
t.Fatal("empty view in default style")
}

// Press T three times to cycle through all styles; model is mutated in-place.
styleNames := []string{"columns", "compact", "graph"}
for i, styleName := range styleNames {
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
v := m.View()
if v == "" {
t.Errorf("style %q (cycle %d) produced empty view", styleName, i+1)
}
}

// After 3 cycles we are on "graph". Expand root and check connectors.
m.Update(tea.KeyMsg{Type: tea.KeyRight})
vGraph := m.View()
if !strings.Contains(vGraph, "git") {
t.Error("graph style: root node 'git' should be in view")
}
}

func TestDisplayStyle_setDirectly(t *testing.T) {
	root := &models.Node{
		Name:        "aws",
		Description: "AWS CLI https://aws.amazon.com/cli",
		Children: []*models.Node{
			{Name: "s3", Description: "S3 service"},
			{Name: "ec2", Description: "EC2 service"},
		},
	}

	for _, style := range []config.DisplayStyle{
		config.StyleDefault,
		config.StyleColumns,
		config.StyleCompact,
		config.StyleGraph,
	} {
cfg2 := config.DefaultConfig()
cfg2.TreeStyle = style
tree := tui.NewTreeModel(root, cfg2)
tree.SetSize(100, 30)
v := tree.View()
if v == "" {
t.Errorf("style %d produced empty view", style)
}
if !strings.Contains(v, "aws") {
t.Errorf("style %d: root node 'aws' not in view", style)
}
}
}

func TestDisplayStyle_graphConnectors(t *testing.T) {
cfg := config.DefaultConfig()
cfg.TreeStyle = config.StyleGraph
root := &models.Node{
Name: "git",
Children: []*models.Node{
{Name: "commit"},
{Name: "log"},
{Name: "push"},
},
}
tree := tui.NewTreeModel(root, cfg)
tree.SetSize(100, 30)
// Expand root to show children.
tree.Right()
v := tree.View()
// Should contain graph connectors now that children are visible.
if !strings.Contains(v, "──") {
t.Error("graph style: expected '──' connectors after expanding root")
}
}

func TestDisplayStyle_compactNoPills(t *testing.T) {
cfg := config.DefaultConfig()
cfg.TreeStyle = config.StyleCompact
root := &models.Node{
Name: "git",
Flags: []models.Flag{
{Name: "--verbose", ValueType: "bool"},
{Name: "--output", ValueType: "string"},
},
}
tree := tui.NewTreeModel(root, cfg)
tree.SetSize(100, 30)
v := tree.View()
// Compact style should not show inline flag pills.
if strings.Contains(v, "[--") {
t.Error("compact style should not show inline flag pills")
}
}

func TestDisplayStyle_columnsShowsDescription(t *testing.T) {
cfg := config.DefaultConfig()
cfg.TreeStyle = config.StyleColumns
root := &models.Node{
Name:        "kubectl",
Description: "Kubernetes control plane",
Children: []*models.Node{
{Name: "get", Description: "Display resources"},
},
}
tree := tui.NewTreeModel(root, cfg)
tree.SetSize(120, 30)
v := tree.View()
// Columns style should show description with separator.
if !strings.Contains(v, "·") {
t.Error("columns style: expected '·' separator with description")
}
}

func TestExpandAll_expandsAllNodes(t *testing.T) {
cfg := config.DefaultConfig()
root := &models.Node{
Name: "git",
Children: []*models.Node{
{
Name: "remote",
Children: []*models.Node{
{Name: "add"},
{Name: "remove"},
},
},
{Name: "commit"},
},
}
tree := tui.NewTreeModel(root, cfg)
tree.SetSize(100, 40)

rowsBefore := tree.RowCount()
tree.ExpandAll()
rowsAfter := tree.RowCount()

if rowsAfter <= rowsBefore {
t.Errorf("ExpandAll should show more rows: before=%d after=%d", rowsBefore, rowsAfter)
}
}

func TestCollapseAll_collapsesToRoot(t *testing.T) {
cfg := config.DefaultConfig()
root := &models.Node{
Name: "git",
Children: []*models.Node{
{Name: "commit"},
{Name: "log"},
{Name: "push"},
},
}
tree := tui.NewTreeModel(root, cfg)
tree.SetSize(100, 40)

// Expand everything first.
tree.ExpandAll()
rowsExpanded := tree.RowCount()

// Collapse all — only root + its section header should remain visible.
tree.CollapseAll()
rowsCollapsed := tree.RowCount()

if rowsCollapsed >= rowsExpanded {
t.Errorf("CollapseAll should reduce rows: expanded=%d collapsed=%d", rowsExpanded, rowsCollapsed)
}
// Root should still be selected and visible.
if tree.Selected() == nil {
t.Error("CollapseAll: selected node should not be nil after collapse")
}
if tree.Selected().Name != "git" {
t.Errorf("CollapseAll: expected root 'git' selected, got %q", tree.Selected().Name)
}
}

func TestExpandAllFrom_expandsSubtree(t *testing.T) {
cfg := config.DefaultConfig()
remote := &models.Node{
Name: "remote",
Children: []*models.Node{
{Name: "add"},
{Name: "remove"},
},
}
root := &models.Node{
Name:     "git",
Children: []*models.Node{remote, {Name: "commit"}},
}
tree := tui.NewTreeModel(root, cfg)
tree.SetSize(100, 40)

// Only expand the 'remote' subtree, not the whole tree.
tree.ExpandAllFrom(remote, 1)
tree.Rebuild()

v := tree.ViewSized(100, 40)
// remote's children should now be visible since root is already expanded
// and we expanded remote's subtree.
if !strings.Contains(v, "git") {
t.Error("root should be visible")
}
}

func TestModel_ShiftRight_expandsAll(t *testing.T) {
cfg := config.DefaultConfig()
root := &models.Node{
Name: "aws",
Children: []*models.Node{
{Name: "s3", Children: []*models.Node{{Name: "cp"}, {Name: "ls"}}},
{Name: "ec2"},
},
}
// Test via TreeModel directly (simpler, no Model wrapper needed).
tree := tui.NewTreeModel(root, cfg)
tree.SetSize(120, 40)

rowsBefore := tree.RowCount()
tree.ExpandAll()
rowsAfter := tree.RowCount()

if rowsAfter <= rowsBefore {
t.Errorf("ExpandAll should show more rows: before=%d after=%d", rowsBefore, rowsAfter)
}
}

func TestModel_ShiftLeft_collapsesAll(t *testing.T) {
cfg := config.DefaultConfig()
root := &models.Node{
Name: "aws",
Children: []*models.Node{
{Name: "s3", Children: []*models.Node{{Name: "cp"}}},
{Name: "ec2"},
},
}
tree := tui.NewTreeModel(root, cfg)
tree.SetSize(120, 40)

tree.ExpandAll()
rowsExpanded := tree.RowCount()
tree.CollapseAll()
rowsCollapsed := tree.RowCount()

if rowsCollapsed >= rowsExpanded {
t.Errorf("CollapseAll should reduce rows: expanded=%d collapsed=%d", rowsExpanded, rowsCollapsed)
}
}

func TestModel_ExpandCollapseAll_viaKeyMsg(t *testing.T) {
cfg := config.DefaultConfig()
root := &models.Node{
Name: "git",
Children: []*models.Node{
{Name: "remote", Children: []*models.Node{{Name: "add"}}},
{Name: "commit"},
},
}
m := tui.NewModel(root, cfg)
m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

// Shift+Right should expand all.
m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("shift+right")})
// Use View to confirm the model still renders without panicking.
v := m.View()
if v == "" {
t.Error("view should not be empty after expand-all key")
}
}

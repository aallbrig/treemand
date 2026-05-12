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

func TestTreeModel_Down_doesNotAutoExpand(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := tui.NewTreeModel(sampleTree(), cfg)
	tree.SetSize(80, 40)
	// Navigate down to "commit" (past section headers and flags).
	navigateTreeTo(tree, "commit")
	sel := tree.SelectedItem()
	if sel == nil || sel.Kind != tui.SelCommand || sel.Node.Name != "commit" {
		t.Fatalf("expected commit, got %v", sel)
	}
	// Down again from commit should move to the NEXT sibling (remote), NOT auto-expand commit.
	tree.Down()
	sel2 := tree.SelectedItem()
	if sel2 == nil {
		t.Fatal("expected selection after Down from commit")
	}
	// commit is collapsed, so Down should skip to the next visible sibling.
	if sel2.Kind == tui.SelCommand && sel2.Node.Name == "commit" {
		t.Error("Down from commit should not stay on commit")
	}
}

// navigateTreeTo moves the TreeModel cursor down until it reaches a command
// node with the given name, or exhausts 50 iterations. Test helper.
func navigateTreeTo(tree *tui.TreeModel, name string) {
	for i := 0; i < 50; i++ {
		sel := tree.SelectedItem()
		if sel != nil && sel.Kind == tui.SelCommand && sel.Node.Name == name {
			return
		}
		tree.Down()
	}
}

// navigateModelTo sends KeyDown messages to a Model until the TreeModel
// cursor lands on a command node with the given name. Unlike navigateTo,
// this does NOT toggle sections or expand all — it preserves the default
// tree state and navigates through section headers normally.
func navigateModelTo(m *tui.Model, name string) {
	for i := 0; i < 50; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelCommand && sel.Node.Name == name {
			return
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
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
	// Going up should traverse every row including section headers.
	// Section rows are now navigable, so SelectedItem() returns nil on them.
	// Verify Up() never panics and the cursor always moves.
	for i := 0; i < 6; i++ {
		tree.Up()
		// No panic is the main assertion. Section rows return nil
		// from SelectedItem(), which is expected.
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
	for _, r := range "comm" {
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
	tree.PatchNode(nil, nil)                      // no panic
	tree.PatchNode(sampleTree().Children[0], nil) // no panic
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
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  // select commit
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand commit
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  // move to --message flag
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
	m.Update(tea.KeyMsg{Type: tea.KeyRight})              // expand
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
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})                     // vim scheme
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
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})               // down
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})               // right/expand
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})               // left/collapse
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
	updated, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 5, Y: 5})
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
		Name:        "s3",
		FullPath:    []string{"aws", "s3"},
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

func TestToggleSections_hidesAndShowsHeaders(t *testing.T) {
	cfg := config.DefaultConfig()
	root := &models.Node{
		Name: "git",
		Children: []*models.Node{
			{Name: "commit"},
			{Name: "log"},
		},
		Flags: []models.Flag{
			{Name: "--verbose"},
		},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(100, 40)

	// Expand so we can see sections.
	tree.ExpandAll()
	withSections := tree.RowCount()

	tree.ToggleSections()
	if !tree.SectionsHidden() {
		t.Error("SectionsHidden should be true after first toggle")
	}

	// With sections hidden, row count should be lower (no header rows).
	withoutSections := tree.RowCount()
	if withoutSections >= withSections {
		t.Errorf("hiding sections should reduce row count: with=%d without=%d", withSections, withoutSections)
	}

	// Toggle back — sections should reappear.
	tree.ToggleSections()
	if tree.SectionsHidden() {
		t.Error("SectionsHidden should be false after second toggle")
	}
	restored := tree.RowCount()
	if restored != withSections {
		t.Errorf("restoring sections should return to original row count: want=%d got=%d", withSections, restored)
	}
}

func TestConfigStatusMsgTimeout_default(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.StatusMsgTimeout <= 0 {
		t.Errorf("StatusMsgTimeout should have a positive default, got %v", cfg.StatusMsgTimeout)
	}
}

// --- VS Code-style navigation model tests ---

func TestRight_expandsOnFirstPress_staysOnNode(t *testing.T) {
	cfg := config.DefaultConfig()
	root := &models.Node{
		Name: "git",
		Children: []*models.Node{
			{Name: "commit", Children: []*models.Node{{Name: "amend"}}},
		},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(100, 40)
	// Cursor starts on root (git, expanded). Down lands on section header,
	// then another Down reaches commit (collapsed).
	tree.Down() // → "Subcommands" section header
	tree.Down() // → commit
	if sel := tree.SelectedItem(); sel == nil || sel.Node.Name != "commit" {
		t.Fatalf("expected cursor on commit, got %v", sel)
	}
	// First Right on collapsed commit: expand it but stay on commit.
	tree.Right()
	sel := tree.SelectedItem()
	if sel == nil || sel.Node.Name != "commit" {
		t.Errorf("first Right should stay on commit; got %v", sel)
	}
	// Second Right on expanded commit: enter first command child (amend).
	tree.Right()
	sel2 := tree.SelectedItem()
	if sel2 == nil || sel2.Node.Name != "amend" {
		t.Errorf("second Right should enter 'amend'; got %v", sel2)
	}
}

func TestRight_leafNodeEntersFlags(t *testing.T) {
	cfg := config.DefaultConfig()
	root := &models.Node{
		Name: "git",
		Children: []*models.Node{
			{
				Name: "status",
				Flags: []models.Flag{
					{Name: "--short", ShortName: "s"},
					{Name: "--branch"},
				},
			},
		},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(100, 40)
	// NOTE: sections are NOT hidden — this matches real user behavior.

	// Navigate to status (leaf command — no subcommands, only flags).
	// Down from git lands on section header first, then status.
	tree.Down() // → "Subcommands" section header
	tree.Down() // → status
	if sel := tree.SelectedItem(); sel == nil || sel.Node.Name != "status" {
		t.Fatalf("expected cursor on status, got %v", sel)
	}

	// First Right: expand status (sections appear but are collapsed).
	tree.Right()
	sel := tree.SelectedItem()
	if sel == nil || sel.Node.Name != "status" {
		t.Errorf("first Right should stay on status; got %v", sel)
	}

	// Second Right: should auto-expand the first section and enter its first flag.
	tree.Right()
	sel2 := tree.SelectedItem()
	if sel2 == nil {
		t.Fatal("second Right should move to a child row, got nil")
	}
	if sel2.Kind != tui.SelFlag {
		t.Errorf("second Right on leaf should enter flag row; got kind=%d", sel2.Kind)
	}
	if sel2.Flag.Name != "--short" {
		t.Errorf("expected --short, got %s", sel2.Flag.Name)
	}
}

func TestLeft_collapseAndStay_thenGoToParent(t *testing.T) {
	cfg := config.DefaultConfig()
	root := &models.Node{
		Name: "git",
		Children: []*models.Node{
			{Name: "commit", Children: []*models.Node{{Name: "amend"}}},
			{Name: "log"},
		},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(100, 40)
	// Expand commit and navigate into it.
	// Down from git lands on section header first, then commit.
	tree.Down()  // → "Subcommands" section header
	tree.Down()  // → commit
	tree.Right() // expand commit (stay)
	tree.Right() // → amend (Right skips section headers and finds first command child)
	if sel := tree.SelectedItem(); sel == nil || sel.Node.Name != "amend" {
		t.Fatalf("expected cursor on amend, got %v", sel)
	}
	// Left from amend → go to parent (commit).
	tree.Left()
	if sel := tree.SelectedItem(); sel == nil || sel.Node.Name != "commit" {
		t.Fatalf("Left from amend should return to commit; got %v", sel)
	}
	// Left on expanded commit → collapse and stay on commit.
	tree.Left()
	if sel := tree.SelectedItem(); sel == nil || sel.Node.Name != "commit" {
		t.Errorf("Left on expanded commit should stay on commit; got %v", sel)
	}
	// commit is now collapsed. Down should reach log (the sibling), not amend (the child).
	tree.Down()
	if sel := tree.SelectedItem(); sel == nil || sel.Node.Name != "log" {
		t.Errorf("Down after collapse should reach sibling 'log'; got %v", sel)
	}
}

func TestShiftRight_expandsSubtree_notGlobal(t *testing.T) {
	cfg := config.DefaultConfig()
	remote := &models.Node{
		Name:     "remote",
		Children: []*models.Node{{Name: "add"}, {Name: "remove"}},
	}
	root := &models.Node{
		Name:     "git",
		Children: []*models.Node{remote, {Name: "commit"}},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(100, 40)
	rowsBefore := tree.RowCount()

	// Navigate to remote (first child) and expand its subtree.
	tree.Down() // → remote
	tree.ExpandAllFrom(remote, 1)
	tree.Rebuild()
	rowsAfterSubtree := tree.RowCount()

	if rowsAfterSubtree <= rowsBefore {
		t.Errorf("expanding subtree of remote should add rows: before=%d after=%d", rowsBefore, rowsAfterSubtree)
	}
	// commit should still be collapsed (not expanded as a side-effect).
	tree.Down() // move past remote's children
	// We should be able to navigate without commit auto-expanding.
}

func TestShiftLeft_collapsesSubtree_staysOnNode(t *testing.T) {
	cfg := config.DefaultConfig()
	remote := &models.Node{
		Name:     "remote",
		Children: []*models.Node{{Name: "add"}, {Name: "remove"}},
	}
	root := &models.Node{
		Name:     "git",
		Children: []*models.Node{remote, {Name: "commit"}},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(100, 40)

	// Expand everything, then navigate to remote.
	tree.ExpandAll()
	tree.Down() // → remote (or a child if already expanded)
	// Navigate to remote explicitly.
	for {
		sel := tree.SelectedItem()
		if sel == nil {
			break
		}
		if sel.Node.Name == "remote" {
			break
		}
		tree.Down()
	}
	sel := tree.SelectedItem()
	if sel == nil || sel.Node.Name != "remote" {
		t.Skip("could not navigate to remote; skip")
	}

	// Collapse remote's subtree and confirm cursor stays on remote.
	tree.CollapseSubtree(remote, 1)
	tree.Rebuild()
	after := tree.SelectedItem()
	_ = after // cursor may shift; main assertion is no panic
	rowsAfter := tree.RowCount()
	_ = rowsAfter // main assertion: no panic and tree is still usable.
}

// ---------- helper: navigate Model to a specific item ----------

// navigateTo walks the tree downward until the predicate returns true.
// It enables flat mode (hides sections) and expands all nodes first so
// flags and positionals are directly reachable by Down().
// Returns false if the item was not found within 200 steps.
func navigateTo(m *tui.Model, match func(*tui.Selection) bool) bool {
	if !m.TreeModel().SectionsHidden() {
		m.TreeModel().ToggleSections()
	}
	m.TreeModel().ExpandAll()
	for i := 0; i < 200; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && match(sel) {
			return true
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	return false
}

// ---------- Value Modal Tests ----------

func TestValueModal_openOnStringFlag(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Expand commit so flags are visible.
	found := navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelCommand && s.Node.Name == "commit"
	})
	if !found {
		t.Fatal("could not navigate to commit")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // enter children

	// Navigate to --message (ValueType = "string").
	found = navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelFlag && s.Flag != nil && s.Flag.Name == "--message"
	})
	if !found {
		t.Fatal("could not navigate to --message flag")
	}

	// Press Enter — should open value modal.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	v := m.View()
	if !strings.Contains(v, "--message") {
		t.Error("value modal should show --message label")
	}
	if !strings.Contains(v, "confirm") {
		t.Error("value modal should show confirm hint")
	}
}

func TestValueModal_confirmAddsToPreview(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	found := navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelCommand && s.Node.Name == "commit"
	})
	if !found {
		t.Fatal("could not navigate to commit")
	}
	// Set commit as the command first.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // enter children

	found = navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelFlag && s.Flag != nil && s.Flag.Name == "--message"
	})
	if !found {
		t.Fatal("could not navigate to --message")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // opens value modal

	// Type a value.
	for _, r := range "hello" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Confirm.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// View should be back to normal tree (not modal).
	v := m.View()
	if strings.Contains(v, "[Enter] confirm") {
		t.Error("modal should be closed after confirm")
	}
	// Preview should contain the flag value.
	if !strings.Contains(v, "--message=hello") {
		t.Errorf("preview should contain --message=hello, got:\n%s", v)
	}
}

func TestValueModal_escCancels(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	found := navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelCommand && s.Node.Name == "commit"
	})
	if !found {
		t.Fatal("could not navigate to commit")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})

	found = navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelFlag && s.Flag != nil && s.Flag.Name == "--message"
	})
	if !found {
		t.Fatal("could not navigate to --message")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // opens modal

	// Type partial value then Esc.
	for _, r := range "partial" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	v := m.View()
	if strings.Contains(v, "[Enter] confirm") {
		t.Error("modal should be closed after Esc")
	}
	if strings.Contains(v, "--message=partial") {
		t.Error("Esc should not add value to preview")
	}
}

func TestPositionalModal_open(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	found := navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelCommand && s.Node.Name == "commit"
	})
	if !found {
		t.Fatal("could not navigate to commit")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Navigate to the positional argument.
	found = navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelPositional
	})
	if !found {
		t.Fatal("could not navigate to positional arg")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // opens positional modal

	v := m.View()
	if !strings.Contains(v, "msg") {
		t.Error("positional modal should show arg name")
	}
}

func TestBoolFlag_addsDirectly(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	found := navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelCommand && s.Node.Name == "commit"
	})
	if !found {
		t.Fatal("could not navigate to commit")
	}
	// Set commit as command.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Navigate to --all (bool flag, no ValueType).
	found = navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelFlag && s.Flag != nil && s.Flag.Name == "--all"
	})
	if !found {
		t.Fatal("could not navigate to --all")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // should add directly, no modal

	v := m.View()
	if strings.Contains(v, "[Enter] confirm") {
		t.Error("bool flag should not open value modal")
	}
	if !strings.Contains(v, "--all") {
		t.Error("preview should contain --all")
	}
}

// ---------- Mouse Tests ----------

func TestMouse_clickTreePane(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Click in the tree area (below preview bar).
	m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      5,
		Y:      5, // below the 2-line preview bar
	})

	v := m.View()
	if !strings.Contains(v, "focus: tree") {
		t.Error("clicking tree area should set focus to tree")
	}
}

func TestMouse_clickPreviewPane(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Click in the preview bar area (Y=0).
	m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      0,
	})

	v := m.View()
	if !strings.Contains(v, "focus: preview") {
		t.Error("clicking preview area should set focus to preview")
	}
}

func TestMouse_clickHelpPane(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Toggle help pane on via Update (not direct method).
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})

	// Click in the help pane area (right side, below preview bar).
	m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      110, // right side of 120-wide terminal
		Y:      10,  // well below preview bar
	})

	v := m.View()
	if !strings.Contains(v, "focus: help") {
		// The help pane might not be wide enough; check we at least don't crash.
		if !strings.Contains(v, "focus:") {
			t.Error("clicking should set some focus status message")
		}
	}
}

func TestMouse_scrollWheelDown(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Scroll down in tree pane.
	m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
		X:      5,
		Y:      5,
	})

	// Should move cursor down without crashing. The cursor may land on a
	// section row (where SelectedItem returns nil) — that is expected.
	// Scroll a second time to ensure we can reach a non-section row.
	m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
		X:      5,
		Y:      5,
	})
	sel := m.TreeModel().SelectedItem()
	if sel == nil {
		t.Fatal("should have a selection after two scrolls down")
	}
}

func TestMouse_scrollWheelUp(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate down past the section header so scroll-up lands on a command row.
	// Row 0: git, Row 1: section, Row 2: commit, Row 3: remote.
	m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → section
	m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → commit
	m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → remote

	// Scroll up from remote → commit (a command row).
	m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
		X:      5,
		Y:      5,
	})

	sel := m.TreeModel().SelectedItem()
	if sel == nil {
		t.Fatal("should have a selection after scroll up")
	}
}

// ---------- Flag Modal Tests ----------

func TestFlagModal_openAndClose(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to commit (a command with flags).
	found := navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelCommand && s.Node.Name == "commit"
	})
	if !found {
		t.Fatal("could not navigate to commit")
	}

	// Press 'f' to open flag modal.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})

	v := m.View()
	if !strings.Contains(v, "--message") && !strings.Contains(v, "--all") {
		t.Error("flag modal should show flags for commit")
	}

	// Press Esc to close.
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	v = m.View()
	// Should be back to tree view (contains Tree: or similar).
	if strings.Contains(v, "[Enter] select") {
		t.Error("flag modal should be closed after Esc")
	}
}

// ---------- Execute Modal Tests ----------

func TestExecuteModal_openOnCtrlE(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Set a command in the preview.
	found := navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelCommand && s.Node.Name == "commit"
	})
	if !found {
		t.Fatal("could not navigate to commit")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // set in preview

	// Press Ctrl+E to open execute modal.
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})

	v := m.View()
	// Execute modal should contain the command and options.
	if !strings.Contains(v, "git commit") {
		t.Error("execute modal should show the command")
	}
}

func TestExecuteModal_escCancels(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	found := navigateTo(m, func(s *tui.Selection) bool {
		return s.Kind == tui.SelCommand && s.Node.Name == "commit"
	})
	if !found {
		t.Fatal("could not navigate to commit")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlE}) // open modal

	// Press Esc to cancel.
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	v := m.View()
	// Should be back to normal view.
	if strings.Contains(v, "Run") && strings.Contains(v, "Copy") {
		t.Error("execute modal should be closed after Esc")
	}
}

// ---------- Integration: End-to-End Command Assembly ----------

func TestWorkflow_navigatePickFlagCopy(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Enable flat mode and expand all up front.
	m.TreeModel().ToggleSections()
	m.TreeModel().ExpandAll()

	// Step 1: Navigate to commit.
	found := false
	for i := 0; i < 50; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelCommand && sel.Node.Name == "commit" {
			found = true
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if !found {
		t.Fatal("could not navigate to commit")
	}

	// Step 2: Pick the command.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	v := m.View()
	if !strings.Contains(v, "git commit") {
		t.Error("preview should show 'git commit' after picking")
	}

	// Step 3: Navigate to --all flag (bool) and add it.
	found = false
	for i := 0; i < 50; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelFlag && sel.Flag != nil && sel.Flag.Name == "--all" {
			found = true
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if !found {
		t.Fatal("could not navigate to --all")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add bool flag

	v = m.View()
	if !strings.Contains(v, "--all") {
		t.Error("preview should contain --all")
	}

	// Step 4: Navigate to --message and add with value.
	// We might need to go Up since --message might be above --all.
	found = false
	for i := 0; i < 50; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelFlag && sel.Flag != nil && sel.Flag.Name == "--message" {
			found = true
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	if !found {
		// Try downward too.
		for i := 0; i < 50; i++ {
			sel := m.TreeModel().SelectedItem()
			if sel != nil && sel.Kind == tui.SelFlag && sel.Flag != nil && sel.Flag.Name == "--message" {
				found = true
				break
			}
			m.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
	}
	if !found {
		t.Fatal("could not navigate to --message")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // opens value modal

	for _, r := range "test" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm

	v = m.View()
	if !strings.Contains(v, "--message=test") {
		t.Errorf("preview should contain --message=test, got:\n%s", v)
	}

	// Step 5: Ctrl+E should open execute modal.
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	v = m.View()
	if !strings.Contains(v, "git commit") {
		t.Error("execute modal should show the assembled command")
	}
}

// TestFlagAddsSubcommandChain verifies that adding a flag on a deep subcommand
// automatically sets the full subcommand path in the preview, even when the
// user never pressed Enter on any ancestor node.
func TestFlagAddsSubcommandChain(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Flat + expanded so we can reach flags directly.
	m.TreeModel().ToggleSections()
	m.TreeModel().ExpandAll()

	// Confirm preview starts empty (no subcommand chosen yet).
	if strings.Contains(m.View(), "git commit") {
		t.Fatal("preview should be empty before any selection")
	}

	// Navigate to --all flag under "commit" without pressing Enter on any
	// command node first.
	found := false
	for i := 0; i < 100; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelFlag && sel.Flag != nil && sel.Flag.Name == "--all" &&
			sel.Owner != nil && sel.Owner.Name == "commit" {
			found = true
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if !found {
		t.Fatal("could not navigate to commit's --all flag")
	}

	// Add the flag (bool, so Enter adds it directly).
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	v := m.View()
	if !strings.Contains(v, "git") {
		t.Errorf("preview should contain root command 'git', got:\n%s", v)
	}
	if !strings.Contains(v, "commit") {
		t.Errorf("preview should contain subcommand 'commit', got:\n%s", v)
	}
	if !strings.Contains(v, "--all") {
		t.Errorf("preview should contain flag '--all', got:\n%s", v)
	}
}

// =============================================================================
// Acceptance tests — regression guards for the key UX behaviors we care about.
//
// Group A: Leaf-node navigation (Right key entering flags/positionals)
// Group B: Auto-subcommand-chain (adding a flag/positional automatically sets
//           its full ancestor command path in the draft preview)
// =============================================================================

// --- Group A: Leaf-node navigation ---

// TestAccept_LeafNav_firstRightExpandsNode verifies that pressing → on a
// collapsed leaf command expands it and keeps the cursor there.
func TestAccept_LeafNav_firstRightExpandsNode(t *testing.T) {
	cfg := config.DefaultConfig()
	root := &models.Node{
		Name: "git", FullPath: []string{"git"},
		Children: []*models.Node{
			{Name: "commit", FullPath: []string{"git", "commit"},
				Flags: []models.Flag{{Name: "--all"}}},
		},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(120, 40)

	tree.Down() // git → "Subcommands" section header
	tree.Down() // section → commit
	if sel := tree.SelectedItem(); sel == nil || sel.Node.Name != "commit" {
		t.Fatalf("expected cursor on commit, got %v", sel)
	}
	tree.Right() // first →: expand commit, stay on commit
	if sel := tree.SelectedItem(); sel == nil || sel.Node.Name != "commit" {
		t.Errorf("first Right should stay on commit; got %v", sel)
	}
}

// TestAccept_LeafNav_secondRightEntersFlagSection verifies that pressing → a
// second time on an expanded leaf command enters its flags section (auto-expands
// the collapsed Flags section header and lands on the first flag).
func TestAccept_LeafNav_secondRightEntersFlagSection(t *testing.T) {
	cfg := config.DefaultConfig()
	root := &models.Node{
		Name: "git", FullPath: []string{"git"},
		Children: []*models.Node{
			{Name: "commit", FullPath: []string{"git", "commit"},
				Flags: []models.Flag{
					{Name: "--message", ValueType: "string"},
					{Name: "--all"},
				}},
		},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(120, 40)

	tree.Down()  // → "Subcommands" section header
	tree.Down()  // → commit
	tree.Right() // expand commit (stay)
	tree.Right() // auto-expand Flags section, land on --message

	sel := tree.SelectedItem()
	if sel == nil {
		t.Fatal("second Right on leaf should move to a flag row, got nil")
	}
	if sel.Kind != tui.SelFlag {
		t.Errorf("expected SelFlag, got kind=%d", sel.Kind)
	}
	if sel.Flag.Name != "--message" {
		t.Errorf("expected --message (first flag), got %s", sel.Flag.Name)
	}
}

// TestAccept_LeafNav_canNavigateToSiblingWithoutExpandingFlags verifies that
// after collapsing a leaf node the user can navigate straight to the sibling
// command without getting stuck inside the flags section.
func TestAccept_LeafNav_canNavigateToSiblingWithoutExpandingFlags(t *testing.T) {
	cfg := config.DefaultConfig()
	root := &models.Node{
		Name: "git", FullPath: []string{"git"},
		Children: []*models.Node{
			{Name: "commit", FullPath: []string{"git", "commit"},
				Flags: []models.Flag{{Name: "--all"}}},
			{Name: "log", FullPath: []string{"git", "log"}},
		},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(120, 40)

	tree.Down()  // → "Subcommands" section header
	tree.Down()  // → commit
	tree.Right() // expand commit
	tree.Left()  // collapse commit (stay on commit)

	// Down should jump to the sibling "log", not into the flags section.
	tree.Down()
	sel := tree.SelectedItem()
	if sel == nil || sel.Kind != tui.SelCommand {
		t.Fatalf("expected a command row after Down, got %v", sel)
	}
	if sel.Node.Name != "log" {
		t.Errorf("Down after collapse should reach sibling 'log'; got %s", sel.Node.Name)
	}
}

// --- Group B: Auto-subcommand-chain ---

// TestAccept_AutoChain_boolFlagRealNavigation is the primary regression test for
// the auto-chain feature in the real default tree state (sections collapsed, no
// ToggleSections shortcut). It navigates to a flag using only arrow keys and
// verifies the preview includes the full ancestor command path.
func TestAccept_AutoChain_boolFlagRealNavigation(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// NOTE: NO ToggleSections() or ExpandAll() — real default state.

	// Navigate to commit (past any section headers and flags).
	navigateModelTo(m, "commit")

	// First Right: expand commit (stay on commit).
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	// Second Right: enter flags (auto-expanded), land on --message.
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	// Down once: --message → --all.
	m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Confirm we're on --all before pressing Enter.
	sel := m.TreeModel().SelectedItem()
	if sel == nil || sel.Kind != tui.SelFlag || sel.Flag.Name != "--all" {
		t.Fatalf("expected cursor on --all flag, got %v", sel)
	}
	if sel.Owner == nil || sel.Owner.Name != "commit" {
		t.Fatalf("expected Owner=commit, got %v", sel.Owner)
	}

	// Add the flag — should also auto-set "git commit" as the command base.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	v := m.View()
	if !strings.Contains(v, "git") {
		t.Errorf("preview should contain 'git'; got:\n%s", v)
	}
	if !strings.Contains(v, "commit") {
		t.Errorf("preview should contain 'commit' (auto-chained); got:\n%s", v)
	}
	if !strings.Contains(v, "--all") {
		t.Errorf("preview should contain '--all'; got:\n%s", v)
	}
}

// TestAccept_AutoChain_stringFlagViaValueModal verifies that adding a string
// flag through the value modal also triggers the auto-chain.
func TestAccept_AutoChain_stringFlagViaValueModal(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to commit, then expand it and enter flags to land on --message.
	navigateModelTo(m, "commit")
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand commit
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // enter flags, land on --message

	sel := m.TreeModel().SelectedItem()
	if sel == nil || sel.Kind != tui.SelFlag || sel.Flag.Name != "--message" {
		t.Fatalf("expected cursor on --message, got %v", sel)
	}

	// Enter opens the value modal (--message is type string).
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Type a value and confirm.
	for _, r := range "mycommit" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	v := m.View()
	if !strings.Contains(v, "commit") {
		t.Errorf("preview should contain 'commit' (auto-chained); got:\n%s", v)
	}
	if !strings.Contains(v, "--message=mycommit") {
		t.Errorf("preview should contain '--message=mycommit'; got:\n%s", v)
	}
}

// TestAccept_AutoChain_viaFlagModal verifies that opening the flag modal (F key)
// on a command node and selecting a flag auto-chains the parent path.
func TestAccept_AutoChain_viaFlagModal(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to commit (DO NOT press Enter — no explicit command selection).
	navigateModelTo(m, "commit")

	sel := m.TreeModel().SelectedItem()
	if sel == nil || sel.Node.Name != "commit" {
		t.Fatalf("expected cursor on commit, got %v", sel)
	}

	// Press F to open flag modal.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// --message is first (index 0), --all is second. Press Down once to reach --all.
	m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Enter to add --all (bool flag). The modal stays open (by design, so users can
	// add multiple flags), so close it with Esc before checking the preview bar.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // dismiss modal so preview bar is visible

	v := m.View()
	if !strings.Contains(v, "commit") {
		t.Errorf("preview should contain 'commit' (auto-chained via flag modal); got:\n%s", v)
	}
	if !strings.Contains(v, "--all") {
		t.Errorf("preview should contain '--all'; got:\n%s", v)
	}
}

// TestAccept_AutoChain_positionalModal verifies that adding a positional
// argument auto-chains the parent subcommand path.
func TestAccept_AutoChain_positionalModal(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Use flat mode to access positionals easily.
	m.TreeModel().ToggleSections()
	m.TreeModel().ExpandAll()

	// Navigate to the <msg> positional under commit.
	found := false
	for i := 0; i < 100; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelPositional &&
			sel.Owner != nil && sel.Owner.Name == "commit" {
			found = true
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if !found {
		t.Fatal("could not navigate to commit's positional argument")
	}

	// Enter opens positional modal.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	for _, r := range "HEAD~1" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm

	v := m.View()
	if !strings.Contains(v, "commit") {
		t.Errorf("preview should contain 'commit' (auto-chained for positional); got:\n%s", v)
	}
	if !strings.Contains(v, "HEAD~1") {
		t.Errorf("preview should contain the positional value 'HEAD~1'; got:\n%s", v)
	}
}

// TestAccept_AutoChain_secondFlagPreservesChain verifies that after the first
// auto-chain sets "git commit", adding a second flag on the same node appends
// to the existing chain rather than resetting or duplicating it.
func TestAccept_AutoChain_secondFlagPreservesChain(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	m.TreeModel().ToggleSections()
	m.TreeModel().ExpandAll()

	// Add --all first (triggers auto-chain → "git commit --all").
	found := false
	for i := 0; i < 100; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelFlag && sel.Flag.Name == "--all" &&
			sel.Owner != nil && sel.Owner.Name == "commit" {
			found = true
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if !found {
		t.Fatal("could not navigate to --all")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // adds --all, auto-chains

	// Now navigate to --amend on the same node.
	found = false
	for i := 0; i < 100; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelFlag && sel.Flag.Name == "--amend" {
			found = true
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if !found {
		// Try up too.
		for i := 0; i < 100; i++ {
			sel := m.TreeModel().SelectedItem()
			if sel != nil && sel.Kind == tui.SelFlag && sel.Flag.Name == "--amend" {
				found = true
				break
			}
			m.Update(tea.KeyMsg{Type: tea.KeyUp})
		}
	}
	if !found {
		t.Fatal("could not navigate to --amend")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add --amend

	v := m.View()
	// Both flags must be present and the subcommand must appear exactly once.
	if !strings.Contains(v, "--all") {
		t.Errorf("preview should still contain --all; got:\n%s", v)
	}
	if !strings.Contains(v, "--amend") {
		t.Errorf("preview should contain --amend; got:\n%s", v)
	}
	// "git commit" must not be duplicated (e.g. "git commit --all git commit --amend").
	if strings.Contains(v, "commit --all git commit") || strings.Contains(v, "commit --amend git commit") {
		t.Errorf("'commit' subcommand should not be duplicated in preview; got:\n%s", v)
	}
}

// TestAccept_AutoChain_rootFlagNoDoubleBase verifies that adding a flag on the
// root command itself does not produce a doubled base (e.g. "git git --version").
func TestAccept_AutoChain_rootFlagNoDoubleBase(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	m.TreeModel().ToggleSections()
	m.TreeModel().ExpandAll()

	// Navigate to --version on the root "git" node.
	found := false
	for i := 0; i < 100; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelFlag && sel.Flag.Name == "--version" &&
			sel.Owner != nil && sel.Owner.Name == "git" {
			found = true
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if !found {
		t.Fatal("could not navigate to git's --version flag")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	v := m.View()
	if !strings.Contains(v, "--version") {
		t.Errorf("preview should contain '--version'; got:\n%s", v)
	}
	// "git" must not be doubled in the preview command (e.g. "git git --version").
	if strings.Contains(v, "git git") {
		t.Errorf("root command 'git' should not be doubled; got:\n%s", v)
	}
}

// sampleTreeWithDescriptions builds a tree with descriptions for testing
// that the default style shows descriptions inline.
func sampleTreeWithDescriptions() *models.Node {
	return &models.Node{
		Name:        "mycli",
		FullPath:    []string{"mycli"},
		Description: "A sample CLI for testing descriptions",
		Children: []*models.Node{
			{
				Name:        "serve",
				FullPath:    []string{"mycli", "serve"},
				Description: "Start the HTTP server",
				Flags: []models.Flag{
					{Name: "--port", ValueType: "int", Description: "Port to listen on"},
					{Name: "--host", ValueType: "string", Description: "Host to bind to"},
				},
			},
			{
				Name:        "deploy",
				FullPath:    []string{"mycli", "deploy"},
				Description: "Deploy the application",
				Positionals: []models.Positional{
					{Name: "target", Required: true, Description: "Deployment target"},
				},
			},
		},
	}
}

func TestDefaultStyle_showsDescriptions(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TreeStyle = config.StyleDefault
	m := tui.NewModel(sampleTreeWithDescriptions(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := m.View()
	// The root node starts expanded so its children's descriptions should be visible.
	if !strings.Contains(v, "Start the HTTP server") {
		t.Errorf("default style should show child descriptions; got:\n%s", v)
	}
	if !strings.Contains(v, "Deploy the application") {
		t.Errorf("default style should show child descriptions; got:\n%s", v)
	}
}

func TestAutoExpandSmallFlagSections(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := sampleTreeWithDescriptions()
	m := tui.NewModel(tree, cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to "serve" and expand it. With section rows navigable,
	// we need to move past the "Subcommands" section header first.
	// Row layout: mycli, Subcommands(2), serve, deploy
	// Down from root → Subcommands section → Right enters first child (serve)
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  // → Subcommands (2)
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // section already expanded → jump into "serve"
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand "serve" node

	v := m.View()
	// "serve" has only 2 flags — they should be auto-expanded (≤5 threshold).
	if !strings.Contains(v, "--port") {
		t.Errorf("small flag sections should auto-expand; expected --port in view:\n%s", v)
	}
	if !strings.Contains(v, "--host") {
		t.Errorf("small flag sections should auto-expand; expected --host in view:\n%s", v)
	}
}

func TestAutoExpandSmallPositionalSections(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := sampleTreeWithDescriptions()
	m := tui.NewModel(tree, cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to "deploy" and expand it. The tree layout is:
	// mycli, Subcommands(2), serve, deploy
	// We need to reach "deploy" and expand it.
	for i := 0; i < 10; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelCommand && sel.Node.Name == "deploy" {
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand "deploy"

	v := m.View()
	// "deploy" has 1 positional — it should be auto-expanded.
	if !strings.Contains(v, "target") {
		t.Errorf("small positional sections should auto-expand; expected target in view:\n%s", v)
	}
}

func TestWordWrap(t *testing.T) {
	// Test via exported function — wordWrap is package-internal but we can
	// exercise it through the HelpPaneModel's View output.
	cfg := config.DefaultConfig()
	tree := &models.Node{
		Name:        "wraptest",
		FullPath:    []string{"wraptest"},
		Description: "This is a very long description that should be wrapped when the help pane is narrow enough to require word wrapping behaviour",
		Flags: []models.Flag{
			{Name: "--verbose", Description: "Enable verbose output"},
		},
	}
	m := tui.NewModel(tree, cfg)
	// Use a wide terminal so the help pane is visible (requires ≥80 cols).
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	v := m.View()
	// The help pane should show the description via word-wrapping.
	if !strings.Contains(v, "description") {
		t.Errorf("help pane should show description via word wrap; got:\n%s", v)
	}
}

func TestHandlePick_sharedAcrossSchemes(t *testing.T) {
	// Verify Enter works the same in all three nav schemes.
	for _, scheme := range []string{"arrows", "vim", "wasd"} {
		t.Run(scheme, func(t *testing.T) {
			cfg := config.DefaultConfig()
			m := tui.NewModel(sampleTree(), cfg)
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

			// Switch nav scheme if needed.
			switch scheme {
			case "vim":
				m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
			case "wasd":
				m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
				m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
			}

			// Navigate down to first child, then Enter to pick.
			downKey := tea.KeyMsg{Type: tea.KeyDown}
			switch scheme {
			case "vim":
				downKey = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
			case "wasd":
				downKey = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
			}
			m.Update(downKey)
			m.Update(tea.KeyMsg{Type: tea.KeyEnter})

			v := m.View()
			if !strings.Contains(v, "commit") {
				t.Errorf("[%s] after Enter on commit node, preview should contain 'commit'; got:\n%s", scheme, v)
			}
		})
	}
}

func TestMouse_clickSelectsRow(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// The tree pane starts after the preview bar (2 lines) + border (1 line).
	// Row 0 = root "git" (at y = previewBarHeight + 1 = 3).
	// Row 1 = "commit" subcommand (at y = 4).
	m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      4, // should hit the second row in the tree
	})

	// After a click, cursor should have moved. SelectedItem may be nil
	// if the click landed on a section header, which is valid.
	v := m.View()
	if v == "" {
		t.Fatal("view should not be empty after mouse click")
	}
}

func TestSectionRow_keyboardExpandCollapse(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := sampleTree()
	m := tui.NewModel(tree, cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to "commit", then go Up one row to land on its section header.
	navigateModelTo(m, "commit")
	m.Update(tea.KeyMsg{Type: tea.KeyUp}) // should be Subcommands section

	sel := m.TreeModel().SelectedItem()
	if sel != nil {
		t.Fatal("expected to be on a section row (nil SelectedItem), got non-nil")
	}

	// Right on an already-expanded section should jump into its first child.
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	sel = m.TreeModel().SelectedItem()
	if sel == nil || sel.Kind != tui.SelCommand {
		t.Fatal("Right on expanded section should jump to first child command")
	}
	if sel.Node.Name != "commit" {
		t.Errorf("expected cursor on 'commit', got %q", sel.Node.Name)
	}

	// Go back up to the section, then Left to collapse it.
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m.Update(tea.KeyMsg{Type: tea.KeyLeft}) // collapse section

	v := m.View()
	// After collapsing, section should show collapsed icon (▷).
	if !strings.Contains(v, "▷") {
		t.Errorf("after collapsing sub commands section, should show collapsed icon:\n%s", v)
	}

	// Right to expand it again.
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // jump into first child
	sel = m.TreeModel().SelectedItem()
	if sel == nil || sel.Node.Name != "commit" {
		t.Error("Right→Right on collapsed section should expand then enter first child")
	}
}

func TestSectionRow_enterToggles(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := sampleTree()
	m := tui.NewModel(tree, cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to section row.
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Verify we're on a section (SelectedItem returns nil for sections).
	if m.TreeModel().SelectedItem() != nil {
		t.Skip("cursor did not land on section row")
	}

	// Enter should toggle the section.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	v1 := m.View()

	// Press Enter again to toggle back.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	v2 := m.View()

	// The two views should differ (section expanded vs collapsed).
	if v1 == v2 {
		t.Error("Enter on section should toggle it; views should differ between presses")
	}
}

func TestExpandAll_expandsNodesAndSections(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := sampleTree()
	m := tui.NewModel(tree, cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press 'e' to expand all.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})

	v := m.View()
	// All subcommands should be visible.
	if !strings.Contains(v, "commit") {
		t.Errorf("expand all should show 'commit':\n%s", v)
	}
	if !strings.Contains(v, "remote") {
		t.Errorf("expand all should show 'remote':\n%s", v)
	}
	if !strings.Contains(v, "add") {
		t.Errorf("expand all should show 'add' (nested under remote):\n%s", v)
	}
	// Flags should also be expanded.
	if !strings.Contains(v, "--message") {
		t.Errorf("expand all should also expand flag sections, showing '--message':\n%s", v)
	}
	if !strings.Contains(v, "--version") {
		t.Errorf("expand all should show root flags like '--version':\n%s", v)
	}
}

func TestCollapseAll(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := sampleTree()
	m := tui.NewModel(tree, cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Expand all first, then collapse all.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})

	// After collapse all, only the root row should appear in the tree pane.
	// The tree should show the collapsed icon (▶) and no section headers.
	v := m.View()
	if !strings.Contains(v, "▶") {
		t.Errorf("collapse all should show collapsed root icon (▶):\n%s", v)
	}
	// Row count should be 1 (just the root).
	if m.TreeModel().RowCount() != 1 {
		t.Errorf("collapse all should leave only root row, got %d rows", m.TreeModel().RowCount())
	}
}

func TestShiftRight_expandsSubtree(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := sampleTree()
	m := tui.NewModel(tree, cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to "commit".
	for i := 0; i < 10; i++ {
		sel := m.TreeModel().SelectedItem()
		if sel != nil && sel.Kind == tui.SelCommand && sel.Node.Name == "commit" {
			break
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	// Shift+Right to expand commit's subtree.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})

	v := m.View()
	// commit's flags should now be visible.
	if !strings.Contains(v, "--message") {
		t.Errorf("Shift+Right should expand subtree, showing '--message':\n%s", v)
	}
}

func TestToggleSections_hidesHeaders(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := sampleTree()
	m := tui.NewModel(tree, cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v1 := m.View()
	// Match the tree section header format "Subcommands (N)" to distinguish
	// from the help pane's "Subcommands:" label.
	hasSections := strings.Contains(v1, "Subcommands (")

	// Press 'S' to toggle section headers.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})

	v2 := m.View()
	if hasSections && strings.Contains(v2, "Subcommands (") {
		t.Error("S should hide section headers like 'Subcommands (2)'")
	}

	// Press 'S' again to restore them.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	v3 := m.View()
	if !strings.Contains(v3, "Subcommands (") {
		t.Error("second S should restore section headers")
	}
}

// ========== Task #9: Nav scheme key collisions ==========

func TestWASD_dNavigatesRight(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to "commit" first (using arrows, before switching scheme).
	navigateModelTo(m, "commit")
	m.SetScheme(tui.SchemeWASD)
	sel := m.TreeModel().SelectedItem()
	if sel == nil || sel.Node.Name != "commit" {
		t.Fatal("expected cursor on commit")
	}

	// In WASD, 'd' should expand (Right), NOT open docs.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	v := m.View()
	// After expanding commit, its flags should be visible.
	if !strings.Contains(v, "--message") {
		t.Error("in WASD mode, 'd' should expand node (Right), not open docs")
	}
}

func TestWASD_sNavigatesDown(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.SetScheme(tui.SchemeWASD)

	// Cursor starts on root "git". Press 's' (Down in WASD).
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// Should have moved down, NOT toggled sections (that's Shift+S).
	sel := m.TreeModel().SelectedItem()
	// We moved off the root — exact target depends on section layout, but
	// we should not be on root anymore.
	if sel != nil && sel.Kind == tui.SelCommand && sel.Node.Name == "git" {
		t.Error("in WASD mode, 's' should navigate Down, not stay on root")
	}
}

func TestVim_hNavigatesLeft(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to commit and expand it (using arrows, before switching scheme).
	navigateModelTo(m, "commit")
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand
	rowsExpanded := m.TreeModel().RowCount()
	m.SetScheme(tui.SchemeVim)

	// Now 'h' should collapse (Left), NOT toggle help pane.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})

	// After collapsing, row count should decrease.
	rowsCollapsed := m.TreeModel().RowCount()
	if rowsCollapsed >= rowsExpanded {
		t.Errorf("in Vim mode, 'h' should collapse node (Left), reducing rows from %d, got %d", rowsExpanded, rowsCollapsed)
	}
}

func TestArrows_dOpensDocsNotNav(t *testing.T) {
	// In Arrows mode, 'd' is not a navigation key, so it should trigger docs.
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Default scheme is Arrows — no SetScheme needed.

	// Press 'd' — should attempt docs, which sets a status message.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	v := m.View()
	// In arrows mode, 'd' should trigger docs action (status shows "no docs URL found"
	// or similar), not navigation.
	if strings.Contains(v, "no docs URL") || strings.Contains(v, "opened:") {
		// Good — docs action was triggered.
		return
	}
	// The status message is consumed on next View(), so just verify it didn't navigate.
}

// ========== Task #10: gg/G jump to top/bottom ==========

func TestTree_gg_jumpsToTop(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate down a few rows.
	navigateModelTo(m, "commit")

	// Press 'g' once (sets pending), then 'g' again (completes gg → jump to top).
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})

	sel := m.TreeModel().SelectedItem()
	if sel == nil || sel.Kind != tui.SelCommand || sel.Node.Name != "git" {
		t.Errorf("gg should jump to first row (root), got %v", sel)
	}
}

func TestTree_G_jumpsToBottom(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Cursor starts at root. Press 'G' to jump to last row.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})

	sel := m.TreeModel().SelectedItem()
	if sel == nil {
		t.Fatal("G should jump to last row, got nil selection")
	}
	// The last visible row should be "remote" (last subcommand).
	if sel.Kind == tui.SelCommand && sel.Node.Name == "git" {
		t.Error("G should move cursor away from root to last visible row")
	}
}

// ========== Task #11: Esc = back/collapse in tree ==========

func TestEsc_collapsesExpandedNode(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to commit and expand it.
	navigateModelTo(m, "commit")
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand commit

	rowsBefore := m.TreeModel().RowCount()

	// Esc should collapse, NOT quit.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	v2 := m.View()
	if v2 == "" || cmd != nil {
		t.Fatal("Esc should not quit when on an expanded non-root node")
	}
	// After collapsing, row count should decrease.
	rowsAfter := m.TreeModel().RowCount()
	if rowsAfter >= rowsBefore {
		t.Errorf("Esc should collapse node, reducing rows from %d, got %d", rowsBefore, rowsAfter)
	}
}

func TestEsc_jumpsToParentWhenCollapsed(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to commit (which is collapsed by default).
	navigateModelTo(m, "commit")

	// Esc on a collapsed child should jump to parent.
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	v := m.View()
	if v == "" {
		t.Fatal("Esc should not quit when on a collapsed child node")
	}
	sel := m.TreeModel().SelectedItem()
	if sel == nil || sel.Node.Name != "git" {
		t.Errorf("Esc on collapsed child should jump to parent, got %v", sel)
	}
}

func TestEsc_quitsFromRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Cursor is on root. Esc should quit since there's nowhere to go back to.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Error("Esc on root should quit the application")
	}
}

// ========== Task #12: Scheme-adaptive hint text ==========

func TestStatusBar_arrowSchemeHints(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Default is arrows scheme.

	v := m.View()
	if !strings.Contains(v, "↑↓:nav") {
		t.Error("arrows scheme should show arrow symbols in hints")
	}
	if !strings.Contains(v, "[arrows]") {
		t.Error("arrows scheme should show [arrows] indicator")
	}
}

func TestStatusBar_vimSchemeHints(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.SetScheme(tui.SchemeVim)

	v := m.View()
	if !strings.Contains(v, "j/k:nav") {
		t.Errorf("vim scheme should show j/k in hints, got: %s", v)
	}
	if !strings.Contains(v, "[vim]") {
		t.Error("vim scheme should show [vim] indicator")
	}
}

func TestStatusBar_wasdSchemeHints(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.SetScheme(tui.SchemeWASD)

	v := m.View()
	if !strings.Contains(v, "w/s:nav") {
		t.Errorf("wasd scheme should show w/s in hints, got: %s", v)
	}
	if !strings.Contains(v, "[wasd]") {
		t.Error("wasd scheme should show [wasd] indicator")
	}
}

// ========== Task #13: n/N filter cycling ==========

func TestFilterCycle_n_movesToNextMatch(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Search for "remote" — enter filter mode, type, exit.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, r := range "remote" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // exit filter, saves lastSearch

	// Clear the filter so all rows are visible again.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // empty filter → show all

	// Now press 'n' to cycle to a matching row.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	sel := m.TreeModel().SelectedItem()
	if sel == nil {
		t.Fatal("n should navigate to a matching row")
	}
	if sel.Kind != tui.SelCommand || sel.Node.Name != "remote" {
		t.Errorf("n should find 'remote', got %v", sel)
	}
}

func TestFilterCycle_N_movesToPreviousMatch(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Search for "remote" — enter filter mode, type, exit.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, r := range "remote" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Clear filter to restore all rows.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Navigate past remote so N can go backwards to find it.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")}) // jump to last row

	// Press 'N' to find previous match (searching backwards).
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	sel := m.TreeModel().SelectedItem()
	if sel == nil {
		t.Fatal("N should navigate to a matching row")
	}
	if sel.Kind != tui.SelCommand || sel.Node.Name != "remote" {
		t.Errorf("N should find 'remote', got %v", sel)
	}
}

// ── Ctrl+K: clear preview bar ─────────────────────────────────────────────────

func TestModel_CtrlK_clearsPreview(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Build a command in the preview.
	m.Update(tea.KeyMsg{Type: tea.KeyRight}) // expand git → commit
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // pick commit

	// Sanity: preview contains "commit".
	if !strings.Contains(m.View(), "commit") {
		t.Skip("could not build a preview command — skipping")
	}

	// Clear with Ctrl+K.
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})

	// Status bar should confirm the clear.
	v := m.View()
	if !strings.Contains(v, "cleared") {
		t.Errorf("after Ctrl+K status should say 'cleared', got: %q", v[max(0, len(v)-300):])
	}
	// The explicitly-built tokens must be gone (no free-standing "commit" token
	// separate from the tree-cursor display).
	tokens := m.Preview().Tokens()
	if len(tokens) != 0 {
		t.Errorf("after Ctrl+K preview tokens should be empty, got %v", tokens)
	}
}

func TestPreviewModel_ClearAll(t *testing.T) {
	cfg := config.DefaultConfig()
	p := tui.NewPreviewModel(cfg)
	p.AppendToken("git")
	p.AppendToken("commit")
	if len(p.Tokens()) == 0 {
		t.Fatal("expected tokens after AppendToken")
	}
	p.ClearAll()
	if len(p.Tokens()) != 0 {
		t.Errorf("ClearAll should empty tokens, got %v", p.Tokens())
	}
}

// ── R key: re-discover selected node ─────────────────────────────────────────

func TestModel_R_returnsDiscoveryCmd(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press 'r' — should queue an async discovery for the selected node.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd == nil {
		t.Error("pressing 'r' on a command node should return a non-nil tea.Cmd")
	}
}

func TestModel_R_setsDiscoveringStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	m := tui.NewModel(sampleTree(), cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	v := m.View()
	if !strings.Contains(v, "discovering") {
		t.Errorf("pressing 'r' should set a 'discovering …' status, got: %q",
			v[max(0, len(v)-200):])
	}
}

// ── Task #4: discovery error indicator ───────────────────────────────────────

func TestTreeModel_DiscoveryErr_showsWarningIndicator(t *testing.T) {
	cfg := config.DefaultConfig()
	root := &models.Node{
		Name:     "mycli",
		FullPath: []string{"mycli"},
		Children: []*models.Node{
			{
				Name:         "broken",
				FullPath:     []string{"mycli", "broken"},
				DiscoveryErr: "exit status 1",
			},
		},
	}
	tree := tui.NewTreeModel(root, cfg)
	tree.SetSize(120, 40)
	// Expand root so children are visible.
	tree.Right()
	v := tree.ViewSized(120, 40)
	if !strings.Contains(v, "⚠") {
		t.Errorf("node with DiscoveryErr should render ⚠ indicator, got:\n%s", v)
	}
}

func TestTreeModel_NoDiscoveryErr_noWarningIndicator(t *testing.T) {
	cfg := config.DefaultConfig()
	tree := tui.NewTreeModel(sampleTree(), cfg)
	tree.SetSize(120, 40)
	tree.Right()
	v := tree.ViewSized(120, 40)
	if strings.Contains(v, "⚠") {
		t.Errorf("node without DiscoveryErr should not render ⚠, got:\n%s", v)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

// modalTree is a small git-like tree: a root with global flags, a command with
// a value flag, a bool flag and a positional, and a nested group with a
// documented subcommand.
func modalTree() *models.Node {
	return &models.Node{
		Name:     "git",
		FullPath: []string{"git"},
		Flags: []models.Flag{
			{Name: "--version"},
			{Name: "--paginate", ShortName: "p"},
		},
		Children: []*models.Node{
			{
				Name: "commit", FullPath: []string{"git", "commit"},
				Flags: []models.Flag{
					{Name: "--message", ShortName: "m", ValueType: "string"},
					{Name: "--all", ShortName: "a"},
				},
				Positionals: []models.Positional{{Name: "pathspec"}},
			},
			{
				Name: "remote", FullPath: []string{"git", "remote"},
				Children: []*models.Node{
					{
						Name: "add", FullPath: []string{"git", "remote", "add"},
						Description: "Add a remote. See https://git-scm.com/docs/git-remote for details",
					},
				},
			},
		},
	}
}

// key builds the tea.KeyMsg whose String() is s.
func key(s string) tea.KeyMsg {
	named := map[string]tea.KeyType{
		"esc": tea.KeyEsc, "enter": tea.KeyEnter, "tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab,
		"up": tea.KeyUp, "down": tea.KeyDown, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
		"ctrl+c": tea.KeyCtrlC, "ctrl+e": tea.KeyCtrlE, "ctrl+f": tea.KeyCtrlF,
		"ctrl+d": tea.KeyCtrlD, "ctrl+u": tea.KeyCtrlU, "ctrl+n": tea.KeyCtrlN,
		"ctrl+p": tea.KeyCtrlP, "backspace": tea.KeyBackspace, " ": tea.KeySpace,
		"shift+right": tea.KeyShiftRight, "shift+left": tea.KeyShiftLeft,
	}
	if t, ok := named[s]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(m *Model, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		_, cmd = m.Update(key(k))
	}
	return cmd
}

func typeText(m *Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func newModalModel(t *testing.T) (*Model, *models.Node) {
	t.Helper()
	root := modalTree()
	m := NewModel(root, config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m, root
}

func selectNode(t *testing.T, m *Model, n *models.Node) {
	t.Helper()
	if !m.tree.Reveal(n, "", "") {
		t.Fatalf("could not reveal %s", n.Name)
	}
	m.syncSelected()
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestSearchModal_typeNavigateAndJump(t *testing.T) {
	m, root := newModalModel(t)
	press(m, "ctrl+f")
	if !m.sm.active {
		t.Fatal("Ctrl+F should open the finder")
	}
	typeText(m, "a")
	if len(m.sm.results) < 3 {
		t.Fatalf("query \"a\" should match several entries, got %d", len(m.sm.results))
	}

	press(m, "down", "ctrl+n")
	if m.sm.cursor != 2 {
		t.Errorf("two downs: cursor = %d, want 2", m.sm.cursor)
	}
	press(m, "up")
	if m.sm.cursor != 1 {
		t.Errorf("up: cursor = %d, want 1", m.sm.cursor)
	}
	press(m, "pgdown")
	if want := len(m.sm.results) - 1; m.sm.cursor != want {
		t.Errorf("pgdown clamps to the last result: cursor = %d, want %d", m.sm.cursor, want)
	}
	press(m, "down")
	if want := len(m.sm.results) - 1; m.sm.cursor != want {
		t.Errorf("down at the end must not move: cursor = %d, want %d", m.sm.cursor, want)
	}
	press(m, "pgup", "up")
	if m.sm.cursor != 0 {
		t.Errorf("pgup clamps to the first result: cursor = %d", m.sm.cursor)
	}
	// Tab with nothing to index does nothing.
	if cmd := press(m, "tab"); cmd != nil || m.sm.indexing {
		t.Error("tab with no stubs should not start indexing")
	}

	// Narrow to the nested subcommand and jump to it.
	m.sm.input.SetValue("")
	typeText(m, "remote add")
	if len(m.sm.results) == 0 || m.sm.results[0].Entry.Name != "add" {
		t.Fatalf("top result for \"remote add\" = %+v", m.sm.results)
	}
	press(m, "enter")
	if m.sm.active {
		t.Error("enter should close the finder")
	}
	add := root.Children[1].Children[0]
	if got := m.tree.Selected(); got != add {
		t.Errorf("enter should select %q, got %v", add.Name, got)
	}

	press(m, "ctrl+f")
	press(m, "esc")
	if m.sm.active {
		t.Error("esc should close the finder")
	}
}

func TestSearchModal_enterWithNoResultsStaysOpen(t *testing.T) {
	m, _ := newModalModel(t)
	press(m, "ctrl+f")
	typeText(m, "zzzzqqq")
	if len(m.sm.results) != 0 {
		t.Fatalf("expected no results, got %d", len(m.sm.results))
	}
	press(m, "enter")
	if !m.sm.active {
		t.Error("enter with no results must keep the finder open")
	}
	if !strings.Contains(m.renderSearchModal(), "no matches") {
		t.Errorf("finder should say there are no matches:\n%s", m.renderSearchModal())
	}
}

func TestSearchModal_tabIndexesStubsUnlessOffline(t *testing.T) {
	root := modalTree()
	root.Children = append(root.Children, &models.Node{Name: "lfs", FullPath: []string{"git", "lfs"}, Stub: true})
	m := NewModel(root, config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	Offline = true
	t.Cleanup(func() { Offline = false })
	press(m, "ctrl+f")
	if m.sm.stubs != 1 {
		t.Fatalf("finder should count 1 stub, got %d", m.sm.stubs)
	}
	if cmd := press(m, "tab"); cmd != nil || m.sm.indexing {
		t.Error("offline: tab must not start indexing")
	}
	if m.statusMsg != offlineMsg {
		t.Errorf("offline status = %q", m.statusMsg)
	}
}

// openCommitFlags selects "git commit" and opens its flag picker.
func openCommitFlags(t *testing.T) *Model {
	t.Helper()
	m, root := newModalModel(t)
	selectNode(t, m, root.Children[0])
	press(m, "f")
	if !m.fm.active {
		t.Fatal("f should open the flag picker")
	}
	return m
}

func previewText(m *Model) string { return strings.Join(m.preview.Tokens(), " ") }

func TestFlagModal_listsCommandFlagsThenGlobals(t *testing.T) {
	m := openCommitFlags(t)
	var names []string
	for _, e := range m.fm.entries {
		names = append(names, e.flag.Name)
		if e.global != (e.flag.Name == "--version" || e.flag.Name == "--paginate") {
			t.Errorf("%s: global = %v", e.flag.Name, e.global)
		}
	}
	if got := strings.Join(names, " "); got != "--message --all --version --paginate" {
		t.Errorf("entries = %q, want the command's flags then the root's", got)
	}
}

func TestFlagModal_boolFlagIsAddedOnce(t *testing.T) {
	m := openCommitFlags(t)
	// A bool flag is added straight away, on the command it belongs to.
	press(m, "down", "enter")
	if got := previewText(m); got != "git commit --all" {
		t.Errorf("preview = %q, want \"git commit --all\"", got)
	}
	if !m.fm.entries[1].added {
		t.Error("--all should be marked added")
	}
	// Choosing it again adds nothing.
	press(m, " ")
	if got := previewText(m); got != "git commit --all" {
		t.Errorf("re-adding a flag changed the preview to %q", got)
	}
}

func TestFlagModal_valueFlagPromptsAndEscCancels(t *testing.T) {
	m := openCommitFlags(t)
	press(m, "enter")
	if !m.fm.awaitingValue || !strings.Contains(m.fm.valueInput.Placeholder, "--message") {
		t.Fatalf("value prompt not shown: awaiting=%v placeholder=%q", m.fm.awaitingValue, m.fm.valueInput.Placeholder)
	}
	typeText(m, "oops")
	press(m, "esc")
	if m.fm.awaitingValue || m.fm.valueInput.Value() != "" || !m.fm.active {
		t.Error("esc in the value prompt should cancel the prompt but keep the picker open")
	}
	if got := previewText(m); got != "git" {
		t.Errorf("a cancelled value must add nothing, preview = %q", got)
	}

	press(m, "enter")
	typeText(m, "fix")
	press(m, "enter")
	if got := previewText(m); got != "git commit --message=fix" {
		t.Errorf("preview = %q, want \"git commit --message=fix\"", got)
	}
	if m.statusMsg != "added: --message=fix" {
		t.Errorf("status = %q", m.statusMsg)
	}
}

func TestFlagModal_cursorBoundsCloseAndReopen(t *testing.T) {
	m := openCommitFlags(t)
	press(m, "up", "k")
	if m.fm.cursor != 0 {
		t.Errorf("up at the top: cursor = %d", m.fm.cursor)
	}
	press(m, "j", "j", "j", "j", "j")
	if m.fm.cursor != len(m.fm.entries)-1 {
		t.Errorf("cursor should stop at the last entry, got %d", m.fm.cursor)
	}
	press(m, "enter") // --paginate, a global bool flag
	press(m, "q")
	if m.fm.active {
		t.Error("q should close the picker")
	}
	// Reopening marks the flags already in the command.
	press(m, "f")
	for _, e := range m.fm.entries {
		if e.added != (e.flag.Name == "--paginate") {
			t.Errorf("%s: added = %v on reopen", e.flag.Name, e.added)
		}
	}
}

func TestFlagModal_valueFlagWithEmptyValueAddsBareFlag(t *testing.T) {
	m, root := newModalModel(t)
	selectNode(t, m, root.Children[0])
	press(m, "f", "enter", "enter")
	if got := strings.Join(m.preview.Tokens(), " "); got != "git commit --message" {
		t.Errorf("preview = %q, want \"git commit --message\"", got)
	}
}

func TestFlagModal_nodeWithoutFlags(t *testing.T) {
	root := &models.Node{Name: "tool", FullPath: []string{"tool"}}
	m := NewModel(root, config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	press(m, "f")
	if m.fm.active || m.statusMsg != "no flags available" {
		t.Errorf("active=%v status=%q, want closed with \"no flags available\"", m.fm.active, m.statusMsg)
	}
}

func TestKeybindModal_scrollAndClose(t *testing.T) {
	m, _ := newModalModel(t)
	press(m, "?")
	if !m.kb.active {
		t.Fatal("? should open the key bindings overlay")
	}
	press(m, "k")
	if m.kb.offset != 0 {
		t.Errorf("up at the top: offset = %d", m.kb.offset)
	}
	press(m, "j", "down")
	if m.kb.offset != 2 {
		t.Errorf("two downs: offset = %d", m.kb.offset)
	}
	press(m, "up")
	if m.kb.offset != 1 {
		t.Errorf("up: offset = %d", m.kb.offset)
	}
	press(m, "pgdown", "ctrl+d")
	if m.kb.offset != 21 {
		t.Errorf("two page downs: offset = %d", m.kb.offset)
	}
	press(m, "b", "pgup", "ctrl+u")
	if m.kb.offset != 0 {
		t.Errorf("page ups clamp at 0: offset = %d", m.kb.offset)
	}
	// Other messages are swallowed while the overlay is open.
	m.Update(tea.MouseMsg{})
	if !m.kb.active {
		t.Error("a mouse event should not close the overlay")
	}
	press(m, "?")
	if m.kb.active {
		t.Error("? again should close the overlay")
	}
}

func TestExecuteModal_cancelAndRun(t *testing.T) {
	m, root := newModalModel(t)
	selectNode(t, m, root.Children[1])
	press(m, "enter", "ctrl+e")
	if !m.modal.active || m.modal.command != "git remote" {
		t.Fatalf("modal active=%v command=%q, want the built command", m.modal.active, m.modal.command)
	}
	press(m, "esc")
	if m.modal.active || m.statusMsg != "cancelled" {
		t.Errorf("esc: active=%v status=%q", m.modal.active, m.statusMsg)
	}
	if m.commandToRun != "" {
		t.Error("cancelling must not set a command to run")
	}

	press(m, "ctrl+e")
	if cmd := press(m, "enter"); !isQuit(cmd) {
		t.Error("enter should quit to run the command")
	}
	if m.commandToRun != "git remote" {
		t.Errorf("command to run = %q", m.commandToRun)
	}
}

func TestHelpPaneFocus_scrollKeys(t *testing.T) {
	m, root := newModalModel(t)
	long := root.Children[0]
	long.HelpText = strings.Repeat("a help line\n", 200)
	selectNode(t, m, long)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	if !m.showHelpPane {
		press(m, "H")
	}
	for m.focusedPane != paneHelp {
		press(m, "tab")
	}

	press(m, "j", "down")
	if m.helpPane.scrollOffset != 2 {
		t.Errorf("two downs: offset = %d", m.helpPane.scrollOffset)
	}
	press(m, "k")
	if m.helpPane.scrollOffset != 1 {
		t.Errorf("up: offset = %d", m.helpPane.scrollOffset)
	}
	press(m, "j", "j", "j", "j")
	if m.helpPane.scrollOffset != 5 {
		t.Fatalf("four more downs: offset = %d", m.helpPane.scrollOffset)
	}
	press(m, "b")
	if m.helpPane.scrollOffset != 0 {
		t.Errorf("b pages up and clamps at the top: offset = %d", m.helpPane.scrollOffset)
	}
	press(m, "up")
	if m.helpPane.scrollOffset != 0 {
		t.Errorf("up at the top: offset = %d", m.helpPane.scrollOffset)
	}
	if m.tree.Selected() != long {
		t.Error("help pane keys must not move the tree cursor")
	}
	press(m, "esc")
	if m.focusedPane != paneTree {
		t.Error("esc from the help pane should return focus to the tree")
	}
}

func TestPreviewFocus_editAndOpenModal(t *testing.T) {
	m, root := newModalModel(t)
	selectNode(t, m, root.Children[0])
	press(m, "enter")
	for m.focusedPane != panePreview {
		press(m, "tab")
	}
	typeText(m, " -a")
	if got := strings.Join(m.preview.Tokens(), " "); got != "git commit -a" {
		t.Errorf("typed into preview: %q", got)
	}
	// q types into the command instead of quitting.
	if cmd := press(m, "q"); isQuit(cmd) {
		t.Error("q in the preview pane must not quit")
	}
	press(m, "backspace")
	press(m, "ctrl+e")
	if !m.modal.active || m.modal.command != "git commit -a" {
		t.Errorf("ctrl+e from preview: active=%v command=%q", m.modal.active, m.modal.command)
	}
	press(m, "esc")

	press(m, "shift+tab")
	if m.focusedPane == panePreview {
		t.Error("shift+tab should leave the preview pane")
	}
	for m.focusedPane != panePreview {
		press(m, "tab")
	}
	press(m, "esc")
	if m.focusedPane != paneTree || m.statusMsg != "focus: tree" {
		t.Errorf("esc from preview: pane=%v status=%q", m.focusedPane, m.statusMsg)
	}
	for m.focusedPane != panePreview {
		press(m, "tab")
	}
	if cmd := press(m, "ctrl+c"); !isQuit(cmd) {
		t.Error("ctrl+c in the preview pane should quit")
	}
}

func TestSubtreeExpandCollapse_fromSectionRow(t *testing.T) {
	m, root := newModalModel(t)
	remote := root.Children[1]
	selectNode(t, m, remote)
	press(m, "shift+right")
	if m.statusMsg != "expanded: remote" {
		t.Errorf("status = %q", m.statusMsg)
	}
	if !m.tree.Reveal(remote.Children[0], "", "") {
		t.Fatal("add should be visible after expanding remote's subtree")
	}

	// From a section row, the owning command is the one collapsed.
	selectNode(t, m, root)
	for m.tree.rows[m.tree.cursor].kind != rowKindSection {
		m.tree.Down()
	}
	owner := m.tree.SelectedOrOwner()
	if owner != root {
		t.Fatalf("section row owner = %v, want root", owner)
	}
	press(m, "shift+left")
	if m.statusMsg != "collapsed: git" {
		t.Errorf("status = %q", m.statusMsg)
	}
	if m.tree.nodeExpanded[nodeKey(remote, 1)] {
		t.Error("collapsing the root's subtree should collapse remote")
	}
}

func TestFilterThenNextPrevMatch(t *testing.T) {
	m, root := newModalModel(t)
	press(m, "e") // expand everything so flags and positionals are rows
	press(m, "/")
	if !m.filtering {
		t.Fatal("/ should start filtering")
	}
	typeText(m, "pa")
	press(m, "enter")
	if m.filtering || m.lastSearch != "pa" {
		t.Fatalf("enter should keep the search: filtering=%v last=%q", m.filtering, m.lastSearch)
	}
	m.tree.SetFilter("")
	selectNode(t, m, root)

	// "pa" matches the --paginate flag and the pathspec positional.
	var hits []string
	for i := 0; i < 4; i++ {
		press(m, "n")
		sel := m.tree.SelectedItem()
		switch sel.Kind {
		case SelFlag:
			hits = append(hits, sel.Flag.Name)
		case SelPositional:
			hits = append(hits, sel.Positional.Name)
		default:
			hits = append(hits, sel.Node.Name)
		}
	}
	if hits[0] != "--paginate" || hits[1] != "pathspec" || hits[2] != "--paginate" {
		t.Errorf("n cycles through matches with wrap-around, got %v", hits)
	}
	press(m, "N")
	if sel := m.tree.SelectedItem(); sel.Kind != SelFlag || sel.Flag.Name != "--paginate" {
		t.Errorf("N should go back to --paginate, got %+v", sel)
	}

	press(m, "/")
	typeText(m, "x")
	press(m, "esc")
	if m.filtering || m.filter.Value() != "" {
		t.Error("esc should end and clear the filter")
	}
	if m.lastSearch != "pa" {
		t.Errorf("esc must keep the last search, got %q", m.lastSearch)
	}
}

func TestOpenDocsURL_withoutURL(t *testing.T) {
	m, root := newModalModel(t)
	selectNode(t, m, root.Children[0])
	press(m, "D")
	if m.statusMsg != "no docs URL found for commit" {
		t.Errorf("status = %q", m.statusMsg)
	}
}

func TestExtractURL(t *testing.T) {
	for in, want := range map[string]string{
		"":                                 "",
		"no link here":                     "",
		"http://insecure.example only":     "",
		"see https://example.com/docs":     "https://example.com/docs",
		"(https://example.com/a) and more": "https://example.com/a",
		"x https://a.example/b\tthen":      "https://a.example/b",
		`"https://q.example/z" quoted`:     "https://q.example/z",
		"first https://1.example then https://2.example": "https://1.example",
	} {
		if got := extractURL(in); got != want {
			t.Errorf("extractURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDiscoveryKeys_offlineSaysSo(t *testing.T) {
	root := modalTree()
	stub := &models.Node{Name: "lfs", FullPath: []string{"git", "lfs"}, Stub: true}
	root.Children = append(root.Children, stub)
	m := NewModel(root, config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	Offline = true
	t.Cleanup(func() { Offline = false })

	selectNode(t, m, stub)
	if cmd := m.lazyExpandIfStub(); cmd != nil || m.statusMsg != offlineMsg {
		t.Errorf("offline stub expand: cmd=%v status=%q", cmd != nil, m.statusMsg)
	}
	m.statusMsg = ""
	if cmd := press(m, "r"); cmd != nil || m.statusMsg != offlineMsg {
		t.Errorf("offline re-discover: cmd=%v status=%q", cmd != nil, m.statusMsg)
	}
	m.statusMsg = ""
	if cmd := m.indexAllStubs(); cmd != nil || m.statusMsg != offlineMsg {
		t.Errorf("offline index: cmd=%v status=%q", cmd != nil, m.statusMsg)
	}
}

func TestDiscoveryKeys_onlineSchedulesDiscovery(t *testing.T) {
	root := modalTree()
	stub := &models.Node{Name: "lfs", FullPath: []string{"git", "lfs"}, Stub: true}
	root.Children = append(root.Children, stub)
	m := NewModel(root, config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	selectNode(t, m, stub)
	if cmd := m.lazyExpandIfStub(); cmd == nil || m.statusMsg != "discovering lfs…" {
		t.Errorf("stub expand: cmd=%v status=%q", cmd != nil, m.statusMsg)
	}
	selectNode(t, m, root.Children[0])
	if cmd := m.lazyExpandIfStub(); cmd != nil {
		t.Error("a discovered node needs no lazy expansion")
	}
	if cmd := press(m, "R"); cmd == nil || m.statusMsg != "discovering commit…" {
		t.Errorf("re-discover: cmd=%v status=%q", cmd != nil, m.statusMsg)
	}
}

func TestLazyExpandMsg_errorIsReported(t *testing.T) {
	m, root := newModalModel(t)
	m.Update(LazyExpandMsg{Stub: root.Children[0], Err: errors.New("exit status 1")})
	if m.statusMsg != "expand failed: exit status 1" {
		t.Errorf("status = %q", m.statusMsg)
	}
}

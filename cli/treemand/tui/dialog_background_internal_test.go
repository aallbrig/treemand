package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/cli/treemand/config"
	"github.com/aallbrig/treemand/cli/treemand/models"
)

// dialogs opens and closes each dialog that takes over the keyboard.
// The real openers need history and preset stores, so some set the flag.
var dialogs = []struct {
	name  string
	open  func(m *Model)
	close func(m *Model)
}{
	{"flag", func(m *Model) { m.openFlagModal() }, func(m *Model) { m.fm.active = false }},
	{"value", func(m *Model) { m.vm.active = true }, func(m *Model) { m.vm.active = false }},
	{"keybind", func(m *Model) { m.kb.active = true }, func(m *Model) { m.kb.active = false }},
	{"recents", func(m *Model) { m.rm.active = true }, func(m *Model) { m.rm.active = false }},
	{"presets", func(m *Model) { m.pm.active = true }, func(m *Model) { m.pm.active = false }},
	{"modal", func(m *Model) { m.modal.active = true }, func(m *Model) { m.modal.active = false }},
	{"finder", func(m *Model) { m.openSearchModal() }, func(m *Model) { m.sm.active = false }},
}

func dialogTree() (*models.Node, *models.Node) {
	stub := &models.Node{Name: "s3", FullPath: []string{"aws", "s3"}, Stub: true}
	root := &models.Node{Name: "aws", FullPath: []string{"aws"},
		Flags: []models.Flag{{Name: "--debug"}}, Children: []*models.Node{stub}}
	return root, stub
}

func TestDialogOpen_lazyExpandResultIsApplied(t *testing.T) {
	for _, d := range dialogs {
		t.Run(d.name, func(t *testing.T) {
			root, stub := dialogTree()
			m := NewModel(root, config.DefaultConfig())
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			openDialog(t, m, d.open)

			disc := &models.Node{Description: "Object storage", HelpText: "help",
				Flags: []models.Flag{{Name: "--region"}}}
			m.Update(LazyExpandMsg{Stub: stub, Discovered: disc})
			d.close(m)

			if stub.Stub || stub.Description != "Object storage" || len(stub.Flags) != 1 {
				t.Errorf("stub not patched while the %s dialog was open: %+v", d.name, stub)
			}
			if m.statusMsg != "expanded: s3" {
				t.Errorf("status = %q, want the expansion reported", m.statusMsg)
			}
		})
	}
}

func TestDialogOpen_resizeAndTimedMessagesStillApply(t *testing.T) {
	for _, d := range dialogs {
		t.Run(d.name, func(t *testing.T) {
			root, _ := dialogTree()
			m := NewModel(root, config.DefaultConfig())
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			m.setTimedMsg("saved")
			openDialog(t, m, d.open)

			m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
			m.Update(clearTimedMsgMsg{expiry: m.timedMsgExp.Add(time.Second)})

			if m.width != 90 || m.height != 30 {
				t.Errorf("size = %dx%d, want 90x30", m.width, m.height)
			}
			if m.timedMsg != "" {
				t.Errorf("timed message %q outlived its tick", m.timedMsg)
			}
		})
	}
}

// clickStub clicks the s3 row of dialogTree's tree at 120x40.
var clickStub = tea.MouseMsg{X: 4, Y: 7, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}

func TestDialogOpen_mouseStaysWithTheDialog(t *testing.T) {
	root, stub := dialogTree()
	control := NewModel(root, config.DefaultConfig())
	control.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	control.Update(clickStub)
	if control.tree.Selected() != stub {
		t.Fatal("with no dialog the click should select s3; the layout moved, fix clickStub")
	}

	for _, d := range dialogs {
		t.Run(d.name, func(t *testing.T) {
			root, stub := dialogTree()
			m := NewModel(root, config.DefaultConfig())
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			openDialog(t, m, d.open)
			m.Update(clickStub)
			if m.tree.Selected() == stub {
				t.Error("a click behind the dialog reached the tree")
			}
		})
	}
}

func TestDialogOpen_crawlAndIndexResultsAreApplied(t *testing.T) {
	root := streamedTree()
	m := NewModel(root, config.DefaultConfig())
	m.EnableCrawl(CrawlOptions{MaxDepth: 1, StubThreshold: 50, Workers: 1})
	defer m.crawl.stop()
	m.crawl.stop()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.openFlagModal()

	alpha := root.Children[0]
	m.Update(crawlBatchMsg{{job: crawlJob{stub: alpha}, discovered: &models.Node{Name: "alpha"}}})
	if alpha.Stub {
		t.Error("crawl result dropped while the flag dialog was open")
	}
	if got := m.crawlProgress(); !strings.HasPrefix(got, "discovering… 1/") {
		t.Errorf("crawl progress = %q, want it advanced", got)
	}

	m.sm.indexing = true
	m.Update(indexMsg{done: true, total: 1})
	if m.sm.indexing {
		t.Error("index-all completion dropped while the flag dialog was open")
	}
}

func openDialog(t *testing.T, m *Model, open func(*Model)) {
	t.Helper()
	open(m)
	if m.activeDialog() == nil {
		t.Fatal("the dialog did not open")
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

// streamedTree is a root read with its subcommands as stubs.
func streamedTree() *models.Node {
	root := &models.Node{Name: "tool", FullPath: []string{"tool"}, Discovered: true}
	for _, n := range []string{"alpha", "beta"} {
		root.Children = append(root.Children, &models.Node{Name: n, FullPath: []string{"tool", n}, Stub: true})
	}
	return root
}

func TestCrawl_resultsFillTheTreeQueueNewStubsAndFinish(t *testing.T) {
	root := streamedTree()
	m := NewModel(root, config.DefaultConfig())
	m.EnableCrawl(CrawlOptions{MaxDepth: 2, StubThreshold: 50, Workers: 1})
	defer m.crawl.stop()
	m.crawl.stop() // no real discovery: results are fed in by hand below
	if !m.Crawling() || m.crawl.outstanding != 2 {
		t.Fatalf("want 2 queued stubs, got %d", m.crawl.outstanding)
	}
	if got := m.crawlProgress(); got != "discovering… 0/2" {
		t.Errorf("progress at start = %q", got)
	}

	alpha, beta := root.Children[0], root.Children[1]
	disc := &models.Node{Name: "alpha", Flags: []models.Flag{{Name: "--force"}},
		Children: []*models.Node{{Name: "deep", FullPath: []string{"tool", "alpha", "deep"}, Stub: true}}}
	m.handleCrawl(crawlMsg{job: crawlJob{stub: alpha}, discovered: disc})
	if alpha.Stub || len(alpha.Flags) != 1 {
		t.Errorf("alpha not filled: %+v", alpha)
	}
	if m.crawl.outstanding != 2 { // beta, plus alpha's newly revealed child
		t.Errorf("outstanding = %d, want 2 (beta + alpha deep)", m.crawl.outstanding)
	}

	m.handleCrawl(crawlMsg{job: crawlJob{stub: beta}, err: errTest})
	m.handleCrawl(crawlMsg{job: crawlJob{stub: alpha.Children[0]}, discovered: &models.Node{Name: "deep"}})
	if m.Crawling() {
		t.Fatalf("crawl should be finished, %d outstanding", m.crawl.outstanding)
	}
	if !strings.HasPrefix(m.timedMsg, "discovered 3 commands") || m.crawlProgress() != "" {
		t.Errorf("finish not reported: %q (progress %q)", m.timedMsg, m.crawlProgress())
	}
	if m.tree.Patches() != 2 {
		t.Errorf("Patches = %d, want 2 (a failed result changes nothing)", m.tree.Patches())
	}
	if cmd := m.handleCrawl(crawlMsg{job: crawlJob{stub: beta}}); cmd != nil {
		t.Error("a result after the crawl finished was not ignored")
	}
}

func TestCrawl_nothingPendingMeansNoCrawl(t *testing.T) {
	root := &models.Node{Name: "tool", FullPath: []string{"tool"}}
	m := NewModel(root, config.DefaultConfig())
	m.EnableCrawl(CrawlOptions{MaxDepth: 2, StubThreshold: 50})
	if m.Crawling() {
		t.Error("a tree without stubs started a crawl")
	}
}

func TestRebuildKeepingSelection_rowsInsertedAboveDoNotMoveTheCursor(t *testing.T) {
	root := streamedTree()
	tree := NewTreeModel(root, config.DefaultConfig())
	tree.SetSize(80, 30)
	// Select beta, then expand alpha (above it) and give it children.
	for tree.Selected() != root.Children[1] {
		before := tree.cursor
		tree.Down()
		if tree.cursor == before {
			t.Fatal("never reached beta")
		}
	}
	alpha := root.Children[0]
	tree.nodeExpanded[nodeKey(alpha, 1)] = true
	alpha.Stub = false
	alpha.Children = []*models.Node{{Name: "x", FullPath: []string{"tool", "alpha", "x"}}, {Name: "y", FullPath: []string{"tool", "alpha", "y"}}}
	tree.rebuildKeepingSelection()
	if tree.Selected() != root.Children[1] {
		t.Errorf("selection moved off beta to %v", tree.Selected())
	}
}

var errTest = testErr("boom")

type testErr string

func (e testErr) Error() string { return string(e) }

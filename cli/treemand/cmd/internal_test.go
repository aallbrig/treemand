package cmd

import (
	"testing"

	"github.com/aallbrig/treemand/cache"
	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

func TestSaveExpansions_createsTheEntryWhenNoneExists(t *testing.T) {
	c, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	save := saveExpansions(c, "k", "tool", "help")
	if err := save(&models.Node{Name: "tool"}); err != nil {
		t.Fatal(err)
	}
	if n, err := c.Get("k", 0); err != nil || n == nil || n.Name != "tool" {
		t.Fatalf("entry not created: %+v %v", n, err)
	}
	if err := save(&models.Node{Name: "tool", Children: []*models.Node{{Name: "sub"}}}); err != nil {
		t.Fatal(err)
	}
	if n, _ := c.Get("k", 0); n == nil || len(n.Children) != 1 {
		t.Fatalf("entry not updated: %+v", n)
	}
}

func TestCrawlDepthAndStreaming(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Depth = -1
	if crawlDepth(cfg) != 99 {
		t.Errorf("unlimited depth should cap at 99")
	}
	cfg.Depth = 2
	if crawlDepth(cfg) != 2 {
		t.Errorf("crawlDepth = %d, want 2", crawlDepth(cfg))
	}
	defer func(i bool, e int) { cfgInteractive, cfgExpandStubs = i, e }(cfgInteractive, cfgExpandStubs)
	cfgInteractive, cfgExpandStubs = true, 0
	if !streamsDiscovery([]string{"help"}) {
		t.Error("an interactive help-only run should stream")
	}
	for _, s := range [][]string{{"help", "man"}, {"completions"}} {
		if streamsDiscovery(s) {
			t.Errorf("strategies %v should discover up front", s)
		}
	}
	cfgExpandStubs = 1
	if streamsDiscovery([]string{"help"}) {
		t.Error("--expand-stubs exports should discover up front")
	}
	cfgInteractive, cfgExpandStubs = false, 0
	if streamsDiscovery([]string{"help"}) {
		t.Error("non-interactive output should discover up front")
	}
}

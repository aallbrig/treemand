package treeload_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/aallbrig/treemand/cache"
	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/overrides"
	"github.com/aallbrig/treemand/presets"
	"github.com/aallbrig/treemand/treeload"
)

// fakeDiscoverer answers with a fixed tree and counts its calls. No process
// is started: the CLI name below is on no PATH.
type fakeDiscoverer struct {
	calls atomic.Int32
	tree  func() *models.Node
	err   error
}

func (f *fakeDiscoverer) Name() string { return "fake" }

func (f *fakeDiscoverer) Discover(context.Context, string, []string) (*models.Node, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return f.tree(), nil
}

const cli = "treeload-fake-cli-not-on-path"

func smallTree() *models.Node {
	return &models.Node{Name: cli, FullPath: []string{cli}, Description: "fake", Children: []*models.Node{
		{Name: "alpha", FullPath: []string{cli, "alpha"}, Description: "first"},
	}}
}

// partialTree has a stub a TUI crawl would have discovered.
func partialTree() *models.Node {
	return &models.Node{Name: cli, FullPath: []string{cli}, Children: []*models.Node{
		{Name: "alpha", FullPath: []string{cli, "alpha"}, Stub: true},
	}}
}

type harness struct {
	fake       *fakeDiscoverer
	loader     treeload.Loader
	rootOnlies []bool
}

func newHarness(tree func() *models.Node) *harness {
	h := &harness{fake: &fakeDiscoverer{tree: tree}}
	h.loader = treeload.Loader{NewDiscoverers: func(_ treeload.Request, rootOnly bool) []discovery.Discoverer {
		h.rootOnlies = append(h.rootOnlies, rootOnly)
		return []discovery.Discoverer{h.fake}
	}}
	return h
}

func request(t *testing.T) treeload.Request {
	t.Helper()
	return treeload.Request{CLI: cli, Depth: 2, StubThreshold: 50, Strategies: []string{"help"}, CacheDir: t.TempDir()}
}

func (h *harness) load(t *testing.T, req treeload.Request) treeload.Result {
	t.Helper()
	res, err := h.loader.Load(context.Background(), req)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.Tree == nil {
		t.Fatal("Load returned no tree")
	}
	return res
}

func TestLoad_aMissDiscoversAndTheNextLoadHitsTheCache(t *testing.T) {
	h := newHarness(smallTree)
	req := request(t)
	first := h.load(t, req)
	if first.Cached || first.Save == nil || h.fake.calls.Load() != 1 {
		t.Fatalf("first load: cached=%v save=%v calls=%d", first.Cached, first.Save != nil, h.fake.calls.Load())
	}
	second := h.load(t, req)
	if !second.Cached || h.fake.calls.Load() != 1 {
		t.Fatalf("second load should hit: cached=%v calls=%d", second.Cached, h.fake.calls.Load())
	}
	if len(second.Tree.Children) != 1 || second.Tree.Children[0].Description != "first" {
		t.Fatalf("cached tree differs: %+v", second.Tree.Children)
	}
}

func TestLoad_refreshDiscoversOnAHitAndRewritesTheEntry(t *testing.T) {
	h := newHarness(smallTree)
	req := request(t)
	h.load(t, req)
	req.Cache = treeload.Refresh
	if res := h.load(t, req); res.Cached || h.fake.calls.Load() != 2 {
		t.Fatalf("refresh should discover: cached=%v calls=%d", res.Cached, h.fake.calls.Load())
	}
	req.Cache = treeload.UseCache
	if res := h.load(t, req); !res.Cached {
		t.Fatal("the refreshed entry should be a hit")
	}
}

func TestLoad_bypassNeitherReadsNorWrites(t *testing.T) {
	h := newHarness(smallTree)
	req := request(t)
	req.Cache = treeload.Bypass
	if res := h.load(t, req); res.Save != nil || res.Cached {
		t.Fatalf("bypass: save=%v cached=%v", res.Save != nil, res.Cached)
	}
	req.Cache = treeload.UseCache
	if res := h.load(t, req); res.Cached {
		t.Fatal("a bypassed load wrote the cache")
	}
}

func TestLoad_expandedTreesAreNeverCached(t *testing.T) {
	h := newHarness(smallTree)
	req := request(t)
	req.ExpandStubs = 1
	h.load(t, req)
	req.ExpandStubs = 0
	if res := h.load(t, req); res.Cached {
		t.Fatal("an expanded export was cached")
	}
}

func TestLoad_theOverrideAppliesOnAMissAndAHitAndKeysTheEntry(t *testing.T) {
	h := newHarness(smallTree)
	req := request(t)
	ov, err := overrides.Parse([]byte("commands:\n  alpha:\n    description: overridden\n"))
	if err != nil {
		t.Fatal(err)
	}
	ov.Hash = "v1"
	req.Override = ov
	for _, want := range []bool{false, true} {
		res := h.load(t, req)
		// The entry holds the overridden tree, so a hit has nothing left to change.
		if res.Cached != want || res.Tree.Children[0].Description != "overridden" || (res.OverrideChanges == 0) != want {
			t.Fatalf("cached=%v: description %q, %d changes", res.Cached, res.Tree.Children[0].Description, res.OverrideChanges)
		}
	}
	ov.Hash = "v2"
	if res := h.load(t, req); res.Cached {
		t.Fatal("an edited override must not reuse the old entry")
	}
}

func TestLoad_thePresetSeedIsGraftedAndAFailingSeedKeepsTheTree(t *testing.T) {
	h := newHarness(smallTree)
	req := request(t)
	req.Preset = &presets.Preset{Name: "p", Seed: func(context.Context, string) (*models.Node, error) {
		return &models.Node{Name: cli, Children: []*models.Node{{Name: "seeded", Stub: true}}}, nil
	}}
	if res := h.load(t, req); res.Tree.Find("seeded") == nil {
		t.Fatalf("seed not grafted: %+v", res.Tree.Children)
	}
	req = request(t)
	req.Preset = &presets.Preset{Name: "p", Seed: func(context.Context, string) (*models.Node, error) {
		return nil, errors.New("no map")
	}}
	if res := h.load(t, req); len(res.Tree.Children) != 1 {
		t.Fatalf("a failing seed lost the tree: %+v", res.Tree.Children)
	}
}

func TestLoad_aCrawlingCallerGetsAPartialTreeAsItIs(t *testing.T) {
	h := newHarness(partialTree)
	req := request(t)
	req.Crawl = true
	h.load(t, req)
	res := h.load(t, req)
	if !res.Cached || !res.Tree.Children[0].Stub {
		t.Fatalf("a crawling caller's hit should stay partial: cached=%v", res.Cached)
	}
	if len(h.rootOnlies) != 1 || !h.rootOnlies[0] {
		t.Fatalf("a crawling help-only load should ask the root alone: %v", h.rootOnlies)
	}
}

func TestLoad_aDiscoveryErrorStillNamesTheDiscoverers(t *testing.T) {
	h := newHarness(smallTree)
	h.fake.err = errors.New("boom")
	res, err := h.loader.Load(context.Background(), request(t))
	if err == nil || res.Tree != nil || len(res.Discoverers) != 1 {
		t.Fatalf("err=%v tree=%v discoverers=%d", err, res.Tree, len(res.Discoverers))
	}
}

func TestLoad_helpTextRunsNothingAndCachesNothing(t *testing.T) {
	h := newHarness(smallTree)
	req := request(t)
	req.HelpText = "Usage:\n  x [command]\n\nAvailable Commands:\n  beta   second\n"
	res := h.load(t, req)
	if h.fake.calls.Load() != 0 || res.Save != nil || res.Tree.Find("beta") == nil {
		t.Fatalf("calls=%d save=%v tree=%+v", h.fake.calls.Load(), res.Save != nil, res.Tree.Children)
	}
}

func TestRootOnly(t *testing.T) {
	base := treeload.Request{Strategies: []string{"help"}, Crawl: true}
	if !treeload.RootOnly(base) {
		t.Error("a crawling help-only load should ask the root alone")
	}
	for name, req := range map[string]treeload.Request{
		"merged":   {Strategies: []string{"help", "man"}, Crawl: true},
		"expanded": {Strategies: []string{"help"}, Crawl: true, ExpandStubs: 1},
		"no crawl": {Strategies: []string{"help"}},
	} {
		if treeload.RootOnly(req) {
			t.Errorf("%s: should discover up front", name)
		}
	}
	if treeload.CrawlDepth(-1) != 99 || treeload.CrawlDepth(2) != 2 {
		t.Error("CrawlDepth: unlimited caps at 99")
	}
}

func TestSaveTo_createsTheEntryWhenNoneExists(t *testing.T) {
	c, err := cache.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	save := treeload.SaveTo(c, "k", "tool", "help")
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

// The TUI saves what it discovered when it exits, long after Load returned.
func TestLoad_saveWorksAfterLoadReturns(t *testing.T) {
	h := newHarness(partialTree)
	req := request(t)
	req.Crawl = true
	res := h.load(t, req)
	res.Tree.Children[0].Stub = false
	res.Tree.Children[0].Description = "crawled"
	if err := res.Save(res.Tree); err != nil {
		t.Fatalf("save after Load: %v", err)
	}
	if again := h.load(t, req); again.Tree.Children[0].Description != "crawled" {
		t.Fatalf("the saved tree was not kept: %+v", again.Tree.Children[0])
	}
}

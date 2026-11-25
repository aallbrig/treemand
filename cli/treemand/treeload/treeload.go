// Package treeload gets a target CLI's command tree: from the cache when a
// fresh entry exists, otherwise from discovery, with the CLI's preset and the
// user's override applied (see Tree load in GLOSSARY.md).
//
// Every caller (the root command, the TUI, `treemand mcp`, `treemand doctor`
// and --stdin) builds a Request; the rules for cache hits, partial trees,
// preset seeds and overrides live here once.
package treeload

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/aallbrig/treemand/cache"
	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/overrides"
	"github.com/aallbrig/treemand/presets"
)

// CachePolicy says whether a load reads and writes the cache.
type CachePolicy int

const (
	// UseCache reads a fresh entry and writes what discovery finds.
	UseCache CachePolicy = iota
	// Refresh discovers even on a hit, and writes the result.
	Refresh
	// Bypass neither reads nor writes the cache.
	Bypass
)

// Request is one tree load.
type Request struct {
	CLI           string
	Depth         int // levels to discover; -1 is unlimited
	StubThreshold int
	Strategies    []string
	Preset        *presets.Preset     // nil without one
	Override      *overrides.Override // nil without one
	Cache         CachePolicy
	CacheDir      string
	CacheTTL      time.Duration
	// Crawl means the caller keeps discovering the tree (the TUI): a partial
	// cached tree is returned as it is, and a help-only discovery asks only
	// the root.
	Crawl bool
	// ExpandStubs discovers stub nodes this many levels deep before
	// returning. An expanded tree is an export: it is never cached.
	ExpandStubs int
	// HelpText, when set, is the root's help text: nothing is run and
	// nothing is cached, so subcommands stay stubs.
	HelpText string
	// Progress wraps each slow step with a message; nil runs it silently.
	Progress func(msg string, step func())
}

// Result is a loaded tree and how to keep it.
type Result struct {
	Tree *models.Node
	// Save writes a grown tree back to the cache entry, or is nil when the
	// load does not use the cache.
	Save func(*models.Node) error
	// Cached reports a tree that came from the cache.
	Cached bool
	// Discoverers are the strategies a discovery ran (nil on a cache hit).
	Discoverers     []discovery.Discoverer
	OverrideChanges int
}

// Loader loads trees. The zero value discovers with the discovery package;
// tests swap in a fake discoverer.
type Loader struct {
	// NewDiscoverers builds the discoverers for a request. rootOnly asks
	// for the root alone, its children as stubs.
	NewDiscoverers func(req Request, rootOnly bool) []discovery.Discoverer
}

// Load gets req's tree with the default Loader.
func Load(ctx context.Context, req Request) (Result, error) {
	return Loader{}.Load(ctx, req)
}

// Load gets req's tree: a cache hit (completed when partial), or a fresh
// discovery seeded from the preset and stored. The override is applied last
// either way. On a discovery error the Result still names the discoverers.
func (l Loader) Load(ctx context.Context, req Request) (Result, error) {
	if req.HelpText != "" {
		node := discovery.FromHelpText(req.CLI, req.HelpText)
		return Result{Tree: node, OverrideChanges: applyOverride(req.Override, node)}, nil
	}
	if req.ExpandStubs > 0 {
		req.Cache = Bypass
	}
	entry := openEntry(req)
	if entry != nil {
		defer entry.c.Close()
	}
	if node := entry.get(req); node != nil {
		save := entry.saver()
		if !req.Crawl {
			l.finishPartial(ctx, req, node, save)
		}
		return Result{Tree: node, Save: save, Cached: true, OverrideChanges: applyOverride(req.Override, node)}, nil
	}

	// The version string is only shown by `cache list`. Read it while
	// discovery runs so a slow --version (aws, pip) adds nothing.
	versionCh := entry.readVersion()
	ds := l.discoverers(req)
	res := Result{Discoverers: ds}
	node, err := discover(ctx, req, ds)
	if err != nil {
		return res, err // versionCh is buffered: its reader never blocks
	}
	if req.Preset != nil && req.Preset.Seed != nil {
		seed(ctx, req, node)
	}
	if req.ExpandStubs > 0 {
		progress(req, "expanding stubs…", func() {
			discovery.ExpandStubs(ctx, node, req.CLI, req.ExpandStubs, 16, 30*time.Second, nil)
		})
	}
	res.Tree = node
	res.OverrideChanges = applyOverride(req.Override, node)
	entry.put(node, versionCh)
	res.Save = entry.saver()
	return res, nil
}

// CrawlDepth is the depth a background crawl or completion stops at,
// matching what an up-front discovery reaches: unlimited (-1) caps at 99.
func CrawlDepth(depth int) int {
	if depth < 0 {
		return 99
	}
	return depth
}

// RootOnly reports whether req discovers the root alone and leaves the rest
// to the caller's crawl. Merged strategies and expanded exports discover
// everything up front.
func RootOnly(req Request) bool {
	return req.Crawl && req.ExpandStubs == 0 && len(req.Strategies) == 1 && req.Strategies[0] == "help"
}

func (l Loader) discoverers(req Request) []discovery.Discoverer {
	if l.NewDiscoverers != nil {
		return l.NewDiscoverers(req, RootOnly(req))
	}
	if RootOnly(req) {
		d := discovery.NewHelpDiscoverer(1)
		d.ChildrenAsStubs = true
		d.StubThreshold = req.StubThreshold
		return []discovery.Discoverer{d}
	}
	return discovery.BuildDiscoverersWithThreshold(req.Strategies, CrawlDepth(req.Depth), req.StubThreshold)
}

func discover(ctx context.Context, req Request, ds []discovery.Discoverer) (*models.Node, error) {
	var (
		node *models.Node
		err  error
	)
	progress(req, "discovering "+req.CLI+"…", func() {
		node, err = discovery.Run(ctx, ds, req.CLI)
	})
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}
	if node == nil {
		return nil, fmt.Errorf("no results from discovery for %q", req.CLI)
	}
	return node, nil
}

// seed grafts the preset's own map of the CLI onto the tree. A failing seed
// keeps the discovered tree.
func seed(ctx context.Context, req Request, node *models.Node) {
	progress(req, "mapping "+req.CLI+"…", func() {
		s, err := req.Preset.Seed(ctx, req.CLI)
		if err != nil {
			log.Warn().Err(err).Str("preset", req.Preset.Name).Msg("preset seed failed")
			return
		}
		presets.Graft(node, s)
	})
}

// finishPartial completes a cached tree that a TUI session saved mid-crawl,
// so a caller that does not crawl never gets a partial tree.
func (l Loader) finishPartial(ctx context.Context, req Request, node *models.Node, save func(*models.Node) error) {
	depth := CrawlDepth(req.Depth)
	if len(discovery.PendingStubs(node, depth, req.StubThreshold)) == 0 {
		return
	}
	progress(req, "discovering "+req.CLI+"…", func() {
		discovery.CompletePending(ctx, node, req.CLI, depth, req.StubThreshold, discovery.DefaultMaxConcurrentHelp())
	})
	if err := save(node); err != nil {
		log.Warn().Err(err).Msg("cache write failed")
	}
}

func progress(req Request, msg string, step func()) {
	if req.Progress == nil {
		step()
		return
	}
	req.Progress(msg, step)
}

// applyOverride merges o into the tree and logs how much it changed.
func applyOverride(o *overrides.Override, node *models.Node) int {
	n := o.Apply(node)
	if o != nil {
		log.Debug().Str("override", o.Path).Int("changes", n).Msg("override applied")
	}
	return n
}

// KeyExtra is the part of the cache key that depends on how the tree was
// discovered: depth, stub threshold, preset and override. Editing the
// override file changes it, so a tree built with the old one is not shown.
func KeyExtra(req Request) string {
	extra := fmt.Sprintf("d=%d,t=%d", req.Depth, req.StubThreshold)
	if req.Preset != nil {
		extra += ",p=" + req.Preset.Name
	}
	if req.Override != nil {
		extra += ",o=" + req.Override.Hash
	}
	return extra
}

// entry is the cache entry one load reads and writes. A nil *entry means the
// load has no cache (Bypass, or the cache failed to open); every method is
// then a no-op.
type entry struct {
	c        *cache.Cache
	key      string
	cli      string
	strategy string
}

func openEntry(req Request) *entry {
	if req.Cache == Bypass {
		return nil
	}
	c, err := cache.Open(req.CacheDir)
	if err != nil {
		log.Warn().Err(err).Msg("could not open cache, running without")
		return nil
	}
	// Key on the binary on disk, not on its --version output: starting aws
	// or pip just to read a version costs seconds on every hit, and a CLI
	// with no --version would never be invalidated.
	ver := cache.BinaryFingerprint(discovery.ResolveBinary(req.CLI))
	if ver == "" {
		ver = cache.CLIVersion(req.CLI)
	}
	return &entry{
		c:        c,
		key:      cache.KeyWith(req.CLI, ver, req.Strategies, KeyExtra(req)),
		cli:      req.CLI,
		strategy: strings.Join(req.Strategies, ","),
	}
}

// get returns the cached tree, or nil on a miss or a refresh.
func (e *entry) get(req Request) *models.Node {
	if e == nil {
		return nil
	}
	if req.Cache == Refresh {
		log.Debug().Str("cli", e.cli).Msg("cache bypassed by refresh")
		return nil
	}
	node, err := e.c.Get(e.key, req.CacheTTL)
	if err != nil || node == nil {
		return nil
	}
	log.Debug().Str("cli", e.cli).Msg("cache hit")
	return node
}

// saver returns the hook that writes a grown tree back to the entry, or nil
// without a cache.
func (e *entry) saver() func(*models.Node) error {
	if e == nil {
		return nil
	}
	return SaveTo(e.c, e.key, e.cli, e.strategy)
}

// SaveTo returns the hook that writes a grown tree back to the cache:
// commands discovered in the TUI (background crawl, stubs expanded on
// demand, indexing) are kept, so the next run starts with them.
func SaveTo(c *cache.Cache, key, cli, strategy string) func(*models.Node) error {
	return func(root *models.Node) error {
		ok, err := c.UpdateTree(key, root)
		if err == nil && !ok {
			err = c.Put(key, cli, "", strategy, root)
		}
		return err
	}
}

func (e *entry) readVersion() chan string {
	if e == nil {
		return nil
	}
	ch := make(chan string, 1)
	go func() { ch <- cache.CLIVersion(e.cli) }()
	return ch
}

func (e *entry) put(node *models.Node, versionCh chan string) {
	if e == nil {
		return
	}
	if err := e.c.Put(e.key, e.cli, <-versionCh, e.strategy, node); err != nil {
		log.Warn().Err(err).Msg("cache write failed")
	}
}

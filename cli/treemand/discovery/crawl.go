package discovery

import (
	"context"
	"sync"

	"github.com/aallbrig/treemand/models"
)

// Pending reports whether n is a stub that a full discovery would have
// discovered: it lies within maxDepth and its parent lists no more than
// stubThreshold subcommands. Stubs under a parent with more subcommands than
// that (aws services) are lazy on purpose and are not pending.
func Pending(n, parent *models.Node, maxDepth, stubThreshold int) bool {
	if n == nil || !n.Stub || parent == nil {
		return false
	}
	if maxDepth >= 0 && len(n.FullPath)-1 > maxDepth {
		return false
	}
	return len(parent.Children) <= stubThreshold
}

// PendingStubs returns every pending stub below root, breadth-first.
func PendingStubs(root *models.Node, maxDepth, stubThreshold int) []*models.Node {
	var out []*models.Node
	for _, p := range pendingWithParents(root, maxDepth, stubThreshold) {
		out = append(out, p.stub)
	}
	return out
}

type pendingStub struct{ stub, parent *models.Node }

func pendingWithParents(root *models.Node, maxDepth, stubThreshold int) []pendingStub {
	var out []pendingStub
	queue := []*models.Node{root}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, c := range n.Children {
			if Pending(c, n, maxDepth, stubThreshold) {
				out = append(out, pendingStub{c, n})
			}
			queue = append(queue, c)
		}
	}
	return out
}

// Crawler discovers stubs one node at a time: each result lists its own
// subcommands as stubs, for the caller to queue in turn. All jobs share one
// HelpDiscoverer, so the help form learned from the first answer is reused.
type Crawler struct {
	CLI      string
	MaxDepth int
	d        *HelpDiscoverer
}

// NewCrawler returns a crawler for cliName down to maxDepth (-1 = unlimited).
func NewCrawler(cliName string, maxDepth int) *Crawler {
	d := NewHelpDiscoverer(1)
	d.ChildrenAsStubs = true
	return &Crawler{CLI: cliName, MaxDepth: maxDepth, d: d}
}

// DiscoverOne discovers the node at path. Its subcommands come back as stubs,
// except at MaxDepth, where a full discovery would not list them either.
func (c *Crawler) DiscoverOne(ctx context.Context, path []string) (*models.Node, error) {
	n, err := c.d.Discover(ctx, c.CLI, path)
	if err == nil && n != nil && c.MaxDepth >= 0 && len(path) >= c.MaxDepth {
		n.Children = nil
	}
	return n, err
}

// CompletePending discovers every pending stub below root (and the pending
// stubs those reveal) with up to workers in flight, filling them in place.
// It makes a tree saved mid-crawl whole again. Returns the number filled.
func CompletePending(ctx context.Context, root *models.Node, cliName string, maxDepth, stubThreshold, workers int) int {
	c := NewCrawler(cliName, maxDepth)
	filled := 0
	for {
		batch := pendingWithParents(root, maxDepth, stubThreshold)
		if len(batch) == 0 || ctx.Err() != nil {
			return filled
		}
		results := make([]*models.Node, len(batch))
		sem := make(chan struct{}, max(1, workers))
		var wg sync.WaitGroup
		for i, p := range batch {
			wg.Add(1)
			go func(i int, path []string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if n, err := c.DiscoverOne(ctx, path); err == nil {
					results[i] = n
				}
			}(i, append([]string(nil), p.stub.FullPath[1:]...))
		}
		wg.Wait()
		progress := 0
		for i, p := range batch {
			if results[i] != nil {
				FillUnder(p.parent, p.stub, results[i])
				progress++
			}
		}
		filled += progress
		if progress == 0 {
			return filled // nothing more can be learned; avoid spinning
		}
	}
}

// FillUnder is Fill for a stub whose parent is known. A subcommand that only
// repeats its parent's help (an unknown-command fallback) gets no children
// of its own; following them would loop down to the depth limit.
func FillUnder(parent, stub, discovered *models.Node) {
	if parent != nil && discovered.HelpText != "" && discovered.HelpText == parent.HelpText {
		discovered.Children = nil
	}
	Fill(stub, discovered)
}

// Fill copies a discovery result into the stub it was made for.
func Fill(stub, discovered *models.Node) {
	stub.Stub = false
	stub.Discovered = true
	stub.Children = discovered.Children
	// A command's own help describes it better than the one-liner its parent
	// lists, which is what a stub starts with; a full discovery prefers it too.
	if discovered.Description != "" {
		stub.Description = discovered.Description
	}
	// A stub may hold only the options its parent's commands all accept
	// (apt's, see rootOptions); its own discovered flags replace those.
	if len(stub.Flags) == 0 || (len(discovered.Flags) > 0 && allInherited(stub.Flags)) {
		stub.Flags = discovered.Flags
	}
	if len(stub.Positionals) == 0 {
		stub.Positionals = discovered.Positionals
	}
	if stub.HelpText == "" {
		stub.HelpText = discovered.HelpText
	}
	if discovered.DiscoveryErr != "" {
		stub.DiscoveryErr = discovered.DiscoveryErr
	}
}

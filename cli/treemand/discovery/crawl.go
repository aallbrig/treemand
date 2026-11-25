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
	queue := []*models.Node{root}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, c := range n.Children {
			if Pending(c, n, maxDepth, stubThreshold) {
				out = append(out, c)
			}
			queue = append(queue, c)
		}
	}
	return out
}

// CompletePending discovers every pending stub below root (and the pending
// stubs those reveal) with up to workers in flight, filling them in place.
// It makes a tree saved mid-crawl whole again. Returns the number filled.
func CompletePending(ctx context.Context, root *models.Node, cliName string, maxDepth, stubThreshold, workers int) int {
	e := NewNodeExpander(cliName, ExpandOptions{MaxDepth: maxDepth})
	filled := 0
	for {
		batch := PendingStubs(root, maxDepth, stubThreshold)
		if len(batch) == 0 || ctx.Err() != nil {
			return filled
		}
		results := make([]*models.Node, len(batch))
		sem := make(chan struct{}, max(1, workers))
		var wg sync.WaitGroup
		for i, stub := range batch {
			wg.Add(1)
			go func(i int, path []string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if n, err := e.Discover(ctx, path); err == nil {
					results[i] = n
				}
			}(i, append([]string(nil), stub.FullPath[1:]...))
		}
		wg.Wait()
		progress := 0
		for i, stub := range batch {
			if results[i] != nil {
				e.Fill(root, stub, results[i])
				progress++
			}
		}
		filled += progress
		if progress == 0 {
			return filled // nothing more can be learned; avoid spinning
		}
	}
}

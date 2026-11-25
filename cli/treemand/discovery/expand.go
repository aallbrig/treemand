package discovery

import (
	"context"
	"sync"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// ExpandStubs discovers every stub node under root, one level at a time, for
// the given number of rounds. Children found are left as name-only stubs
// (but a lone one; see OneLevel) so each round goes one level deeper. onProgress (optional) is called after
// each node with the running count for the current round.
//
// It exists for exporting complete snapshots of very large CLIs (aws, stripe)
// whose trees are otherwise created lazily. opts.Timeout bounds each node;
// its depth and eagerness are set here.
func ExpandStubs(ctx context.Context, root *models.Node, cliName string, rounds, workers int, opts ExpandOptions, onProgress func(done, total int)) {
	if workers < 1 {
		workers = 8
	}
	opts.EagerBelow, opts.MaxDepth = OneLevel, -1
	e := NewNodeExpander(cliName, opts)
	for r := 0; r < rounds; r++ {
		var stubs []*models.Node
		root.Walk(func(n *models.Node) {
			if n.Stub && n != root && len(n.FullPath) > 1 {
				stubs = append(stubs, n)
			}
		})
		if len(stubs) == 0 || ctx.Err() != nil {
			break
		}
		results := make([]*models.Node, len(stubs))
		var (
			wg   sync.WaitGroup
			mu   sync.Mutex
			done int
		)
		sem := make(chan struct{}, workers)
		for i, stub := range stubs {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, path []string) {
				defer wg.Done()
				defer func() { <-sem }()
				if res, err := e.Discover(ctx, path); err == nil {
					results[i] = res
				}
				if onProgress != nil {
					mu.Lock()
					done++
					onProgress(done, len(stubs))
					mu.Unlock()
				}
			}(i, stub.FullPath[1:])
		}
		wg.Wait()
		// Merged here, not in the workers: a fill reads the stub's ancestors.
		for i, stub := range stubs {
			e.Fill(root, stub, results[i])
		}
	}
}

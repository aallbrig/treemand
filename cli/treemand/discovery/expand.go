package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/aallbrig/treemand/models"
)

// ExpandStubs discovers every stub node under root, one level at a time, for
// the given number of rounds. Children found are left as name-only stubs so
// each round goes one level deeper. onProgress (optional) is called after
// each node with the running count for the current round.
//
// It exists for exporting complete snapshots of very large CLIs (aws, stripe)
// whose trees are otherwise created lazily.
func ExpandStubs(ctx context.Context, root *models.Node, cliName string, rounds, workers int, perNode time.Duration, onProgress func(done, total int)) {
	if workers < 1 {
		workers = 8
	}
	for r := 0; r < rounds; r++ {
		var stubs []*models.Node
		root.Walk(func(n *models.Node) {
			if n.Stub && n != root && len(n.FullPath) > 1 {
				stubs = append(stubs, n)
			}
		})
		if len(stubs) == 0 {
			break
		}
		var (
			wg   sync.WaitGroup
			mu   sync.Mutex
			done int
		)
		sem := make(chan struct{}, workers)
		for _, stub := range stubs {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(stub *models.Node) {
				defer wg.Done()
				defer func() { <-sem }()
				d := NewHelpDiscoverer(1)
				d.StubThreshold = 1 // children stay name-only stubs
				nctx, cancel := context.WithTimeout(ctx, perNode)
				defer cancel()
				res, err := d.Discover(nctx, cliName, stub.FullPath[1:])
				if err == nil && res != nil {
					adopt(stub, res)
				}
				if onProgress != nil {
					mu.Lock()
					done++
					onProgress(done, len(stubs))
					mu.Unlock()
				}
			}(stub)
		}
		wg.Wait()
	}
	models.MarkInheritedFlags(root)
}

// adopt copies freshly discovered data into a stub node.
func adopt(stub, res *models.Node) {
	stub.Stub = false
	stub.Discovered = true
	stub.Children = res.Children
	if stub.Description == "" {
		stub.Description = res.Description
	}
	stub.Flags = res.Flags
	stub.Positionals = res.Positionals
	stub.HelpText = res.HelpText
	stub.DiscoveryErr = res.DiscoveryErr
}

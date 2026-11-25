package discovery

import (
	"context"
	"time"

	"github.com/aallbrig/treemand/models"
)

// DefaultExpandTimeout bounds the discovery of one node.
const DefaultExpandTimeout = 30 * time.Second

// DefaultStubThreshold is the subcommand count above which a HelpDiscoverer
// lists children as stubs when no threshold is set.
const DefaultStubThreshold = 50

// OneLevel is the EagerBelow for expanding one level at a time: a node's
// subcommands stay stubs, except a lone one, which is discovered with it.
const OneLevel = 1

// ExpandOptions configures a NodeExpander.
type ExpandOptions struct {
	// EagerBelow discovers a result's own subcommands too when it lists at
	// most this many (the TUI's expand on demand); 0 lists them as stubs.
	EagerBelow int
	// MaxDepth is the deepest path a result lists subcommands for, as a full
	// discovery to that depth would (-1 = unlimited).
	MaxDepth int
	// Timeout bounds each node's discovery; 0 means DefaultExpandTimeout.
	Timeout time.Duration
	// Override, when set, re-applies the CLI's override to the whole tree
	// after each merge, since a result carries none of its corrections.
	Override func(root *models.Node)
}

// NodeExpander discovers single nodes of one CLI and merges each result into
// the tree it came from. It is the one place that decides how a result is
// merged: Fill for a stub, Refresh for a re-discovery the user asked for.
// Discover is safe for concurrent use; Fill and Refresh change the tree and
// belong to the goroutine that owns it.
type NodeExpander struct {
	cli  string
	opts ExpandOptions
	d    *HelpDiscoverer // shared, so the help form the first answer used is reused
}

// NewNodeExpander returns an expander for cliName.
func NewNodeExpander(cliName string, opts ExpandOptions) *NodeExpander {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultExpandTimeout
	}
	d := NewHelpDiscoverer(1)
	if opts.EagerBelow > 0 {
		d.StubThreshold = opts.EagerBelow
	} else {
		d.ChildrenAsStubs = true
	}
	return &NodeExpander{cli: cliName, opts: opts, d: d}
}

// Discover asks the CLI for the node at path (below the root). At MaxDepth
// it lists no subcommands, as a full discovery would not either.
func (e *NodeExpander) Discover(ctx context.Context, path []string) (*models.Node, error) {
	ctx, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()
	n, err := e.d.Discover(ctx, e.cli, path)
	if err == nil && n != nil && e.opts.MaxDepth >= 0 && len(path) >= e.opts.MaxDepth {
		n.Children = nil
	}
	return n, err
}

// Expand discovers stub, a node of the tree at root, and fills it in.
func (e *NodeExpander) Expand(ctx context.Context, root, stub *models.Node) error {
	path := []string{}
	if len(stub.FullPath) > 1 {
		path = stub.FullPath[1:]
	}
	res, err := e.Discover(ctx, path)
	if err != nil || res == nil {
		return err
	}
	e.Fill(root, stub, res)
	return nil
}

// Fill merges res into stub, a node of the tree at root. What the stub
// already knows (a preset's or the parent's data) is kept where the result
// adds nothing better; see fillNode.
func (e *NodeExpander) Fill(root, stub, res *models.Node) {
	e.merge(root, stub, res, fillNode)
}

// Refresh merges a re-discovery of node into it: the node takes the fresh
// result's own data, and keeps the subtrees of children still listed.
func (e *NodeExpander) Refresh(root, node, res *models.Node) {
	e.merge(root, node, res, refreshNode)
}

func (e *NodeExpander) merge(root, n, res *models.Node, into func(n, res *models.Node)) {
	if n == nil || res == nil {
		return
	}
	ancestors := ancestorsOf(root, n)
	if len(ancestors) > 0 && repeatsParent(ancestors[len(ancestors)-1].HelpText, res.HelpText) {
		res.Children = nil
	}
	into(n, res)
	models.MarkInheritedFlagsBelow(ancestors, n)
	if e.opts.Override != nil && root != nil {
		e.opts.Override(root)
	}
}

// repeatsParent reports whether a subcommand printed its parent's help: the
// CLI ignored the subcommand (an unknown-command fallback), and following the
// children it lists would loop down to the depth limit.
func repeatsParent(parentHelp, help string) bool {
	return help != "" && help == parentHelp
}

// ancestorsOf returns the chain of n's ancestors in the tree at root, root
// first, or nil when the tree does not hold n. It follows n's FullPath and
// falls back to a search when the path does not lead to n.
func ancestorsOf(root, n *models.Node) []*models.Node {
	if root == nil || root == n {
		return nil
	}
	if len(n.FullPath) > 1 {
		chain := []*models.Node{root}
		cur := root
		for _, name := range n.FullPath[1 : len(n.FullPath)-1] {
			if cur = cur.Find(name); cur == nil {
				break
			}
			chain = append(chain, cur)
		}
		if cur != nil && cur.Find(n.Name) == n {
			return chain
		}
	}
	return searchAncestors(root, n)
}

func searchAncestors(at, n *models.Node) []*models.Node {
	for _, c := range at.Children {
		if c == n {
			return []*models.Node{at}
		}
		if chain := searchAncestors(c, n); chain != nil {
			return append([]*models.Node{at}, chain...)
		}
	}
	return nil
}

// fillNode copies a discovery result into the stub it was made for.
func fillNode(stub, res *models.Node) {
	stub.Stub = false
	stub.Discovered = true
	stub.Children = res.Children
	// A command's own help describes it better than the one-liner its parent
	// lists, which is what a stub starts with; a full discovery prefers it too.
	if res.Description != "" {
		stub.Description = res.Description
	}
	// A stub may hold only the options its parent's commands all accept
	// (apt's, see rootOptions); its own discovered flags replace those.
	if len(stub.Flags) == 0 || (len(res.Flags) > 0 && allInherited(stub.Flags)) {
		stub.Flags = res.Flags
	}
	if len(stub.Positionals) == 0 {
		stub.Positionals = res.Positionals
	}
	if stub.HelpText == "" {
		stub.HelpText = res.HelpText
	}
	if res.DiscoveryErr != "" {
		stub.DiscoveryErr = res.DiscoveryErr
	}
}

// refreshNode replaces a node's own data with a fresh result: a re-discovery
// the user asked for must show what the CLI's help says now. Only a
// description the result lacks is kept, since the parent's one-liner still
// describes the command. Children are matched by name: one already
// discovered keeps its subtree, a new one arrives as the result lists it,
// and one the help no longer lists is removed.
func refreshNode(n, res *models.Node) {
	n.Stub = false
	n.Discovered = true
	if res.Description != "" {
		n.Description = res.Description
	}
	n.Flags = res.Flags
	n.Positionals = res.Positionals
	n.HelpText = res.HelpText
	n.DiscoveryErr = res.DiscoveryErr
	kids := make([]*models.Node, 0, len(res.Children))
	for _, c := range res.Children {
		if old := n.Find(c.Name); old != nil && old.Discovered && !old.Stub {
			c = old
		}
		kids = append(kids, c)
	}
	n.Children = kids
}

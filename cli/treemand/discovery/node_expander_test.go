package discovery_test

import (
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/discovery"
	"github.com/aallbrig/treemand/cli/treemand/models"
)

// node builds a node below path with the given children.
func node(path []string, kids ...*models.Node) *models.Node {
	return &models.Node{Name: path[len(path)-1], FullPath: path, Discovered: true, Children: kids}
}

func stubAt(path ...string) *models.Node {
	return &models.Node{Name: path[len(path)-1], FullPath: path, Stub: true}
}

// tool -> remote (stub) and tool -> run.
func smallTree() (root, remote *models.Node) {
	remote = stubAt("tool", "remote")
	remote.Description = "parent's one-liner"
	root = node([]string{"tool"}, remote, node([]string{"tool", "run"}))
	root.HelpText = "tool help"
	root.Flags = []models.Flag{{Name: "--verbose"}}
	return root, remote
}

func expander() *discovery.NodeExpander {
	return discovery.NewNodeExpander("tool", discovery.ExpandOptions{MaxDepth: -1})
}

func TestNodeExpanderFill_copiesTheResultIntoTheStub(t *testing.T) {
	root, remote := smallTree()
	expander().Fill(root, remote, &models.Node{
		Name: "remote", Description: "Manage remotes", HelpText: "remote help",
		Flags:       []models.Flag{{Name: "--verbose"}, {Name: "--prune"}},
		Positionals: []models.Positional{{Name: "NAME"}},
		Children:    []*models.Node{stubAt("tool", "remote", "add")},
	})
	if remote.Stub || !remote.Discovered {
		t.Errorf("stub %v discovered %v, want a discovered node", remote.Stub, remote.Discovered)
	}
	if remote.Description != "Manage remotes" || remote.HelpText != "remote help" || len(remote.Positionals) != 1 {
		t.Errorf("got %+v, want the result's description, help and positionals", remote)
	}
	if len(remote.Children) != 1 || remote.Children[0].Name != "add" {
		t.Errorf("children = %+v, want add", remote.Children)
	}
	// --verbose is the root's: the fill marks it inherited against the tree.
	for _, f := range remote.Flags {
		if f.Inherited != (f.Name == "--verbose") {
			t.Errorf("%s inherited = %v", f.Name, f.Inherited)
		}
	}
}

// A stub's inherited options give way to the flags its own help lists.
func TestNodeExpanderFill_ownFlagsReplaceInheritedOnes(t *testing.T) {
	root, remote := smallTree()
	remote.Flags = []models.Flag{{Name: "-o", Inherited: true}}
	expander().Fill(root, remote, &models.Node{Name: "remote", Flags: []models.Flag{{Name: "--own"}}})
	if names := flagNames(remote.Flags); !names["--own"] || names["-o"] {
		t.Errorf("flags = %v, want only --own", names)
	}
	root, _ = smallTree()
	run := root.Children[1]
	run.Flags = []models.Flag{{Name: "-o", Inherited: true}}
	expander().Fill(root, run, &models.Node{Name: "remote"})
	if !flagNames(run.Flags)["-o"] {
		t.Errorf("flags = %v, want the inherited -o kept", run.Flags)
	}
}

// What a stub already knows (from a preset or override) is kept.
func TestNodeExpanderFill_keepsWhatTheStubKnows(t *testing.T) {
	root, remote := smallTree()
	remote.HelpText = "seeded help"
	remote.Positionals = []models.Positional{{Name: "SEEDED"}}
	expander().Fill(root, remote, &models.Node{Name: "remote", HelpText: "fresh", Positionals: []models.Positional{{Name: "FRESH"}}})
	if remote.HelpText != "seeded help" || remote.Positionals[0].Name != "SEEDED" {
		t.Errorf("help %q positionals %v, want the seeded ones", remote.HelpText, remote.Positionals)
	}
	if remote.Description != "parent's one-liner" {
		t.Errorf("description = %q, want the one-liner kept when the result has none", remote.Description)
	}
}

// A subcommand that prints its parent's help (an unknown-command fallback)
// gets no children: following them would loop down to the depth limit.
func TestNodeExpander_childRepeatingItsParentsHelpGetsNoChildren(t *testing.T) {
	for _, refresh := range []bool{false, true} {
		root, remote := smallTree()
		res := &models.Node{Name: "remote", HelpText: "tool help", Children: []*models.Node{stubAt("tool", "remote", "remote")}}
		if refresh {
			expander().Refresh(root, remote, res)
		} else {
			expander().Fill(root, remote, res)
		}
		if len(remote.Children) != 0 || !remote.Discovered {
			t.Errorf("refresh=%v: children = %+v, want none", refresh, remote.Children)
		}
	}
}

// A re-discovery replaces what a populated node held (#471).
func TestNodeExpanderRefresh_replacesTheNodesOwnData(t *testing.T) {
	root, remote := smallTree()
	remote.Stub, remote.Discovered = false, true
	remote.HelpText, remote.DiscoveryErr = "old help", "timed out"
	remote.Flags = []models.Flag{{Name: "--old"}}
	remote.Positionals = []models.Positional{{Name: "OLD"}}
	expander().Refresh(root, remote, &models.Node{
		Name: "remote", HelpText: "new help",
		Flags: []models.Flag{{Name: "--old"}, {Name: "--new"}, {Name: "--verbose"}},
	})
	if names := flagNames(remote.Flags); !names["--new"] || !names["--old"] || len(remote.Flags) != 3 {
		t.Errorf("flags = %v, want --old, --new and --verbose", remote.Flags)
	}
	for _, f := range remote.Flags {
		if f.Inherited != (f.Name == "--verbose") {
			t.Errorf("%s inherited = %v after a refresh", f.Name, f.Inherited)
		}
	}
	if remote.HelpText != "new help" || len(remote.Positionals) != 0 || remote.DiscoveryErr != "" {
		t.Errorf("help %q, positionals %v, err %q: want the fresh result's", remote.HelpText, remote.Positionals, remote.DiscoveryErr)
	}
	if remote.Description != "parent's one-liner" {
		t.Errorf("description = %q, want the one-liner kept when the result has none", remote.Description)
	}
}

// Children are matched by name: a discovered one keeps its subtree, a new
// one arrives as the result has it, and one the help no longer lists goes.
func TestNodeExpanderRefresh_matchesChildrenByName(t *testing.T) {
	root, _ := smallTree()
	deep := node([]string{"tool", "run", "fast", "now"})
	fast := node([]string{"tool", "run", "fast"}, deep)
	fast.Flags = []models.Flag{{Name: "--turbo"}}
	gone := stubAt("tool", "run", "gone")
	run := root.Children[1]
	run.Children = []*models.Node{fast, gone}

	expander().Refresh(root, run, &models.Node{Name: "run", HelpText: "run help", Children: []*models.Node{
		stubAt("tool", "run", "fast"), stubAt("tool", "run", "slow"),
	}})
	if len(run.Children) != 2 {
		t.Fatalf("children = %+v, want fast and slow", run.Children)
	}
	if run.Children[0] != fast || len(fast.Children) != 1 || fast.Children[0] != deep || len(fast.Flags) != 1 {
		t.Errorf("fast = %+v, want the discovered node kept with its subtree", run.Children[0])
	}
	if s := run.Children[1]; s.Name != "slow" || !s.Stub {
		t.Errorf("second child = %+v, want the new stub slow", s)
	}
}

// A refresh drops an override's corrections; the expander puts them back.
func TestNodeExpanderRefresh_reappliesTheOverride(t *testing.T) {
	root, remote := smallTree()
	applied := 0
	e := discovery.NewNodeExpander("tool", discovery.ExpandOptions{MaxDepth: -1, Override: func(r *models.Node) {
		applied++
		if n := r.Find("remote"); n != nil {
			n.Description = "from the override"
		}
	}})
	e.Refresh(root, remote, &models.Node{Name: "remote", Description: "from the help"})
	if applied != 1 || remote.Description != "from the override" {
		t.Errorf("override applied %d times, description %q", applied, remote.Description)
	}
	e.Fill(root, root.Children[1], &models.Node{Name: "run", Description: "from the help"})
	if applied != 2 || remote.Description != "from the override" {
		t.Errorf("a fill should re-apply the override too: applied %d", applied)
	}
}

// A node the tree does not hold is still filled, with no parent to guard by.
func TestNodeExpanderFill_nodeOutsideTheTree(t *testing.T) {
	root, _ := smallTree()
	loose := stubAt("tool", "loose")
	expander().Fill(root, loose, &models.Node{Name: "loose", HelpText: "tool help", Children: []*models.Node{stubAt("tool", "loose", "x")}})
	if loose.Stub || len(loose.Children) != 1 {
		t.Errorf("loose = %+v, want it filled", loose)
	}
}

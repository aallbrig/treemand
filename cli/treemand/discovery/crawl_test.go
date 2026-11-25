package discovery

import (
	"context"
	"testing"

	"github.com/aallbrig/treemand/models"
)

func TestPending_followsDepthAndThresholdRules(t *testing.T) {
	parent := &models.Node{FullPath: []string{"x"}}
	stub := &models.Node{Stub: true, FullPath: []string{"x", "a"}}
	parent.Children = []*models.Node{stub, {FullPath: []string{"x", "b"}}}
	cases := []struct {
		name      string
		n, p      *models.Node
		depth, th int
		want      bool
	}{
		{"pending", stub, parent, 3, 50, true},
		{"unlimited depth", stub, parent, -1, 50, true},
		{"beyond max depth", stub, parent, 0, 50, false},
		{"parent over threshold: lazy on purpose", stub, parent, 3, 1, false},
		{"not a stub", parent.Children[1], parent, 3, 50, false},
		{"no parent", stub, nil, 3, 50, false},
	}
	for _, c := range cases {
		if got := Pending(c.n, c.p, c.depth, c.th); got != c.want {
			t.Errorf("%s: Pending = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCompletePending_fillsAStreamedTreeLikeAFullCrawl(t *testing.T) {
	bin, _ := formCLI(t, 0, flagOnly)
	full, err := NewHelpDiscoverer(3).Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}

	d := NewHelpDiscoverer(1)
	d.ChildrenAsStubs = true
	root, err := d.Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(PendingStubs(root, 3, 50)); got != 2 {
		t.Fatalf("want 2 pending stubs after the root, got %d", got)
	}
	if n := CompletePending(context.Background(), root, bin, 3, 50, 4); n != 2 {
		t.Errorf("CompletePending filled %d, want 2", n)
	}
	if left := PendingStubs(root, 3, 50); len(left) != 0 {
		t.Errorf("%d stubs still pending", len(left))
	}
	for i, c := range root.Children {
		want := full.Children[i]
		if c.Name != want.Name || len(c.Flags) != len(want.Flags) || c.Stub {
			t.Errorf("child %d: streamed %+v, full %+v", i, c, want)
		}
	}
}

func TestCompletePending_stopsWhenNothingCanBeLearned(t *testing.T) {
	root := &models.Node{Name: "nope", FullPath: []string{"treemand-no-such-cli"}}
	root.Children = []*models.Node{{Name: "a", Stub: true, FullPath: []string{"treemand-no-such-cli", "a"}}}
	// The CLI does not exist; discovery still returns a node (with an error),
	// so the stub is filled once and the loop ends instead of spinning.
	if n := CompletePending(context.Background(), root, "treemand-no-such-cli", 3, 50, 2); n > 1 {
		t.Errorf("filled %d", n)
	}
}

func TestNodeExpanderDiscover_listsNoChildrenAtMaxDepth(t *testing.T) {
	bin, _ := formCLI(t, 0, flagOnly)
	n, err := NewNodeExpander(bin, ExpandOptions{MaxDepth: 1}).Discover(context.Background(), []string{"alpha"})
	if err != nil || n == nil || len(n.Flags) == 0 {
		t.Fatalf("alpha not discovered: %+v %v", n, err)
	}
	root, _ := NewNodeExpander(bin, ExpandOptions{MaxDepth: 1}).Discover(context.Background(), nil)
	if len(root.Children) != 2 || !root.Children[0].Stub {
		t.Errorf("the root's subcommands should come back as stubs: %+v", root.Children)
	}
	deep, _ := NewNodeExpander(bin, ExpandOptions{MaxDepth: 0}).Discover(context.Background(), nil)
	if len(deep.Children) != 0 {
		t.Errorf("at max depth no subcommands should be listed, got %d", len(deep.Children))
	}
}

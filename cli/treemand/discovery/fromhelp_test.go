package discovery_test

import (
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

func TestFromHelpText_rootIsParsedAndChildrenAreStubs(t *testing.T) {
	help := "Usage: tool [OPTIONS] COMMAND\n\nA small tool\n\nCommands:\n  add     Add a thing\n  remove  Remove a thing\n\nOptions:\n  -q, --quiet  Say less\n"
	n := discovery.FromHelpText("tool", help)
	if n.Name != "tool" || !n.Discovered || n.Stub {
		t.Fatalf("root = %+v", n)
	}
	if n.Description != "A small tool" {
		t.Errorf("description = %q", n.Description)
	}
	if len(n.Flags) == 0 || n.Flags[0].Name != "--quiet" {
		t.Errorf("flags = %+v, want --quiet", n.Flags)
	}
	if len(n.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(n.Children))
	}
	for _, c := range n.Children {
		if c.Discovered || !c.Stub {
			t.Errorf("child %s discovered=%v stub=%v, want an undiscovered stub", c.Name, c.Discovered, c.Stub)
		}
		if len(c.FullPath) != 2 || c.FullPath[0] != "tool" {
			t.Errorf("child %s path = %v", c.Name, c.FullPath)
		}
	}
	if n.Children[0].Description != "Add a thing" {
		t.Errorf("add description = %q", n.Children[0].Description)
	}
}

func TestFromHelpText_noCommandsMeansNoChildren(t *testing.T) {
	n := discovery.FromHelpText("leaf", "Usage: leaf [-v] FILE\n\n  -v  verbose\n")
	if len(n.Children) != 0 {
		t.Errorf("children = %+v, want none", n.Children)
	}
}

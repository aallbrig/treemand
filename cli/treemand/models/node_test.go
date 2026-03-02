package models_test

import (
	"testing"

	"github.com/aallbrig/treemand/models"
)

func TestNodeFullCommand(t *testing.T) {
	n := &models.Node{Name: "add", FullPath: []string{"git", "remote", "add"}}
	if got := n.FullCommand(); got != "git remote add" {
		t.Errorf("FullCommand() = %q, want %q", got, "git remote add")
	}
}

func TestNodeFullCommandNoPath(t *testing.T) {
	n := &models.Node{Name: "git"}
	if got := n.FullCommand(); got != "git" {
		t.Errorf("FullCommand() = %q, want %q", got, "git")
	}
}

func TestNodeIsLeaf(t *testing.T) {
	leaf := &models.Node{Name: "status"}
	if !leaf.IsLeaf() {
		t.Error("expected leaf node")
	}
	parent := &models.Node{Name: "git", Children: []*models.Node{leaf}}
	if parent.IsLeaf() {
		t.Error("expected non-leaf node")
	}
}

func TestNodeFind(t *testing.T) {
	child := &models.Node{Name: "status"}
	parent := &models.Node{Name: "git", Children: []*models.Node{child}}
	if found := parent.Find("status"); found == nil {
		t.Error("expected to find 'status'")
	}
	if found := parent.Find("nonexistent"); found != nil {
		t.Error("expected nil for nonexistent child")
	}
}

func TestNodeWalk(t *testing.T) {
	tree := &models.Node{
		Name: "git",
		Children: []*models.Node{
			{Name: "commit"},
			{Name: "remote", Children: []*models.Node{{Name: "add"}}},
		},
	}
	var names []string
	tree.Walk(func(n *models.Node) { names = append(names, n.Name) })
	expected := []string{"git", "commit", "remote", "add"}
	if len(names) != len(expected) {
		t.Fatalf("Walk() visited %d nodes, want %d", len(names), len(expected))
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("Walk() visited[%d] = %q, want %q", i, names[i], name)
		}
	}
}

func TestNodeClone(t *testing.T) {
	orig := &models.Node{
		Name:     "git",
		FullPath: []string{"git"},
		Flags:    []models.Flag{{Name: "--verbose"}},
		Children: []*models.Node{{Name: "commit"}},
	}
	clone := orig.Clone()
	if clone.Name != orig.Name {
		t.Errorf("Clone().Name = %q, want %q", clone.Name, orig.Name)
	}
	// Modifying clone should not affect original
	clone.Name = "modified"
	if orig.Name == "modified" {
		t.Error("modifying clone affected original")
	}
}

func TestNodeHasFlags(t *testing.T) {
	n := &models.Node{Name: "commit", Flags: []models.Flag{{Name: "--message"}}}
	if !n.HasFlags() {
		t.Error("expected HasFlags() = true")
	}
	empty := &models.Node{Name: "status"}
	if empty.HasFlags() {
		t.Error("expected HasFlags() = false")
	}
}

func TestMarkInheritedFlags(t *testing.T) {
	root := &models.Node{
		Name: "app",
		Flags: []models.Flag{
			{Name: "--verbose"},
			{Name: "--config"},
		},
		Children: []*models.Node{
			{
				Name: "sub",
				Flags: []models.Flag{
					{Name: "--verbose"}, // inherited
					{Name: "--config"},  // inherited
					{Name: "--force"},   // own
				},
				Children: []*models.Node{
					{
						Name: "subsub",
						Flags: []models.Flag{
							{Name: "--verbose"}, // inherited
							{Name: "--only"},    // own
						},
					},
				},
			},
		},
	}

	models.MarkInheritedFlags(root)

	// Root flags are never inherited.
	for _, f := range root.Flags {
		if f.Inherited {
			t.Errorf("root flag %q should not be marked inherited", f.Name)
		}
	}

	// sub: --verbose and --config should be inherited, --force should not.
	sub := root.Children[0]
	for _, f := range sub.Flags {
		switch f.Name {
		case "--verbose", "--config":
			if !f.Inherited {
				t.Errorf("sub flag %q should be inherited", f.Name)
			}
		case "--force":
			if f.Inherited {
				t.Errorf("sub flag %q should NOT be inherited", f.Name)
			}
		}
	}

	// subsub: --verbose should be inherited, --only should not.
	subsub := sub.Children[0]
	for _, f := range subsub.Flags {
		switch f.Name {
		case "--verbose":
			if !f.Inherited {
				t.Errorf("subsub flag %q should be inherited", f.Name)
			}
		case "--only":
			if f.Inherited {
				t.Errorf("subsub flag %q should NOT be inherited", f.Name)
			}
		}
	}
}

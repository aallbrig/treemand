// Package models defines the core data structures for CLI command hierarchies.
package models

// Flag represents a CLI flag/option with its metadata.
type Flag struct {
	Name        string `json:"name"`
	ShortName   string `json:"short_name,omitempty"`
	ValueType   string `json:"value_type,omitempty"` // string, bool, int, etc.
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	// Inherited is set when this flag is also present on an ancestor node
	// (e.g. Cobra global flags propagated to every subcommand).
	Inherited bool `json:"inherited,omitempty"`
}

// Positional represents a positional argument in a CLI command.
type Positional struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Variadic    bool   `json:"variadic,omitempty"`
}

// Node represents a command or subcommand in a CLI hierarchy.
type Node struct {
	Name        string       `json:"name"`
	FullPath    []string     `json:"full_path"`
	Description string       `json:"description,omitempty"`
	Flags       []Flag       `json:"flags,omitempty"`
	Positionals []Positional `json:"positionals,omitempty"`
	Children    []*Node      `json:"children,omitempty"`
	HelpText    string       `json:"help_text,omitempty"`
	Discovered  bool         `json:"discovered"`
	// Virtual marks a display-only group node (e.g. a Godot flag section like
	// "run-options"). Virtual nodes organise flags visually but do not
	// produce command tokens in the preview bar.
	Virtual bool `json:"virtual,omitempty"`
}

// FullCommand returns the full command string (e.g., "git remote add").
func (n *Node) FullCommand() string {
	if len(n.FullPath) == 0 {
		return n.Name
	}
	result := ""
	for i, p := range n.FullPath {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}

// IsLeaf returns true if this node has no children.
func (n *Node) IsLeaf() bool {
	return len(n.Children) == 0
}

// HasFlags returns true if this node has flags.
func (n *Node) HasFlags() bool {
	return len(n.Flags) > 0
}

// HasPositionals returns true if this node has positional args.
func (n *Node) HasPositionals() bool {
	return len(n.Positionals) > 0
}

// Find searches for a child node by name.
func (n *Node) Find(name string) *Node {
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

// Walk calls fn for each node in the tree (depth-first pre-order).
func (n *Node) Walk(fn func(*Node)) {
	fn(n)
	for _, child := range n.Children {
		child.Walk(fn)
	}
}

// Clone returns a deep copy of the node.
func (n *Node) Clone() *Node {
	c := &Node{
		Name:        n.Name,
		FullPath:    make([]string, len(n.FullPath)),
		Description: n.Description,
		HelpText:    n.HelpText,
		Discovered:  n.Discovered,
	}
	copy(c.FullPath, n.FullPath)
	c.Flags = make([]Flag, len(n.Flags))
	copy(c.Flags, n.Flags)
	c.Positionals = make([]Positional, len(n.Positionals))
	copy(c.Positionals, n.Positionals)
	for _, child := range n.Children {
		c.Children = append(c.Children, child.Clone())
	}
	return c
}

// MarkInheritedFlags walks the tree and marks flags on child nodes that are
// also present on a direct ancestor as Inherited=true. This handles CLIs like
// Cobra-based tools where global flags propagate to every subcommand.
func MarkInheritedFlags(root *Node) {
	markInherited(root, map[string]bool{})
}

func markInherited(n *Node, ancestorFlags map[string]bool) {
	// Build the set for this node's own flags before marking.
	// A flag is "inherited" if its name appears in any ancestor.
	for i := range n.Flags {
		if ancestorFlags[n.Flags[i].Name] {
			n.Flags[i].Inherited = true
		}
	}
	// Pass down the combined ancestor+own flag set to children.
	combined := make(map[string]bool, len(ancestorFlags)+len(n.Flags))
	for k := range ancestorFlags {
		combined[k] = true
	}
	for _, f := range n.Flags {
		combined[f.Name] = true
	}
	for _, child := range n.Children {
		markInherited(child, combined)
	}
}

// Package models defines the core data structures for CLI command hierarchies.
package models

// Flag represents a CLI flag/option with its metadata.
type Flag struct {
	Name        string `json:"name"`
	ShortName   string `json:"short_name,omitempty"`
	ValueType   string `json:"value_type,omitempty"` // string, bool, int, etc.
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
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

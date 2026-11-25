package discovery

import (
	"regexp"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// scriptCommandNameRe is what a word must look like to be read as a
// subcommand name from a completion script.
var scriptCommandNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:+-]*$`)

// scriptTree builds the command tree a completion script describes.
type scriptTree struct {
	root *models.Node
}

func newScriptTree(cliName string) *scriptTree {
	return &scriptTree{root: &models.Node{Name: cliName, FullPath: []string{cliName}, Discovered: true}}
}

// at returns the node at path below the root, creating missing nodes.
func (t *scriptTree) at(path []string) *models.Node {
	n := t.root
	for _, name := range path {
		n = childNamed(n, name)
	}
	return n
}

func childNamed(n *models.Node, name string) *models.Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	full := make([]string, len(n.FullPath)+1)
	copy(full, n.FullPath)
	full[len(n.FullPath)] = name
	c := &models.Node{Name: name, FullPath: full}
	n.Children = append(n.Children, c)
	return c
}

// addCommand records a subcommand under path; a later description fills an
// empty one.
func (t *scriptTree) addCommand(path []string, name, desc string) {
	if !scriptCommandNameRe.MatchString(name) {
		return
	}
	c := childNamed(t.at(path), name)
	if c.Description == "" {
		c.Description = desc
	}
}

// addFlag records a flag at path unless one with its name is there already.
func (t *scriptTree) addFlag(path []string, f models.Flag) {
	n := t.at(path)
	for _, have := range n.Flags {
		if have.Name == f.Name {
			return
		}
	}
	n.Flags = append(n.Flags, f)
}

// node returns the tree, or nil when the script described nothing.
func (t *scriptTree) node() *models.Node {
	if len(t.root.Children) == 0 && len(t.root.Flags) == 0 {
		return nil
	}
	return t.root
}

// scriptFlag builds a flag from the long and short names a script gives.
func scriptFlag(long, short, desc string, takesValue bool) (models.Flag, bool) {
	f := models.Flag{Description: desc, ValueType: "bool"}
	if takesValue {
		f.ValueType = "string"
	}
	switch {
	case long != "":
		f.Name, f.ShortName = "--"+long, short
	case short != "":
		f.Name = "-" + short
	default:
		return f, false
	}
	return f, true
}

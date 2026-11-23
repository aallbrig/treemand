// Package overrides lets users correct how treemand reads a CLI without a new
// release: a YAML file per CLI in the config directory adds or replaces
// descriptions, adds commands and flags, and hides nodes.
//
// The file is <config dir>/treemand/overrides/<cli>.yaml, for example
// ~/.config/treemand/overrides/mytool.yaml:
//
//	commands:
//	  "":                  # the CLI itself
//	    description: My tool does things
//	    hide_flags: [--debug-internal]
//	  remote add:          # a command path below the CLI
//	    description: Add a remote
//	    flags:
//	      - name: --track  # added, or replaces the flag of the same name
//	        short: -t
//	        value_type: string
//	        description: Track one branch
//	  remote prune:
//	    hide: true
//	  config set:          # not in the tree: added, with its parents
//	    description: Set a value
package overrides

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/aallbrig/treemand/models"
)

// Override is one CLI's override file.
type Override struct {
	Commands map[string]Command `yaml:"commands"`

	// Path is the file it was read from, Hash a digest of its contents.
	Path string `yaml:"-"`
	Hash string `yaml:"-"`
}

// Command is what an override says about one command.
type Command struct {
	Description string   `yaml:"description"`
	Flags       []Flag   `yaml:"flags"`
	HideFlags   []string `yaml:"hide_flags"`
	Hide        bool     `yaml:"hide"`
}

// Flag is a flag an override adds or replaces.
type Flag struct {
	Name        string `yaml:"name"`
	Short       string `yaml:"short"`
	ValueType   string `yaml:"value_type"`
	Description string `yaml:"description"`
}

// Parse reads an override file's contents. Unknown fields are errors, so a
// typo does not silently do nothing.
func Parse(data []byte) (*Override, error) {
	o := &Override{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(o); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	for path, c := range o.Commands {
		for _, f := range c.Flags {
			if strings.TrimSpace(f.Name) == "" {
				return nil, fmt.Errorf("command %q: a flag has no name", path)
			}
		}
	}
	sum := sha256.Sum256(data)
	o.Hash = hex.EncodeToString(sum[:8])
	return o, nil
}

// LoadFrom reads the override for cli (a name or a path; its base name is
// used) from dir. No file is not an error: it returns nil, nil.
func LoadFrom(dir, cli string) (*Override, error) {
	name := strings.TrimSuffix(filepath.Base(cli), ".exe")
	if name == "" || name == "." || name == string(filepath.Separator) {
		return nil, nil
	}
	path := filepath.Join(dir, name+".yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("override %s: %w", path, err)
	}
	o, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("override %s: %w", path, err)
	}
	o.Path = path
	return o, nil
}

// Apply merges the override into the tree rooted at root and returns how many
// changes it made. Applying it again makes none, so a tree that already
// carries the override (from the cache) comes out the same.
func (o *Override) Apply(root *models.Node) int {
	if o == nil || root == nil {
		return 0
	}
	paths := make([]string, 0, len(o.Commands))
	for p := range o.Commands {
		paths = append(paths, p)
	}
	slices.Sort(paths) // parents before children, and a stable order
	changes := 0
	for _, p := range paths {
		changes += applyCommand(root, strings.Fields(p), o.Commands[p])
	}
	return changes
}

func applyCommand(root *models.Node, path []string, c Command) int {
	if c.Hide {
		return hide(root, path)
	}
	n, changes := ensure(root, path)
	if c.Description != "" && n.Description != c.Description {
		n.Description = c.Description
		changes++
	}
	for _, f := range c.Flags {
		changes += setFlag(n, f)
	}
	for _, name := range c.HideFlags {
		before := len(n.Flags)
		n.Flags = slices.DeleteFunc(n.Flags, func(f models.Flag) bool { return f.Name == name })
		changes += before - len(n.Flags)
	}
	return changes
}

// hide removes the command at path, if the tree has it.
func hide(root *models.Node, path []string) int {
	if len(path) == 0 {
		return 0 // the CLI itself cannot be hidden
	}
	parent := lookup(root, path[:len(path)-1])
	if parent == nil {
		return 0
	}
	before := len(parent.Children)
	parent.Children = slices.DeleteFunc(parent.Children, func(c *models.Node) bool { return c.Name == path[len(path)-1] })
	return before - len(parent.Children)
}

func lookup(n *models.Node, path []string) *models.Node {
	for _, name := range path {
		n = child(n, name)
		if n == nil {
			return nil
		}
	}
	return n
}

func child(n *models.Node, name string) *models.Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ensure returns the command at path, adding it and any missing parents. An
// added command counts as discovered: there is nothing to probe for it.
func ensure(root *models.Node, path []string) (*models.Node, int) {
	n, added := root, 0
	for _, name := range path {
		next := child(n, name)
		if next == nil {
			next = &models.Node{
				Name:       name,
				FullPath:   append(slices.Clone(n.FullPath), name),
				Discovered: true,
			}
			n.Children = append(n.Children, next)
			added++
		}
		n = next
	}
	return n, added
}

// setFlag adds f to n, or replaces the fields it sets on n's flag of that name.
func setFlag(n *models.Node, f Flag) int {
	want := models.Flag{Name: f.Name, ShortName: f.Short, ValueType: f.ValueType, Description: f.Description}
	i := slices.IndexFunc(n.Flags, func(x models.Flag) bool { return x.Name == f.Name })
	if i < 0 {
		n.Flags = append(n.Flags, want)
		return 1
	}
	old := n.Flags[i]
	merged := old
	if f.Short != "" {
		merged.ShortName = f.Short
	}
	if f.ValueType != "" {
		merged.ValueType = f.ValueType
	}
	if f.Description != "" {
		merged.Description = f.Description
	}
	if merged == old {
		return 0
	}
	n.Flags[i] = merged
	return 1
}

// Package presets holds per-CLI tuning for large, well-known command line
// interfaces (aws, stripe, ...) so that exploring them is fast and complete.
//
// A preset can lower the default discovery depth and can "seed" the tree with
// names that the CLI exposes cheaply (e.g. `stripe --map`), so fuzzy search
// covers every command before any per-command help has been fetched.
package presets

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aallbrig/treemand/models"
)

// Preset describes how to explore one well-known CLI.
type Preset struct {
	Name    string
	Summary string
	// Depth is the discovery depth used when the user did not pass --depth.
	// Zero means "no preset opinion".
	Depth int
	// Seed returns a tree of additional (usually stub) nodes rooted at the
	// CLI, or nil when none are available.
	Seed func(ctx context.Context, cli string) (*models.Node, error)
}

var registry = map[string]*Preset{
	"stripe": {
		Name:    "stripe",
		Summary: "Stripe CLI: full resource/operation map from `stripe --map`, details on demand",
		Depth:   1,
		Seed:    seedStripe,
	},
	"aws": {
		Name:    "aws",
		Summary: "AWS CLI: ~400 services as lazy stubs; commands load when a service is opened",
		Depth:   1,
	},
}

// Lookup returns the preset for a CLI name or path (basename is used).
func Lookup(cli string) (*Preset, bool) {
	p, ok := registry[strings.ToLower(filepath.Base(cli))]
	return p, ok
}

// Names lists registered preset names.
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	return out
}

// Graft merges seed children into root. Nodes already present in root keep
// their discovered data; missing nodes (and missing descendants) are added.
func Graft(root, seed *models.Node) {
	if root == nil || seed == nil {
		return
	}
	byName := make(map[string]*models.Node, len(root.Children))
	for _, c := range root.Children {
		byName[c.Name] = c
	}
	for _, sc := range seed.Children {
		if existing, ok := byName[sc.Name]; ok {
			Graft(existing, sc)
			continue
		}
		root.Children = append(root.Children, sc)
	}
}

func seedStripe(ctx context.Context, cli string) (*models.Node, error) {
	out, err := exec.CommandContext(ctx, cli, "--map").Output() //nolint:gosec
	if err != nil {
		return nil, err
	}
	return ParseTreeMap(string(out), filepath.Base(cli)), nil
}

// ParseTreeMap parses box-drawing tree output (├── └── │) such as
// `stripe --map` into a node tree. Leaves become undiscovered stubs so the
// TUI expands them on demand.
func ParseTreeMap(s, cli string) *models.Node {
	root := &models.Node{Name: cli, FullPath: []string{cli}}
	stack := []*models.Node{root} // stack[d] is the last node at depth d
	for _, line := range strings.Split(s, "\n") {
		name, depth, ok := treeLine(line)
		if !ok || depth < 1 || depth > len(stack) {
			continue
		}
		parent := stack[depth-1]
		n := &models.Node{
			Name:     name,
			FullPath: append(append([]string{}, parent.FullPath...), name),
		}
		parent.Children = append(parent.Children, n)
		stack = append(stack[:depth], n)
	}
	// Leaves have no known children yet: mark them for lazy discovery.
	root.Walk(func(n *models.Node) {
		if n != root && len(n.Children) == 0 {
			n.Stub = true
		}
	})
	return root
}

// treeLine extracts the node name and nesting depth from one tree line.
func treeLine(line string) (name string, depth int, ok bool) {
	for _, marker := range []string{"├── ", "└── "} {
		if i := strings.Index(line, marker); i >= 0 {
			prefix := []rune(line[:i])
			return strings.TrimSpace(line[i+len(marker):]), len(prefix)/4 + 1, true
		}
	}
	return "", 0, false
}

// Package discovery - merge multiple discoverer results into one tree.
package discovery

import (
	"context"

	"github.com/aallbrig/treemand/models"
)

// Merge combines results from multiple discoverers into a single tree.
// Later discoverers fill in gaps from earlier ones.
func Merge(trees []*models.Node) *models.Node {
	if len(trees) == 0 {
		return nil
	}
	result := trees[0].Clone()
	for _, t := range trees[1:] {
		mergeInto(result, t)
	}
	return result
}

func mergeInto(dst, src *models.Node) {
	if src == nil {
		return
	}
	if dst.Description == "" {
		dst.Description = src.Description
	}
	if dst.HelpText == "" {
		dst.HelpText = src.HelpText
	}

	// Merge flags (deduplicate by name)
	flagSet := map[string]bool{}
	for _, f := range dst.Flags {
		flagSet[f.Name] = true
	}
	for _, f := range src.Flags {
		if !flagSet[f.Name] {
			dst.Flags = append(dst.Flags, f)
		}
	}

	// Merge positionals (deduplicate by name)
	posSet := map[string]bool{}
	for _, p := range dst.Positionals {
		posSet[p.Name] = true
	}
	for _, p := range src.Positionals {
		if !posSet[p.Name] {
			dst.Positionals = append(dst.Positionals, p)
		}
	}

	// Merge children
	for _, srcChild := range src.Children {
		found := false
		for _, dstChild := range dst.Children {
			if dstChild.Name == srcChild.Name {
				mergeInto(dstChild, srcChild)
				found = true
				break
			}
		}
		if !found {
			dst.Children = append(dst.Children, srcChild.Clone())
		}
	}
}

// Run executes all discoverers and merges their results.
func Run(ctx context.Context, discoverers []Discoverer, cliName string) (*models.Node, error) {
	if len(discoverers) == 0 {
		d := NewHelpDiscoverer(-1)
		return d.Discover(ctx, cliName, nil)
	}

	var trees []*models.Node
	var lastErr error
	for _, d := range discoverers {
		tree, err := d.Discover(ctx, cliName, nil)
		if err != nil {
			lastErr = err
			continue
		}
		trees = append(trees, tree)
	}
	if len(trees) == 0 {
		return nil, lastErr
	}
	return Merge(trees), nil
}

// BuildDiscoverers creates Discoverer instances from strategy names.
func BuildDiscoverers(strategies []string, maxDepth int) []Discoverer {
	var result []Discoverer
	for _, s := range strategies {
		switch s {
		case "help":
			result = append(result, NewHelpDiscoverer(maxDepth))
		// Future: case "completions": result = append(result, NewCompletionsDiscoverer())
		}
	}
	if len(result) == 0 {
		result = append(result, NewHelpDiscoverer(maxDepth))
	}
	return result
}

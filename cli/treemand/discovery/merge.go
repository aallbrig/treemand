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

	dst.Flags = mergeFlags(dst.Flags, src.Flags)

	dst.Positionals = mergePositionals(dst.Positionals, src.Positionals)

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

// mergeFlags appends the flags of src that dst lacks. Flags are the same
// when their names match, or when a short-only "-v" is the short form of a
// "--verbose": whichever side has it, the two become one --verbose flag.
func mergeFlags(dst, src []models.Flag) []models.Flag {
	at := map[string]int{} // a flag's name, and "-" + its short name -> index in dst
	index := func(i int) {
		at[dst[i].Name] = i
		if dst[i].ShortName != "" {
			at["-"+dst[i].ShortName] = i
		}
	}
	for i := range dst {
		index(i)
	}
	for _, f := range src {
		if _, ok := at[f.Name]; ok {
			continue
		}
		if i, ok := at["-"+f.ShortName]; ok && f.ShortName != "" {
			dst[i] = foldShort(dst[i], f)
			index(i)
			continue
		}
		dst = append(dst, f)
		index(len(dst) - 1)
	}
	return dst
}

// foldShort returns have with long's name when have is the short-only form
// of long, keeping have's description and value type, which came first.
func foldShort(have, long models.Flag) models.Flag {
	if have.Name != "-"+long.ShortName {
		return have // a different long flag already owns this short name
	}
	if have.Description != "" {
		long.Description = have.Description
	}
	if have.ValueType != "" {
		long.ValueType = have.ValueType
	}
	return long
}

// mergePositionals appends the positionals of src that dst lacks by name.
func mergePositionals(dst, src []models.Positional) []models.Positional {
	have := map[string]bool{}
	for _, p := range dst {
		have[p.Name] = true
	}
	for _, p := range src {
		if !have[p.Name] {
			dst = append(dst, p)
		}
	}
	return dst
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
		if tree != nil {
			trees = append(trees, tree)
		}
	}
	if len(trees) == 0 {
		return nil, lastErr
	}
	return Merge(trees), nil
}

// BuildDiscoverers creates Discoverer instances from strategy names.
func BuildDiscoverers(strategies []string, maxDepth int) []Discoverer {
	return BuildDiscoverersWithThreshold(strategies, maxDepth, 150)
}

// BuildDiscoverersWithThreshold creates Discoverer instances with a configurable stub threshold.
func BuildDiscoverersWithThreshold(strategies []string, maxDepth, stubThreshold int) []Discoverer {
	var result []Discoverer
	for _, s := range strategies {
		switch s {
		case "help":
			d := NewHelpDiscoverer(maxDepth)
			if stubThreshold > 0 {
				d.StubThreshold = stubThreshold
			}
			result = append(result, d)
		case "man":
			result = append(result, NewManDiscoverer())
		case "completions":
			result = append(result, NewCompletionsDiscoverer())
		case "scripts":
			result = append(result, NewScriptsDiscoverer())
		}
	}
	if len(result) == 0 {
		d := NewHelpDiscoverer(maxDepth)
		if stubThreshold > 0 {
			d.StubThreshold = stubThreshold
		}
		result = append(result, d)
	}
	return result
}

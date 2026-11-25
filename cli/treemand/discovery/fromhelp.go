package discovery

import (
	"github.com/aallbrig/treemand/cli/treemand/models"
)

// FromHelpText builds the tree a help text describes without running
// anything: the root is cliName, parsed from text, and every subcommand the
// help lists is an undiscovered stub (its own help was never read). It backs
// `treemand --stdin`, for a help text saved, copied from another machine, or
// printed by a CLI treemand cannot execute.
func FromHelpText(cliName, text string) *models.Node {
	parsed := ParseHelpOutputFor(text, cliName)
	fullPath := []string{cliName}
	node := &models.Node{
		Name:        cliName,
		FullPath:    fullPath,
		Discovered:  true,
		HelpText:    text,
		Description: parsed.Description,
		Positionals: parsed.Positionals,
		Flags:       parsed.Flags,
	}
	if len(parsed.Subcommands) > 0 {
		node.Children = stubChildren(fullPath, parsed, nil)
	}
	models.MarkInheritedFlags(node)
	return node
}

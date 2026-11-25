package cmd

import (
	"github.com/rs/zerolog/log"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/overrides"
)

// loadOverride reads the user's override file for cliName, or returns nil
// when there is none. A file that cannot be read or parsed is an error: an
// override that silently does nothing would be worse.
func loadOverride(cliName string) (*overrides.Override, error) {
	dir, err := config.OverridesDir()
	if err != nil {
		// No home or config directory to read from: run without an override.
		log.Debug().Err(err).Msg("no config directory for overrides")
		return nil, nil
	}
	return overrides.LoadFrom(dir, cliName)
}

// overrideKey is the cache key part for an override: editing the file
// changes it, so the next run does not show a tree built with the old one.
func overrideKey(o *overrides.Override) string {
	if o == nil {
		return ""
	}
	return ",o=" + o.Hash
}

// overrideFunc returns the hook that re-applies o after discovery merges a
// node into the tree, or nil when there is no override.
func overrideFunc(o *overrides.Override) func(*models.Node) {
	if o == nil {
		return nil
	}
	return func(root *models.Node) { o.Apply(root) }
}

// applyOverride merges o into the tree and logs how much it changed.
func applyOverride(o *overrides.Override, node *models.Node) int {
	n := o.Apply(node)
	if o != nil {
		log.Debug().Str("override", o.Path).Int("changes", n).Msg("override applied")
	}
	return n
}

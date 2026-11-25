package cmd

import (
	"github.com/rs/zerolog/log"

	"github.com/aallbrig/treemand/cli/treemand/config"
	"github.com/aallbrig/treemand/cli/treemand/models"
	"github.com/aallbrig/treemand/cli/treemand/overrides"
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

// overrideFunc returns the hook that re-applies o after discovery merges a
// node into the tree, or nil when there is no override.
func overrideFunc(o *overrides.Override) func(*models.Node) {
	if o == nil {
		return nil
	}
	return func(root *models.Node) { o.Apply(root) }
}

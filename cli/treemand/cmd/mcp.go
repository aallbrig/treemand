package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/mcp"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/presets"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run an MCP server on stdio that gives AI agents CLI discovery as tools",
	Long: `Run a Model Context Protocol (MCP) server on stdin and stdout, so an AI agent
can map command-line tools without parsing --help text itself.

Tools:
  discover   the command tree of a CLI (the JSON that 'treemand schema' describes)
  search     fuzzy-search a CLI's commands, flags and arguments
  assemble   build a shell-quoted command line, checked against the CLI's help

The server never runs a discovered command. The only processes it starts are
the same --help probes 'treemand <cli>' makes, for commands on PATH named by
bare name. Each tool call is limited by --timeout, and trees are cached like
any other run (--no-cache turns that off). Logs go to stderr.

Register it with an MCP client by giving the command 'treemand mcp'.`,
	Example: `  claude mcp add treemand -- treemand mcp
  # or in a client's JSON config:
  #   "treemand": {"command": "treemand", "args": ["mcp"]}`,
	Args: cobra.NoArgs,
	RunE: runMCP,
}

func runMCP(cmd *cobra.Command, _ []string) error {
	setupLogging()
	configureDiscovery()
	// No signal handler: SIGINT and SIGTERM end the process at once, even
	// while it waits on stdin.
	s := &mcp.Server{
		Name:     "treemand",
		Version:  Version,
		Discover: mcpDiscover,
		Expand:   mcpExpand,
		Timeout:  time.Duration(cfgTimeout) * time.Second,
	}
	return s.Serve(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
}

// mcpDiscover discovers cli to depth through the same cache as a CLI run.
func mcpDiscover(ctx context.Context, cli string, depth int) (*models.Node, error) {
	if err := discovery.CheckAvailable(cli); err != nil {
		return nil, err
	}
	cfg := config.DefaultConfig()
	config.ApplyViper(cfg)
	cfg.NoCache = cfgNoCache || cfg.NoCache
	cfg.Depth = depth
	strategies := []string{"help"}
	preset, ok := presets.Lookup(cli)
	if !ok {
		preset = nil
	}
	rc := openRunCache(cfg, cli, strategies, cacheKeyExtra(cfg, preset))
	if rc != nil {
		defer rc.c.Close()
	}
	if node := rc.cached(cfg); node != nil {
		if save := rc.saver(); save != nil {
			finishPartialTree(ctx, node, cli, cfg, save)
		}
		return node, nil
	}
	versionCh := rc.readVersion()
	node, err := discovery.Run(ctx, discovery.BuildDiscoverersWithThreshold(strategies, depth, cfg.StubThreshold), cli)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}
	if node == nil {
		return nil, fmt.Errorf("no results from discovery for %q", cli)
	}
	if preset != nil && preset.Seed != nil {
		seedFromPreset(ctx, node, cli, preset)
	}
	rc.put(node, versionCh)
	return node, nil
}

// mcpExpand discovers one stub node in place, one level deep.
func mcpExpand(ctx context.Context, cli string, node *models.Node) error {
	if len(node.FullPath) < 2 {
		return nil // the root is always discovered
	}
	d := discovery.NewHelpDiscoverer(1)
	d.StubThreshold = 1 // its children stay name-only stubs
	res, err := d.Discover(ctx, cli, node.FullPath[1:])
	if err != nil {
		return err
	}
	if res == nil {
		return nil
	}
	node.Stub, node.Discovered = false, true
	if len(node.Children) == 0 {
		node.Children = res.Children
	}
	if node.Description == "" {
		node.Description = res.Description
	}
	node.Flags, node.Positionals = res.Flags, res.Positionals
	node.HelpText, node.DiscoveryErr = res.HelpText, res.DiscoveryErr
	return nil
}

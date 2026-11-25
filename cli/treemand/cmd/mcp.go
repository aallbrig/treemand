package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/mcp"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/presets"
	"github.com/aallbrig/treemand/treeload"
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
bare name. Each tool call is limited by --timeout (0 for no limit), and trees
are cached like any other run (--no-cache turns that off). Logs go to stderr.

Register it with an MCP client by giving the command 'treemand mcp'.`,
	Example: `  claude mcp add treemand -- treemand mcp
  # or in a client's JSON config:
  #   "treemand": {"command": "treemand", "args": ["mcp"]}`,
	Args: noArgs,
	RunE: runMCP,
}

func runMCP(cmd *cobra.Command, _ []string) error {
	setupLogging()
	if err := checkTimeout(cfgTimeout); err != nil {
		return err
	}
	configureDiscovery()
	timeout := time.Duration(cfgTimeout) * time.Second
	if cfgTimeout == 0 {
		timeout = -1 // the server reads a negative timeout as no limit
	}
	// No signal handler: SIGINT and SIGTERM end the process at once, even
	// while it waits on stdin.
	s := &mcp.Server{
		Name:     "treemand",
		Version:  Version,
		Discover: mcpDiscover,
		Expand:   mcpExpand,
		Timeout:  timeout,
	}
	return s.Serve(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
}

// mcpDiscover loads a tool call's tree through the same Tree load as a CLI
// run, so the CLI and an agent get the same tree for the same request.
func mcpDiscover(ctx context.Context, tr mcp.TreeRequest) (*models.Node, error) {
	if err := discovery.CheckAvailable(tr.CLI); err != nil {
		return nil, err
	}
	cfg := config.DefaultConfig()
	config.ApplyViper(cfg)
	ov, err := loadOverride(tr.CLI)
	if err != nil {
		return nil, err
	}
	res, err := treeload.Load(ctx, treeload.Request{
		CLI:           tr.CLI,
		Depth:         tr.Depth,
		StubThreshold: cfg.StubThreshold,
		Strategies:    []string{"help"},
		Preset:        lookupPreset(tr.CLI),
		Override:      ov,
		Cache:         cachePolicy(cfgNoCache || cfg.NoCache, tr.Refresh),
		CacheDir:      cfg.CacheDir,
		CacheTTL:      cfg.CacheTTL,
	})
	return res.Tree, err
}

// mcpExpand discovers one stub node of root's tree in place, one level deep.
func mcpExpand(ctx context.Context, cli string, root, node *models.Node) error {
	if len(node.FullPath) < 2 {
		return nil // the root is always discovered
	}
	ov, err := loadOverride(cli)
	if err != nil {
		return err
	}
	return discovery.NewNodeExpander(cli, discovery.ExpandOptions{
		EagerBelow: discovery.OneLevel, MaxDepth: -1, Override: overrideFunc(ov), HelpSubcommand: presets.HelpSubcommand(cli),
	}).
		Expand(ctx, root, node)
}

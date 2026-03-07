// Package cmd implements the treemand CLI commands.
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aallbrig/treemand/cache"
	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/render"
	"github.com/aallbrig/treemand/tui"
)

var (
	cfgFile         string
	cfgInteractive  bool
	cfgStrategy     string
	cfgDepth        int
	cfgFilter       string
	cfgExclude      string
	cfgCommandsOnly bool
	cfgFullPath     bool
	cfgOutput       string
	cfgNoColor      bool
	cfgNoCache      bool
	cfgTimeout      int
	cfgDebug        bool
	cfgIcons        string
	cfgLineLength   int
	cfgStubThreshold int
)

// rootCmd is the cobra root command.
var rootCmd = &cobra.Command{
	Use:   "treemand <cli>",
	Short: "Visualize CLI command hierarchies as a tree",
	Long: `treemand discovers and visualizes any CLI tool as a command tree.

Point it at any binary and it maps out subcommands, flags, and positionals
by probing the tool's own --help output — no plugins, no config files.

  treemand git            prints a colored ASCII tree of git's commands
  treemand -i aws         opens an interactive TUI to explore aws

Non-interactive output includes inline flags, positional arguments, and
short descriptions. Large CLIs (aws, kubectl) create stub nodes on first
run — use -i to expand them on demand, or increase --depth.

Interactive TUI controls (press ? inside TUI for full help):
  ↑↓ / j k    navigate tree        Space / Enter  add node to command
  h H          toggle help pane     f              pick a flag
  /            fuzzy filter         Ctrl+E         copy / execute
  Esc          quit

Discovery strategies (--strategy):
  help          parse --help output (default, works on nearly every CLI)
  completions   use shell completion data (richer flag metadata)

Output formats (--output):
  text          colored tree (default)
  json          machine-readable full tree with flags and descriptions

Examples:
  treemand git                        # full git tree
  treemand -i aws                     # interactive aws explorer
  treemand --depth=2 kubectl          # kubectl tree, 2 levels deep
  treemand --commands-only docker     # subcommands only, no flags
  treemand --output=json gh | jq .    # pipe JSON to jq
  treemand --filter=remote git        # only show nodes matching "remote"
  treemand treemand                   # introspect treemand itself

Docs: https://aallbrig.github.io/treemand`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runRoot,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default: ~/.config/treemand/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&cfgInteractive, "interactive", "i", false, "Launch interactive TUI")
	rootCmd.PersistentFlags().StringVarP(&cfgStrategy, "strategy", "s", "help", "Discovery strategies (comma-separated: help,completions)")
	rootCmd.PersistentFlags().IntVar(&cfgDepth, "depth", -1, "Max tree depth (-1 = unlimited)")
	rootCmd.PersistentFlags().StringVar(&cfgFilter, "filter", "", "Only show nodes matching pattern")
	rootCmd.PersistentFlags().StringVar(&cfgExclude, "exclude", "", "Exclude nodes matching pattern")
	rootCmd.PersistentFlags().BoolVar(&cfgCommandsOnly, "commands-only", false, "Hide flags and positionals")
	rootCmd.PersistentFlags().BoolVar(&cfgFullPath, "full-path", false, "Show full command paths")
	rootCmd.PersistentFlags().StringVar(&cfgOutput, "output", "text", "Output format: text, json")
	rootCmd.PersistentFlags().BoolVar(&cfgNoColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVar(&cfgNoCache, "no-cache", false, "Disable caching")
	rootCmd.PersistentFlags().IntVar(&cfgTimeout, "timeout", 30, "Discovery timeout in seconds")
	rootCmd.PersistentFlags().BoolVar(&cfgDebug, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVar(&cfgIcons, "icons", "", "Icon preset: unicode (default), ascii, nerd")
	rootCmd.PersistentFlags().IntVar(&cfgLineLength, "line-length", 0, "Max description chars before truncation (default 80)")
	rootCmd.PersistentFlags().IntVar(&cfgStubThreshold, "stub-threshold", 0, "Max eager children before creating stubs (default 50)")

	_ = viper.BindPFlag("icons", rootCmd.PersistentFlags().Lookup("icons"))
	_ = viper.BindPFlag("desc_line_length", rootCmd.PersistentFlags().Lookup("line-length"))
	_ = viper.BindPFlag("stub_threshold", rootCmd.PersistentFlags().Lookup("stub-threshold"))
	_ = viper.BindPFlag("no_color", rootCmd.PersistentFlags().Lookup("no-color"))
}

func initConfig() {
	if err := config.InitViper(cfgFile); err != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not read config file:", err)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	logLevel := zerolog.WarnLevel
	if cfgDebug {
		logLevel = zerolog.DebugLevel
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(logLevel)

	cliName := args[0]

	// Fail early with a clear message if the binary cannot be found.
	if err := discovery.CheckAvailable(cliName); err != nil {
		return fmt.Errorf("%w\nHint: check spelling and ensure the command is on your PATH", err)
	}

	cfg := config.DefaultConfig()
	cfg.NoColor = cfgNoColor || cfg.NoColor
	cfg.Depth = cfgDepth
	cfg.NoCache = cfgNoCache
	// Apply viper-loaded config file values (flags > env > file > defaults).
	config.ApplyViper(cfg)
	// CLI flags override config file values when explicitly set.
	if cfgIcons != "" {
		cfg.IconPreset = cfgIcons
		cfg.Icons = config.IconSetForPreset(cfgIcons)
	}
	if cfgLineLength > 0 {
		cfg.DescLineLength = cfgLineLength
	}
	if cfgStubThreshold > 0 {
		cfg.StubThreshold = cfgStubThreshold
	}
	strategies := config.ParseStrategies(cfgStrategy)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfgTimeout)*time.Second)
	defer cancel()

	// Attempt cache lookup
	var (
		cacheInst *cache.Cache
		cacheKey  string
	)
	if !cfg.NoCache {
		var err error
		cacheInst, err = cache.Open(cfg.CacheDir)
		if err != nil {
			log.Warn().Err(err).Msg("could not open cache, running without")
		} else {
			defer cacheInst.Close()
			ver := cache.CLIVersion(cliName)
			cacheKey = cache.Key(cliName, ver, strategies)
			if node, err := cacheInst.Get(cacheKey, 24*time.Hour); err == nil && node != nil {
				log.Debug().Str("cli", cliName).Msg("cache hit")
				return output(cmd, node, cfg)
			}
		}
	}

	// Discover tree
	maxDepth := cfg.Depth
	if maxDepth < 0 {
		maxDepth = 3
	}
	discoverers := discovery.BuildDiscoverersWithThreshold(strategies, maxDepth, cfg.StubThreshold)
	node, err := discovery.Run(ctx, discoverers, cliName)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}
	if node == nil {
		return fmt.Errorf("no results from discovery for %q", cliName)
	}

	// Persist to cache
	if cacheInst != nil && cacheKey != "" {
		ver := cache.CLIVersion(cliName)
		if putErr := cacheInst.Put(cacheKey, cliName, ver, cfgStrategy, node); putErr != nil {
			log.Warn().Err(putErr).Msg("cache write failed")
		}
	}

	return output(cmd, node, cfg)
}

func output(cmd *cobra.Command, node *models.Node, cfg *config.Config) error {
	if cfgInteractive {
		return tui.Run(node, cfg)
	}
	opts := render.Options{
		MaxDepth:       cfgDepth,
		Filter:         cfgFilter,
		Exclude:        cfgExclude,
		CommandsOnly:   cfgCommandsOnly,
		FullPath:       cfgFullPath,
		Output:         cfgOutput,
		NoColor:        cfg.NoColor,
		Colors:         cfg.Colors,
		Icons:          cfg.Icons,
		DescLineLength: cfg.DescLineLength,
	}
	r := render.New(opts)
	return r.Render(cmd.OutOrStdout(), node)
}

// Execute runs the root command.
func Execute() {
	// Wire --version flag to show the same string as `treemand version`.
	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cacheCmd)
	rootCmd.AddCommand(genDocsCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// NewRootCmd returns a fresh root command for testing (flags reset to defaults).
func NewRootCmd() *cobra.Command {
	c := &cobra.Command{
		Use:           rootCmd.Use,
		Short:         rootCmd.Short,
		Long:          rootCmd.Long,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runRoot,
	}
	c.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file")
	c.PersistentFlags().BoolVarP(&cfgInteractive, "interactive", "i", false, "Launch interactive TUI")
	c.PersistentFlags().StringVarP(&cfgStrategy, "strategy", "s", "help", "Discovery strategies")
	c.PersistentFlags().IntVar(&cfgDepth, "depth", -1, "Max tree depth")
	c.PersistentFlags().StringVar(&cfgFilter, "filter", "", "Filter pattern")
	c.PersistentFlags().StringVar(&cfgExclude, "exclude", "", "Exclude pattern")
	c.PersistentFlags().BoolVar(&cfgCommandsOnly, "commands-only", false, "Hide flags/positionals")
	c.PersistentFlags().BoolVar(&cfgFullPath, "full-path", false, "Full command paths")
	c.PersistentFlags().StringVar(&cfgOutput, "output", "text", "Output format")
	c.PersistentFlags().BoolVar(&cfgNoColor, "no-color", false, "Disable color")
	c.PersistentFlags().BoolVar(&cfgNoCache, "no-cache", false, "Disable cache")
	c.PersistentFlags().IntVar(&cfgTimeout, "timeout", 5, "Discovery timeout")
	c.PersistentFlags().BoolVar(&cfgDebug, "debug", false, "Debug logging")
	c.PersistentFlags().StringVar(&cfgIcons, "icons", "", "Icon preset")
	c.PersistentFlags().IntVar(&cfgLineLength, "line-length", 0, "Max description line length")
	c.PersistentFlags().IntVar(&cfgStubThreshold, "stub-threshold", 0, "Stub threshold")
	c.AddCommand(versionCmd)
	return c
}

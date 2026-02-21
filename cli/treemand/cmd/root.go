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

"github.com/aallbrig/treemand/cache"
"github.com/aallbrig/treemand/config"
"github.com/aallbrig/treemand/discovery"
"github.com/aallbrig/treemand/models"
"github.com/aallbrig/treemand/render"
"github.com/aallbrig/treemand/tui"
)

var (
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
)

// rootCmd is the cobra root command.
var rootCmd = &cobra.Command{
Use:   "treemand <cli>",
Short: "Visualize CLI command hierarchies as a tree",
Long: `treemand discovers and visualizes CLI command hierarchies.

Examples:
  treemand git                  # Non-interactive tree for git
  treemand -i aws               # Interactive TUI for aws
  treemand --depth=2 kubectl    # Tree limited to 2 levels
  treemand --output=json git    # JSON output`,
Args:          cobra.ExactArgs(1),
SilenceErrors: true,
SilenceUsage:  true,
RunE:          runRoot,
}

func init() {
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
}

func runRoot(cmd *cobra.Command, args []string) error {
logLevel := zerolog.WarnLevel
if cfgDebug {
logLevel = zerolog.DebugLevel
}
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(logLevel)

cliName := args[0]
cfg := config.DefaultConfig()
cfg.NoColor = cfgNoColor || cfg.NoColor
cfg.Depth = cfgDepth
cfg.NoCache = cfgNoCache
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
discoverers := discovery.BuildDiscoverers(strategies, maxDepth)
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
MaxDepth:     cfgDepth,
Filter:       cfgFilter,
Exclude:      cfgExclude,
CommandsOnly: cfgCommandsOnly,
FullPath:     cfgFullPath,
Output:       cfgOutput,
NoColor:      cfg.NoColor,
Colors:       cfg.Colors,
}
r := render.New(opts)
return r.Render(cmd.OutOrStdout(), node)
}

// Execute runs the root command.
func Execute() {
rootCmd.AddCommand(versionCmd)
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
c.AddCommand(versionCmd)
return c
}

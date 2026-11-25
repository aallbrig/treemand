// Package cmd implements the treemand CLI commands.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aallbrig/treemand/cli/treemand/cache"
	"github.com/aallbrig/treemand/cli/treemand/config"
	"github.com/aallbrig/treemand/cli/treemand/discovery"
	"github.com/aallbrig/treemand/cli/treemand/models"
	"github.com/aallbrig/treemand/cli/treemand/overrides"
	"github.com/aallbrig/treemand/cli/treemand/presets"
	"github.com/aallbrig/treemand/cli/treemand/render"
	"github.com/aallbrig/treemand/cli/treemand/treeload"
	"github.com/aallbrig/treemand/cli/treemand/tui"
)

var (
	cfgFile          string
	cfgInteractive   bool
	cfgStrategy      string
	cfgDepth         int
	cfgFilter        string
	cfgExclude       string
	cfgCommandsOnly  bool
	cfgFullPath      bool
	cfgOutput        string
	cfgNoColor       bool
	cfgNoCache       bool
	cfgRefresh       bool
	cfgTimeout       int
	cfgExpandStubs   int
	cfgJobs          int
	cfgDebug         bool
	cfgIcons         string
	cfgLineLength    int
	cfgStubThreshold int
	cfgTreeStyle     string
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
  man           parse the man page
  scripts       read the installed fish/zsh/bash completion script (never run)

Output formats (--output):
  text          colored tree (default)
  json          machine-readable full tree with flags and descriptions
  yaml          YAML output (same structure as JSON)
                (treemand schema prints the JSON Schema both follow)
  html          one self-contained page: collapsible tree, search box,
                no network (share it in a wiki or PR)

Examples:
  treemand git                        # full git tree
  treemand -i aws                     # interactive aws explorer
  treemand --depth=2 kubectl          # kubectl tree, 2 levels deep
  treemand --commands-only docker     # subcommands only, no flags
  treemand --output=json gh | jq .    # pipe JSON to jq
  treemand --output=html git > git.html  # shareable tree page
  treemand --filter=remote git        # only show nodes matching "remote"
  docker --help | treemand --stdin docker  # parse a help text, run nothing
  treemand treemand                   # introspect treemand itself
  treemand mcp                        # MCP server: discovery as AI agent tools

Docs: https://treemand.dev`,
	Args:          rootArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runRoot,
}

func init() {
	cobra.OnInitialize(initConfig)

	defineRootFlags(rootCmd)

	for _, b := range configFlags {
		_ = viper.BindPFlag(b.key, rootCmd.PersistentFlags().Lookup(b.flag))
	}
}

// configFlags pairs each config key a root flag sets with that flag.
var configFlags = []struct{ key, flag string }{
	{"icons", "icons"},
	{"desc_line_length", "line-length"},
	{"stub_threshold", "stub-threshold"},
	{"no_color", "no-color"},
	{"tree_style", "tree-style"},
	{"depth", "depth"},
	{"strategies", "strategy"},
	{"no_cache", "no-cache"},
}

// defineRootFlags defines the root command's flags on c. Production and the
// test command factory share it, so their defaults cannot drift.
func defineRootFlags(c *cobra.Command) {
	c.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default: ~/.config/treemand/config.yaml)")
	c.PersistentFlags().BoolVarP(&cfgInteractive, "interactive", "i", false, "Launch interactive TUI")
	c.PersistentFlags().StringVarP(&cfgStrategy, "strategy", "s", "help", "Discovery strategies (comma-separated: help,completions,man,scripts)")
	c.PersistentFlags().IntVar(&cfgDepth, "depth", 3, "Max tree depth (default 3; -1 = unlimited)")
	c.PersistentFlags().StringVar(&cfgFilter, "filter", "", "Only show nodes matching pattern")
	c.PersistentFlags().StringVar(&cfgExclude, "exclude", "", "Exclude nodes matching pattern")
	c.PersistentFlags().BoolVar(&cfgCommandsOnly, "commands-only", false, "Hide flags and positionals")
	c.PersistentFlags().BoolVar(&cfgFullPath, "full-path", false, "Show full command paths")
	c.PersistentFlags().StringVar(&cfgOutput, "output", "text", "Output format: text, json, yaml, html")
	c.PersistentFlags().BoolVar(&cfgNoColor, "no-color", false, "Disable color output")
	c.PersistentFlags().BoolVar(&cfgNoCache, "no-cache", false, "Disable caching")
	c.Flags().BoolVar(&cfgRefresh, "refresh", false, "Re-discover the CLI even if it is cached, and update the cache")
	c.Flags().BoolVar(&cfgStdin, "stdin", false, "Read the CLI's help text from stdin instead of running it (subcommands stay undiscovered)")
	c.Flags().StringVar(&cfgPreset, "preset", "", "Open the TUI with this saved preset loaded (see treemand presets)")
	c.PersistentFlags().IntVar(&cfgJobs, "jobs", 0, "Max concurrent help processes during discovery (0 = auto: 2 x CPUs, between 4 and 16)")
	c.PersistentFlags().IntVar(&cfgExpandStubs, "expand-stubs", 0, "Expand undiscovered (stub, marked (…)) nodes N levels deep before output: gets their descriptions and children (slow on huge CLIs; disables cache)")
	c.PersistentFlags().IntVar(&cfgTimeout, "timeout", 30, "Discovery timeout in seconds (0 = no limit; each help command still has its own limit)")
	c.PersistentFlags().BoolVar(&cfgDebug, "debug", false, "Enable debug logging")
	c.PersistentFlags().StringVar(&cfgIcons, "icons", "", "Icon preset: unicode (default), ascii, nerd")
	c.PersistentFlags().IntVar(&cfgLineLength, "line-length", 0, "Max description chars before truncation (default 80)")
	c.PersistentFlags().IntVar(&cfgStubThreshold, "stub-threshold", 0, "Max eager children before creating stubs (default 150)")
	c.PersistentFlags().StringVar(&cfgTreeStyle, "tree-style", "default", "TUI tree presentation style: default, columns, compact, graph")
}

func initConfig() {
	if err := config.InitViper(cfgFile); err != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not read config file:", err)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	setupLogging()

	cliName := args[0]
	// A mistake in how treemand was invoked is reported before any probing.
	if err := checkOutputFormat(cfgOutput); err != nil {
		return err
	}
	if err := checkTimeout(cfgTimeout); err != nil {
		return err
	}
	if err := resolvePreset(cliName); err != nil {
		return err
	}
	if cfgStdin {
		return runFromStdin(cmd, cliName)
	}
	configureDiscovery()

	// Fail early with a clear message if the binary cannot be found.
	if err := checkTarget(cmd.Root(), cliName); err != nil {
		return err
	}

	cfg := configFromFlags()
	req, err := loadRequest(cmd, cliName, cfg)
	if err != nil {
		return err
	}
	ctx, cancel := discoveryContext(cfgTimeout)
	defer cancel()
	res, err := treeload.Load(ctx, req)
	if err != nil {
		return err
	}
	return output(cmd, res.Tree, cfg, res.Save, req.Override)
}

// loadRequest reads the run's flags into a tree load request for cliName:
// its strategies, preset (whose depth applies unless --depth was given),
// override and cache policy.
func loadRequest(cmd *cobra.Command, cliName string, cfg *config.Config) (treeload.Request, error) {
	ov, err := loadOverride(cliName)
	if err != nil {
		return treeload.Request{}, err
	}
	preset := lookupPreset(cliName)
	applyPresetDepth(cmd, cfg, preset)
	req := treeload.Request{
		CLI:           cliName,
		Depth:         cfg.Depth,
		StubThreshold: cfg.StubThreshold,
		Strategies:    config.ParseStrategies(cfgStrategy),
		Preset:        preset,
		Override:      ov,
		Cache:         cachePolicy(cfg.NoCache, cfgRefresh),
		CacheDir:      cfg.CacheDir,
		CacheTTL:      cfg.CacheTTL,
		Crawl:         cfgInteractive,
		ExpandStubs:   cfgExpandStubs,
		Progress:      withSpinner,
	}
	return req, nil
}

// lookupPreset is the built-in preset for cliName, or nil.
func lookupPreset(cliName string) *presets.Preset {
	if p, ok := presets.Lookup(cliName); ok {
		return p
	}
	return nil
}

// cachePolicy is the cache policy for --no-cache and --refresh.
func cachePolicy(noCache, refresh bool) treeload.CachePolicy {
	switch {
	case noCache:
		return treeload.Bypass
	case refresh:
		return treeload.Refresh
	}
	return treeload.UseCache
}

// checkAvailable reports a CLI treemand must not or cannot run. A missing
// CLI gets a hint; a refused one already says what to do instead.
func checkAvailable(cliName string) error {
	err := discovery.CheckAvailable(cliName)
	var never *discovery.NeverProbeError
	if err == nil || errors.As(err, &never) {
		return err
	}
	return fmt.Errorf("%w\nHint: check spelling and ensure the command is on your PATH", err)
}

// checkTarget is checkAvailable for the root's CLI argument: a missing CLI
// whose name is close to a treemand subcommand also suggests that subcommand.
func checkTarget(root *cobra.Command, cliName string) error {
	err := checkAvailable(cliName)
	var never *discovery.NeverProbeError
	if err == nil || errors.As(err, &never) {
		return err
	}
	return fmt.Errorf("%w%s", err, subcommandHint(root, cliName))
}

// discoveryContext bounds a run by --timeout seconds. Zero means no overall
// limit; each help command is still bounded by its own timeout.
func discoveryContext(seconds int) (context.Context, context.CancelFunc) {
	if seconds == 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
}

// setupLogging sends logs to stderr at warn level, or debug with --debug.
func setupLogging() {
	logLevel := zerolog.WarnLevel
	if cfgDebug {
		logLevel = zerolog.DebugLevel
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(logLevel)
}

// configureDiscovery applies the process-wide discovery settings: the help
// concurrency limit and a debug note when help output hits the read cap.
func configureDiscovery() {
	discovery.SetMaxConcurrentHelp(cfgJobs)
	discovery.OnOutputTruncated = func(name string, a []string, limit int) {
		log.Debug().Str("command", name).Strs("args", a).Int("limit_bytes", limit).Msg("help output truncated at the read cap")
	}
}

// configFromFlags builds the run's config: defaults, then the config file
// (flags > env > file > defaults), then flags that were explicitly set.
func configFromFlags() *config.Config {
	cfg := config.DefaultConfig()
	cfg.NoColor = cfgNoColor || cfg.NoColor
	cfg.Depth = cfgDepth
	cfg.NoCache = cfgNoCache
	config.ApplyViper(cfg)
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
	if cfgExpandStubs > 0 {
		cfg.NoCache = true // expanded trees are exports, not cache entries
	}
	if cfgTreeStyle != "" && cfgTreeStyle != "default" {
		cfg.TreeStyle = config.ParseTreeStyle(cfgTreeStyle)
	}
	return cfg
}

// applyPresetDepth lets a preset tune a well-known CLI's depth (aws,
// stripe, ...). An explicit --depth wins. preset is nil without one.
func applyPresetDepth(cmd *cobra.Command, cfg *config.Config, preset *presets.Preset) {
	if preset != nil && preset.Depth != 0 && !cmd.Flags().Changed("depth") {
		cfg.Depth = preset.Depth
	}
}

// withSpinner shows msg on stderr while fn runs.
func withSpinner(msg string, fn func()) {
	spin := NewSpinner(os.Stderr)
	spin.Start(msg)
	fn()
	spin.Stop()
}

func output(cmd *cobra.Command, node *models.Node, cfg *config.Config, save func(*models.Node) error, ov *overrides.Override) error {
	if cfgInteractive {
		var crawl *tui.CrawlOptions
		if !tui.Offline {
			crawl = &tui.CrawlOptions{MaxDepth: treeload.CrawlDepth(cfg.Depth), StubThreshold: cfg.StubThreshold}
		}
		return tui.RunWithOptions(node, cfg, tui.RunOptions{
			Save: save, Crawl: crawl, History: historyStore(cfg, node.Name),
			Presets: tuiPresets(node.Name), Preset: startPreset,
			Override: overrideFunc(ov), Settings: configSettings(cmd),
		})
	}
	opts := render.Options{
		MaxDepth:       cfg.Depth,
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
	if err := r.Render(cmd.OutOrStdout(), node); err != nil {
		return err
	}
	if cfgOutput == "" || cfgOutput == "text" {
		if n := countStubs(node); n > 0 {
			// stderr, so stdout stays clean for pipes.
			fmt.Fprint(cmd.ErrOrStderr(), stubNote(n))
		}
	}
	return nil
}

// stubNote explains the (…) marks under a text tree.
func stubNote(n int) string {
	if cfgStdin {
		return fmt.Sprintf("\n%d subcommands are marked (…): --stdin reads one help text and runs nothing, so their own help was not read.\n", n)
	}
	return fmt.Sprintf("\n%d undiscovered subcommands are marked (…). Use -i to expand them on demand, or --expand-stubs=1 for a complete snapshot (slow on large CLIs).\n", n)
}

// countStubs returns how many nodes below root were not discovered (lazily
// created, shown as (…) in text output).
func countStubs(root *models.Node) int {
	n := 0
	root.Walk(func(x *models.Node) {
		if x.Stub && x != root {
			n++
		}
	})
	return n
}

// Execute runs the root command.
func Execute() {
	// Inject treemand's own version into the cache key so upgrades
	// automatically invalidate stale entries.
	Version = resolveVersion(Version, debug.ReadBuildInfo)
	cache.TreemandVersion = Version

	// Wire --version flag to show the same string as `treemand version`.
	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(schemaCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(cacheCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(genDocsCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(presetsCmd)
	rootCmd.ValidArgsFunction = completeCLIName
	rootCmd.SetFlagErrorFunc(flagError)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, FormatError(err))
		os.Exit(ExitCodeFor(err))
	}
}

// NewRootCmd returns a fresh root command for testing (flags reset to defaults).
func NewRootCmd() *cobra.Command {
	c := &cobra.Command{
		Use:           rootCmd.Use,
		Short:         rootCmd.Short,
		Long:          rootCmd.Long,
		Args:          rootArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runRoot,
	}
	c.SetFlagErrorFunc(flagError)
	defineRootFlags(c)
	c.AddCommand(versionCmd)
	c.AddCommand(schemaCmd)
	c.AddCommand(mcpCmd)
	c.AddCommand(cacheCmd)
	c.AddCommand(configCmd)
	c.AddCommand(genDocsCmd)
	c.AddCommand(completionCmd)
	c.AddCommand(doctorCmd)
	c.AddCommand(historyCmd)
	c.AddCommand(presetsCmd)
	c.ValidArgsFunction = completeCLIName
	return c
}

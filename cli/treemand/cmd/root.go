// Package cmd implements the treemand CLI commands.
package cmd

import (
	"context"
	"errors"
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
	"github.com/aallbrig/treemand/presets"
	"github.com/aallbrig/treemand/render"
	"github.com/aallbrig/treemand/tui"
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

Output formats (--output):
  text          colored tree (default)
  json          machine-readable full tree with flags and descriptions
  yaml          YAML output (same structure as JSON)
                (treemand schema prints the JSON Schema both follow)

Examples:
  treemand git                        # full git tree
  treemand -i aws                     # interactive aws explorer
  treemand --depth=2 kubectl          # kubectl tree, 2 levels deep
  treemand --commands-only docker     # subcommands only, no flags
  treemand --output=json gh | jq .    # pipe JSON to jq
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default: ~/.config/treemand/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&cfgInteractive, "interactive", "i", false, "Launch interactive TUI")
	rootCmd.PersistentFlags().StringVarP(&cfgStrategy, "strategy", "s", "help", "Discovery strategies (comma-separated: help,completions)")
	rootCmd.PersistentFlags().IntVar(&cfgDepth, "depth", 3, "Max tree depth (default 3; -1 = unlimited)")
	rootCmd.PersistentFlags().StringVar(&cfgFilter, "filter", "", "Only show nodes matching pattern")
	rootCmd.PersistentFlags().StringVar(&cfgExclude, "exclude", "", "Exclude nodes matching pattern")
	rootCmd.PersistentFlags().BoolVar(&cfgCommandsOnly, "commands-only", false, "Hide flags and positionals")
	rootCmd.PersistentFlags().BoolVar(&cfgFullPath, "full-path", false, "Show full command paths")
	rootCmd.PersistentFlags().StringVar(&cfgOutput, "output", "text", "Output format: text, json, yaml")
	rootCmd.PersistentFlags().BoolVar(&cfgNoColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVar(&cfgNoCache, "no-cache", false, "Disable caching")
	rootCmd.Flags().BoolVar(&cfgRefresh, "refresh", false, "Re-discover the CLI even if it is cached, and update the cache")
	rootCmd.Flags().BoolVar(&cfgStdin, "stdin", false, "Read the CLI's help text from stdin instead of running it (subcommands stay undiscovered)")
	rootCmd.PersistentFlags().IntVar(&cfgJobs, "jobs", 0, "Max concurrent help processes during discovery (0 = auto: 2 x CPUs, between 4 and 16)")
	rootCmd.PersistentFlags().IntVar(&cfgExpandStubs, "expand-stubs", 0, "Expand undiscovered (stub, marked (…)) nodes N levels deep before output: gets their descriptions and children (slow on huge CLIs; disables cache)")
	rootCmd.PersistentFlags().IntVar(&cfgTimeout, "timeout", 30, "Discovery timeout in seconds (0 = no limit; each help command still has its own limit)")
	rootCmd.PersistentFlags().BoolVar(&cfgDebug, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVar(&cfgIcons, "icons", "", "Icon preset: unicode (default), ascii, nerd")
	rootCmd.PersistentFlags().IntVar(&cfgLineLength, "line-length", 0, "Max description chars before truncation (default 80)")
	rootCmd.PersistentFlags().IntVar(&cfgStubThreshold, "stub-threshold", 0, "Max eager children before creating stubs (default 150)")
	rootCmd.PersistentFlags().StringVar(&cfgTreeStyle, "tree-style", "default", "TUI tree presentation style: default, columns, compact, graph")

	_ = viper.BindPFlag("icons", rootCmd.PersistentFlags().Lookup("icons"))
	_ = viper.BindPFlag("desc_line_length", rootCmd.PersistentFlags().Lookup("line-length"))
	_ = viper.BindPFlag("stub_threshold", rootCmd.PersistentFlags().Lookup("stub-threshold"))
	_ = viper.BindPFlag("no_color", rootCmd.PersistentFlags().Lookup("no-color"))
	_ = viper.BindPFlag("tree_style", rootCmd.PersistentFlags().Lookup("tree-style"))
	_ = viper.BindPFlag("depth", rootCmd.PersistentFlags().Lookup("depth"))
	_ = viper.BindPFlag("strategies", rootCmd.PersistentFlags().Lookup("strategy"))
	_ = viper.BindPFlag("no_cache", rootCmd.PersistentFlags().Lookup("no-cache"))
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
	if cfgStdin {
		return runFromStdin(cmd, cliName)
	}
	configureDiscovery()

	// Fail early with a clear message if the binary cannot be found.
	if err := checkTarget(cmd.Root(), cliName); err != nil {
		return err
	}

	cfg := configFromFlags()
	strategies := config.ParseStrategies(cfgStrategy)
	preset, hasPreset := presets.Lookup(cliName)
	if !hasPreset {
		preset = nil
	}
	applyPresetDepth(cmd, cfg, preset)
	ov, err := loadOverride(cliName)
	if err != nil {
		return err
	}

	ctx, cancel := discoveryContext(cfgTimeout)
	defer cancel()

	rc := openRunCache(cfg, cliName, strategies, cacheKeyExtra(cfg, preset)+overrideKey(ov))
	if rc != nil {
		defer rc.c.Close()
	}
	if node := rc.cached(cfg); node != nil {
		save := rc.saver()
		if !cfgInteractive {
			finishPartialTree(ctx, node, cliName, cfg, save)
		}
		applyOverride(ov, node)
		return output(cmd, node, cfg, save)
	}

	// The version string is only shown by `cache list`. Read it while discovery
	// runs so a slow --version (aws, pip) adds nothing to a cold run.
	versionCh := rc.readVersion()

	node, err := discover(ctx, cliName, cfg, strategies)
	if err != nil {
		return err
	}
	if preset != nil && preset.Seed != nil {
		seedFromPreset(ctx, node, cliName, preset)
	}
	if cfgExpandStubs > 0 {
		withSpinner("expanding stubs…", func() {
			discovery.ExpandStubs(ctx, node, cliName, cfgExpandStubs, 16, 30*time.Second, nil)
		})
	}
	applyOverride(ov, node)

	rc.put(node, versionCh)
	return output(cmd, node, cfg, rc.saver())
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
		cfgDepth = preset.Depth
	}
}

// cacheKeyExtra is the part of the cache key that depends on how the tree
// was discovered: depth, stub threshold and preset (nil without one).
func cacheKeyExtra(cfg *config.Config, preset *presets.Preset) string {
	extra := fmt.Sprintf("d=%d,t=%d", cfg.Depth, cfg.StubThreshold)
	if preset != nil {
		extra += ",p=" + preset.Name
	}
	return extra
}

// runCache is the cache entry one run reads and writes. A nil *runCache
// means the run has no cache (--no-cache, or the cache failed to open), and
// every method is then a no-op.
type runCache struct {
	c        *cache.Cache
	key      string
	cliName  string
	strategy string
}

// openRunCache opens the cache and computes this run's key, or returns nil
// when caching is off or the cache cannot be opened.
func openRunCache(cfg *config.Config, cliName string, strategies []string, extra string) *runCache {
	if cfg.NoCache {
		return nil
	}
	c, err := cache.Open(cfg.CacheDir)
	if err != nil {
		log.Warn().Err(err).Msg("could not open cache, running without")
		return nil
	}
	// Key on the binary on disk, not on its --version output: starting
	// aws or pip just to read a version costs seconds on every cache
	// hit, and a CLI with no --version would never be invalidated.
	ver := cache.BinaryFingerprint(discovery.ResolveBinary(cliName))
	if ver == "" {
		ver = cache.CLIVersion(cliName)
	}
	return &runCache{c: c, key: cache.KeyWith(cliName, ver, strategies, extra), cliName: cliName, strategy: cfgStrategy}
}

// cached returns the cached tree, or nil on a miss or with --refresh.
func (rc *runCache) cached(cfg *config.Config) *models.Node {
	if rc == nil {
		return nil
	}
	if cfgRefresh {
		log.Debug().Str("cli", rc.cliName).Msg("cache bypassed by --refresh")
		return nil
	}
	node, err := rc.c.Get(rc.key, cfg.CacheTTL)
	if err != nil || node == nil {
		return nil
	}
	log.Debug().Str("cli", rc.cliName).Msg("cache hit")
	return node
}

// saver returns the hook that writes a grown tree back to the cache, or nil
// without a cache.
func (rc *runCache) saver() func(*models.Node) error {
	if rc == nil {
		return nil
	}
	return saveExpansions(rc.c, rc.key, rc.cliName, rc.strategy)
}

// readVersion starts reading the CLI's --version in the background, or
// returns nil without a cache.
func (rc *runCache) readVersion() chan string {
	if rc == nil {
		return nil
	}
	ch := make(chan string, 1)
	go func() { ch <- cache.CLIVersion(rc.cliName) }()
	return ch
}

// put stores a freshly discovered tree with the version readVersion read.
func (rc *runCache) put(node *models.Node, versionCh chan string) {
	if rc == nil {
		return
	}
	if err := rc.c.Put(rc.key, rc.cliName, <-versionCh, rc.strategy, node); err != nil {
		log.Warn().Err(err).Msg("cache write failed")
	}
}

// finishPartialTree completes a cached tree that a TUI session saved
// mid-crawl, so non-interactive output is never partial.
func finishPartialTree(ctx context.Context, node *models.Node, cliName string, cfg *config.Config, save func(*models.Node) error) {
	if len(discovery.PendingStubs(node, crawlDepth(cfg), cfg.StubThreshold)) == 0 {
		return
	}
	withSpinner("discovering "+cliName+"…", func() {
		discovery.CompletePending(ctx, node, cliName, crawlDepth(cfg), cfg.StubThreshold, discovery.DefaultMaxConcurrentHelp())
		models.MarkInheritedFlags(node)
	})
	if err := save(node); err != nil {
		log.Warn().Err(err).Msg("cache write failed")
	}
}

// discover runs the configured strategies on cliName. An interactive help
// run asks only the root; the TUI discovers the rest in the background.
func discover(ctx context.Context, cliName string, cfg *config.Config, strategies []string) (*models.Node, error) {
	var discoverers []discovery.Discoverer
	if streamsDiscovery(strategies) {
		// The TUI opens as soon as the root is read and discovers the rest
		// in the background (EnableCrawl), so only the root is asked here.
		d := discovery.NewHelpDiscoverer(1)
		d.ChildrenAsStubs = true
		d.StubThreshold = cfg.StubThreshold
		discoverers = []discovery.Discoverer{d}
	} else {
		// -1 means unlimited; crawlDepth caps it at 99 to prevent infinite loops.
		discoverers = discovery.BuildDiscoverersWithThreshold(strategies, crawlDepth(cfg), cfg.StubThreshold)
	}
	var (
		node *models.Node
		err  error
	)
	withSpinner("discovering "+cliName+"…", func() {
		node, err = discovery.Run(ctx, discoverers, cliName)
	})
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}
	if node == nil {
		return nil, fmt.Errorf("no results from discovery for %q", cliName)
	}
	return node, nil
}

// seedFromPreset grafts the preset's own map of the CLI onto the tree.
func seedFromPreset(ctx context.Context, node *models.Node, cliName string, preset *presets.Preset) {
	withSpinner("mapping "+cliName+"…", func() {
		seed, err := preset.Seed(ctx, cliName)
		if err != nil {
			log.Warn().Err(err).Str("preset", preset.Name).Msg("preset seed failed")
			return
		}
		presets.Graft(node, seed)
	})
}

// withSpinner shows msg on stderr while fn runs.
func withSpinner(msg string, fn func()) {
	spin := NewSpinner(os.Stderr)
	spin.Start(msg)
	fn()
	spin.Stop()
}

// streamsDiscovery reports whether this run opens the TUI on the root alone
// and discovers the rest in the background. Merged strategies and
// --expand-stubs exports still discover everything up front.
func streamsDiscovery(strategies []string) bool {
	return cfgInteractive && cfgExpandStubs == 0 && len(strategies) == 1 && strategies[0] == "help"
}

// crawlDepth is the depth a background crawl or completion stops at, matching
// what an up-front discovery would have reached.
func crawlDepth(cfg *config.Config) int {
	if cfg.Depth < 0 {
		return 99
	}
	return cfg.Depth
}

// saveExpansions returns the hook that writes a grown tree back to the cache:
// commands discovered in the TUI (background crawl, stubs expanded on demand,
// indexing) are kept, so the next run starts with them.
func saveExpansions(c *cache.Cache, key, cliName, strategy string) func(*models.Node) error {
	return func(root *models.Node) error {
		ok, err := c.UpdateTree(key, root)
		if err == nil && !ok {
			err = c.Put(key, cliName, "", strategy, root)
		}
		return err
	}
}

func output(cmd *cobra.Command, node *models.Node, cfg *config.Config, save func(*models.Node) error) error {
	if cfgInteractive {
		var crawl *tui.CrawlOptions
		if !tui.Offline {
			crawl = &tui.CrawlOptions{MaxDepth: crawlDepth(cfg), StubThreshold: cfg.StubThreshold}
		}
		return tui.RunWithOptions(node, cfg, tui.RunOptions{Save: save, Crawl: crawl, History: historyStore(cfg, node.Name)})
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
	c.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file")
	c.PersistentFlags().BoolVarP(&cfgInteractive, "interactive", "i", false, "Launch interactive TUI")
	c.PersistentFlags().StringVarP(&cfgStrategy, "strategy", "s", "help", "Discovery strategies")
	c.PersistentFlags().IntVar(&cfgDepth, "depth", 3, "Max tree depth (default 3; -1 = unlimited)")
	c.PersistentFlags().StringVar(&cfgFilter, "filter", "", "Filter pattern")
	c.PersistentFlags().StringVar(&cfgExclude, "exclude", "", "Exclude pattern")
	c.PersistentFlags().BoolVar(&cfgCommandsOnly, "commands-only", false, "Hide flags/positionals")
	c.PersistentFlags().BoolVar(&cfgFullPath, "full-path", false, "Full command paths")
	c.PersistentFlags().StringVar(&cfgOutput, "output", "text", "Output format")
	c.PersistentFlags().BoolVar(&cfgNoColor, "no-color", false, "Disable color")
	c.PersistentFlags().BoolVar(&cfgNoCache, "no-cache", false, "Disable cache")
	c.PersistentFlags().IntVar(&cfgTimeout, "timeout", 5, "Discovery timeout in seconds (0 = no limit)")
	c.PersistentFlags().BoolVar(&cfgDebug, "debug", false, "Debug logging")
	c.PersistentFlags().StringVar(&cfgIcons, "icons", "", "Icon preset")
	c.PersistentFlags().IntVar(&cfgLineLength, "line-length", 0, "Max description line length")
	c.PersistentFlags().IntVar(&cfgStubThreshold, "stub-threshold", 0, "Stub threshold")
	c.PersistentFlags().IntVar(&cfgJobs, "jobs", 0, "Max concurrent help processes")
	c.PersistentFlags().IntVar(&cfgExpandStubs, "expand-stubs", 0, "Expand undiscovered (stub) nodes N levels deep before output")
	c.Flags().BoolVar(&cfgRefresh, "refresh", false, "Re-discover even if cached")
	c.Flags().BoolVar(&cfgStdin, "stdin", false, "Read the CLI's help text from stdin")
	c.PersistentFlags().StringVar(&cfgTreeStyle, "tree-style", "default", "TUI tree style")
	c.AddCommand(versionCmd)
	c.AddCommand(schemaCmd)
	c.AddCommand(mcpCmd)
	c.AddCommand(cacheCmd)
	c.AddCommand(configCmd)
	c.AddCommand(genDocsCmd)
	c.AddCommand(completionCmd)
	c.AddCommand(doctorCmd)
	c.AddCommand(historyCmd)
	c.ValidArgsFunction = completeCLIName
	return c
}

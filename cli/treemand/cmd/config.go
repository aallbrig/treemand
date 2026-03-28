package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aallbrig/treemand/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage treemand configuration",
	Long: `View, validate, and modify the treemand configuration file.

The config file is YAML and is searched in order:
  1. Path passed via --config flag
  2. $XDG_CONFIG_HOME/treemand/config.yaml  (typically ~/.config/treemand/)
  3. $HOME/.treemand/config.yaml

If no config file exists, built-in defaults are used.

Examples:
  treemand config view             # show merged config with file location
  treemand config validate         # check config for errors/warnings
  treemand config set icons nerd   # set a config value
  treemand config init             # create a default config file
  treemand config path             # print config file path
  treemand config edit             # open config in $EDITOR`,
}

// ── config view ───────────────────────────────────────────────────────────────

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Display the current effective configuration",
	Long: `Show the merged configuration with the config file location.
All values reflect the full precedence chain: flags > env > file > defaults.`,
	RunE: runConfigView,
}

func runConfigView(cmd *cobra.Command, args []string) error {
	cfgPath := resolveConfigPath()
	cfg := config.DefaultConfig()
	config.ApplyViper(cfg)

	yamlStr, err := config.ConfigToYAML(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	out := cmd.OutOrStdout()
	if cfgPath != "" {
		fmt.Fprintf(out, "# Config file: %s\n\n", cfgPath)
	} else {
		fmt.Fprintln(out, "# No config file found (using defaults)")
	}
	fmt.Fprint(out, yamlStr)
	return nil
}

// ── config validate ───────────────────────────────────────────────────────────

var cfgValidateStrict bool

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the config file for errors and warnings",
	Long: `Check the config file for unknown keys, invalid values, and syntax errors.

Warnings are reported for unknown/unsupported keys (the config still loads).
Errors are reported for invalid values or YAML syntax problems.

Use --strict to promote all warnings to errors (non-zero exit code).`,
	RunE: runConfigValidate,
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	cfgPath := resolveConfigPath()
	out := cmd.OutOrStdout()

	if cfgPath == "" {
		fmt.Fprintln(out, "No config file found — nothing to validate.")
		fmt.Fprintln(out, "Run 'treemand config init' to create one.")
		return nil
	}

	fmt.Fprintf(out, "Config file: %s\n", cfgPath)

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	result, err := config.ValidateYAML(data)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	if cfgValidateStrict {
		result.PromoteWarnings()
	}

	for _, d := range result.Diagnostics {
		fmt.Fprintln(out, d.String())
	}

	nErr := len(result.Errors())
	nWarn := len(result.Warnings())

	if nErr == 0 && nWarn == 0 {
		fmt.Fprintln(out, "✓ config is valid")
		return nil
	}

	if cfgValidateStrict && (nErr > 0 || nWarn > 0) {
		// After promotion all are errors
		total := len(result.Diagnostics)
		fmt.Fprintf(out, "✗ %d error(s) (warnings promoted to errors with --strict)\n", total)
		return fmt.Errorf("validation failed: %d error(s)", total)
	}

	if nErr > 0 {
		fmt.Fprintf(out, "✗ %d error(s), %d warning(s)\n", nErr, nWarn)
		return fmt.Errorf("validation failed: %d error(s)", nErr)
	}

	fmt.Fprintf(out, "✓ 0 errors, %d warning(s)\n", nWarn)
	return nil
}

// ── config set ────────────────────────────────────────────────────────────────

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Write a key-value pair to the config file, creating it if necessary.

Supports dot-notation for nested keys (e.g. colors.subcmd).
Values are validated against the config schema before writing.

Examples:
  treemand config set icons nerd
  treemand config set colors.subcmd "#FF5555"
  treemand config set depth 5`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	// Validate key and value.
	entry, known := config.LookupKey(key)
	if !known {
		msg := fmt.Sprintf("unknown config key %q", key)
		if suggestion := config.SuggestKey(key); suggestion != "" {
			msg += fmt.Sprintf(" (did you mean %q?)", suggestion)
		}
		// List available keys.
		msg += "\n\nAvailable keys:\n"
		w := &strings.Builder{}
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, e := range config.KnownKeys {
			fmt.Fprintf(tw, "  %s\t%s\n", e.Key, e.Description)
		}
		tw.Flush()
		msg += w.String()
		return fmt.Errorf("%s", msg)
	}

	if err := config.ValidateValue(entry, value); err != nil {
		return err
	}

	// Ensure config file exists.
	cfgPath := resolveConfigPath()
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
		if err := config.WriteDefaultConfig(cfgPath, false); err != nil {
			// File might already exist; that's fine, we'll overwrite the key.
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}

	// Use viper to set and write.
	viper.Set(key, value)
	viper.SetConfigFile(cfgPath)
	if err := viper.WriteConfig(); err != nil {
		// Try SafeWriteConfig if WriteConfig fails (file may not exist yet).
		if err2 := viper.SafeWriteConfig(); err2 != nil {
			return fmt.Errorf("write config: %w", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s in %s\n", key, value, cfgPath)
	return nil
}

// ── config init ───────────────────────────────────────────────────────────────

var cfgInitForce bool

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default config file with comments",
	Long: `Generate a commented default configuration file.

The file is placed at the preferred config location
($XDG_CONFIG_HOME/treemand/config.yaml).
Refuses to overwrite an existing file unless --force is passed.`,
	RunE: runConfigInit,
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	cfgPath := config.DefaultConfigPath()
	if err := config.WriteDefaultConfig(cfgPath, cfgInitForce); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", cfgPath)
	return nil
}

// ── config path ───────────────────────────────────────────────────────────────

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the config file path",
	Long: `Print the resolved config file path.
If no config file exists, prints the default location where one would be created.

Useful for scripting:
  cat $(treemand config path)
  $EDITOR $(treemand config path)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := resolveConfigPath()
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
			fmt.Fprintf(cmd.OutOrStdout(), "%s (does not exist yet)\n", cfgPath)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), cfgPath)
		}
		return nil
	},
}

// ── config edit ───────────────────────────────────────────────────────────────

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the config file in $EDITOR",
	Long: `Open the config file in your preferred editor ($EDITOR).
If no config file exists, one is created with defaults first.
Falls back to 'vi' if $EDITOR is not set.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := resolveConfigPath()
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
			if err := config.WriteDefaultConfig(cfgPath, false); err != nil {
				if !strings.Contains(err.Error(), "already exists") {
					return err
				}
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Created default config at %s\n", cfgPath)
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		proc := exec.Command(editor, cfgPath)
		proc.Stdin = os.Stdin
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		return proc.Run()
	},
}

// ── helpers ───────────────────────────────────────────────────────────────────

// resolveConfigPath returns the path viper loaded config from, or "".
func resolveConfigPath() string {
	return viper.ConfigFileUsed()
}

func init() {
	configValidateCmd.Flags().BoolVar(&cfgValidateStrict, "strict", false, "Promote warnings to errors")
	configInitCmd.Flags().BoolVar(&cfgInitForce, "force", false, "Overwrite existing config file")

	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configEditCmd)
}

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultConfigYAML is the commented default config written by `config init`.
const defaultConfigYAML = `# treemand configuration file
# Docs: https://aallbrig.github.io/treemand
# Run 'treemand config validate' to check this file.

# Icon preset: unicode (default), ascii, nerd
icons: unicode

# Max description characters before truncation (default: 80)
desc_line_length: 80

# Max eager children before creating stubs (default: 50)
stub_threshold: 50

# TUI tree presentation style: default, columns, compact, graph
tree_style: default

# Disable colored output (default: false)
no_color: false

# Max tree depth, -1 for unlimited (default: -1)
depth: -1

# Disable discovery cache (default: false)
no_cache: false

# Discovery strategies, comma-separated (default: help)
# Available: help, completions, man
strategies: help

# Color scheme (hex colors, all optional)
colors:
  base: "#FFFFFF"
  subcmd: "#5EA4F5"
  flag: "#50FA7B"
  flag_bool: "#50FA7B"
  flag_string: "#8BE9FD"
  flag_int: "#FFB86C"
  flag_other: "#BD93F9"
  pos: "#F1FA8C"
  value: "#FF79C6"
  invalid: "#FF5555"
  selected: "#00BFFF"
  selected_text: "#000000"
`

// DefaultConfigPath returns the preferred config file location.
// It prefers XDG_CONFIG_HOME/treemand/config.yaml, falling back to
// ~/.treemand/config.yaml.
func DefaultConfigPath() string {
	if cfgDir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(cfgDir, "treemand", "config.yaml")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".treemand", "config.yaml")
	}
	return "config.yaml"
}

// WriteDefaultConfig writes a commented default configuration file to path.
// It creates parent directories as needed. If the file already exists and
// force is false, it returns an error.
func WriteDefaultConfig(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(defaultConfigYAML), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

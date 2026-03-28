package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// InitViper sets up viper with config file search paths and environment variable
// bindings. Call this once during CLI initialization, before reading any values.
//
// If cfgFile is non-empty it is used directly; otherwise viper searches:
//  1. $XDG_CONFIG_HOME/treemand/config.yaml  (typically ~/.config/treemand/)
//  2. $HOME/.treemand/config.yaml
func InitViper(cfgFile string) error {
	viper.SetEnvPrefix("TREEMAND")
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Prefer XDG / platform config directory.
		if cfgDir, err := os.UserConfigDir(); err == nil {
			viper.AddConfigPath(filepath.Join(cfgDir, "treemand"))
		}
		// Fallback: ~/.treemand/ (also where the cache lives).
		if home, err := os.UserHomeDir(); err == nil {
			viper.AddConfigPath(filepath.Join(home, ".treemand"))
		}
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// It's fine if the config file doesn't exist yet.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}
	return nil
}

// ApplyViper overlays viper-loaded values onto an existing *Config.
// Flag bindings in cmd/root.go write into viper first, so this call
// respects the full precedence chain: flags > env > file > defaults.
func ApplyViper(cfg *Config) {
	if v := viper.GetString("icons"); v != "" {
		preset := v
		cfg.IconPreset = preset
		cfg.Icons = IconSetForPreset(preset)
	}
	if v := viper.GetInt("desc_line_length"); v > 0 {
		cfg.DescLineLength = v
	}
	if v := viper.GetInt("stub_threshold"); v > 0 {
		cfg.StubThreshold = v
	}
	if viper.GetBool("no_color") {
		cfg.NoColor = true
	}
	if v := viper.GetString("tree_style"); v != "" {
		cfg.TreeStyle = ParseTreeStyle(v)
	}
	if v := viper.GetInt("depth"); v != 0 {
		cfg.Depth = v
	}
	if v := viper.GetString("strategies"); v != "" {
		cfg.Strategies = ParseStrategies(v)
	}
	if viper.GetBool("no_cache") {
		cfg.NoCache = true
	}

	// Color overrides — each sub-key under "colors" is optional.
	if v := viper.GetString("colors.base"); v != "" {
		cfg.Colors.Base = v
	}
	if v := viper.GetString("colors.subcmd"); v != "" {
		cfg.Colors.Subcmd = v
	}
	if v := viper.GetString("colors.flag"); v != "" {
		cfg.Colors.Flag = v
	}
	if v := viper.GetString("colors.flag_bool"); v != "" {
		cfg.Colors.FlagBool = v
	}
	if v := viper.GetString("colors.flag_string"); v != "" {
		cfg.Colors.FlagString = v
	}
	if v := viper.GetString("colors.flag_int"); v != "" {
		cfg.Colors.FlagInt = v
	}
	if v := viper.GetString("colors.flag_other"); v != "" {
		cfg.Colors.FlagOther = v
	}
	if v := viper.GetString("colors.pos"); v != "" {
		cfg.Colors.Pos = v
	}
	if v := viper.GetString("colors.value"); v != "" {
		cfg.Colors.Value = v
	}
	if v := viper.GetString("colors.invalid"); v != "" {
		cfg.Colors.Invalid = v
	}
	if v := viper.GetString("colors.selected"); v != "" {
		cfg.Colors.Selected = v
	}
	if v := viper.GetString("colors.selected_text"); v != "" {
		cfg.Colors.SelectedText = v
	}
}

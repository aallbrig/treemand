// Package config provides color scheme and configuration management.
package config

import (
	"os"
	"strings"
)

// ColorScheme defines the color palette for tree rendering.
type ColorScheme struct {
	Base       string // root/base command color (hex)
	Subcmd     string // subcommand color
	Flag       string // flag color (fallback / bool flags)
	FlagBool   string // boolean (switch) flags: --verbose, --debug
	FlagString string // string-value flags: --output=text
	FlagInt    string // integer-value flags: --timeout=30
	FlagOther  string // other typed flags: duration, float, slice, …
	Pos        string // positional argument color
	Value      string // value/type color (e.g. =string suffix in preview)
	Invalid    string // invalid/error color
	Selected   string // selected item in TUI
}

// DefaultColors returns the default color scheme.
func DefaultColors() ColorScheme {
	return ColorScheme{
		Base:       "#FFFFFF",
		Subcmd:     "#5EA4F5",
		Flag:       "#50FA7B", // fallback (also used for bool)
		FlagBool:   "#50FA7B", // green  — quick toggles
		FlagString: "#8BE9FD", // cyan   — string values
		FlagInt:    "#FFB86C", // orange — numeric values
		FlagOther:  "#BD93F9", // purple — duration, float, slice, …
		Pos:        "#F1FA8C",
		Value:      "#FF79C6",
		Invalid:    "#FF5555",
		Selected:   "#00BFFF",
	}
}

// Config holds all treemand configuration.
type Config struct {
	Colors    ColorScheme
	NoColor   bool
	Depth     int
	NoCache   bool
	CacheDir  string
	Strategies []string
}

// DefaultConfig returns config with sensible defaults.
func DefaultConfig() *Config {
	cacheDir := os.Getenv("TREEMAND_CACHE_DIR")
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = home + "/.treemand"
	}
	return &Config{
		Colors:    DefaultColors(),
		NoColor:   os.Getenv("NO_COLOR") != "" || os.Getenv("TREEMAND_NO_COLOR") != "",
		Depth:     -1, // unlimited
		NoCache:   false,
		CacheDir:  cacheDir,
		Strategies: defaultStrategies(),
	}
}

func defaultStrategies() []string {
	if s := os.Getenv("TREEMAND_STRATEGIES"); s != "" {
		return strings.Split(s, ",")
	}
	return []string{"help"}
}

// ParseStrategies splits a comma-separated strategy string.
func ParseStrategies(s string) []string {
	if s == "" {
		return defaultStrategies()
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

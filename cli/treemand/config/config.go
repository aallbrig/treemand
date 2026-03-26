// Package config provides color scheme and configuration management for treemand.
//
// Configuration is loaded from multiple sources with the following precedence
// (highest to lowest):
//
//  1. CLI flags
//  2. Environment variables (TREEMAND_*)
//  3. Config file (~/.config/treemand/config.yaml or ~/.treemand/config.yaml)
//  4. Built-in defaults
//
// The config file is optional YAML. Example:
//
//	icons: ascii          # unicode | ascii | nerd
//	desc_line_length: 80
//	stub_threshold: 50
//	colors:
//	  subcmd: "#5EA4F5"
package config

import (
	"os"
	"strings"
	"time"
)

// ColorScheme defines the color palette for tree rendering.
type ColorScheme struct {
	Base         string // root/base command color (hex)
	Subcmd       string // subcommand color
	Flag         string // flag color (fallback / bool flags)
	FlagBool     string // boolean (switch) flags: --verbose, --debug
	FlagString   string // string-value flags: --output=text
	FlagInt      string // integer-value flags: --timeout=30
	FlagOther    string // other typed flags: duration, float, slice, …
	Pos          string // positional argument color
	Value        string // value/type color (e.g. =string suffix in preview)
	Invalid      string // invalid/error color
	Selected     string // selected item background in TUI
	SelectedText string // selected item foreground in TUI
}

// DefaultColors returns the default color scheme.
func DefaultColors() ColorScheme {
	return ColorScheme{
		Base:         "#FFFFFF",
		Subcmd:       "#5EA4F5",
		Flag:         "#50FA7B", // fallback (also used for bool)
		FlagBool:     "#50FA7B", // green  — quick toggles
		FlagString:   "#8BE9FD", // cyan   — string values
		FlagInt:      "#FFB86C", // orange — numeric values
		FlagOther:    "#BD93F9", // purple — duration, float, slice, …
		Pos:          "#F1FA8C",
		Value:        "#FF79C6",
		Invalid:      "#FF5555",
		Selected:     "#00BFFF",
		SelectedText: "#000000", // black text on bright highlight for contrast
	}
}

// IconSet defines the glyphs used when drawing the command tree.
// All strings should include a trailing space so they align with node names.
type IconSet struct {
	// Branch is shown next to nodes that have children (expanded state).
	Branch string
	// Collapsed is shown next to nodes with children in their collapsed state (TUI only).
	Collapsed string
	// Leaf is shown next to terminal nodes with no children.
	Leaf string
	// Virtual is shown next to synthetic grouping nodes (e.g. "Flags" section headers).
	Virtual string
	// SectionExpanded is shown for expanded flag/positional section rows in the TUI.
	SectionExpanded string
	// SectionCollapsed is shown for collapsed flag/positional section rows in the TUI.
	SectionCollapsed string
}

// Built-in icon preset names.
const (
	IconPresetUnicode = "unicode" // default — Unicode box-drawing glyphs
	IconPresetASCII   = "ascii"   // safe 7-bit ASCII for terminals without Unicode
	IconPresetNerd    = "nerd"    // Nerd Font glyphs (requires a patched font)
)

// IconSetForPreset returns the IconSet for a named preset.
// Unknown names fall back to the unicode preset.
func IconSetForPreset(name string) IconSet {
	switch name {
	case IconPresetASCII:
		return IconSet{
			Branch: "v ", Collapsed: "> ", Leaf: "- ", Virtual: "* ",
			SectionExpanded: "v ", SectionCollapsed: "> ",
		}
	case IconPresetNerd:
		// Nerd Font glyphs: folder-open, folder, file, diamond.
		return IconSet{
			Branch: " ", Collapsed: " ", Leaf: " ", Virtual: " ",
			SectionExpanded: " ", SectionCollapsed: " ",
		}
	default:
		return DefaultIconSet()
	}
}

// DefaultIconSet returns the default unicode icon set.
func DefaultIconSet() IconSet {
	return IconSet{
		Branch: "▼ ", Collapsed: "▶ ", Leaf: "• ", Virtual: "◆ ",
		SectionExpanded: "▽ ", SectionCollapsed: "▷ ",
	}
}

// DisplayStyle controls how the tree is rendered in the TUI.
type DisplayStyle int

const (
	// StyleDefault renders collapsible nodes with inline flag pills (baseline).
	StyleDefault DisplayStyle = iota
	// StyleColumns renders name on the left and description right-aligned after a · separator.
	StyleColumns
	// StyleCompact renders nodes with no icons and no inline flags — maximum density.
	StyleCompact
	// StyleGraph renders classic tree connectors (├── / └──) like the `tree` command.
	StyleGraph
)

// DisplayStyleNames maps each style to a short display name for the status bar.
var DisplayStyleNames = []string{"default", "columns", "compact", "graph"}

// Config holds all treemand runtime configuration.
type Config struct {
	Colors           ColorScheme
	Icons            IconSet
	IconPreset       string // "unicode" | "ascii" | "nerd" — tracks which preset is active
	DescLineLength   int    // max runes in a description before truncation (default 80)
	StubThreshold    int    // max eager children before switching to stubs (default 50)
	NoColor          bool
	Depth            int
	NoCache          bool
	CacheDir         string
	Strategies       []string
	TreeStyle        DisplayStyle  // controls TUI tree presentation variant
	StatusMsgTimeout time.Duration // how long a timed status message is shown (default 3s)
}

// DefaultConfig returns config with sensible defaults.
func DefaultConfig() *Config {
	cacheDir := os.Getenv("TREEMAND_CACHE_DIR")
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = home + "/.treemand"
	}
	return &Config{
		Colors:           DefaultColors(),
		Icons:            DefaultIconSet(),
		IconPreset:       IconPresetUnicode,
		DescLineLength:   80,
		StubThreshold:    50,
		NoColor:          os.Getenv("NO_COLOR") != "" || os.Getenv("TREEMAND_NO_COLOR") != "",
		Depth:            -1, // unlimited
		NoCache:          false,
		CacheDir:         cacheDir,
		Strategies:       defaultStrategies(),
		TreeStyle:        StyleDefault,
		StatusMsgTimeout: 3 * time.Second,
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

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aallbrig/treemand/config"
)

func TestDefaultColors(t *testing.T) {
	c := config.DefaultColors()
	if c.Base == "" || c.Subcmd == "" || c.Flag == "" {
		t.Error("expected non-empty default colors")
	}
	if c.Base != "#FFFFFF" {
		t.Errorf("Base color = %q, want #FFFFFF", c.Base)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.Depth != -1 {
		t.Errorf("Depth = %d, want -1", cfg.Depth)
	}
	if len(cfg.Strategies) == 0 {
		t.Error("expected at least one default strategy")
	}
	if cfg.DescLineLength != 80 {
		t.Errorf("DescLineLength = %d, want 80", cfg.DescLineLength)
	}
	if cfg.StubThreshold != 50 {
		t.Errorf("StubThreshold = %d, want 50", cfg.StubThreshold)
	}
	if cfg.IconPreset != config.IconPresetUnicode {
		t.Errorf("IconPreset = %q, want %q", cfg.IconPreset, config.IconPresetUnicode)
	}
}

func TestParseStrategies(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"help", []string{"help"}},
		{"help,completions", []string{"help", "completions"}},
		{"help, completions", []string{"help", "completions"}},
		{"", []string{"help"}},
	}
	for _, tt := range tests {
		got := config.ParseStrategies(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("ParseStrategies(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i, s := range tt.expected {
			if got[i] != s {
				t.Errorf("ParseStrategies(%q)[%d] = %q, want %q", tt.input, i, got[i], s)
			}
		}
	}
}

func TestIconSetPresets(t *testing.T) {
	unicode := config.IconSetForPreset(config.IconPresetUnicode)
	if unicode.Branch != "▼ " {
		t.Errorf("unicode Branch = %q, want '▼ '", unicode.Branch)
	}
	if unicode.Collapsed != "▶ " {
		t.Errorf("unicode Collapsed = %q, want '▶ '", unicode.Collapsed)
	}
	if unicode.Leaf != "• " {
		t.Errorf("unicode Leaf = %q, want '• '", unicode.Leaf)
	}
	if unicode.SectionExpanded != "▽ " {
		t.Errorf("unicode SectionExpanded = %q, want '▽ '", unicode.SectionExpanded)
	}

	ascii := config.IconSetForPreset(config.IconPresetASCII)
	if ascii.Branch != "v " {
		t.Errorf("ascii Branch = %q, want 'v '", ascii.Branch)
	}
	if ascii.Collapsed != "> " {
		t.Errorf("ascii Collapsed = %q, want '> '", ascii.Collapsed)
	}
	if ascii.Leaf != "- " {
		t.Errorf("ascii Leaf = %q, want '- '", ascii.Leaf)
	}

	// Unknown preset falls back to unicode.
	fallback := config.IconSetForPreset("unknown-preset")
	if fallback.Branch != unicode.Branch {
		t.Errorf("unknown preset Branch = %q, want unicode fallback %q", fallback.Branch, unicode.Branch)
	}
}

func TestDefaultIconSet(t *testing.T) {
	icons := config.DefaultIconSet()
	if icons.Branch == "" || icons.Leaf == "" || icons.Virtual == "" {
		t.Error("DefaultIconSet should have non-empty fields")
	}
	if icons.SectionExpanded == "" || icons.SectionCollapsed == "" {
		t.Error("DefaultIconSet should have non-empty section icons")
	}
}

func TestParseTreeStyle(t *testing.T) {
	tests := []struct {
		input string
		want  config.DisplayStyle
	}{
		{"default", config.StyleDefault},
		{"columns", config.StyleColumns},
		{"compact", config.StyleCompact},
		{"graph", config.StyleGraph},
		{"unknown", config.StyleDefault},
		{"", config.StyleDefault},
	}
	for _, tt := range tests {
		got := config.ParseTreeStyle(tt.input)
		if got != tt.want {
			t.Errorf("ParseTreeStyle(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestLoadConfigFile(t *testing.T) {
	// Write a temporary config file and verify ApplyViper reads it.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `icons: ascii
desc_line_length: 120
stub_threshold: 25
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := config.InitViper(cfgPath); err != nil {
		t.Fatalf("InitViper error: %v", err)
	}

	cfg := config.DefaultConfig()
	config.ApplyViper(cfg)

	if cfg.DescLineLength != 120 {
		t.Errorf("DescLineLength = %d, want 120", cfg.DescLineLength)
	}
	if cfg.StubThreshold != 25 {
		t.Errorf("StubThreshold = %d, want 25", cfg.StubThreshold)
	}
	if cfg.IconPreset != config.IconPresetASCII {
		t.Errorf("IconPreset = %q, want %q", cfg.IconPreset, config.IconPresetASCII)
	}
	if cfg.Icons.Branch != "v " {
		t.Errorf("Icons.Branch = %q, want 'v '", cfg.Icons.Branch)
	}
}

func TestLoadConfigFile_treeStyleAndColors(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `tree_style: graph
colors:
  base: "#FF0000"
  subcmd: "#00FF00"
  selected: "#0000FF"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := config.InitViper(cfgPath); err != nil {
		t.Fatalf("InitViper error: %v", err)
	}

	cfg := config.DefaultConfig()
	config.ApplyViper(cfg)

	if cfg.TreeStyle != config.StyleGraph {
		t.Errorf("TreeStyle = %d, want StyleGraph (%d)", cfg.TreeStyle, config.StyleGraph)
	}
	if cfg.Colors.Base != "#FF0000" {
		t.Errorf("Colors.Base = %q, want #FF0000", cfg.Colors.Base)
	}
	if cfg.Colors.Subcmd != "#00FF00" {
		t.Errorf("Colors.Subcmd = %q, want #00FF00", cfg.Colors.Subcmd)
	}
	if cfg.Colors.Selected != "#0000FF" {
		t.Errorf("Colors.Selected = %q, want #0000FF", cfg.Colors.Selected)
	}
}

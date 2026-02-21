package config_test

import (
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

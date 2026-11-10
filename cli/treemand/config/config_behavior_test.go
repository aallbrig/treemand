package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/aallbrig/treemand/config"
)

func TestDiagnostic_String(t *testing.T) {
	e := config.Diagnostic{Level: "error", Key: "depth", Message: "depth must be an integer"}
	w := config.Diagnostic{Level: "warning", Key: "icons", Message: "unknown preset"}
	if got := e.String(); !strings.Contains(got, "error") || !strings.Contains(got, "depth must be an integer") {
		t.Errorf("error diagnostic rendered as %q", got)
	}
	if got := w.String(); !strings.Contains(got, "warning") || !strings.Contains(got, "unknown preset") {
		t.Errorf("warning diagnostic rendered as %q", got)
	}
	if e.String() == w.String() {
		t.Error("error and warning diagnostics must render differently")
	}
}

func TestValidationResult_HasErrors(t *testing.T) {
	onlyWarning := &config.ValidationResult{Diagnostics: []config.Diagnostic{{Level: "warning", Message: "w"}}}
	if onlyWarning.HasErrors() {
		t.Error("a warning alone must not count as an error")
	}
	withError := &config.ValidationResult{Diagnostics: []config.Diagnostic{
		{Level: "warning", Message: "w"}, {Level: "error", Message: "e"},
	}}
	if !withError.HasErrors() {
		t.Error("an error-level diagnostic must be reported")
	}
	if len(withError.Warnings()) != 1 || len(withError.Errors()) != 1 {
		t.Errorf("Warnings/Errors split wrong: %d / %d", len(withError.Warnings()), len(withError.Errors()))
	}
}

func TestDefaultConfigPath_followsXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	want := filepath.Join(dir, "treemand", "config.yaml")
	if got := config.DefaultConfigPath(); got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestWriteDefaultConfig_protectsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "config.yaml")
	if err := config.WriteDefaultConfig(path, false); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config not created (parents should be): %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %o, want 600", perm)
	}

	if err := os.WriteFile(path, []byte("depth: 9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteDefaultConfig(path, false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("overwrite without force = %v, want an 'already exists' error", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "depth: 9\n" {
		t.Errorf("file was modified despite the refusal: %q", b)
	}

	if err := config.WriteDefaultConfig(path, true); err != nil {
		t.Fatalf("forced overwrite failed: %v", err)
	}
	if b, _ := os.ReadFile(path); strings.TrimSpace(string(b)) == "depth: 9" {
		t.Error("forced overwrite left the old content in place")
	}
}

func TestInitViperAndApplyViper_fileOverridesDefaults(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	file := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "icons: ascii\ndepth: 5\nstub_threshold: 77\nno_cache: true\nstrategies: help,man\n" +
		"tree_style: default\ncolors:\n  base: \"#123456\"\n  flag: \"#abcdef\"\n"
	if err := os.WriteFile(file, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.InitViper(file); err != nil {
		t.Fatalf("InitViper: %v", err)
	}
	cfg := config.DefaultConfig()
	config.ApplyViper(cfg)

	if cfg.IconPreset != "ascii" || cfg.Icons.Branch != "v " {
		t.Errorf("icons not applied: preset %q branch %q", cfg.IconPreset, cfg.Icons.Branch)
	}
	if cfg.Depth != 5 || cfg.StubThreshold != 77 || !cfg.NoCache {
		t.Errorf("scalars not applied: depth %d stub %d nocache %v", cfg.Depth, cfg.StubThreshold, cfg.NoCache)
	}
	if got := strings.Join(cfg.Strategies, ","); got != "help,man" {
		t.Errorf("strategies = %q, want help,man", got)
	}
	if cfg.Colors.Base != "#123456" || cfg.Colors.Flag != "#abcdef" {
		t.Errorf("colors not applied: base %q flag %q", cfg.Colors.Base, cfg.Colors.Flag)
	}
}

func TestInitViper_environmentBeatsMissingFile(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no config file anywhere
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TREEMAND_DEPTH", "4")
	if err := config.InitViper(""); err != nil {
		t.Fatalf("a missing config file must not be an error: %v", err)
	}
	cfg := config.DefaultConfig()
	config.ApplyViper(cfg)
	if cfg.Depth != 4 {
		t.Errorf("TREEMAND_DEPTH not applied, depth = %d", cfg.Depth)
	}
}

func TestInitViper_malformedFileIsAnError(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("depth: [unterminated\n  : :\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.InitViper(file); err == nil {
		t.Error("a malformed config file must be reported, not ignored")
	}
}

func TestIconSetForPreset(t *testing.T) {
	ascii := config.IconSetForPreset(config.IconPresetASCII)
	nerd := config.IconSetForPreset(config.IconPresetNerd)
	unicode := config.IconSetForPreset("no-such-preset")
	if ascii.Branch != "v " || ascii.Collapsed != "> " {
		t.Errorf("ascii preset = %+v, want plain 7-bit markers", ascii)
	}
	for _, r := range ascii.Branch + ascii.Collapsed + ascii.Leaf {
		if r > 127 {
			t.Errorf("ascii preset contains non-ASCII rune %q", r)
		}
	}
	if nerd == ascii || nerd == unicode {
		t.Error("the nerd preset must differ from ascii and unicode")
	}
	if unicode != config.DefaultIconSet() {
		t.Error("an unknown preset must fall back to the default (unicode) set")
	}
}

func TestDefaultConfig_strategiesFromEnvironment(t *testing.T) {
	t.Setenv("TREEMAND_STRATEGIES", "help,man,completions")
	got := config.DefaultConfig().Strategies
	if strings.Join(got, ",") != "help,man,completions" {
		t.Errorf("strategies = %v, want the TREEMAND_STRATEGIES list", got)
	}
	t.Setenv("TREEMAND_STRATEGIES", "")
	if got := config.DefaultConfig().Strategies; len(got) != 1 || got[0] != "help" {
		t.Errorf("default strategies = %v, want [help]", got)
	}
}

func TestToYAML_unknownTreeStyleFallsBackToDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TreeStyle = config.DisplayStyle(999)
	out, err := config.ToYAML(cfg)
	if err != nil {
		t.Fatalf("ToYAML: %v", err)
	}
	if !strings.Contains(out, "tree_style: default") {
		t.Errorf("unknown style should render as default:\n%s", out)
	}
}

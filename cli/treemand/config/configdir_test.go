package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/viper"

	"github.com/aallbrig/treemand/config"
)

// Unix CLIs keep their config under ~/.config, on macOS too; a relative
// XDG_CONFIG_HOME is invalid per the XDG spec and must be ignored.
func TestDefaultConfigPath_unixDefaultsToDotConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses %AppData%")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, xdg := range []string{"", "relative/dir"} {
		t.Setenv("XDG_CONFIG_HOME", xdg)
		want := filepath.Join(home, ".config", "treemand", "config.yaml")
		if got := config.DefaultConfigPath(); got != want {
			t.Errorf("XDG_CONFIG_HOME=%q: DefaultConfigPath() = %q, want %q", xdg, got, want)
		}
	}
}

// A config written by an older release on macOS, under
// ~/Library/Application Support, keeps working until the user moves it.
func TestInitViper_readsLegacyMacOSConfig(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	legacy := filepath.Join(home, "Library", "Application Support", "treemand", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("depth: 7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	t.Cleanup(viper.Reset)
	if err := config.InitViper(""); err != nil {
		t.Fatalf("InitViper: %v", err)
	}
	if got := viper.ConfigFileUsed(); got != legacy {
		t.Errorf("ConfigFileUsed() = %q, want the legacy file %q", got, legacy)
	}
}

// When both exist, the ~/.config file wins over the legacy one.
func TestInitViper_prefersDotConfigOverLegacy(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	current := filepath.Join(home, ".config", "treemand", "config.yaml")
	legacy := filepath.Join(home, "Library", "Application Support", "treemand", "config.yaml")
	for _, p := range []string{current, legacy} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("depth: 3\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	viper.Reset()
	t.Cleanup(viper.Reset)
	if err := config.InitViper(""); err != nil {
		t.Fatalf("InitViper: %v", err)
	}
	if got := viper.ConfigFileUsed(); got != current {
		t.Errorf("ConfigFileUsed() = %q, want %q", got, current)
	}
}

// Override files sit beside config.yaml, in treemand/overrides.
func TestOverridesDir_isBesideTheConfigFile(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, err := config.OverridesDir()
	if want := filepath.Join(xdg, "treemand", "overrides"); err != nil || got != want {
		t.Errorf("OverridesDir() = %q, %v; want %q", got, err, want)
	}
}

// Saved presets sit beside config.yaml, in treemand/presets.
func TestPresetsDir_isBesideTheConfigFile(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, err := config.PresetsDir()
	if want := filepath.Join(xdg, "treemand", "presets"); err != nil || got != want {
		t.Errorf("PresetsDir() = %q, %v; want %q", got, err, want)
	}
}

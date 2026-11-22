package config

import (
	"path/filepath"
	"testing"
)

func withGOOS(t *testing.T, os string) {
	t.Helper()
	old := goos
	goos = os
	t.Cleanup(func() { goos = old })
}

func TestConfigHome_perPlatform(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	for _, os := range []string{"linux", "darwin", "freebsd"} {
		withGOOS(t, os)
		if got, err := configHome(); err != nil || got != filepath.Join(home, ".config") {
			t.Errorf("%s: configHome() = %q, %v; want ~/.config", os, got, err)
		}
	}
	withGOOS(t, "windows")
	if _, err := configHome(); err != nil {
		t.Errorf("windows: configHome() error %v", err)
	}
}

func TestConfigHome_absoluteXDGWinsEverywhere(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	for _, os := range []string{"linux", "darwin", "windows"} {
		withGOOS(t, os)
		if got, _ := configHome(); got != xdg {
			t.Errorf("%s: configHome() = %q, want %q", os, got, xdg)
		}
	}
}

func TestConfigHome_noHome(t *testing.T) {
	withGOOS(t, "linux")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if _, err := configHome(); err == nil {
		t.Error("configHome() with no HOME should fail")
	}
	withGOOS(t, "darwin")
	if got := legacyConfigDir(); got != "" {
		t.Errorf("legacyConfigDir() with no HOME = %q, want empty", got)
	}
}

func TestLegacyConfigDir_onlyOnMacOS(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withGOOS(t, "linux")
	if got := legacyConfigDir(); got != "" {
		t.Errorf("linux: legacyConfigDir() = %q, want empty", got)
	}
	withGOOS(t, "darwin")
	want := filepath.Join(home, "Library", "Application Support", "treemand")
	if got := legacyConfigDir(); got != want {
		t.Errorf("darwin: legacyConfigDir() = %q, want %q", got, want)
	}
}

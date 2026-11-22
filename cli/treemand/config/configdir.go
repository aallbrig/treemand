package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// goos is runtime.GOOS, a variable so tests can cover every platform's branch.
var goos = runtime.GOOS

// configHome returns the base directory treemand's config directory lives in.
//
// An absolute $XDG_CONFIG_HOME wins on every platform (the XDG spec says a
// relative one is invalid and must be ignored). Otherwise Unix systems,
// macOS included, use ~/.config like most command-line tools; Go's
// os.UserConfigDir would pick ~/Library/Application Support on macOS, which is
// meant for GUI apps. Windows keeps %AppData%.
func configHome() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" && filepath.IsAbs(x) {
		return x, nil
	}
	if goos == "windows" {
		return os.UserConfigDir()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// legacyConfigDir is where releases before ~/.config support looked on macOS.
// It is still read, after the current location, so existing configs keep
// working; it is never written. Empty on other platforms.
func legacyConfigDir() string {
	if goos != "darwin" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "treemand")
}

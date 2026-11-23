//go:build !windows

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A command saved under a name in the TUI is a plain-text line in the config
// directory, `treemand presets` lists it, and `--preset` reopens the TUI
// with it loaded.
func TestPresets_saveInTheTUIThenReopenWithPreset(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), "config")
	cfgEnv := []string{"XDG_CONFIG_HOME=" + configHome}

	s := startWithEnv(t, cfgEnv, "-i", "--no-cache", "fakecobra")
	s.waitFor("deploy", 15*time.Second)
	s.stepDownUntil("fakecobra build", 12)
	s.send("\r")
	s.waitFor("set: fakecobra build", 5*time.Second)
	s.send("\x0f") // Ctrl+O
	s.waitFor("Saved Presets", 5*time.Second)
	s.send("a")
	s.waitFor("as:", 5*time.Second)
	s.send("bld\r")
	s.waitFor("saved preset bld", 5*time.Second)
	s.send("\x03")
	s.exitCode(10 * time.Second)

	data, err := os.ReadFile(filepath.Join(configHome, "treemand", "presets", "fakecobra.txt"))
	if err != nil || string(data) != "bld  fakecobra build\n" {
		t.Fatalf("presets file = %q (%v)", data, err)
	}

	res := run(t, 10*time.Second, cfgEnv, "presets", "fakecobra")
	if res.exit != 0 || res.stdout != "bld  fakecobra build\n" {
		t.Fatalf("presets: exit %d stdout %q stderr %q", res.exit, res.stdout, res.stderr)
	}
	res = run(t, 10*time.Second, cfgEnv, "presets", "fakecobra", "--output", "json")
	var list []struct{ Name, Command string }
	if err := json.Unmarshal([]byte(res.stdout), &list); err != nil || len(list) != 1 || list[0].Name != "bld" {
		t.Fatalf("presets json = %q (%v)", res.stdout, err)
	}

	// --preset opens the TUI (no -i needed) with the command assembled in
	// the preview bar. (The footer's "loaded preset" note can be replaced by
	// the background crawl's own message, so the preview is what is checked.)
	s = startWithEnv(t, cfgEnv, "--no-cache", "--preset", "bld", "fakecobra")
	s.waitFor("► fakecobra build", 15*time.Second)
	s.send("\x03")
	s.exitCode(10 * time.Second)
}

func TestPresets_unknownAndEmpty(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), "config")
	cfgEnv := []string{"XDG_CONFIG_HOME=" + configHome}

	res := run(t, 10*time.Second, cfgEnv, "presets", "fakecobra")
	if res.exit != 0 || res.stdout != "" || !strings.Contains(res.stderr, "No presets for fakecobra yet") {
		t.Fatalf("empty: exit %d stdout %q stderr %q", res.exit, res.stdout, res.stderr)
	}

	dir := filepath.Join(configHome, "treemand", "presets")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fakecobra.txt"), []byte("# mine\nbld fakecobra build\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res = run(t, 10*time.Second, cfgEnv, "--preset", "nope", "fakecobra")
	if res.exit != 2 || !strings.Contains(res.stderr, `no saved preset "nope" for fakecobra`) ||
		!strings.Contains(res.stderr, "Saved presets: bld") {
		t.Fatalf("unknown preset: exit %d stderr %q", res.exit, res.stderr)
	}
}

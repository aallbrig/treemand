//go:build !windows

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A value changed on the TUI's settings screen (,) is written to the config
// file `treemand config` uses, keeping its comments and unknown keys, and
// `treemand config get` then returns it. An invalid value is refused with
// the same message `treemand config set` gives.
func TestSettings_editInTheTUIThenConfigGet(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), "config")
	cfgEnv := []string{"XDG_CONFIG_HOME=" + configHome}
	cfgPath := filepath.Join(configHome, "treemand", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	orig := "# my settings\nicons: ascii # keep ascii\nfuture_key: kept\n"
	if err := os.WriteFile(cfgPath, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}

	s := startWithEnv(t, cfgEnv, "-i", "--no-cache", "fakecobra")
	s.waitFor("deploy", 15*time.Second)
	s.waitFor(",:settings", 5*time.Second)
	s.send(",")
	s.waitFor("Settings", 5*time.Second)
	s.waitFor("desc_line_length", 5*time.Second)
	s.send("\x1b[B") // down: icons -> desc_line_length
	s.send("\r")
	s.waitFor("value:", 5*time.Second)
	s.send("\x15abc\r") // Ctrl+U clears the field
	s.waitFor("expected an integer", 5*time.Second)
	s.send("\x15120\r")
	s.waitFor("desc_line_length = 120", 5*time.Second)
	s.waitFor("applies on next run", 5*time.Second)
	s.send(",") // , closes the screen again
	s.waitFor(",:settings", 5*time.Second)
	s.send("q")
	s.exitCode(10 * time.Second)

	res := run(t, 10*time.Second, cfgEnv, "config", "get", "desc_line_length")
	if res.exit != 0 || res.stdout != "120\n" {
		t.Fatalf("config get: exit %d stdout %q stderr %q", res.exit, res.stdout, res.stderr)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "config file", string(data), "# my settings", "# keep ascii", "future_key: kept", "desc_line_length: 120")
}

// `treemand config get` prints the default for a key the file does not set,
// and refuses an unknown key.
func TestConfigGet_defaultAndUnknown(t *testing.T) {
	res := run(t, 10*time.Second, nil, "config", "get", "tree_style")
	if res.exit != 0 || res.stdout != "default\n" {
		t.Fatalf("config get tree_style: exit %d stdout %q stderr %q", res.exit, res.stdout, res.stderr)
	}
	res = run(t, 10*time.Second, nil, "config", "get", "icon")
	if res.exit != 2 || !strings.Contains(res.stderr, `did you mean "icons"`) {
		t.Fatalf("config get icon: exit %d stderr %q", res.exit, res.stderr)
	}
}

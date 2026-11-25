package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/cmd"
)

func TestConfigSet_writesTheKeyToTheConfigFile(t *testing.T) {
	e := newFakeEnv(t)
	cfgPath := filepath.Join(e.dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("depth: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd("--config="+cfgPath, "config", "set", "icons", "nerd")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Set icons = nerd in "+cfgPath) {
		t.Errorf("output = %q", out)
	}
	b, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(b), "icons: nerd") || !strings.Contains(string(b), "depth: 2") {
		t.Errorf("config file after set:\n%s", b)
	}
}

func TestConfigSet_createsTheDefaultConfigWhenNoneExists(t *testing.T) {
	e := newFakeEnv(t)
	want := filepath.Join(e.dir, "config", "treemand", "config.yaml")
	out, err := runCmd("config", "set", "depth", "5")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, want) {
		t.Errorf("output %q does not name %s", out, want)
	}
	b, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("no config file created: %v", err)
	}
	if !strings.Contains(string(b), "depth: \"5\"") && !strings.Contains(string(b), "depth: 5") {
		t.Errorf("depth not written:\n%s", b)
	}
}

func TestConfigSet_unknownKeySuggestsTheNearestAndListsKeys(t *testing.T) {
	newFakeEnv(t)
	_, err := runCmd("config", "set", "icon", "nerd")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{`unknown config key "icon"`, `did you mean "icons"?`, "Available keys:", "stub_threshold"} {
		if !strings.Contains(cmd.FormatError(err), want) {
			t.Errorf("error missing %q:\n%s", want, cmd.FormatError(err))
		}
	}
	if got := cmd.ExitCodeFor(err); got != cmd.ExitUsage {
		t.Errorf("exit code = %d, want %d: an unknown key is a usage mistake", got, cmd.ExitUsage)
	}
}

// Subcommand flags outlive a run (other tests pass --help and --force), so
// each run here sets them explicitly.
func TestConfigInit_createsOnceAndOverwritesOnlyWithForce(t *testing.T) {
	e := newFakeEnv(t)
	want := filepath.Join(e.dir, "config", "treemand", "config.yaml")
	out, err := runCmd("config", "init", "--help=false", "--force=false")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "Created "+want {
		t.Errorf("output = %q", out)
	}
	if err := os.WriteFile(want, []byte("# mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd("config", "init", "--help=false", "--force=false"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("a second init should refuse to overwrite, got %v", err)
	}
	if b, _ := os.ReadFile(want); string(b) != "# mine\n" {
		t.Errorf("the refused init changed the file:\n%s", b)
	}
	if _, err := runCmd("config", "init", "--help=false", "--force"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(want); !strings.Contains(string(b), "depth") {
		t.Errorf("--force did not rewrite the defaults:\n%s", b)
	}
}

func TestConfigEdit_createsTheDefaultAndOpensTheEditor(t *testing.T) {
	e := newFakeEnv(t)
	t.Setenv("EDITOR", e.tool(t, "fake-editor"))
	t.Setenv(fakeModeEnv, "editor")
	want := filepath.Join(e.dir, "config", "treemand", "config.yaml")
	out, err := runCmd("config", "edit", "--help=false")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Created default config at "+want) {
		t.Errorf("output = %q", out)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("no default config created: %v", err)
	}
	if calls := strings.TrimSpace(e.calls(t)); calls != want {
		t.Errorf("editor called with %q, want %q", calls, want)
	}
}

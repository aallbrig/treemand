package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakedoctorOverride corrects fakedoctor: a root description, a flag on bare,
// a command its help never lists, and broken hidden.
const fakedoctorOverride = `commands:
  "":
    description: Checks things, corrected
  bare:
    flags:
      - name: --loud
        description: Shout the result
  extra run:
    description: Run an extra check
  broken:
    hide: true
`

// writeOverride puts body at <config>/treemand/overrides/<cli>.yaml and
// returns the env that makes treemand read that config directory.
func writeOverride(t *testing.T, cli, body string) []string {
	t.Helper()
	cfg := t.TempDir()
	dir := filepath.Join(cfg, "treemand", "overrides")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, cli+".yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return []string{"XDG_CONFIG_HOME=" + cfg}
}

type overrideNode struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Flags       []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"flags"`
	Children []*overrideNode `json:"children"`
}

func (n *overrideNode) child(name string) *overrideNode {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// A user's override file corrects the tree without a new release (#95).
func TestOverrideFileCorrectsTheTree(t *testing.T) {
	env := writeOverride(t, "fakedoctor", fakedoctorOverride)
	r := run(t, limit, env, "--output=json", "--no-cache", "fakedoctor")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	var root overrideNode
	if err := json.Unmarshal([]byte(r.stdout), &root); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, r.stdout)
	}
	if root.Description != "Checks things, corrected" {
		t.Errorf("root description = %q", root.Description)
	}
	if root.child("broken") != nil {
		t.Error("broken is still in the tree")
	}
	if bare := root.child("bare"); bare == nil || len(bare.Flags) == 0 || bare.Flags[len(bare.Flags)-1].Name != "--loud" {
		t.Errorf("bare = %+v, want --loud added", bare)
	}
	if extra := root.child("extra"); extra == nil || extra.child("run") == nil || extra.child("run").Description != "Run an extra check" {
		t.Errorf("extra run was not added: %+v", extra)
	}
}

// Editing the override takes effect on the next run, even from the cache.
func TestOverrideEditTakesEffectThroughTheCache(t *testing.T) {
	env := writeOverride(t, "fakedoctor", "commands:\n  \"\":\n    description: first\n")
	// Each run gets a fresh HOME, where the cache lives; share one.
	cacheEnv := append(slices.Clone(env), "HOME="+t.TempDir())
	if r := run(t, limit, cacheEnv, "--output=json", "fakedoctor"); !strings.Contains(r.stdout, `"first"`) {
		t.Fatalf("first run: exit %d\n%s\n%s", r.exit, r.stdout, r.stderr)
	}
	path := filepath.Join(strings.TrimPrefix(env[0], "XDG_CONFIG_HOME="), "treemand", "overrides", "fakedoctor.yaml")
	if err := os.WriteFile(path, []byte("commands:\n  \"\":\n    description: second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := run(t, limit, cacheEnv, "--output=json", "fakedoctor"); !strings.Contains(r.stdout, `"second"`) {
		t.Errorf("after the edit: exit %d\n%s\n%s", r.exit, r.stdout, r.stderr)
	}
}

// A broken override fails loudly, naming the file, instead of being ignored.
func TestBrokenOverrideNamesTheFile(t *testing.T) {
	env := writeOverride(t, "fakedoctor", "commands:\n  bare:\n    hidden: true\n")
	r := run(t, limit, env, "--no-cache", "fakedoctor")
	if r.exit == 0 {
		t.Fatalf("exit 0 with a broken override\n%s", r.stdout)
	}
	mustContain(t, "stderr", r.stderr, "fakedoctor.yaml", "hidden")
}

// treemand doctor says an override was applied and where it lives.
func TestDoctorMentionsTheOverride(t *testing.T) {
	env := writeOverride(t, "fakedoctor", fakedoctorOverride)
	r := run(t, limit, env, "doctor", "fakedoctor")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	mustContain(t, "doctor report", r.stdout, "override", "fakedoctor.yaml", "6 changes")
}

// --stdin runs nothing but still applies the override.
func TestStdinAppliesTheOverride(t *testing.T) {
	env := writeOverride(t, "fakedoctor", fakedoctorOverride)
	r := runIn(t, limit, env, "fakedoctor checks things\n\nUsage:\n  fakedoctor [command]\n\nAvailable Commands:\n  good   Good things\n  broken Broken things\n",
		"--stdin", "--output=json", "fakedoctor")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	mustContain(t, "stdout", r.stdout, "Checks things, corrected")
	if strings.Contains(r.stdout, `"broken"`) {
		t.Errorf("broken is still in the tree:\n%s", r.stdout)
	}
}

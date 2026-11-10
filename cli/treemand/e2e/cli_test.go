package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

const limit = 30 * time.Second

// node mirrors the JSON output shape the tests rely on.
type node struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Discovered  bool   `json:"discovered"`
	Flags       []struct {
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
		ValueType string `json:"value_type"`
	} `json:"flags"`
	Children []node `json:"children"`
}

func (n node) child(name string) (node, bool) {
	for _, c := range n.Children {
		if c.Name == name {
			return c, true
		}
	}
	return node{}, false
}

func (n node) hasFlag(name string) bool {
	for _, f := range n.Flags {
		if f.Name == name {
			return true
		}
	}
	return false
}

func parseJSON(t *testing.T, out string) node {
	t.Helper()
	var n node
	if err := json.Unmarshal([]byte(out), &n); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	return n
}

func TestText_cobraTree(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--no-color", "fakecobra")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	mustContain(t, "text tree", r.stdout, "fakecobra", "build", "deploy", "Build things", "Deploy things", "--output")
}

func TestJSON_cobraTree(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=json", "fakecobra")
	if r.exit != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr: %q", r.exit, r.stderr)
	}
	root := parseJSON(t, r.stdout)
	if root.Name != "fakecobra" || !root.Discovered {
		t.Errorf("root = %q discovered=%v", root.Name, root.Discovered)
	}
	for _, f := range []string{"--help", "--verbose", "--quiet", "--no-cache"} {
		if !root.hasFlag(f) {
			t.Errorf("root flag %s missing: flags %v", f, root.Flags)
		}
	}
	build, ok := root.child("build")
	if !ok || build.Description != "Build things" {
		t.Fatalf("build child = %+v (found %v)", build, ok)
	}
	if !build.hasFlag("--output") {
		t.Errorf("build is missing --output: %v", build.Flags)
	}
	for _, f := range build.Flags {
		if f.Name == "--output" && (f.ValueType != "string" || f.ShortName != "o") {
			t.Errorf("--output = %+v, want short o and value type string", f)
		}
	}
	if deploy, ok := root.child("deploy"); !ok || !deploy.hasFlag("--dry-run") {
		t.Errorf("deploy child = %+v (found %v), want --dry-run", deploy, ok)
	}
}

func TestYAML_clapTree(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=yaml", "fakeclap")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	var doc struct {
		Name     string
		Flags    []struct{ Name string }
		Children []struct{ Name string }
	}
	if err := yaml.Unmarshal([]byte(r.stdout), &doc); err != nil {
		t.Fatalf("stdout is not valid YAML: %v\n%s", err, r.stdout)
	}
	if doc.Name != "fakeclap" {
		t.Errorf("name = %q", doc.Name)
	}
	var kids []string
	for _, c := range doc.Children {
		kids = append(kids, c.Name)
	}
	if strings.Join(kids, ",") != "add,list" {
		t.Errorf("children = %v, want [add list]", kids)
	}
	var flags []string
	for _, f := range doc.Flags {
		flags = append(flags, f.Name)
	}
	mustContain(t, "root flags", strings.Join(flags, ","), "--help", "--version")
}

func TestArgparse_rootFlagsAreParsed(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=json", "fakeargp")
	root := parseJSON(t, r.stdout)
	if !root.hasFlag("--config") || root.Description != "Fake argparse tool" {
		t.Errorf("root = %+v", root)
	}
}

func TestArgparse_subcommandsAreDiscovered(t *testing.T) {
	t.Skip("known gap #81: argparse {init,run} subcommands are not discovered")
	r := run(t, limit, nil, "--no-cache", "--output=json", "fakeargp")
	root := parseJSON(t, r.stdout)
	for _, name := range []string{"init", "run"} {
		if _, ok := root.child(name); !ok {
			t.Errorf("subcommand %s not discovered", name)
		}
	}
}

func TestClap_childDescriptionsComeFromTheSubcommandList(t *testing.T) {
	t.Skip("known gap #82: clap children get the 'name 1.0' version line as description")
	r := run(t, limit, nil, "--no-cache", "--output=json", "fakeclap")
	root := parseJSON(t, r.stdout)
	if add, ok := root.child("add"); !ok || add.Description != "Add a thing" {
		t.Errorf("add = %+v (found %v), want description 'Add a thing'", add, ok)
	}
}

func TestOutputFlags(t *testing.T) {
	t.Run("commands-only hides flags", func(t *testing.T) {
		r := run(t, limit, nil, "--no-cache", "--no-color", "--commands-only", "fakecobra")
		mustContain(t, "tree", r.stdout, "build", "deploy")
		if strings.Contains(r.stdout, "--verbose") || strings.Contains(r.stdout, "--output") {
			t.Errorf("--commands-only still shows flags:\n%s", r.stdout)
		}
	})
	t.Run("depth 0 keeps only the root", func(t *testing.T) {
		r := run(t, limit, nil, "--no-cache", "--no-color", "--depth=0", "fakecobra")
		if strings.Contains(r.stdout, "build") {
			t.Errorf("--depth=0 still lists children:\n%s", r.stdout)
		}
	})
	t.Run("filter narrows the tree", func(t *testing.T) {
		r := run(t, limit, nil, "--no-cache", "--no-color", "--filter=deploy", "fakecobra")
		mustContain(t, "filtered tree", r.stdout, "deploy")
		if strings.Contains(r.stdout, "Build things") {
			t.Errorf("--filter=deploy still shows build:\n%s", r.stdout)
		}
	})
}

func TestNonTTYOutputHasNoEscapeCodes(t *testing.T) {
	// stdout is a pipe here, as when treemand feeds another program.
	for name, extra := range map[string][]string{
		"default":  nil,
		"NO_COLOR": {"NO_COLOR=1"},
	} {
		t.Run(name, func(t *testing.T) {
			r := run(t, limit, extra, "--no-cache", "fakecobra")
			if strings.Contains(r.stdout, "\x1b") {
				t.Errorf("piped output contains escape codes: %q", r.stdout)
			}
			mustContain(t, "tree", r.stdout, "build", "deploy")
		})
	}
}

func TestFailurePaths(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"missing CLI", []string{"no-such-cli-xyz"}, "not found"},
		{"no argument", nil, "accepts 1 arg"},
		{"unknown flag", []string{"--no-such-flag", "fakecobra"}, "unknown flag"},
		{"unknown output format", []string{"--output=xml", "fakecobra"}, "unknown output format"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := run(t, limit, nil, c.args...)
			if r.exit == 0 {
				t.Errorf("exit 0, want non-zero")
			}
			mustContain(t, "stderr", r.stderr, c.wantStderr)
			if strings.TrimSpace(r.stdout) != "" {
				t.Errorf("errors must not write to stdout, got %q", r.stdout)
			}
		})
	}
}

func TestCLIWithoutUsableHelpStillYieldsANode(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=json", "fakenohelp")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	root := parseJSON(t, r.stdout)
	if root.Name != "fakenohelp" || len(root.Children) != 0 {
		t.Errorf("root = %+v, want a childless node named fakenohelp", root)
	}
}

func TestTimeoutBoundsADiscoveryThatNeverAnswers(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--timeout=2", "fakehang")
	if r.elapsed > 12*time.Second {
		t.Errorf("--timeout=2 took %v against a CLI that never answers", r.elapsed)
	}
	mustContain(t, "tree", r.stdout, "fakehang")
}

func TestCacheRoundTrip(t *testing.T) {
	// One home for both runs: the first writes the cache, the second reads it.
	home := t.TempDir()
	shared := []string{"HOME=" + home, "XDG_CACHE_HOME=" + filepath.Join(home, "cache")}
	first := run(t, limit, shared, "--output=json", "fakecobra")
	second := run(t, limit, shared, "--output=json", "fakecobra")
	if first.exit != 0 || second.exit != 0 {
		t.Fatalf("exit codes %d, %d (stderr %q)", first.exit, second.exit, second.stderr)
	}
	if first.stdout != second.stdout {
		t.Errorf("cached run returned a different tree:\n%s\nvs\n%s", first.stdout, second.stdout)
	}
	var dbs []string
	_ = filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".db") {
			dbs = append(dbs, p)
		}
		return nil
	})
	if len(dbs) == 0 {
		t.Error("no cache database was written under the home directory")
	}
}

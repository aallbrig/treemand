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

func TestSynopsisOnly_flagsComeFromTheUsageLine(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=json", "fakebsd")
	root := parseJSON(t, r.stdout)
	for _, name := range []string{"-H", "-L", "-P", "-E", "-X", "-d", "-s", "-x", "-f"} {
		if !root.hasFlag(name) {
			t.Errorf("flag %s missing from %+v", name, root.Flags)
		}
	}
	if root.hasFlag("-EXdsx") || len(root.Children) != 0 {
		t.Errorf("root = %+v, want single-letter flags and no subcommands", root)
	}
}

func TestClap_childDescriptionsComeFromTheSubcommandList(t *testing.T) {
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
	// Exit codes are part of the contract (documented in the README):
	// 1 = runtime failure, 2 = usage error.
	cases := []struct {
		name       string
		args       []string
		wantExit   int
		wantStderr []string
	}{
		{"missing CLI", []string{"no-such-cli-xyz"}, 1, []string{"not found", "PATH"}},
		{"no argument", nil, 2, []string{"missing CLI name", "treemand git", "--help"}},
		{"extra argument", []string{"fakecobra", "build"}, 2, []string{"one CLI name", "fakecobra build"}},
		{"unknown flag", []string{"--no-such-flag", "fakecobra"}, 2, []string{"unknown flag", "--help"}},
		{"mistyped flag", []string{"--outptu=json", "fakecobra"}, 2, []string{"unknown flag: --outptu", "Did you mean --output?"}},
		{"bad flag value", []string{"--depth=abc", "fakecobra"}, 2, []string{"--depth"}},
		{"unknown output format", []string{"--output=xml", "fakecobra"}, 2, []string{"unknown output format", "text, json, yaml or html"}},
		{"completion without shell", []string{"completion"}, 2, []string{"missing shell", "powershell"}},
		{"completion unknown shell", []string{"completion", "tcsh"}, 2, []string{"unknown shell", "tcsh"}},
		{"mistyped cache subcommand", []string{"cache", "lst"}, 2, []string{`unknown command "lst" for "treemand cache"`, "Did you mean 'treemand cache list'?"}},
		{"mistyped config subcommand", []string{"config", "veiw"}, 2, []string{`unknown command "veiw"`, "Did you mean 'treemand config view'?"}},
		{"unknown cache subcommand", []string{"cache", "zzzz"}, 2, []string{`unknown command "zzzz"`, "treemand cache --help"}},
		{"mistyped treemand subcommand", []string{"confg"}, 1, []string{"not found", "Did you mean 'treemand config'?"}},
		{"config set without arguments", []string{"config", "set"}, 2, []string{"missing key and value", "Usage: treemand config set <key> <value>", "treemand config set icons nerd"}},
		{"config set without value", []string{"config", "set", "icons"}, 2, []string{"missing value for icons", "Usage: treemand config set <key> <value>"}},
		{"config set extra argument", []string{"config", "set", "icons", "nerd", "x"}, 2, []string{"expected a key and a value, got 3", "must be quoted"}},
		{"config set unknown key", []string{"config", "set", "icns", "nerd"}, 2, []string{`unknown config key "icns"`, `did you mean "icons"?`, "Available keys:"}},
		{"doctor without CLI", []string{"doctor"}, 2, []string{"missing CLI name", "Usage: treemand doctor <cli>", "treemand doctor git"}},
		{"doctor extra argument", []string{"doctor", "git", "status"}, 2, []string{"expected one CLI name, got 2", "Usage: treemand doctor <cli>"}},
		{"cache clear extra argument", []string{"cache", "clear", "git", "gh"}, 2, []string{"expected at most one CLI name, got 2", "Usage: treemand cache clear [cli]"}},
		{"schema with argument", []string{"schema", "tree"}, 2, []string{`treemand schema takes no arguments, got "tree"`, "Usage: treemand schema"}},
		{"version with argument", []string{"version", "x"}, 2, []string{`treemand version takes no arguments, got "x"`}},
		{"config path with argument", []string{"config", "path", "x"}, 2, []string{`treemand config path takes no arguments`, "Usage: treemand config path"}},
		{"config set bad value", []string{"config", "set", "icons", "sparkles"}, 2, []string{"icons", "sparkles"}},
		{"mcp with argument", []string{"mcp", "git"}, 2, []string{`treemand mcp takes no arguments, got "git"`, "Usage: treemand mcp"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := run(t, limit, nil, c.args...)
			if r.exit != c.wantExit {
				t.Errorf("exit %d, want %d (stderr: %s)", r.exit, c.wantExit, r.stderr)
			}
			mustContain(t, "stderr", r.stderr, c.wantStderr...)
			if strings.TrimSpace(r.stdout) != "" {
				t.Errorf("errors must not write to stdout, got %q", r.stdout)
			}
		})
	}
}

func TestSuccessExitsZero(t *testing.T) {
	for _, args := range [][]string{
		{"--version"},
		{"--help"},
		{"version"},
		{"completion", "bash"},
		{"completion", "zsh"},
		{"completion", "fish"},
		{"completion", "powershell"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			r := run(t, limit, nil, args...)
			if r.exit != 0 || strings.TrimSpace(r.stdout) == "" {
				t.Errorf("exit %d, stdout %q, stderr %q", r.exit, r.stdout, r.stderr)
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
	// No root help is no tree: a bare root would pass for a CLI with no
	// commands (#229).
	if r.exit != 1 || strings.TrimSpace(r.stdout) != "" {
		t.Errorf("exit %d, stdout %q; want exit 1 and no tree", r.exit, r.stdout)
	}
	mustContain(t, "stderr", r.stderr, "help for fakehang")
}

// --timeout=0 means no overall limit (#85): the tree is discovered in full,
// not cut down to a bare root.
func TestTimeoutZeroMeansNoLimit(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=json", "--timeout=0", "fakecobra")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	root := parseJSON(t, r.stdout)
	if len(root.Children) == 0 {
		t.Errorf("--timeout=0 gave a childless root; want the full tree")
	}
}

// A negative --timeout is a usage error, reported before any probing (#85).
func TestNegativeTimeoutIsAUsageError(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--timeout=-1", "fakecobra")
	if r.exit != 2 || strings.TrimSpace(r.stdout) != "" {
		t.Errorf("exit %d, stdout %q; want exit 2 and no tree", r.exit, r.stdout)
	}
	mustContain(t, "stderr", r.stderr, "--timeout")
}

// A root whose help times out once (kubectl on a loaded CI runner, #229) is
// asked again with a longer budget, and its children are found.
func TestARootThatTimesOutOnceIsRetried(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "calls.log")
	r := run(t, limit, []string{"FAKE_LOG=" + logPath}, "--no-cache", "--output=json", "fakeslowstart")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	root := parseJSON(t, r.stdout)
	if len(root.Children) != 1 || root.Children[0].Name != "alpha" {
		t.Errorf("root = %+v, want the child alpha found by the retry", root)
	}
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

// A cache hit is near-instant: it must not run the CLI's own --version (aws
// and pip take seconds to start). The cache key follows the binary on disk.
func TestCacheHitDoesNotPayForTheVersionProbe(t *testing.T) {
	home := t.TempDir()
	shared := []string{"HOME=" + home, "XDG_CACHE_HOME=" + filepath.Join(home, "cache")}
	first := run(t, limit, shared, "--output=json", "fakeslowver")
	if first.exit != 0 {
		t.Fatalf("first run exit %d: %s", first.exit, first.stderr)
	}
	second := run(t, limit, shared, "--output=json", "fakeslowver")
	if second.exit != 0 || second.stdout != first.stdout {
		t.Fatalf("second run differs or failed (exit %d)", second.exit)
	}
	if second.elapsed > time.Second {
		t.Errorf("warm run took %v; --version takes 3s, so the cache key is still running it", second.elapsed)
	}
}

// An upgraded CLI must not be served from the old cache entry.
func TestCacheIsInvalidatedWhenTheCLIChanges(t *testing.T) {
	home := t.TempDir()
	shared := []string{"HOME=" + home, "XDG_CACHE_HOME=" + filepath.Join(home, "cache")}
	path := filepath.Join(fakeDir, "fakeupgrade")
	script := func(extra string) {
		body := "#!/bin/sh\ncase \"$1\" in\nalpha) printf 'Alpha thing\\n';;\n*) printf 'fakeupgrade does things\\n\\nAvailable Commands:\\n  alpha   Alpha thing\\n" + extra + "';;\nesac\n"
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	script("")
	before := parseJSON(t, run(t, limit, shared, "--output=json", "fakeupgrade").stdout)
	if _, ok := before.child("beta"); ok {
		t.Fatal("beta exists before the upgrade")
	}
	script("  beta    Beta thing\\n") // the "new version" adds a command and changes size
	after := parseJSON(t, run(t, limit, shared, "--output=json", "fakeupgrade").stdout)
	if _, ok := after.child("beta"); !ok {
		t.Errorf("the upgraded CLI was served from a stale cache entry: %+v", after.Children)
	}
}

// apt prints no option in any help (#65): the root takes its options from the
// man page, and every command, whose help is not its own, inherits them.
func TestApt_commandsShowTheOptionsTheManPageDocuments(t *testing.T) {
	manDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(manDir, "man"), []byte(fakeAptMan), 0o755); err != nil {
		t.Fatal(err)
	}
	path := "PATH=" + manDir + string(os.PathListSeparator) + fakeDir + string(os.PathListSeparator) + os.Getenv("PATH")
	r := run(t, limit, []string{path}, "--no-cache", "--output=json", "fakeapt")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %q", r.exit, r.stderr)
	}
	root := parseJSON(t, r.stdout)
	for _, f := range []string{"-o", "-t", "-a"} {
		if !root.hasFlag(f) {
			t.Errorf("root flag %s missing: %v", f, root.Flags)
		}
	}
	for _, name := range []string{"install", "list", "show"} {
		c, ok := root.child(name)
		if !ok {
			t.Fatalf("child %s missing", name)
		}
		if !c.hasFlag("-o") || !c.hasFlag("-t") {
			t.Errorf("%s flags = %v, want the inherited -o and -t", name, c.Flags)
		}
		if len(c.Children) != 0 {
			t.Errorf("%s has %d children, want none", name, len(c.Children))
		}
	}
}

// The go command keeps subcommand flags in "go help <cmd>" (#66): treemand
// follows the pointer "<cmd> --help" prints and reads every flag shape there.
func TestGoStyle_subcommandFlagsComeFromTheHelpCommand(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=json", "fakegotool")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	root := parseJSON(t, r.stdout)
	want := map[string][]string{
		"build": {"-o", "-race"},
		"tidy":  {"-e", "-v", "-diff", "-compat"},
	}
	for sub, flags := range want {
		c, ok := root.child(sub)
		if !ok {
			t.Fatalf("no %q node in %s", sub, r.stdout)
		}
		for _, f := range flags {
			if !c.hasFlag(f) {
				t.Errorf("%s: missing flag %s (flags: %+v)", sub, f, c.Flags)
			}
		}
	}
}

// Homebrew lists its commands only as example rows (#276): they become the
// tree, a usage row is not the description, and no command gains phantom
// children from its own examples.
func TestJSON_exampleRowsNameCommands(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=json", "fakebrew")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	root := parseJSON(t, r.stdout)
	if root.Description != "" {
		t.Errorf("root description = %q, want empty", root.Description)
	}
	for _, name := range []string{"search", "install", "update", "doctor"} {
		c, ok := root.child(name)
		if !ok {
			t.Errorf("missing command %s; children %+v", name, root.Children)
			continue
		}
		if len(c.Children) != 0 {
			t.Errorf("%s has phantom children %+v", name, c.Children)
		}
	}
	if len(root.Children) != 4 {
		t.Errorf("children = %+v, want exactly 4", root.Children)
	}
}

// bsdtar prints its modes as "-x Word" pairs on one row and one-word
// descriptions after a gap (#277): every mode is a flag, and none of the
// words is taken as a value.
func TestJSON_flagPairRowAndOneWordDescriptions(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=json", "fakebsdtar")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	root := parseJSON(t, r.stdout)
	want := map[string]bool{"-c": true, "-r": true, "-t": true, "-u": true, "-x": true, "-v": true, "-w": true}
	for _, f := range root.Flags {
		if want[f.Name] {
			if f.ValueType != "bool" {
				t.Errorf("%s value type = %q, want bool", f.Name, f.ValueType)
			}
			delete(want, f.Name)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing flags %v; got %+v", want, root.Flags)
	}
}

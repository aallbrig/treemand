package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/models"
)

func readScriptFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "completion-scripts", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func childNames(n *models.Node) []string {
	var names []string
	for _, c := range n.Children {
		names = append(names, c.Name)
	}
	return names
}

func flagByName(n *models.Node, name string) (models.Flag, bool) {
	for _, f := range n.Flags {
		if f.Name == name {
			return f, true
		}
	}
	return models.Flag{}, false
}

func childByName(t *testing.T, n *models.Node, name string) *models.Node {
	t.Helper()
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("%s has no child %q; children: %v", n.Name, name, childNames(n))
	return nil
}

func wantFlag(t *testing.T, n *models.Node, want models.Flag) {
	t.Helper()
	got, ok := flagByName(n, want.Name)
	if !ok {
		t.Fatalf("%s has no flag %s; flags: %+v", n.Name, want.Name, n.Flags)
	}
	if got != want {
		t.Errorf("%s flag %s = %+v, want %+v", n.Name, want.Name, got, want)
	}
}

func TestScriptCommands_quoting(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		dialect scriptDialect
		want    [][]string
	}{
		{"fish escapes in single quotes", `complete -d 'it\'s \\ here'`, dialectFish, [][]string{{"complete", "-d", `it's \ here`}}},
		{"fish unquoted tab escape", `a\tb ''`, dialectFish, [][]string{{"a\tb", ""}}},
		{"double quotes keep newlines", "x \"one\ntwo\"\ny", dialectFish, [][]string{{"x", "one\ntwo"}, {"y"}}},
		{"comments and semicolons", "a # note\nb; c", dialectFish, [][]string{{"a"}, {"b"}, {"c"}}},
		{"line continuation", "a \\\n  b", dialectPOSIX, [][]string{{"a", "b"}}},
		{"posix single quotes are literal", `'it'\''s'`, dialectPOSIX, [][]string{{"it's"}}},
		{"array spans lines", "c=(\n'a:x'\n'b:y'\n)\nnext", dialectPOSIX, [][]string{{"c=(", "a:x", "b:y", ")"}, {"next"}}},
		{"case arm", "  ruff)\n opts=\"-h check\"", dialectPOSIX, [][]string{{"ruff", ")"}, {"opts=-h check"}}},
		{"ansi-c quoting", `$'a\tb'`, dialectPOSIX, [][]string{{"a\tb"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scriptCommands(tt.src, tt.dialect); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("scriptCommands(%q) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestExpandBraces(t *testing.T) {
	word := scriptCommands(`'(-h --help)'{-h,--help}'[Show help]'`, dialectPOSIX)[0][0]
	got := expandBraces(word)
	want := []string{"(-h --help)-h[Show help]", "(-h --help)--help[Show help]"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expandBraces = %q, want %q", got, want)
	}
	if got := expandBraces(scriptCommands("a{b", dialectPOSIX)[0][0]); !reflect.DeepEqual(got, []string{"a{b"}) {
		t.Errorf("unmatched brace = %q, want literal", got)
	}
}

func TestParseFishScript_clapFlagsAndValues(t *testing.T) {
	root := parseFishScript(readScriptFixture(t, "watchexec.fish"), "watchexec").node()
	if root == nil {
		t.Fatal("no tree from watchexec.fish")
	}
	if len(root.Children) != 0 {
		t.Errorf("flag values were read as subcommands: %v", childNames(root))
	}
	wantFlag(t, root, models.Flag{Name: "--shell", ValueType: "string", Description: "Use a different shell"})
	wantFlag(t, root, models.Flag{Name: "--env", ShortName: "E", ValueType: "string", Description: "Add env vars to the command"})
	wantFlag(t, root, models.Flag{Name: "--completions", ValueType: "string", Description: "Generate a shell completions script"})
	wantFlag(t, root, models.Flag{Name: "--signal", ShortName: "s", ValueType: "string", Description: "Send a signal to the process when it's still running"})
}

func TestParseFishScript_clapNestedSubcommands(t *testing.T) {
	root := parseFishScript(readScriptFixture(t, "ruff.fish"), "ruff").node()
	want := []string{"check", "rule", "config", "linter", "clean", "generate-shell-completion", "format", "server", "analyze", "version", "help"}
	if got := childNames(root); !reflect.DeepEqual(got, want) {
		t.Errorf("root children = %v, want %v", got, want)
	}
	if check := childByName(t, root, "check"); check.Description != "Run Ruff on the given files or directories" {
		t.Errorf("check description = %q", check.Description)
	}
	wantFlag(t, root, models.Flag{Name: "--verbose", ShortName: "v", ValueType: "bool", Description: "Enable verbose logging"})
	wantFlag(t, root, models.Flag{Name: "--color", ValueType: "string", Description: "Control when colored output is used"})

	analyze := childByName(t, root, "analyze")
	if got := childNames(analyze); !reflect.DeepEqual(got, []string{"graph", "help"}) {
		t.Errorf("analyze children = %v, want [graph help]", got)
	}
	wantFlag(t, analyze, models.Flag{Name: "--verbose", ShortName: "v", ValueType: "bool", Description: "Enable verbose logging"})
	graph := childByName(t, analyze, "graph")
	if !reflect.DeepEqual(graph.FullPath, []string{"ruff", "analyze", "graph"}) {
		t.Errorf("graph FullPath = %v", graph.FullPath)
	}
	wantFlag(t, graph, models.Flag{Name: "--min-dots", ValueType: "string", Description: "The minimum number of dots in a string import to consider it a valid import"})
	wantFlag(t, graph, models.Flag{Name: "--python", ValueType: "string", Description: "Path to a virtual environment to use for resolving additional dependencies"})
}

func TestParseFishScript_seenSubcommandFrom(t *testing.T) {
	root := parseFishScript(readScriptFixture(t, "poetry.fish"), "poetry").node()
	for _, name := range []string{"about", "add", "cache", "debug", "env", "self", "show"} {
		childByName(t, root, name)
	}
	if got := childNames(childByName(t, root, "cache")); !reflect.DeepEqual(got, []string{"clear", "list"}) {
		t.Errorf("cache children = %v, want [clear list]", got)
	}
	if d := childByName(t, childByName(t, root, "debug"), "resolve").Description; d != "Debugs dependency resolution." {
		t.Errorf("debug resolve description = %q", d)
	}
	wantFlag(t, root, models.Flag{Name: "--no-ansi", ValueType: "bool", Description: "Disable ANSI output."})
}

func TestParseFishScript_skipsWhatItCannotPlace(t *testing.T) {
	src := strings.Join([]string{
		`complete -c tool -n '__fish_use_subcommand' -a 'start stop' -d 'lifecycle'`,
		`complete -c tool -n '__fish_use_subcommand' -a '(__fish_tool_dynamic)'`,
		`complete -c tool -n '__fish_tool_custom_test' -l mystery`,
		`complete -c tool -n '__fish_seen_subcommand_from a; or __fish_seen_subcommand_from b' -l either`,
		`complete -c other -l not-mine`,
		`complete -c tool -n '__fish_seen_subcommand_from start' -a 'fileA fileB'`,
		`complete -c tool -e -l erased`,
		`complete -c tool -l plain -s p -x`,
	}, "\n")
	root := parseFishScript(src, "tool").node()
	if got := childNames(root); !reflect.DeepEqual(got, []string{"start", "stop"}) {
		t.Errorf("children = %v, want [start stop]", got)
	}
	if got := childByName(t, root, "start"); len(got.Children) != 0 || got.Description != "" {
		t.Errorf("start = %+v; values or a shared description leaked in", got)
	}
	if len(root.Flags) != 1 {
		t.Fatalf("flags = %+v, want only --plain", root.Flags)
	}
	wantFlag(t, root, models.Flag{Name: "--plain", ShortName: "p", ValueType: "string"})
}

func TestParseZshScript_clapRootFlagsAndCommandArrays(t *testing.T) {
	root := parseZshScript(readScriptFixture(t, "_ruff"), "ruff").node()
	childByName(t, root, "check")
	if d := childByName(t, root, "version").Description; d != "Display Ruff's version" {
		t.Errorf("version description = %q", d)
	}
	if got := childNames(childByName(t, root, "analyze")); !reflect.DeepEqual(got, []string{"graph", "help"}) {
		t.Errorf("analyze children = %v, want [graph help]", got)
	}
	wantFlag(t, root, models.Flag{Name: "--verbose", ShortName: "v", ValueType: "bool", Description: "Enable verbose logging"})
	wantFlag(t, root, models.Flag{Name: "--color", ValueType: "string", Description: "Control when colored output is used"})
	wantFlag(t, root, models.Flag{Name: "--isolated", ValueType: "bool", Description: "Ignore all configuration files"})

	watch := parseZshScript(readScriptFixture(t, "_watchexec"), "watchexec").node()
	wantFlag(t, watch, models.Flag{Name: "--env", ShortName: "E", ValueType: "string", Description: "Add env vars to the command"})
	wantFlag(t, watch, models.Flag{Name: "--signal", ShortName: "s", ValueType: "string", Description: "Send a signal to the process when it's still running"})
}

func TestZshFlag_specShapes(t *testing.T) {
	tests := []struct {
		spec string
		want models.Flag
	}{
		{"(-q --quiet)--quiet[Be quiet]", models.Flag{Name: "--quiet", ValueType: "bool", Description: "Be quiet"}},
		{"*--env=[Set env]:KEY=VALUE:_default", models.Flag{Name: "--env", ValueType: "string", Description: "Set env"}},
		{"-o+[Output file]:FILE:_files", models.Flag{Name: "-o", ValueType: "string", Description: "Output file"}},
		{"--depth[Depth \\] here]:N:", models.Flag{Name: "--depth", ValueType: "string", Description: "Depth ] here"}},
		{"--bare", models.Flag{Name: "--bare", ValueType: "bool"}},
	}
	for _, tt := range tests {
		got, ok := zshFlag(tt.spec)
		if !ok || got != tt.want {
			t.Errorf("zshFlag(%q) = %+v, %v; want %+v", tt.spec, got, ok, tt.want)
		}
	}
	for _, spec := range []string{":: :_cmds", "*::: :->x", "1:file:_files", "(--x"} {
		if f, ok := zshFlag(spec); ok {
			t.Errorf("zshFlag(%q) = %+v, want no flag", spec, f)
		}
	}
}

func TestParseBashScript_rootWords(t *testing.T) {
	clap := parseBashScript(readScriptFixture(t, "watchexec.bash-completion"), "watchexec").node()
	if len(clap.Children) != 0 {
		t.Errorf("watchexec children = %v; flag values were read as subcommands", childNames(clap))
	}
	wantFlag(t, clap, models.Flag{Name: "--shell", ValueType: "bool"})
	if _, ok := flagByName(clap, "-E"); !ok {
		t.Errorf("watchexec is missing -E: %+v", clap.Flags)
	}

	gapp := parseBashScript(readScriptFixture(t, "gapplication.bash-completion"), "gapplication").node()
	want := []string{"help", "version", "list-apps", "launch", "action", "list-actions"}
	if got := childNames(gapp); !reflect.DeepEqual(got, want) {
		t.Errorf("gapplication children = %v, want %v", got, want)
	}
}

func TestParseBashScript_ignoresListsForLaterArguments(t *testing.T) {
	src := "_tool() {\n  case \"$prev\" in\n    --format) COMPREPLY=($(compgen -W \"json yaml\" -- \"$cur\")) ;;\n  esac\n}\n"
	if n := parseBashScript(src, "tool").node(); n != nil {
		t.Errorf("tree = %+v, want nil: the list completes a flag value", n)
	}
}

func TestScriptsDiscoverer_prefersFishAndNeverReadsOversizedFiles(t *testing.T) {
	fish, zsh := t.TempDir(), t.TempDir()
	write := func(path, text string) {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(fish, "tool.fish"), "complete -c tool -n __fish_use_subcommand -a fromfish\n")
	write(filepath.Join(zsh, "_tool"), "_tool_commands() {\n local commands; commands=('fromzsh:z')\n}\n")
	d := &ScriptsDiscoverer{Dirs: func() ScriptDirs { return ScriptDirs{Fish: []string{fish}, Zsh: []string{zsh}} }}

	node, err := d.Discover(t.Context(), "tool", nil)
	if err != nil || node == nil {
		t.Fatalf("Discover = %v, %v", node, err)
	}
	if got := childNames(node); !reflect.DeepEqual(got, []string{"fromfish"}) {
		t.Errorf("children = %v, want [fromfish]", got)
	}

	old := MaxScriptBytes
	t.Cleanup(func() { MaxScriptBytes = old })
	MaxScriptBytes = 10
	node, _ = d.Discover(t.Context(), "tool", nil)
	if node != nil {
		t.Errorf("oversized scripts were read: %+v", node)
	}
}

func TestScriptsDiscoverer_answersOnlyPlainRootNames(t *testing.T) {
	d := &ScriptsDiscoverer{Dirs: func() ScriptDirs { return ScriptDirs{Fish: []string{t.TempDir()}} }}
	for _, tc := range []struct {
		cli  string
		args []string
	}{{"tool", []string{"sub"}}, {"..", nil}, {"-rf", nil}, {"nothing-installed", nil}} {
		if n, err := d.Discover(t.Context(), tc.cli, tc.args); n != nil || err != nil {
			t.Errorf("Discover(%q, %v) = %v, %v; want nil, nil", tc.cli, tc.args, n, err)
		}
	}
}

func TestDefaultScriptDirs_followXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/data/home")
	t.Setenv("XDG_DATA_DIRS", "/one/share:relative:/two/share")
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	d := DefaultScriptDirs()
	wantPrefix := []string{"/cfg/fish/completions", "/data/home/fish/vendor_completions.d", "/data/home/fish/completions", "/one/share/fish/vendor_completions.d"}
	if !reflect.DeepEqual(d.Fish[:4], wantPrefix) {
		t.Errorf("fish dirs = %v, want prefix %v", d.Fish, wantPrefix)
	}
	for _, dir := range append(append(d.Fish, d.Zsh...), d.Bash...) {
		if strings.Contains(dir, "relative") {
			t.Errorf("relative XDG_DATA_DIRS entry searched: %s", dir)
		}
	}
	if !strings.Contains(strings.Join(d.Zsh, " "), "/two/share/zsh/site-functions") {
		t.Errorf("zsh dirs = %v", d.Zsh)
	}
}

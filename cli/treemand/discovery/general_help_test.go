package discovery_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/discovery"
	"github.com/aallbrig/treemand/cli/treemand/models"
)

// apt's options appear in no help: its man page SYNOPSIS gives them.
func TestSynopsisFlags_equalsValue(t *testing.T) {
	flags := discovery.SynopsisFlags("apt [-h] [-o=config_string] [-c=config_file] [-t=target_release]")
	want := map[string]string{"-h": "bool", "-o": "config_string", "-c": "config_file", "-t": "target_release"}
	if len(flags) != len(want) {
		t.Fatalf("got %+v, want %d flags", flags, len(want))
	}
	for _, f := range flags {
		if want[f.Name] != f.ValueType {
			t.Errorf("%s value = %q, want %q", f.Name, f.ValueType, want[f.Name])
		}
	}
}

func TestManFlags_aptSynopsis(t *testing.T) {
	b, err := os.ReadFile("testdata/apt_man_synopsis.txt")
	if err != nil {
		t.Fatal(err)
	}
	names := flagNames(discovery.ManFlags(string(b)))
	for _, n := range []string{"-h", "-o", "-c", "-t", "-a"} {
		if !names[n] {
			t.Errorf("ManFlags missing %s (got %v)", n, names)
		}
	}
}

func flagNames(flags []models.Flag) map[string]bool {
	m := map[string]bool{}
	for _, f := range flags {
		m[f.Name] = true
	}
	return m
}

// writeAptLike installs a CLI shaped like apt: the root help lists commands
// and no options, every subcommand prints the upstream tool's general help
// (or fails, as the Linux Mint wrapper does when it needs sudo), and only the
// man page names the options.
func writeAptLike(t *testing.T) {
	t.Helper()
	general, err := os.ReadFile("testdata/apt_general_help.txt")
	if err != nil {
		t.Fatal(err)
	}
	man, err := os.ReadFile("testdata/apt_man_synopsis.txt")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, body := range map[string]string{"general.txt": string(general), "man.txt": string(man)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cli := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"install) echo 'sudo: a terminal is required to read the password' >&2; exit 1;;\n" +
		"list|show|search) cat '" + filepath.Join(dir, "general.txt") + "';;\n" +
		"*) printf 'Usage: aptlike command [options]\\n\\nCommands:\\n  install          - Install packages\\n  list             - List packages\\n  search           - Search for a package\\n  show             - Display a package\\n';;\n" +
		"esac\n"
	manScript := "#!/bin/sh\n[ \"$1\" = aptlike ] || exit 16\ncat '" + filepath.Join(dir, "man.txt") + "'\n"
	for name, body := range map[string]string{"aptlike": cli, "man": manScript} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestHelpDiscoverer_aptLike_rootTakesManOptions(t *testing.T) {
	writeAptLike(t)
	node, err := discovery.NewHelpDiscoverer(3).Discover(context.Background(), "aptlike", nil)
	if err != nil {
		t.Fatal(err)
	}
	if names := flagNames(node.Flags); !names["-o"] || !names["-t"] {
		t.Errorf("root flags = %v, want the man page's -o and -t", names)
	}
}

func TestHelpDiscoverer_aptLike_subcommandsInheritOptions(t *testing.T) {
	writeAptLike(t)
	node, err := discovery.NewHelpDiscoverer(3).Discover(context.Background(), "aptlike", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Children) != 4 {
		t.Fatalf("got %d children, want 4", len(node.Children))
	}
	for _, c := range node.Children {
		if len(c.Children) != 0 {
			t.Errorf("%s: descended into the general help (%d children)", c.Name, len(c.Children))
		}
		names := flagNames(c.Flags)
		if !names["-o"] {
			t.Errorf("%s flags = %v, want the root's -o", c.Name, names)
		}
		for _, f := range c.Flags {
			if !f.Inherited {
				t.Errorf("%s flag %s should be marked inherited", c.Name, f.Name)
			}
		}
	}
}

// The root's options go to a flagless child only when the root's usage says
// they go with any command; "own [-q]" says nothing of the kind.
func TestHelpDiscoverer_rootOptionsOnlyWhenUsageTakesThemWithAnyCommand(t *testing.T) {
	dir := t.TempDir()
	cli := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"run) printf 'Usage: own run NAME\\n\\nRun a thing.\\n';;\n" +
		"*) printf 'Usage: own [-q]\\n       own run NAME\\n\\nOptions:\\n  -q   quiet\\n\\nCommands:\\n  run    - Run a thing\\n';;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "own"), []byte(cli), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	node, err := discovery.NewHelpDiscoverer(2).Discover(context.Background(), "own", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Children) != 1 || len(node.Children[0].Flags) != 0 {
		t.Fatalf("children = %+v, want run with no flags", node.Children)
	}
}

// The TUI lists commands as stubs and crawls them one at a time; the stubs
// carry the options too, and a crawl that finds none keeps them.
func TestCrawler_aptLike_stubsKeepTheOptions(t *testing.T) {
	writeAptLike(t)
	d := discovery.NewHelpDiscoverer(3)
	d.ChildrenAsStubs = true
	root, err := d.Discover(context.Background(), "aptlike", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := discovery.CompletePending(context.Background(), root, "aptlike", discovery.ExpandOptions{MaxDepth: 3}, 150, 4); got != 4 {
		t.Fatalf("filled %d stubs, want 4", got)
	}
	for _, c := range root.Children {
		if c.Stub || !flagNames(c.Flags)["-o"] {
			t.Errorf("%s: stub=%v flags=%v, want a filled node with -o", c.Name, c.Stub, flagNames(c.Flags))
		}
	}
}

// A root whose help lists its options leaves its commands' flags to their own
// helps, even when its usage takes options with any command (docker).
func TestHelpDiscoverer_listedRootOptionsAreNotPushedDown(t *testing.T) {
	dir := t.TempDir()
	cli := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"run) printf 'Usage: listed run NAME\\n\\nRun a thing.\\n';;\n" +
		"*) printf 'Usage: listed [OPTIONS] COMMAND\\n\\nOptions:\\n  -q   quiet\\n\\nCommands:\\n  run    - Run a thing\\n';;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "listed"), []byte(cli), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	node, err := discovery.NewHelpDiscoverer(2).Discover(context.Background(), "listed", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Children) != 1 || len(node.Children[0].Flags) != 0 {
		t.Fatalf("children = %+v, want run with no flags", node.Children[0])
	}
}

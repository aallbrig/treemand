package discovery_test

import (
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/discovery"
)

// Single-dash long options (issue #156): Go's flag package and mitchellh/cli.
func TestParseHelp_singleDashLongOptions(t *testing.T) {
	cases := map[string]struct {
		help string
		want map[string]string // flag -> value type
	}{
		"go flag (gofmt)": {
			"usage: gofmt [flags] [path ...]\n" +
				"  -cpuprofile string\n    \twrite cpu profile to this file\n" +
				"  -d\tdisplay diffs instead of rewriting files\n" +
				"  -l\tlist files whose formatting differs from gofmt's\n" +
				"  -r string\n    \trewrite rule (e.g., 'a[b:len(a)] -> a[b:]')\n" +
				"  -s\tsimplify code\n  -w\twrite result to (source) file instead of stdout\n",
			map[string]string{"-cpuprofile": "string", "-d": "bool", "-l": "bool", "-r": "string", "-w": "bool"},
		},
		"mitchellh/cli (terraform)": {
			"Usage: terraform [global options] <subcommand> [args]\n\nGlobal options (use these before the subcommand, if any):\n" +
				"  -chdir=DIR    Switch to a different working directory before executing the\n" +
				"                given subcommand.\n" +
				"  -help         Show this help output, or the help for a specified subcommand.\n" +
				"  -version      An alias for the \"version\" subcommand.\n",
			map[string]string{"-chdir": "DIR", "-help": "bool", "-version": "bool"},
		},
		"compound placeholder (cmake)": {
			"Usage\n\nOptions\n  -D <var>[:<type>]=<value>    = Create or update a cmake cache entry.\n  -Wdev                        = Enable developer warnings.\n",
			map[string]string{"-D": "<var>[:<type>]=<value>", "-Wdev": "bool"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := map[string]string{}
			for _, f := range discovery.ParseHelpOutput(c.help).Flags {
				got[f.Name] = f.ValueType
			}
			for flag, vt := range c.want {
				if got[flag] != vt {
					t.Errorf("%s: value type %q, want %q (all: %v)", flag, got[flag], vt, got)
				}
			}
		})
	}
}

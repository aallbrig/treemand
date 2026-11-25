package discovery_test

import (
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/discovery"
)

// BSD tools and ssh/tmux answer --help with only a usage synopsis (#274):
// clustered letters ("[-EXdsx]"), alternatives ("[-H | -L | -P]") and
// value options, often wrapped over indented continuation lines.
func TestParseHelp_flagsFromSynopsisOnlyHelp(t *testing.T) {
	cases := map[string]struct {
		help    string
		want    map[string]string
		notWant []string
	}{
		"find@bsd": {
			help: "find: illegal option -- -\n" +
				"usage: find [-H | -L | -P] [-EXdsx] [-f path] path ... [expression]\n" +
				"       find [-H | -L | -P] [-EXdsx] -f path [path ...] [expression]\n",
			want:    map[string]string{"-H": "bool", "-L": "bool", "-P": "bool", "-E": "bool", "-X": "bool", "-d": "bool", "-s": "bool", "-x": "bool", "-f": "path"},
			notWant: []string{"-EXdsx"},
		},
		"ls@bsd": {
			help: "usage: ls [-@ABCFGHILOPRSTUWXabcdefghiklmnopqrstuvwxy1%,] [--color=when] [-D format] [file ...]\n",
			want: map[string]string{"-@": "bool", "-a": "bool", "-l": "bool", "-1": "bool", "-%": "bool", "-,": "bool", "--color": "when", "-D": "format"},
		},
		"sed@bsd continuation": {
			help: "usage: sed script [-EHalnru] [-i extension] [file ...]\n" +
				"\tsed [-EHalnu] [-i extension] [-e script] ... [-f script_file] ... [file ...]\n",
			want: map[string]string{"-E": "bool", "-n": "bool", "-i": "extension", "-e": "script", "-f": "script_file"},
		},
		"ssh wrapped": {
			help: "usage: ssh [-46AaCfGgKkMNnqsTtVvXxYy] [-B bind_interface] [-b bind_address]\n" +
				"           [-c cipher_spec] [-D [bind_address:]port] [-E log_file]\n" +
				"           destination [command [argument ...]]\n" +
				"       ssh [-Q query_option]\n",
			want: map[string]string{"-4": "bool", "-A": "bool", "-y": "bool", "-B": "bind_interface", "-c": "cipher_spec", "-E": "log_file", "-Q": "query_option"},
		},
		"tmux wrapped": {
			help: "usage: tmux [-2CDlNuVv] [-c shell-command] [-f file] [-L socket-name]\n" +
				"            [-S socket-path] [-T features] [command [flags]]\n",
			want: map[string]string{"-2": "bool", "-v": "bool", "-c": "shell-command", "-S": "socket-path", "-T": "features"},
		},
		// A lowercase cluster counts once another bracket proves the tool clusters.
		"lowercase cluster beside a proven one": {
			help: "usage: rm [-f | -i] [-dIPRrvWx] [-pv] file ...\n",
			want: map[string]string{"-f": "bool", "-i": "bool", "-I": "bool", "-p": "bool", "-v": "bool"},
		},
		// A synopsis short option that a row lists as an alias is not another flag.
		"alias of a listed long option": {
			help: "usage: tool [-q] [-B SECONDS] [-EXd]\n\nOptions:\n" +
				"  -B SECONDS, --background SECONDS\n                run in the background\n" +
				"  -E, --extended  extended\n",
			want:    map[string]string{"--background": "SECONDS", "--extended": "bool", "-q": "bool"},
			notWant: []string{"-B", "-E"},
		},
		// Go word options are not clusters.
		"go word options": {
			help:    "usage: go list [-f format] [-json] [-m] [packages]\n",
			want:    map[string]string{"-json": "bool", "-m": "bool", "-f": "format"},
			notWant: []string{"-j", "-s", "-o"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := discovery.ParseHelpOutput(c.help)
			got := flagTypes(p)
			for flag, vt := range c.want {
				if got[flag] != vt {
					t.Errorf("%s: value type %q, want %q (all: %v)", flag, got[flag], vt, got)
				}
			}
			for _, flag := range c.notWant {
				if _, ok := got[flag]; ok {
					t.Errorf("%s should not be a flag (all: %v)", flag, got)
				}
			}
			if len(p.Subcommands) != 0 {
				t.Errorf("subcommands %v, want none", p.Subcommands)
			}
		})
	}
}

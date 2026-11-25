package discovery_test

import (
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/discovery"
)

func flagTypes(p discovery.ParsedHelp) map[string]string {
	got := map[string]string{}
	for _, f := range p.Flags {
		got[f.Name] = f.ValueType
	}
	return got
}

// The go command documents flags in "go help <cmd>" (#66): tab-indented
// "-name value" rows with the description on the following, deeper line.
func TestParseHelp_goHelpTabIndentedFlagBlock(t *testing.T) {
	help := "usage: go build [-o output] [build flags] [packages]\n\n" +
		"Build compiles the packages named by the import paths.\n\n" +
		"The build flags are shared by the build, clean, get, install, list, run,\n" +
		"and test commands:\n\n" +
		"\t-C dir\n\t\tChange to dir before running the command.\n" +
		"\t-a\n\t\tforce rebuilding of packages that are already up-to-date.\n" +
		"\t-covermode set,count,atomic\n\t\tset the mode for coverage analysis.\n" +
		"\t-race\n\t\tenable data race detection.\n"
	p := discovery.ParseHelpOutput(help)
	got := flagTypes(p)
	want := map[string]string{"-C": "dir", "-a": "bool", "-covermode": "set,count,atomic", "-race": "bool", "-o": "output"}
	for name, vt := range want {
		if got[name] != vt {
			t.Errorf("%s: value type %q, want %q (all: %v)", name, got[name], vt, got)
		}
	}
	for _, f := range p.Flags {
		if f.Name == "-race" && f.Description != "enable data race detection." {
			t.Errorf("-race description %q, want the line below it", f.Description)
		}
	}
}

// A synopsis that lists options one per bracket ("[-e] [-v] [-diff]") names
// the CLI's flags even when the prose documents them ("The -v flag causes...").
func TestParseHelp_flagsFromUsageSynopsisBrackets(t *testing.T) {
	cases := map[string]struct {
		help    string
		want    map[string]string
		notWant []string
	}{
		"go mod tidy": {
			help: "usage: go mod tidy [-e] [-v] [-x] [-diff] [-go=version] [-compat=version]\n\n" +
				"Tidy makes sure go.mod matches the source code in the module.\n\n" +
				"The -v flag causes tidy to print information about removed modules.\n",
			want: map[string]string{"-e": "bool", "-v": "bool", "-diff": "bool", "-go": "version", "-compat": "version"},
		},
		"go list": {
			help: "usage: go list [-f format] [-json] [-m] [list flags] [build flags] [packages]\n\nList lists the named packages.\n",
			want: map[string]string{"-f": "format", "-json": "bool", "-m": "bool"},
		},
		"long options": {
			help: "usage: tool [--verbose] [--config <file>] [-C <path>] <arg>\n",
			want: map[string]string{"--verbose": "bool", "--config": "file", "-C": "path"},
		},
		// BSD tools cluster single-letter options: "-pv" is -p and -v, not a
		// flag named "-pv". Without a lone "[-x]" bracket the cluster stays out.
		"BSD cluster": {
			help:    "usage: mkdir [-pv] [-m mode] directory_name ...\n",
			want:    map[string]string{"-m": "mode"},
			notWant: []string{"-pv"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := flagTypes(discovery.ParseHelpOutput(c.help))
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
		})
	}
}

// "go help mod edit" documents its flags only in prose paragraphs that open
// with "The -name flag" (#66).
func TestParseHelp_flagsFromProseThatNamesThem(t *testing.T) {
	help := "usage: go mod edit [editing flags] [-fmt|-print|-json] [go.mod]\n\n" +
		"Edit provides a command-line interface for editing go.mod.\n\n" +
		"The -fmt flag reformats the go.mod file without making other changes.\n\n" +
		"The -go=version flag sets the expected Go language version.\n\n" +
		"The -require=path@version and -droprequire=path flags\n" +
		"add and drop a requirement on the given module path and version.\n\n" +
		"Note that -require overrides any existing requirements on path.\n"
	got := flagTypes(discovery.ParseHelpOutput(help))
	want := map[string]string{"-fmt": "bool", "-go": "version", "-require": "path@version", "-droprequire": "path"}
	for name, vt := range want {
		if got[name] != vt {
			t.Errorf("%s: value type %q, want %q (all: %v)", name, got[name], vt, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("flags %v, want only %v", got, want)
	}
}

// Flags read from the usage line belong to no flag section.
func TestParseHelp_usageLineFlagsJoinNoSection(t *testing.T) {
	help := "usage: tool [-q] [--color <when>]\n\nGeneral options:\n  -a, --all    all\n  -b, --brief  brief\n\nDebug options:\n  -d, --debug  debug\n  -t, --trace  trace\n"
	p := discovery.ParseHelpOutput(help)
	if got := flagTypes(p); got["-q"] != "bool" || got["--color"] != "when" {
		t.Fatalf("usage flags missing: %v", got)
	}
	for _, s := range p.Sections {
		for _, f := range s.Flags {
			if f.Name == "-q" || f.Name == "--color" {
				t.Errorf("usage-line flag %s filed under section %q", f.Name, s.Name)
			}
		}
	}
}

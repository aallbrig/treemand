package discovery_test

import (
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/discovery"
)

// Option shapes from real help (issue #161), copied from the corpus fixtures.
func TestParseHelp_optionShapes(t *testing.T) {
	cases := []struct {
		family, line, long, short, value string
	}{
		{"GNU value after short", "  -C DIRECTORY, --directory=DIRECTORY\n                              Change to DIRECTORY before doing anything.", "--directory", "C", "DIRECTORY"},
		{"GNU three names", "  -f FILE, --file=FILE, --makefile=FILE\n                              Read FILE as a makefile.", "--file", "f", "FILE"},
		{"GNU optional value", "  -j [N], --jobs[=N]          Allow N jobs at once; infinite jobs with no arg.", "--jobs", "j", "N"},
		{"node ellipsis value", "  -e, --eval=...              evaluate script", "--eval", "e", "..."},
		{"node nested optional", "  --inspect[=[host:]port]     activate inspector on host:port", "--inspect", "", "[host:]port"},
		{"sed attached optional", "  -i[SUFFIX], --in-place[=SUFFIX]\n                 edit files in place", "--in-place", "i", "SUFFIX"},
		{"sed two longs", "  -n, --quiet, --silent\n                 suppress automatic printing of pattern space", "--quiet", "n", "bool"},
		{"sed two shorts", "  -E, -r, --regexp-extended\n                 use extended regular expressions", "--regexp-extended", "E", "bool"},
		{"tar alias", "  -x, --extract, --get       extract files from an archive", "--extract", "x", "bool"},
		{"clap repeatable", "  -p, --plain...\n          Only show plain style, no decorations.", "--plain", "p", "bool"},
		{"httpie long first", "  --json, -j\n      (default) Data items from the command line are serialized as a JSON object.", "--json", "j", "bool"},
		{"maven multi-letter short", " -pl,--projects <arg>                   Comma-delimited list of specified", "--projects", "pl", "arg"},
		{"rg indented value", "    -g GLOB, --glob=GLOB\n        Include or exclude files and directories for searching.", "--glob", "g", "GLOB"},
	}
	for _, c := range cases {
		t.Run(c.family, func(t *testing.T) {
			p := discovery.ParseHelpOutput("Usage: tool [OPTIONS]\n\nOptions:\n" + c.line + "\n")
			for _, f := range p.Flags {
				if f.Name == c.long {
					if f.ShortName != c.short {
						t.Errorf("%s: short = %q, want %q", c.long, f.ShortName, c.short)
					}
					if f.ValueType != c.value {
						t.Errorf("%s: value = %q, want %q", c.long, f.ValueType, c.value)
					}
					return
				}
			}
			t.Errorf("%s not parsed from %q; got %+v", c.long, c.line, p.Flags)
		})
	}
}

// A wrapped description line that starts with a header word and a parenthesis
// ("name (like 'C++' …") is prose, not a section: bat lost every flag after it.
func TestParseHelp_proseWithParenthesisIsNotAHeader(t *testing.T) {
	help := "Options:\n" +
		"  -l, --language <language>\n" +
		"          Set the language for syntax highlighting. The language can be specified as a\n" +
		"          name (like 'C++' or 'LaTeX') or possible file extension (like 'cpp', 'hpp' or 'md'). Use\n" +
		"          '--list-languages' to show all supported language names and file extensions.\n" +
		"\n" +
		"      --paging <when>\n" +
		"          Specify when to use the pager.\n"
	p := discovery.ParseHelpOutput(help)
	for _, f := range p.Flags {
		if f.Name == "--paging" {
			return
		}
	}
	t.Errorf("--paging lost after a prose line; got %+v", p.Flags)
}

func TestParseHelp_choiceListValue(t *testing.T) {
	p := discovery.ParseHelpOutput("Usage: ruby [switches]\n  --enable={jit|rubyopt|...}[,...]\n  --disable={jit|rubyopt|...}[,...]\n                  disable features\n")
	got := map[string]string{}
	for _, f := range p.Flags {
		got[f.Name] = f.ValueType
	}
	if got["--disable"] != "{jit|rubyopt|...}" || got["--enable"] == "" {
		t.Errorf("choice-list options not parsed: %v", got)
	}
}

func TestParseHelp_kingpinNegatableFlags(t *testing.T) {
	p := discovery.ParseHelpOutput("Flags:\n  -h, --[no-]help            Show context-sensitive help.\n      --[no-]version         Show application version.\n")
	got := map[string]string{}
	for _, f := range p.Flags {
		got[f.Name] = f.ShortName
	}
	if _, ok := got["--help"]; !ok || got["--help"] != "h" {
		t.Errorf("--[no-]help not parsed: %v", got)
	}
	if _, ok := got["--version"]; !ok {
		t.Errorf("--[no-]version not parsed: %v", got)
	}
}

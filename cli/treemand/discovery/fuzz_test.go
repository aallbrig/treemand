package discovery_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

// Seeds for the help-parser fuzzers: every committed fixture (real CLI help)
// plus small hand-written shapes. `go test` runs them as ordinary tests;
// `go test -fuzz=FuzzParseHelpOutput ./discovery` mutates them. Crashing
// inputs go in testdata/fuzz/<Name>/ so they stay regression tests.
func addHelpSeeds(f *testing.F) {
	f.Helper()
	for _, pattern := range []string{"testdata/*.txt", "testdata/corpus/*.txt"} {
		files, _ := filepath.Glob(pattern)
		for _, file := range files {
			if b, err := os.ReadFile(file); err == nil {
				f.Add(string(b))
			}
		}
	}
	f.Add("usage: tool [-ab] [-c file]\n           [command [flags]]\n")
	f.Add("Tool\n\nUsage:\n  tool [command]\n\nAvailable Commands:\n  a   first\n  b   second\n\nFlags:\n  -h, --help   help\n")
	f.Add("NAME\n       tool — does things\n\nSYNOPSIS\n       tool [options]\n")
	f.Add("Options:\n    -A, --all=false:\n\tdesc\n    --n=500:\n\tdesc\n")
	f.Add("")
	f.Add("\x1b[1mBOLD\x1b[0m\n\b\b\b\n\t\t\t")
}

var validSubcommandName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func FuzzParseHelpOutput(f *testing.F) {
	addHelpSeeds(f)
	f.Fuzz(func(t *testing.T, text string) {
		a := discovery.ParseHelpOutputFor(text, "tool")
		b := discovery.ParseHelpOutputFor(text, "tool")
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("parsing is not deterministic:\n first: %+v\nsecond: %+v", a, b)
		}

		seen := map[string]bool{}
		for _, s := range a.Subcommands {
			if !validSubcommandName.MatchString(s) {
				t.Fatalf("invalid subcommand name %q", s)
			}
			if seen[s] {
				t.Fatalf("duplicate subcommand %q", s)
			}
			seen[s] = true
		}
		for _, fl := range a.Flags {
			if !strings.HasPrefix(fl.Name, "-") {
				t.Fatalf("flag name %q does not start with '-'", fl.Name)
			}
		}
		if strings.ContainsAny(a.Description, "\r\n") {
			t.Fatalf("description spans lines: %q", a.Description)
		}
	})
}

func FuzzSanitizeHelp(f *testing.F) {
	addHelpSeeds(f)
	f.Fuzz(func(t *testing.T, text string) {
		a := discovery.SanitizeHelp(text)
		if b := discovery.SanitizeHelp(text); a != b {
			t.Fatalf("SanitizeHelp is not deterministic")
		}
		if strings.Contains(a, "\x1b[") && strings.Contains(a, "m") && len(a) > len(text) {
			t.Fatalf("sanitizing grew the text: %d -> %d bytes", len(text), len(a))
		}
	})
}

package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Fuzz targets for the parser's line-level hot paths: flag rows, section
// headers and usage synopses. Seeds are the matching lines of every committed
// help fixture. `go test` runs the seeds as ordinary tests; the fuzz probe
// (task measure -- fuzz, skill treemand-fuzz-campaign) mutates them.

// fixtureLines returns every line of the committed help fixtures for which keep is true.
func fixtureLines(f *testing.F, keep func(string) bool) []string {
	f.Helper()
	var out []string
	seen := map[string]bool{}
	for _, pattern := range []string{"testdata/*.txt", "testdata/corpus/*.txt"} {
		files, _ := filepath.Glob(pattern)
		for _, file := range files {
			b, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			for _, l := range strings.Split(string(b), "\n") {
				l = strings.TrimRight(l, "\r")
				if !seen[l] && keep(l) {
					seen[l] = true
					out = append(out, l)
				}
			}
		}
	}
	return out
}

func FuzzParseFlag(f *testing.F) {
	for _, l := range fixtureLines(f, func(l string) bool {
		return strings.HasPrefix(strings.TrimLeft(l, " \t"), "-")
	}) {
		f.Add(l)
	}
	for _, s := range []string{
		"  -h, --help   help for tool",
		"  -C DIRECTORY, --directory=DIRECTORY   change directory",
		"  -i[SUFFIX], --in-place[=SUFFIX]  edit in place",
		"  --[no-]color  colour output",
		"  -chdir=DIR    switch directory",
		"  -cpuprofile string",
		"  --level={debug|info}[,...]  log level",
		"  -n, --quiet, --silent  quiet",
		"--json, -j  JSON output",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, line string) {
		if strings.ContainsAny(line, "\r\n") {
			t.Skip("a help line never holds a line break")
		}
		a, okA := parseFlag(line)
		b, okB := parseFlag(line)
		if okA != okB || !reflect.DeepEqual(a, b) {
			t.Fatalf("parseFlag is not deterministic: %+v/%v vs %+v/%v", a, okA, b, okB)
		}
		if !okA {
			return
		}
		if !strings.HasPrefix(a.Name, "-") || strings.TrimLeft(a.Name, "-") == "" {
			t.Fatalf("flag name %q is not an option name", a.Name)
		}
		if strings.ContainsAny(a.Name, " \t,") || strings.ContainsAny(a.ShortName, " \t,") {
			t.Fatalf("flag names hold separators: name %q short %q", a.Name, a.ShortName)
		}
		if strings.HasPrefix(a.ShortName, "-") {
			t.Fatalf("short name %q keeps its dash", a.ShortName)
		}
		if a.ValueType == "" {
			t.Fatalf("flag %q has no value type", a.Name)
		}
	})
}

// sectionValues is every section detectSection may return.
func sectionValues() map[string]bool {
	v := map[string]bool{secNone: true}
	for _, s := range sectionHeaders {
		v[s] = true
	}
	return v
}

func FuzzDetectSection(f *testing.F) {
	for kw := range sectionHeaders {
		f.Add(kw)
		f.Add(kw + ":")
	}
	for _, l := range fixtureLines(f, func(l string) bool {
		t := strings.TrimSpace(l)
		return t != "" && len(t) <= 60 && !strings.HasPrefix(l, " ")
	}) {
		f.Add(strings.ToLower(l))
	}
	f.Add("cipher commands (see the 'enc' command for more details)")
	f.Add("-v, --verbose   verbose output")
	valid := sectionValues()
	f.Fuzz(func(t *testing.T, lower string) {
		got := detectSection(lower)
		if !valid[got] {
			t.Fatalf("detectSection(%q) = %q, not a known section", lower, got)
		}
		if again := detectSection(lower); again != got {
			t.Fatalf("detectSection(%q) is not deterministic: %q then %q", lower, got, again)
		}
		if padded := detectSection(" \t" + lower + " "); padded != got {
			t.Fatalf("surrounding space changed the section of %q: %q vs %q", lower, got, padded)
		}
	})
}

func FuzzParsePositionals(f *testing.F) {
	for _, l := range fixtureLines(f, func(l string) bool {
		return strings.Contains(strings.ToLower(l), "usage")
	}) {
		f.Add(l)
	}
	f.Add("usage: git [--exec-path[=<path>]] <command> [<args>]")
	f.Add("Usage: tool [OPTIONS] FILE... [DEST]")
	f.Fuzz(func(t *testing.T, line string) {
		a := parsePositionals(line)
		if b := parsePositionals(line); !reflect.DeepEqual(a, b) {
			t.Fatalf("parsePositionals is not deterministic: %+v vs %+v", a, b)
		}
		seen := map[string]bool{}
		for _, p := range a {
			if p.Name == "" || strings.ContainsAny(p.Name, "= ") {
				t.Fatalf("invalid positional name %q", p.Name)
			}
			if seen[p.Name] {
				t.Fatalf("duplicate positional %q", p.Name)
			}
			seen[p.Name] = true
		}
	})
}

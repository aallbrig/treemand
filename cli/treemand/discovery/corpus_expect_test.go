package discovery_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

// corpusExpectation is what a corpus CLI really offers, independent of the
// parser: the regression baseline only guards against getting worse, this
// guards against being wrong.
type corpusExpectation struct {
	SubcommandsInclude []string `json:"subcommands_include"`
	MaxSubcommands     *int     `json:"max_subcommands"`
	FlagsInclude       []string `json:"flags_include"`
	DescriptionInclude string   `json:"description_include"`
	// DescriptionExclude is text the description must not contain: for a CLI
	// with no prose description, garbage is worse than none.
	DescriptionExclude string `json:"description_exclude"`
	// Gaps are the issues tracking known failures of this CLI's expectations.
	Gaps []int `json:"gaps"`
}

func loadExpectations(t *testing.T) map[string]corpusExpectation {
	t.Helper()
	raw, err := os.ReadFile(corpusDir + "/expect.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("expect.json: %v", err)
	}
	out := map[string]corpusExpectation{}
	for k, v := range m {
		if strings.HasPrefix(k, "_") {
			continue
		}
		var e corpusExpectation
		if err := json.Unmarshal(v, &e); err != nil {
			t.Fatalf("expect.json %s: %v", k, err)
		}
		out[k] = e
	}
	return out
}

// misses lists the expectations a parse result fails.
func (e corpusExpectation) misses(p discovery.ParsedHelp) []string {
	var out []string
	for _, s := range e.SubcommandsInclude {
		if !slices.Contains(p.Subcommands, s) {
			out = append(out, "missing subcommand "+s)
		}
	}
	if e.MaxSubcommands != nil && len(p.Subcommands) > *e.MaxSubcommands {
		out = append(out, fmt.Sprintf("%d subcommands, at most %d expected: %v", len(p.Subcommands), *e.MaxSubcommands, p.Subcommands))
	}
	flags := map[string]bool{}
	for _, f := range p.Flags {
		flags[f.Name] = true
		if f.ShortName != "" {
			flags[f.ShortName] = true
		}
	}
	for _, f := range e.FlagsInclude {
		if !flags[f] {
			out = append(out, "missing flag "+f)
		}
	}
	if e.DescriptionInclude != "" && !strings.Contains(p.Description, e.DescriptionInclude) {
		out = append(out, fmt.Sprintf("description %q lacks %q", p.Description, e.DescriptionInclude))
	}
	if e.DescriptionExclude != "" && strings.Contains(p.Description, e.DescriptionExclude) {
		out = append(out, fmt.Sprintf("description %q contains %q", p.Description, e.DescriptionExclude))
	}
	return out
}

func TestCorpus_matchesGroundTruth(t *testing.T) {
	expect := loadExpectations(t)
	for _, f := range corpusFixtures(t) {
		cli := strings.TrimSuffix(filepath.Base(f), ".txt")
		e, ok := expect[cli]
		if !ok {
			t.Errorf("%s: no entry in expect.json; every corpus CLI needs ground truth", cli)
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		p := discovery.ParseHelpOutputFor(discovery.SanitizeHelp(string(b)), cli)
		misses := e.misses(p)
		switch {
		case len(misses) > 0 && len(e.Gaps) > 0:
			t.Logf("known gap %v: %s: %s", e.Gaps, cli, strings.Join(misses, "; "))
		case len(misses) > 0:
			t.Errorf("%s: %s", cli, strings.Join(misses, "; "))
		case len(e.Gaps) > 0:
			t.Errorf("%s: now meets its expectations; remove gaps %v from expect.json (and close them if fully fixed)", cli, e.Gaps)
		}
	}
}

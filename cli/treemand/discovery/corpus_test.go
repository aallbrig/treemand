package discovery_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

// The regression corpus: sanitized root help text of real CLIs under
// testdata/corpus, parsed on every test run and compared with a committed
// baseline. A parser change that loses subcommands, flags or the description
// of any corpus CLI fails here. The baseline changes only on purpose:
//
//	go test ./discovery -run TestCorpus -update     (or: task corpus:update-baseline)
var updateBaseline = flag.Bool("update", false, "rewrite testdata/corpus/baseline.json from the current parser output")

const (
	corpusDir      = "testdata/corpus"
	corpusBaseline = corpusDir + "/baseline.json"
	corpusReadme   = "../../../README.md"
)

type corpusMetrics struct {
	Subcommands           int  `json:"subcommands"`
	Flags                 int  `json:"flags"`
	HasDescription        bool `json:"has_description"`
	DescriptionIsSynopsis bool `json:"description_is_synopsis"`
}

func looksLikeSynopsis(desc string) bool {
	l := strings.ToLower(desc)
	return strings.HasPrefix(desc, "[") || strings.HasPrefix(l, "usage") || strings.Contains(desc, "[--") || strings.Contains(desc, "[-")
}

func corpusFixtures(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(corpusDir + "/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no fixtures in %s", corpusDir)
	}
	sort.Strings(files)
	return files
}

func measureCorpus(t *testing.T) map[string]corpusMetrics {
	t.Helper()
	got := map[string]corpusMetrics{}
	for _, f := range corpusFixtures(t) {
		cli := strings.TrimSuffix(filepath.Base(f), ".txt")
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		p := discovery.ParseHelpOutputFor(discovery.SanitizeHelp(string(b)), cli)
		got[cli] = corpusMetrics{
			Subcommands:           len(p.Subcommands),
			Flags:                 len(p.Flags),
			HasDescription:        p.Description != "",
			DescriptionIsSynopsis: p.Description != "" && looksLikeSynopsis(p.Description),
		}
	}
	return got
}

func TestCorpus_noRegressionAgainstBaseline(t *testing.T) {
	got := measureCorpus(t)

	if *updateBaseline {
		b, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corpusBaseline, append(b, '\n'), 0o644); err != nil { //nolint:gosec
			t.Fatal(err)
		}
		t.Logf("baseline rewritten for %d CLIs; review the diff before committing", len(got))
		return
	}

	raw, err := os.ReadFile(corpusBaseline)
	if err != nil {
		t.Fatalf("read baseline: %v (create it with -update)", err)
	}
	var want map[string]corpusMetrics
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}

	hint := "if intended, run `task corpus:update-baseline` and commit the diff"
	for cli, base := range want {
		now, ok := got[cli]
		if !ok {
			t.Errorf("%s: in baseline but has no fixture (%s)", cli, hint)
			continue
		}
		if now.Subcommands < base.Subcommands {
			t.Errorf("%s: subcommands regressed %d -> %d (%s)", cli, base.Subcommands, now.Subcommands, hint)
		}
		if now.Flags < base.Flags {
			t.Errorf("%s: flags regressed %d -> %d (%s)", cli, base.Flags, now.Flags, hint)
		}
		if base.HasDescription && !now.HasDescription {
			t.Errorf("%s: lost its description (%s)", cli, hint)
		}
		if !base.DescriptionIsSynopsis && now.DescriptionIsSynopsis {
			t.Errorf("%s: description is now a usage synopsis (%s)", cli, hint)
		}
		if now.Subcommands > base.Subcommands || now.Flags > base.Flags {
			t.Logf("%s improved (subcommands %d -> %d, flags %d -> %d): update the baseline to lock it in", cli, base.Subcommands, now.Subcommands, base.Flags, now.Flags)
		}
	}
	for cli := range got {
		if _, ok := want[cli]; !ok {
			t.Errorf("%s: fixture has no baseline entry (%s)", cli, hint)
		}
	}
}

// The README names every CLI in the corpus, so what is tested is public.
func TestCorpus_readmeListsTestedCLIs(t *testing.T) {
	b, err := os.ReadFile(corpusReadme)
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	start := strings.Index(readme, "## Tested CLIs")
	if start < 0 {
		t.Fatal("README has no '## Tested CLIs' section")
	}
	section := readme[start:]
	if next := strings.Index(section[3:], "\n## "); next >= 0 {
		section = section[:next+3]
	}
	var missing []string
	for _, f := range corpusFixtures(t) {
		cli := strings.TrimSuffix(filepath.Base(f), ".txt")
		if !strings.Contains(section, "`"+cli+"`") {
			missing = append(missing, cli)
		}
	}
	if len(missing) > 0 {
		t.Errorf("README '## Tested CLIs' is missing: %s", fmt.Sprint(missing))
	}
}

// Parsing the same text must always give the same result (map iteration
// order must never leak into the parse).
func TestCorpus_parsingIsDeterministic(t *testing.T) {
	first := measureCorpus(t)
	for i := 0; i < 25; i++ {
		again := measureCorpus(t)
		for cli, m := range first {
			if again[cli] != m {
				t.Fatalf("%s parsed differently on repeat %d: %+v then %+v", cli, i, m, again[cli])
			}
		}
	}
}

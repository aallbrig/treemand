package fuzzy_test

import (
	"testing"

	"github.com/aallbrig/treemand/fuzzy"
	"github.com/aallbrig/treemand/models"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		pat, text string
		ok        bool
	}{
		{"", "anything", true},
		{"cm", "commit", true},
		{"cmt", "commit", true},
		{"tmc", "commit", false},
		{"COM", "commit", true},
		{"abcd", "abc", false},
	}
	for _, c := range cases {
		_, _, ok := fuzzy.Match(c.pat, c.text)
		if ok != c.ok {
			t.Errorf("Match(%q,%q) ok=%v want %v", c.pat, c.text, ok, c.ok)
		}
	}
}

func TestMatch_prefersBoundaryAndConsecutive(t *testing.T) {
	a, _, _ := fuzzy.Match("pr", "pr")
	b, _, _ := fuzzy.Match("pr", "upgrade-ruby")
	if a <= b {
		t.Errorf("exact (%d) should outrank scattered (%d)", a, b)
	}
}

func sample() *models.Node {
	root := &models.Node{Name: "git", FullPath: []string{"git"},
		Flags: []models.Flag{{Name: "verbose", Description: "be chatty"}}}
	commit := &models.Node{Name: "commit", FullPath: []string{"git", "commit"},
		Description: "Record changes to the repository",
		Flags: []models.Flag{
			{Name: "amend", Description: "replace the tip commit"},
			{Name: "verbose", Inherited: true},
		},
		Positionals: []models.Positional{{Name: "pathspec"}},
		HelpText:    "full help mentioning idempotency keys",
	}
	root.Children = []*models.Node{commit}
	return root
}

func TestBuild_skipsInheritedFlags(t *testing.T) {
	es := fuzzy.Build(sample())
	n := 0
	for _, e := range es {
		if e.Kind == fuzzy.KindFlag && e.Name == "--verbose" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("--verbose indexed %d times, want 1", n)
	}
}

func TestSearch_fields(t *testing.T) {
	es := fuzzy.Build(sample())
	cases := []struct{ q, wantName, wantIn string }{
		{"comm", "commit", "name"},
		{"amend", "--amend", "name"},
		{"repository", "commit", "desc"},
		{"idempotency", "commit", "help"},
		{"pathsp", "pathspec", "name"},
	}
	for _, c := range cases {
		r := fuzzy.Search(es, c.q, 5)
		if len(r) == 0 {
			t.Fatalf("%q: no results", c.q)
		}
		if r[0].Entry.Name != c.wantName || r[0].MatchedIn != c.wantIn {
			t.Errorf("%q: got %s/%s want %s/%s", c.q, r[0].Entry.Name, r[0].MatchedIn, c.wantName, c.wantIn)
		}
	}
	if len(fuzzy.Search(es, "zzzqqq", 5)) != 0 {
		t.Error("expected no results")
	}
}

func TestSearch_multiTermAND(t *testing.T) {
	es := fuzzy.Build(sample())
	if r := fuzzy.Search(es, "commit tip", 5); len(r) == 0 || r[0].Entry.Name != "--amend" {
		t.Errorf("got %+v", r)
	}
}

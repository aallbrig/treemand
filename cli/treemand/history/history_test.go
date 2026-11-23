package history_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aallbrig/treemand/history"
)

func t0(min int) time.Time {
	return time.Date(2026, 11, 23, 12, min, 0, 0, time.UTC)
}

func commands(t *testing.T, s *history.Store) []string {
	t.Helper()
	entries, err := s.Recent()
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Command
	}
	return out
}

func TestRecentIsEmptyWithoutFile(t *testing.T) {
	s := history.Open(t.TempDir(), "git")
	if got := commands(t, s); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestAddRecordsNewestFirst(t *testing.T) {
	s := history.Open(t.TempDir(), "git")
	for i, c := range []string{"git status", "git log --oneline", "git diff"} {
		if err := s.Add(c, t0(i)); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"git diff", "git log --oneline", "git status"}
	got := commands(t, s)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestAddDeduplicatesAndCounts(t *testing.T) {
	s := history.Open(t.TempDir(), "git")
	_ = s.Add("git status", t0(0))
	_ = s.Add("git diff", t0(1))
	_ = s.Add("  git   status ", t0(2))
	entries, err := s.Recent()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Command != "git status" || entries[0].Count != 2 {
		t.Fatalf("got %+v, want git status first with count 2", entries)
	}
	if !entries[0].Last.Equal(t0(2)) {
		t.Fatalf("last = %v, want %v", entries[0].Last, t0(2))
	}
}

func TestAddIgnoresEmpty(t *testing.T) {
	s := history.Open(t.TempDir(), "git")
	if err := s.Add("   ", t0(0)); err != nil {
		t.Fatal(err)
	}
	if got := commands(t, s); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestAddCapsSize(t *testing.T) {
	s := history.Open(t.TempDir(), "git")
	for i := 0; i < history.MaxEntries+5; i++ {
		if err := s.Add("git log -n "+string(rune('a'+i%26))+string(rune('a'+i/26)), t0(i)); err != nil {
			t.Fatal(err)
		}
	}
	got := commands(t, s)
	if len(got) != history.MaxEntries {
		t.Fatalf("len = %d, want %d", len(got), history.MaxEntries)
	}
}

func TestStoresArePerCLI(t *testing.T) {
	dir := t.TempDir()
	_ = history.Open(dir, "git").Add("git status", t0(0))
	_ = history.Open(dir, "kubectl").Add("kubectl get pods", t0(1))
	if got := commands(t, history.Open(dir, "git")); len(got) != 1 || got[0] != "git status" {
		t.Fatalf("git history = %v", got)
	}
}

func TestCLINameCannotEscapeDir(t *testing.T) {
	dir := t.TempDir()
	s := history.Open(dir, "../../evil")
	if err := s.Add("x", t0(0)); err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(s.Path()) || filepath.Dir(s.Path()) != dir {
		t.Fatalf("path %q is not directly inside %q", s.Path(), dir)
	}
}

func TestCorruptFileReadsAsError(t *testing.T) {
	dir := t.TempDir()
	s := history.Open(dir, "git")
	if err := os.WriteFile(s.Path(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recent(); err == nil {
		t.Fatal("want an error for a corrupt file")
	}
	// Add replaces a corrupt file rather than failing forever.
	if err := s.Add("git status", t0(0)); err != nil {
		t.Fatalf("Add over corrupt file: %v", err)
	}
	if got := commands(t, s); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}

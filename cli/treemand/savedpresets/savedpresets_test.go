package savedpresets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// twoPresets is a store holding amend (saved twice) and log1.
func twoPresets(t *testing.T) *Store {
	t.Helper()
	s := Open(t.TempDir(), "git")
	for _, p := range []Preset{{"amend", "git commit  --amend --no-edit"}, {"log1", "git log --oneline"}, {"amend", "git commit --amend"}} {
		if err := s.Save(p.Name, p.Command); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestList_emptyStore(t *testing.T) {
	if got, err := Open(t.TempDir(), "git").List(); err != nil || len(got) != 0 {
		t.Fatalf("empty store: %v, %v", got, err)
	}
}

func TestSave_replacesInPlace(t *testing.T) {
	got, err := twoPresets(t).List()
	if err != nil {
		t.Fatal(err)
	}
	want := []Preset{{"amend", "git commit --amend"}, {"log1", "git log --oneline"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("List = %v, want %v", got, want)
	}
}

func TestGet(t *testing.T) {
	s := twoPresets(t)
	if p, ok, _ := s.Get("log1"); !ok || p.Command != "git log --oneline" {
		t.Errorf("Get(log1) = %v %v", p, ok)
	}
	if _, ok, _ := s.Get("nope"); ok {
		t.Error("Get(nope) found something")
	}
}

func TestDelete(t *testing.T) {
	s := twoPresets(t)
	if ok, err := s.Delete("amend"); !ok || err != nil {
		t.Fatalf("Delete(amend) = %v %v", ok, err)
	}
	if ok, _ := s.Delete("amend"); ok {
		t.Error("second Delete(amend) reported a deletion")
	}
	if got, _ := s.List(); len(got) != 1 || got[0].Name != "log1" {
		t.Errorf("after delete: %v", got)
	}
}

func TestFileIsPlainTextAndHandEditable(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir, "git")
	if err := s.Save("st", "git status -sb"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "git.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "st  git status -sb\n") {
		t.Errorf("file = %q", data)
	}

	edited := "# my presets\n\n  wip   git commit -m wip  \nbad-line-without-command\n" +
		"bad?name x\nst git status\n"
	if err := os.WriteFile(s.Path(), []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	// Invalid lines are skipped; a later duplicate name wins.
	if len(got) != 2 || got[0] != (Preset{"wip", "git commit -m wip"}) || got[1] != (Preset{"st", "git status"}) {
		t.Fatalf("List = %v", got)
	}
	// Saving keeps the user's comment lines.
	if err := s.Save("new", "git fetch"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(s.Path())
	if !strings.HasPrefix(string(data), "# my presets\n") {
		t.Errorf("comment dropped: %q", data)
	}
}

func TestSaveRejectsBadInput(t *testing.T) {
	s := Open(t.TempDir(), "git")
	for _, name := range []string{"", "two words", "a/b", "#x", strings.Repeat("n", MaxNameLen+1)} {
		if err := s.Save(name, "git status"); err == nil {
			t.Errorf("Save(%q) accepted", name)
		}
	}
	if err := s.Save("empty", "   "); err == nil {
		t.Error("Save with an empty command accepted")
	}
	if err := s.Save("nl", "git status\nrm -rf x"); err != nil {
		t.Fatal(err)
	}
	if p, _, _ := s.Get("nl"); p.Command != "git status rm -rf x" {
		t.Errorf("newline not flattened: %q", p.Command)
	}
}

func TestFileNameCannotLeaveDir(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir, "../../etc/passwd")
	if filepath.Dir(s.Path()) != dir {
		t.Errorf("path %q escapes %q", s.Path(), dir)
	}
	if got := filepath.Base(Open(dir, "").Path()); got != "_.txt" {
		t.Errorf("empty cli file = %q", got)
	}
}

func TestUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "git.txt"), 0o700); err != nil {
		t.Fatal(err)
	}
	s := Open(dir, "git")
	if _, err := s.List(); err == nil {
		t.Error("List of a directory did not fail")
	}
	if err := s.Save("a", "git"); err == nil {
		t.Error("Save over a directory did not fail")
	}
	if _, err := s.Delete("a"); err == nil {
		t.Error("Delete over a directory did not fail")
	}
}

func TestValidName(t *testing.T) {
	for _, ok := range []string{"a", "deploy-prod", "v1.2_x"} {
		if ValidName(ok) != nil {
			t.Errorf("ValidName(%q) rejected", ok)
		}
	}
}

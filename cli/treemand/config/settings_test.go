package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/config"
)

func newSettings(t *testing.T, content string) *config.Settings {
	t.Helper()
	path := filepath.Join(t.TempDir(), "treemand", "config.yaml")
	if content != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &config.Settings{Path: path, Getenv: func(string) string { return "" }}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSettings_setKeepsCommentsUnknownKeysAndOrder(t *testing.T) {
	s := newSettings(t, "# top\nicons: ascii # inline\nfuture: {a: 1}\ncolors:\n  base: \"#111111\"\n")
	for _, kv := range [][2]string{{"depth", "5"}, {"colors.subcmd", "#ABCDEF"}, {"icons", "nerd"}, {"no_color", "TRUE"}} {
		if err := s.Set(kv[0], kv[1]); err != nil {
			t.Fatalf("Set(%s): %v", kv[0], err)
		}
	}
	got := readFile(t, s.Path)
	want := "# top\nicons: nerd # inline\nfuture: {a: 1}\ncolors:\n  base: \"#111111\"\n  subcmd: \"#ABCDEF\"\ndepth: 5\nno_color: true\n"
	if got != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
}

func TestSettings_setOnAMissingFileWritesTheCommentedDefaults(t *testing.T) {
	s := newSettings(t, "")
	if err := s.Set("tree_style", "graph"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, s.Path)
	if !strings.Contains(got, "# treemand configuration file") || !strings.Contains(got, "tree_style: graph") {
		t.Errorf("file =\n%s", got)
	}
}

func TestSettings_setRefusesWhatConfigSetRefuses(t *testing.T) {
	s := newSettings(t, "icons: ascii\n")
	err := s.Set("depth", "abc")
	if err == nil || !strings.Contains(err.Error(), "expected an integer") {
		t.Errorf("Set(depth, abc) = %v", err)
	}
	err = s.Set("icon", "nerd")
	var unknown *config.UnknownKeyError
	if !errors.As(err, &unknown) || unknown.Suggestion != "icons" {
		t.Errorf("Set(icon) = %v", err)
	}
	if got := readFile(t, s.Path); got != "icons: ascii\n" {
		t.Errorf("a refused Set changed the file: %q", got)
	}
}

func TestSettings_setNestsUnderAScalarColorsKey(t *testing.T) {
	s := newSettings(t, "colors:\n")
	if err := s.Set("colors.pos", "#FFF"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get("colors.pos"); err != nil || got.Value != "#FFF" || got.Source != config.SourceFile {
		t.Errorf("Get = %+v, %v", got, err)
	}
	s = newSettings(t, "colors: red\n")
	if err := s.Set("colors.pos", "#FFF"); err == nil {
		t.Error("Set under a scalar colors: want an error")
	}
}

func TestSettings_sourcesFollowPrecedence(t *testing.T) {
	s := newSettings(t, "icons: ascii\ndepth: 4\nno_cache: true\n")
	s.Flags = map[string]string{"depth": "7"}
	s.Getenv = func(k string) string {
		if k == "TREEMAND_NO_CACHE" {
			return "false"
		}
		return ""
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(config.KnownKeys) {
		t.Fatalf("List has %d keys, want %d", len(list), len(config.KnownKeys))
	}
	by := map[string]config.Setting{}
	for _, st := range list {
		by[st.Key] = st
	}
	for key, want := range map[string]struct {
		value  string
		source config.Source
	}{
		"icons":      {"ascii", config.SourceFile},
		"depth":      {"7", config.SourceFlag},
		"no_cache":   {"false", config.SourceEnv},
		"tree_style": {"default", config.SourceDefault},
	} {
		if got := by[key]; got.Value != want.value || got.Source != want.source {
			t.Errorf("%s = %q from %s, want %q from %s", key, got.Value, got.Source, want.value, want.source)
		}
	}
}

func TestSettings_readErrors(t *testing.T) {
	s := newSettings(t, "icons: [unclosed\n")
	if _, err := s.List(); err == nil {
		t.Error("List on bad YAML: want an error")
	}
	if _, err := s.Get("icons"); err == nil {
		t.Error("Get on bad YAML: want an error")
	}
	if err := s.Set("icons", "ascii"); err == nil {
		t.Error("Set on bad YAML: want an error")
	}
	if _, err := s.Get("nope"); err == nil {
		t.Error("Get(nope): want an error")
	}
	s = newSettings(t, "- a\n- b\n")
	if err := s.Set("icons", "ascii"); err == nil {
		t.Error("Set on a list document: want an error")
	}
	s = newSettings(t, "")
	if got, err := s.Get("icons"); err != nil || got.Source != config.SourceDefault || got.Value != "unicode" {
		t.Errorf("Get on a missing file = %+v, %v", got, err)
	}
}

func TestApplyLive_themeAndLayoutOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	for _, kv := range [][2]string{{"tree_style", "graph"}, {"icons", "ascii"}, {"colors.selected_text", "#123456"}} {
		if !config.ApplyLive(cfg, kv[0], kv[1]) {
			t.Errorf("ApplyLive(%s) = false", kv[0])
		}
	}
	if cfg.TreeStyle != config.StyleGraph || cfg.IconPreset != "ascii" || cfg.Icons.Leaf != "- " || cfg.Colors.SelectedText != "#123456" {
		t.Errorf("cfg = %+v", cfg)
	}
	for _, key := range []string{"depth", "colors.nope"} {
		if config.ApplyLive(cfg, key, "1") {
			t.Errorf("ApplyLive(%s) = true", key)
		}
	}
	// Every colors.* schema key has a field.
	for _, e := range config.KnownKeys {
		if strings.HasPrefix(e.Key, "colors.") && !config.ApplyLive(cfg, e.Key, "#000") {
			t.Errorf("no live field for %s", e.Key)
		}
	}
}

// Review findings on #266: the file's text is edited in place.
func TestSettings_setEditsTheTextInPlace(t *testing.T) {
	cases := []struct {
		name, in, key, value, want string
	}{
		{"blank lines and comments kept", "# a\n\nicons: ascii  # b\n\ndepth: 3\n", "depth", "5",
			"# a\n\nicons: ascii  # b\n\ndepth: 5\n"},
		{"comment-only file", "# only a comment\n# icons: nerd\n", "icons", "nerd",
			"# only a comment\n# icons: nerd\nicons: nerd\n"},
		{"bare document marker", "---\n", "depth", "4", "---\ndepth: 4\n"},
		{"no trailing newline", "icons: ascii", "depth", "4", "icons: ascii\ndepth: 4\n"},
		{"quoted values", "a: 'x'\ncolors:\n  base: \"#111\" # c\n  pos: '#222'\n", "colors.pos", "#ABC",
			"a: 'x'\ncolors:\n  base: \"#111\" # c\n  pos: \"#ABC\"\n"},
		{"new nested key under a parent", "colors:\n    base: \"#111\"\ndepth: 3\n", "colors.pos", "#ABC",
			"colors:\n    base: \"#111\"\n    pos: \"#ABC\"\ndepth: 3\n"},
		{"new parent", "depth: 3\n", "colors.pos", "#ABC", "depth: 3\ncolors:\n  pos: \"#ABC\"\n"},
		{"alias replaced, anchor kept", "depth: &d 3\nstub_threshold: *d\n", "stub_threshold", "9",
			"depth: &d 3\nstub_threshold: 9\n"},
		{"string that needs quoting", "strategies: help\n", "strategies", "help, man # x",
			"strategies: 'help, man # x'\n"},
		{"crlf kept", "icons: ascii\r\ndepth: 3\r\n", "icons", "nerd", "icons: nerd\r\ndepth: 3\r\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSettings(t, c.in)
			if err := s.Set(c.key, c.value); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, s.Path); got != c.want {
				t.Errorf("file =\n%q\nwant\n%q", got, c.want)
			}
		})
	}
}

func TestSettings_setRefusesWhatItCannotEditInPlace(t *testing.T) {
	for name, in := range map[string]string{
		"anchored value":   "depth: &d 3\nstub_threshold: *d\n",
		"flow parent":      "colors: {base: \"#111\"}\n",
		"flow top":         "{depth: 3}\n",
		"null top":         "~\n",
		"block scalar":     "depth: |\n  3\n",
		"multi-line quote": "depth: \"3\n  \"\n",
	} {
		t.Run(name, func(t *testing.T) {
			s := newSettings(t, in)
			key := "depth"
			if strings.Contains(in, "colors") {
				key = "colors.pos"
			}
			if err := s.Set(key, "4"); err == nil && key == "depth" {
				t.Errorf("Set: want an error, file now %q", readFile(t, s.Path))
			}
			if key == "colors.pos" {
				if err := s.Set(key, "#FFF"); err == nil {
					t.Errorf("Set: want an error, file now %q", readFile(t, s.Path))
				}
			}
			if got := readFile(t, s.Path); got != in {
				t.Errorf("a refused Set changed the file to %q", got)
			}
		})
	}
}

func TestSettings_aliasIsReadAndSymlinkWrittenThrough(t *testing.T) {
	s := newSettings(t, "depth: &d 3\nstub_threshold: *d\n")
	if got, _ := s.Get("stub_threshold"); got.Value != "3" || got.Source != config.SourceFile {
		t.Errorf("aliased stub_threshold = %+v", got)
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles.yaml")
	if err := os.WriteFile(target, []byte("icons: ascii\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("no symlinks here:", err)
	}
	s = &config.Settings{Path: link, Getenv: func(string) string { return "" }}
	if err := s.Set("icons", "nerd"); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link was replaced: %v %v", fi, err)
	}
	if got := readFile(t, target); got != "icons: nerd\n" {
		t.Errorf("target = %q", got)
	}
}

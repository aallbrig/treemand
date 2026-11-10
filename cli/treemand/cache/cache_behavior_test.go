package cache_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aallbrig/treemand/cache"
	"github.com/aallbrig/treemand/models"
)

func openCache(t *testing.T) (*cache.Cache, string) {
	t.Helper()
	dir := t.TempDir()
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c, dir
}

func TestOpen_failsWhenCacheDirIsAFile(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := cache.Open(notADir)
	if err == nil {
		c.Close()
		t.Fatal("Open() on a regular file should fail")
	}
}

func TestKey_dependsOnEveryInput(t *testing.T) {
	base := cache.KeyWith("git", "2.40", []string{"help"}, "")
	if base != cache.Key("git", "2.40", []string{"help"}) {
		t.Error("Key must equal KeyWith with an empty discriminator")
	}
	if base != cache.KeyWith("git", "2.40", []string{"help"}, "") {
		t.Error("keys must be deterministic")
	}
	variants := map[string]string{
		"cli":      cache.KeyWith("hg", "2.40", []string{"help"}, ""),
		"version":  cache.KeyWith("git", "2.41", []string{"help"}, ""),
		"strategy": cache.KeyWith("git", "2.40", []string{"help", "man"}, ""),
		"extra":    cache.KeyWith("git", "2.40", []string{"help"}, "depth=2"),
	}
	for what, k := range variants {
		if k == base {
			t.Errorf("changing the %s must change the key", what)
		}
	}
}

func TestGet_honoursMaxAge(t *testing.T) {
	c, _ := openCache(t)
	key := cache.Key("git", "1", []string{"help"})
	if err := c.Put(key, "git", "1", "help", &models.Node{Name: "git"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// cached_at has one-second resolution, so wait past it for a tiny max age.
	time.Sleep(1100 * time.Millisecond)
	if got, err := c.Get(key, time.Millisecond); err != nil || got != nil {
		t.Errorf("expired entry: got %v, err %v; want nil, nil", got, err)
	}
	if got, err := c.Get(key, 0); err != nil || got == nil {
		t.Errorf("maxAge 0 means no expiry: got %v, err %v", got, err)
	}
	if got, err := c.Get(key, time.Hour); err != nil || got == nil {
		t.Errorf("fresh entry within maxAge: got %v, err %v", got, err)
	}
}

func TestGet_corruptEntryIsAnErrorNotAPanic(t *testing.T) {
	c, dir := openCache(t)
	key := cache.Key("git", "1", []string{"help"})
	if err := c.Put(key, "git", "1", "help", &models.Node{Name: "git"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	raw, err := sql.Open("sqlite", filepath.Join(dir, "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`UPDATE trees SET data = '{not json' WHERE key = ?`, key); err != nil {
		t.Fatal(err)
	}
	if got, err := c.Get(key, 0); err == nil || got != nil {
		t.Errorf("corrupt entry: got %v, err %v; want a decode error", got, err)
	}
}

func TestListCLIs_distinctAndSorted(t *testing.T) {
	c, _ := openCache(t)
	puts := []struct{ cli, ver string }{{"zed", "1"}, {"git", "1"}, {"git", "2"}, {"awk", "1"}}
	for _, p := range puts {
		key := cache.Key(p.cli, p.ver, []string{"help"})
		if err := c.Put(key, p.cli, p.ver, "help", &models.Node{Name: p.cli}); err != nil {
			t.Fatalf("Put %v: %v", p, err)
		}
	}
	got, err := c.ListCLIs()
	if err != nil {
		t.Fatalf("ListCLIs: %v", err)
	}
	if strings.Join(got, ",") != "awk,git,zed" {
		t.Errorf("ListCLIs = %v, want [awk git zed]", got)
	}
}

func fakeCLI(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCLIVersion_firstLineTruncationAndFailures(t *testing.T) {
	fakeCLI(t, "vercli", "printf 'vercli 1.2.3\\nbuilt yesterday\\n'\n")
	if got := cache.CLIVersion("vercli"); got != "vercli 1.2.3" {
		t.Errorf("CLIVersion = %q, want the first line only", got)
	}

	fakeCLI(t, "longcli", "printf '%s\\n' \"$(printf 'x%.0s' $(seq 1 200))\"\n")
	if got := cache.CLIVersion("longcli"); len(got) != 64 {
		t.Errorf("long version line has length %d, want it cut to 64", len(got))
	}

	fakeCLI(t, "failcli", "exit 3\n")
	if got := cache.CLIVersion("failcli"); got != "unknown" {
		t.Errorf("failing --version = %q, want unknown", got)
	}
	if got := cache.CLIVersion("definitely-not-installed-xyz"); got != "unknown" {
		t.Errorf("missing CLI = %q, want unknown", got)
	}
}

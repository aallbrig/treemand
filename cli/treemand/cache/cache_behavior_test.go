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

func TestBinaryFingerprint_tracksTheBinaryNotItsOutput(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tool")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(bin, []byte(body), 0o755); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}
	write("#!/bin/sh\necho one\n")
	first := cache.BinaryFingerprint(bin)
	if first == "" {
		t.Fatal("no fingerprint for an existing file")
	}
	if again := cache.BinaryFingerprint(bin); again != first {
		t.Errorf("fingerprint is not stable: %q then %q", first, again)
	}

	write("#!/bin/sh\necho a different and longer script\n")
	if changed := cache.BinaryFingerprint(bin); changed == first {
		t.Error("an upgraded binary (different size) must change the fingerprint")
	}

	same := cache.BinaryFingerprint(bin)
	later := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(bin, later, later); err != nil {
		t.Fatal(err)
	}
	if touched := cache.BinaryFingerprint(bin); touched == same {
		t.Error("a rebuilt binary of the same size (new mtime) must change the fingerprint")
	}

	if got := cache.BinaryFingerprint(filepath.Join(dir, "missing")); got != "" {
		t.Errorf("fingerprint of a missing file = %q, want empty", got)
	}
}

func TestBinaryFingerprint_followsSymlinkRetargets(t *testing.T) {
	// Package managers often upgrade by repointing a "current" link.
	dir := t.TempDir()
	for _, name := range []string{"v1", "v2"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho "+name+"\n"), 0o755); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, "current")
	if err := os.Symlink(filepath.Join(dir, "v1"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	before := cache.BinaryFingerprint(link)
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "v2"), link); err != nil {
		t.Fatal(err)
	}
	if after := cache.BinaryFingerprint(link); after == before || after == "" {
		t.Errorf("repointing the link must change the fingerprint: %q then %q", before, after)
	}
}

// largeTree is shaped like a cached aws run: a root with 18 flags and 422
// service nodes.
func largeTree() *models.Node {
	root := &models.Node{Name: "aws", FullPath: []string{"aws"}, Discovered: true, HelpText: strings.Repeat("The AWS command line interface.\n", 1500)}
	for i := 0; i < 18; i++ {
		root.Flags = append(root.Flags, models.Flag{Name: "--global-" + strings.Repeat("x", i%5+1), ValueType: "string", Description: "a global flag"})
	}
	for i := 0; i < 422; i++ {
		name := "svc" + strings.Repeat("a", i%7+1) + string(rune('a'+i%26))
		root.Children = append(root.Children, &models.Node{Name: name, FullPath: []string{"aws", name}, Stub: true, Description: "service " + name})
	}
	return root
}

// BenchmarkGetLargeTree is the cost of a cache hit on a 400-node tree: the
// SQLite read plus JSON decode (a few milliseconds).
func BenchmarkGetLargeTree(b *testing.B) {
	c, err := cache.Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	key := cache.Key("aws", "bench", []string{"help"})
	if err := c.Put(key, "aws", "bench", "help", largeTree()); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if n, err := c.Get(key, 0); err != nil || n == nil {
			b.Fatalf("Get: %v %v", n, err)
		}
	}
}

// A hit on a 400-node tree must stay far below the 300ms budget for a whole
// warm run; this guards the cache read itself.
func TestGetLargeTreeIsFast(t *testing.T) {
	c, _ := openCache(t)
	key := cache.Key("aws", "t", []string{"help"})
	if err := c.Put(key, "aws", "t", "help", largeTree()); err != nil {
		t.Fatal(err)
	}
	best := time.Hour
	for i := 0; i < 5; i++ {
		start := time.Now()
		got, err := c.Get(key, 0)
		if err != nil || got == nil || len(got.Children) != 422 {
			t.Fatalf("Get returned %v, %v", got, err)
		}
		if d := time.Since(start); d < best {
			best = d
		}
	}
	if best > 100*time.Millisecond {
		t.Errorf("cache read of a 422-node tree took %v, want well under 100ms", best)
	}
}

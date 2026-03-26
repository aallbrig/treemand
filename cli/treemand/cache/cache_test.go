package cache_test

import (
	"os"
	"testing"
	"time"

	"github.com/aallbrig/treemand/cache"
	"github.com/aallbrig/treemand/models"
)

func TestCacheOpenAndClose(t *testing.T) {
	dir := t.TempDir()
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer c.Close()
}

func TestCachePutGet(t *testing.T) {
	dir := t.TempDir()
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer c.Close()

	node := &models.Node{Name: "git", Description: "version control"}
	key := cache.Key("git", "2.40.0", []string{"help"})

	if err := c.Put(key, "git", "2.40.0", "help", node); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	got, err := c.Get(key, 0)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Name != node.Name {
		t.Errorf("Name = %q, want %q", got.Name, node.Name)
	}
}

func TestCacheGet_notFound(t *testing.T) {
	dir := t.TempDir()
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer c.Close()

	got, err := c.Get("nonexistent", 0)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing key")
	}
}

func TestCacheGet_expired(t *testing.T) {
	dir := t.TempDir()
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer c.Close()

	node := &models.Node{Name: "git"}
	key := cache.Key("git", "2.40.0", []string{"help"})
	if err := c.Put(key, "git", "2.40.0", "help", node); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Use a very small maxAge so it's expired immediately
	got, err := c.Get(key, time.Nanosecond)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for expired entry")
	}
}

func TestCacheDelete(t *testing.T) {
	dir := t.TempDir()
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer c.Close()

	node := &models.Node{Name: "git"}
	key := cache.Key("git", "2.40.0", []string{"help"})
	_ = c.Put(key, "git", "2.40.0", "help", node)
	_ = c.Delete(key)

	got, err := c.Get(key, 0)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestCacheClear(t *testing.T) {
	dir := t.TempDir()
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer c.Close()

	node := &models.Node{Name: "git"}
	for _, v := range []string{"1.0", "2.0"} {
		k := cache.Key("git", v, []string{"help"})
		_ = c.Put(k, "git", v, "help", node)
	}
	if err := c.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}
	k := cache.Key("git", "1.0", []string{"help"})
	got, _ := c.Get(k, 0)
	if got != nil {
		t.Error("expected nil after clear")
	}
}

func TestKey(t *testing.T) {
	k1 := cache.Key("git", "2.40", []string{"help"})
	k2 := cache.Key("git", "2.40", []string{"help"})
	k3 := cache.Key("git", "2.41", []string{"help"})
	if k1 != k2 {
		t.Error("identical inputs should produce same key")
	}
	if k1 == k3 {
		t.Error("different inputs should produce different keys")
	}
}

func TestCLIVersion(t *testing.T) {
	// echo is always available and supports --version on most systems
	v := cache.CLIVersion("go")
	if v == "" {
		t.Error("expected non-empty version for go")
	}
	// Non-existent CLI should return "unknown"
	v2 := cache.CLIVersion("nonexistent_cli_99999")
	if v2 != "unknown" {
		t.Errorf("expected 'unknown' for nonexistent CLI, got %q", v2)
	}
}

func TestCacheClearCLI(t *testing.T) {
	dir := t.TempDir()
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer c.Close()

	node := &models.Node{Name: "git"}
	for _, ver := range []string{"1.0", "2.0"} {
		k := cache.Key("git", ver, []string{"help"})
		_ = c.Put(k, "git", ver, "help", node)
	}
	// Put an entry for a different CLI too.
	kGo := cache.Key("go", "1.22", []string{"help"})
	_ = c.Put(kGo, "go", "1.22", "help", &models.Node{Name: "go"})

	if err := c.ClearCLI("git"); err != nil {
		t.Fatalf("ClearCLI() error: %v", err)
	}

	// git entries should be gone.
	k := cache.Key("git", "1.0", []string{"help"})
	got, _ := c.Get(k, 0)
	if got != nil {
		t.Error("expected nil for cleared CLI 'git'")
	}

	// go entry should still exist.
	gotGo, _ := c.Get(kGo, 0)
	if gotGo == nil {
		t.Error("expected 'go' entry to survive ClearCLI('git')")
	}
}

func TestCacheListCLIs(t *testing.T) {
	dir := t.TempDir()
	c, err := cache.Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer c.Close()

	// Empty cache should return empty list.
	clis, err := c.ListCLIs()
	if err != nil {
		t.Fatalf("ListCLIs() error: %v", err)
	}
	if len(clis) != 0 {
		t.Errorf("expected 0 CLIs, got %v", clis)
	}

	// Add two distinct CLIs.
	for _, name := range []string{"aws", "git"} {
		k := cache.Key(name, "1.0", []string{"help"})
		_ = c.Put(k, name, "1.0", "help", &models.Node{Name: name})
		// Second version for git — should not duplicate in list.
		k2 := cache.Key(name, "2.0", []string{"help"})
		_ = c.Put(k2, name, "2.0", "help", &models.Node{Name: name})
	}

	clis, err = c.ListCLIs()
	if err != nil {
		t.Fatalf("ListCLIs() error: %v", err)
	}
	if len(clis) != 2 {
		t.Errorf("expected 2 distinct CLIs, got %v", clis)
	}
	// Should be sorted.
	if clis[0] != "aws" || clis[1] != "git" {
		t.Errorf("expected [aws git], got %v", clis)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

//go:build !windows

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestInteractive_rediscoverRefreshesADiscoveredNode is issue #471: pressing r
// on a command that was already discovered must show what its help says now,
// not keep the flags and help text from the first discovery.
func TestInteractive_rediscoverRefreshesADiscoveredNode(t *testing.T) {
	grown := filepath.Join(t.TempDir(), "grown")
	s := startWithEnv(t, []string{"FAKE_GROWN=" + grown}, "-i", "--no-cache", "fakegrow")
	s.waitFor("sprout", 15*time.Second)
	s.waitFor("discovered", 15*time.Second) // the background crawl filled sprout
	if strings.Contains(s.screen(), "--grown-flag") {
		t.Fatalf("--grown-flag shown before the CLI gained it; screen:\n%s", s.screen())
	}
	s.waitFor("--old-flag", 5*time.Second)
	s.stepDownUntil("fakegrow sprout", 6)

	if err := os.WriteFile(grown, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s.send("r")
	s.waitFor("expanded: sprout", 10*time.Second)
	s.waitFor("--grown-flag", 5*time.Second)

	s.send("q")
	if code := s.exitCode(10 * time.Second); code != 0 {
		t.Errorf("exit code %d after q, want 0", code)
	}
}

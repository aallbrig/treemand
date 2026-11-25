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

// TestInteractive_overrideSurvivesDiscoveryInTheTUI is issue #479: the
// background crawl and a re-discovery merge through one expander, which
// re-applies the CLI's override, so its corrections stay on screen.
func TestInteractive_overrideSurvivesDiscoveryInTheTUI(t *testing.T) {
	env := writeOverride(t, "fakegrow", "commands:\n  sprout:\n    description: Corrected by the override\n")
	grown := filepath.Join(t.TempDir(), "grown")
	s := startWithEnv(t, append(env, "FAKE_GROWN="+grown), "-i", "--no-cache", "fakegrow")
	s.waitFor("discovered", 15*time.Second) // the background crawl filled sprout
	s.stepDownUntil("fakegrow sprout", 6)

	if err := os.WriteFile(grown, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s.send("r")
	s.waitFor("--grown-flag", 10*time.Second)
	s.waitFor("sprout  ·  Corrected", 5*time.Second)
	// The screen is the whole session, so the help's own one-liner must never
	// have replaced the override's on the tree row.
	if strings.Contains(s.screen(), "sprout  ·  Sprout things") {
		t.Errorf("discovery replaced the override's description; screen:\n%s", s.screen())
	}

	s.send("q")
	if code := s.exitCode(10 * time.Second); code != 0 {
		t.Errorf("exit code %d after q, want 0", code)
	}
}

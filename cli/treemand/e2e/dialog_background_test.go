//go:build !windows

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestInteractive_discoveryLandsWhileADialogIsOpen is issue #472: a discovery
// result that arrives while the flag dialog is open must still reach the tree,
// not be dropped because the dialog held the keyboard.
func TestInteractive_discoveryLandsWhileADialogIsOpen(t *testing.T) {
	dir := t.TempDir()
	arm, gate, done := filepath.Join(dir, "arm"), filepath.Join(dir, "gate"), filepath.Join(dir, "done")
	s := startWithEnv(t, []string{"FAKE_ARM=" + arm, "FAKE_GATE=" + gate, "FAKE_DONE=" + done},
		"-i", "--no-cache", "fakelate")
	s.waitFor("ripen", 15*time.Second)
	s.waitFor("--early-flag", 15*time.Second)
	s.stepDownUntil("fakelate ripen", 6)

	if err := os.WriteFile(arm, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s.send("r")                                   // re-discover: the fake now waits on the gate
	s.waitFor("discovering ripen", 5*time.Second) // keys sent together arrive as one
	s.send("f")
	s.waitFor("Add Flag", 5*time.Second)

	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitForFile(t, done, 10*time.Second)
	time.Sleep(500 * time.Millisecond) // let treemand parse and deliver the result

	s.send("\x1b") // close the dialog
	s.waitFor("expanded: ripen", 5*time.Second)
	s.waitFor("--late-flag", 5*time.Second)

	s.send("q")
	if code := s.exitCode(10 * time.Second); code != 0 {
		t.Errorf("exit code %d after q, want 0", code)
	}
}

func waitForFile(t *testing.T, path string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s never appeared", path)
}

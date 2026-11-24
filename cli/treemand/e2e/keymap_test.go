//go:build !windows

package e2e

import (
	"testing"
	"time"
)

// The ? overlay, the footer and the keys themselves come from one key map
// (#314): what the footer and overlay name is what the keys do, in every
// navigation scheme. Shift+→ expands a subtree in vim too (#312).
func TestInteractive_keyMapDrivesFooterOverlayAndKeys(t *testing.T) {
	s := startInteractive(t, "-i", "--no-cache", "fakecobra")
	s.waitFor("deploy", 15*time.Second)
	s.waitFor("?:keys", 5*time.Second)

	s.send("\x13") // Ctrl+S: arrows -> vim
	s.waitFor("nav: vim", 5*time.Second)

	s.send("\x1b[1;2C") // Shift+Right on the root
	s.waitFor("expanded: fakecobra", 5*time.Second)

	s.send("?")
	s.waitFor("Key Bindings", 5*time.Second)
	s.waitFor("Expand entire subtree", 5*time.Second)
	s.send("\x1b")                            // Esc closes the overlay
	s.waitFor("[vim] j/k:nav", 5*time.Second) // the vim footer, back after the overlay
	s.send("q")
	if code := s.exitCode(10 * time.Second); code != 0 {
		t.Errorf("exit code %d after q, want 0\nscreen:\n%s", code, s.screen())
	}
}

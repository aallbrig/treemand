//go:build !windows

package e2e

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07`)

// session is treemand -i running on a pseudo-terminal.
type session struct {
	t    *testing.T
	cmd  *exec.Cmd
	ptmx interface {
		Write([]byte) (int, error)
		Close() error
	}
	mu   sync.Mutex
	buf  bytes.Buffer
	done chan error
}

func startInteractive(t *testing.T, args ...string) *session {
	t.Helper()
	return startWithEnv(t, nil, args...)
}

func startWithEnv(t *testing.T, extraEnv []string, args ...string) *session {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = env(t, extraEnv...)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 100})
	if err != nil {
		t.Fatalf("starting treemand on a pty: %v", err)
	}
	s := &session{t: t, cmd: cmd, ptmx: ptmx, done: make(chan error, 1)}
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := ptmx.Read(chunk)
			if n > 0 {
				s.mu.Lock()
				s.buf.Write(chunk[:n])
				s.mu.Unlock()
				s.answerTerminalQueries(chunk[:n])
			}
			if err != nil {
				break
			}
		}
		s.done <- cmd.Wait()
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = ptmx.Close()
	})
	return s
}

// answerTerminalQueries plays the terminal: TUI libraries ask for the cursor
// position and the background color and wait for the reply, so a silent pty
// makes every start take several seconds.
func (s *session) answerTerminalQueries(out []byte) {
	if bytes.Contains(out, []byte("\x1b[6n")) {
		_, _ = s.ptmx.Write([]byte("\x1b[1;1R"))
	}
	if bytes.Contains(out, []byte("\x1b]11;?")) {
		_, _ = s.ptmx.Write([]byte("\x1b]11;rgb:0000/0000/0000\x07"))
	}
	if bytes.Contains(out, []byte("\x1b]10;?")) {
		_, _ = s.ptmx.Write([]byte("\x1b]10;rgb:ffff/ffff/ffff\x07"))
	}
}

func (s *session) screen() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ansi.ReplaceAllString(s.buf.String(), "")
}

func (s *session) waitFor(text string, within time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if strings.Contains(s.screen(), text) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	s.t.Fatalf("screen never showed %q within %v; screen so far:\n%s", text, within, s.screen())
}

func (s *session) send(keys string) {
	s.t.Helper()
	if _, err := s.ptmx.Write([]byte(keys)); err != nil {
		s.t.Fatalf("sending %q: %v", keys, err)
	}
}

// exitCode waits for the process to end and returns its exit code.
func (s *session) exitCode(within time.Duration) int {
	s.t.Helper()
	select {
	case err := <-s.done:
		if err == nil {
			return 0
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		s.t.Fatalf("waiting for treemand: %v", err)
	case <-time.After(within):
		s.t.Fatalf("treemand still running %v after quit; screen:\n%s", within, s.screen())
	}
	return -1
}

// appears reports whether text shows on screen within d, without failing.
func (s *session) appears(text string, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if strings.Contains(s.screen(), text) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// stepDownUntil presses Down until the status bar names the wanted item.
func (s *session) stepDownUntil(want string, maxSteps int) {
	s.t.Helper()
	for i := 0; i < maxSteps; i++ {
		s.send("\x1b[B")
		if s.appears(want, 400*time.Millisecond) {
			return
		}
	}
	s.t.Fatalf("selection never reached %q after %d Down presses; screen:\n%s", want, maxSteps, s.screen())
}

func TestInteractive_navigateAndQuit(t *testing.T) {
	s := startInteractive(t, "-i", "--no-cache", "fakecobra")

	s.waitFor("deploy", 15*time.Second) // the tree is drawn
	s.waitFor("fakecobra", time.Second)

	// The status bar names the selected item: first the root's flags (which
	// include the ones whose descriptions end in a section keyword), then the
	// subcommands.
	s.stepDownUntil("fakecobra --verbose", 6)
	s.stepDownUntil("fakecobra build", 8)
	s.stepDownUntil("fakecobra deploy", 3)

	s.send("q")
	if code := s.exitCode(10 * time.Second); code != 0 {
		t.Errorf("exit code %d after q, want 0\nscreen:\n%s", code, s.screen())
	}
}

func TestInteractive_ctrlCQuits(t *testing.T) {
	s := startInteractive(t, "-i", "--no-cache", "fakecobra")
	s.waitFor("deploy", 15*time.Second)
	s.send("\x03")
	s.exitCode(10 * time.Second) // any clean end is fine; it must not hang
}

func TestInteractive_missingCLIFailsFastWithoutAScreen(t *testing.T) {
	s := startInteractive(t, "-i", "no-such-cli-xyz")
	if code := s.exitCode(10 * time.Second); code == 0 {
		t.Error("exit 0 for a missing CLI, want non-zero")
	}
	mustContain(t, "terminal output", s.screen(), "not found")
}

// sgr matches a color/style escape sequence (ESC [ ... m).
var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// raw returns the output with escape sequences intact.
func (s *session) raw() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestColorOnATerminal(t *testing.T) {
	t.Run("colors by default", func(t *testing.T) {
		s := startWithEnv(t, nil, "--no-cache", "fakecobra")
		if code := s.exitCode(20 * time.Second); code != 0 {
			t.Fatalf("exit %d", code)
		}
		if !sgr.MatchString(s.raw()) {
			t.Errorf("no color sequences on a terminal:\n%q", s.raw())
		}
	})
	for name, c := range map[string]struct {
		env  []string
		args []string
	}{
		"NO_COLOR":   {env: []string{"NO_COLOR=1"}, args: []string{"--no-cache", "fakecobra"}},
		"--no-color": {args: []string{"--no-cache", "--no-color", "fakecobra"}},
	} {
		t.Run(name+" turns color off", func(t *testing.T) {
			s := startWithEnv(t, c.env, c.args...)
			if code := s.exitCode(20 * time.Second); code != 0 {
				t.Fatalf("exit %d", code)
			}
			if m := sgr.FindString(s.raw()); m != "" {
				t.Errorf("color sequence %q although color is off:\n%q", m, s.raw())
			}
			mustContain(t, "tree", s.screen(), "build", "deploy")
		})
	}
}

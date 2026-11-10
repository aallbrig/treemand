// Package e2e drives the built treemand binary as a user would, against small
// fake CLIs written as shell scripts. Hermetic: it needs only a Go toolchain
// and /bin/sh, never a real CLI such as git or docker.
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	binPath string // the built treemand binary
	fakeDir string // directory of fake CLIs, prepended to PATH
)

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "treemand-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdtemp:", err)
		return 1
	}
	defer os.RemoveAll(tmp)

	binPath = filepath.Join(tmp, "treemand")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = ".." // the module root, cli/treemand
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building treemand failed: %v\n%s", err, out)
		return 1
	}

	fakeDir = filepath.Join(tmp, "bin")
	if err := os.Mkdir(fakeDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		return 1
	}
	for name, body := range fakeCLIs {
		if err := os.WriteFile(filepath.Join(fakeDir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil { //nolint:gosec
			fmt.Fprintln(os.Stderr, "write fake CLI:", err)
			return 1
		}
	}
	return m.Run()
}

// result is one finished treemand run.
type result struct {
	stdout, stderr string
	exit           int
	elapsed        time.Duration
}

// env is the environment for treemand: fake CLIs first on PATH, and cache and
// config isolated in a fresh home so no test can see another's state.
func env(t *testing.T, extra ...string) []string {
	t.Helper()
	home := t.TempDir()
	e := []string{
		"PATH=" + fakeDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, "config"),
		"XDG_CACHE_HOME=" + filepath.Join(home, "cache"),
		"TERM=xterm-256color",
	}
	return append(e, extra...)
}

// run executes treemand with args and a hard limit, capturing both streams.
func run(t *testing.T, limit time.Duration, extraEnv []string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Env = env(t, extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	err := cmd.Run()
	res := result{stdout: stdout.String(), stderr: stderr.String(), elapsed: time.Since(start)}
	switch e := err.(type) {
	case nil:
	case *exec.ExitError:
		res.exit = e.ExitCode()
	default:
		t.Fatalf("running treemand %v: %v", args, err)
	}
	if ctx.Err() != nil {
		t.Fatalf("treemand %v did not finish within %v\nstdout: %s\nstderr: %s", args, limit, res.stdout, res.stderr)
	}
	return res
}

func mustContain(t *testing.T, what, text string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(text, w) {
			t.Errorf("%s is missing %q:\n%s", what, w, text)
		}
	}
}

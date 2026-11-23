package discovery

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckProbeAllowed(t *testing.T) {
	for _, name := range []string{"kill", "pkill", "/sbin/reboot", "SHUTDOWN.EXE", "poweroff", "taskkill.exe"} {
		var never *NeverProbeError
		if err := CheckProbeAllowed(name); !errors.As(err, &never) {
			t.Errorf("CheckProbeAllowed(%q) = %v, want a NeverProbeError", name, err)
		}
	}
	for _, name := range []string{"git", "killjoy", "go", "rebooter"} {
		if err := CheckProbeAllowed(name); err != nil {
			t.Errorf("CheckProbeAllowed(%q) = %v, want nil", name, err)
		}
	}
}

func TestCheckProbeAllowed_followsALinkOnPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "halt")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "innocent")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	err := CheckProbeAllowed("innocent")
	var never *NeverProbeError
	if !errors.As(err, &never) {
		t.Fatalf("a link to halt was allowed: %v", err)
	}
	if !strings.Contains(err.Error(), "never runs innocent") || !strings.Contains(err.Error(), "treemand --stdin innocent") {
		t.Errorf("message = %q", err.Error())
	}
}

func TestIsolateProbe_pointsHomeAndTempAtAFreshDirectory(t *testing.T) {
	cmd := exec.Command("true")
	cleanup, err := IsolateProbe(cmd, []string{"PAGER=cat"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, kv := range cmd.Env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			got[k] = v // later entries win, as exec does
		}
	}
	home, tmp := got["HOME"], got["TMPDIR"]
	if realHome, _ := os.UserHomeDir(); home == realHome {
		t.Fatal("HOME is the real home")
	}
	for _, d := range []string{home, tmp} {
		if info, err := os.Stat(d); err != nil || !info.IsDir() {
			t.Errorf("%s does not exist: %v", d, err)
		}
	}
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
		if !strings.HasPrefix(got[k], home) {
			t.Errorf("%s = %q, want under %s", k, got[k], home)
		}
	}
	if got["PAGER"] != "cat" {
		t.Errorf("extraEnv was dropped: PAGER=%q", got["PAGER"])
	}
	cleanup()
	if _, err := os.Stat(filepath.Dir(home)); !os.IsNotExist(err) {
		t.Errorf("cleanup left %s behind: %v", filepath.Dir(home), err)
	}
}

func TestIsolateProbe_refusesTheNeverProbeList(t *testing.T) {
	out, _, err := runCapped(context.Background(), "/sbin/reboot", []string{"--help"}, nil, 100)
	var never *NeverProbeError
	if !errors.As(err, &never) || len(out) != 0 {
		t.Fatalf("runCapped ran reboot: out=%q err=%v", out, err)
	}
}

func TestToolchainRoots_pinsExistingRootsToTheRealHome(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".pyenv"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PYENV_ROOT", "")
	t.Setenv("CARGO_HOME", "/elsewhere")
	saved := toolchainEnv
	toolchainEnv = nil
	t.Cleanup(func() { toolchainEnv = saved })
	toolchainEnv = computeToolchainRoots()
	joined := strings.Join(toolchainEnv, "\n")
	if !strings.Contains(joined, "PYENV_ROOT="+filepath.Join(home, ".pyenv")) {
		t.Errorf("PYENV_ROOT not pinned: %v", toolchainEnv)
	}
	if strings.Contains(joined, "CARGO_HOME=") || strings.Contains(joined, "RBENV_ROOT=") {
		t.Errorf("pinned a set or missing root: %v", toolchainEnv)
	}
}

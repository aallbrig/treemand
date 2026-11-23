package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProbesRunInAThrowawayHome drives a fake CLI that writes into HOME,
// XDG_CONFIG_HOME and TMPDIR on every start, as a tool that creates config or
// caches on first run would. The user's own directories must stay untouched,
// and the directories the probes saw must be gone when treemand exits.
func TestProbesRunInAThrowawayHome(t *testing.T) {
	home := t.TempDir()
	tmp := t.TempDir()
	log := filepath.Join(t.TempDir(), "seen.log")
	r := run(t, limit, []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, "config"),
		"TMPDIR=" + tmp,
		"FAKE_LOG=" + log,
	}, "--no-cache", "--output=json", "fakewriter")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	root := parseJSON(t, r.stdout)
	if _, ok := root.child("sync"); !ok {
		t.Errorf("the tree lost the sync subcommand: %s", r.stdout)
	}

	for _, dir := range []string{home, tmp} {
		var found []string
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, _ error) error {
			if strings.Contains(filepath.Base(p), "fakewriter") {
				found = append(found, p)
			}
			return nil
		})
		if len(found) > 0 {
			t.Errorf("a probe wrote into the user's %s: %v", dir, found)
		}
	}

	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the fake CLI never ran: %v", err)
	}
	lines := strings.Fields(string(raw))
	if len(lines) == 0 {
		t.Fatal("the fake CLI logged nothing")
	}
	for _, seen := range lines {
		// The probe directory is created inside the user's TMPDIR, so only an
		// exact match means the probe saw the user's own directory.
		if seen == home || seen == tmp || strings.HasPrefix(seen, home+string(os.PathSeparator)) {
			t.Errorf("a probe saw the user's directory %s", seen)
		}
		if _, err := os.Stat(seen); err == nil {
			t.Errorf("probe directory %s still exists after treemand exited", seen)
		}
	}
}

// TestNeverProbeListIsRefused: a tool that kills processes or powers the
// machine down is never started, even with --help; treemand says why and
// exits 1. A trap of that name on PATH proves nothing ran.
func TestNeverProbeListIsRefused(t *testing.T) {
	for _, name := range []string{"reboot", "killall", "shutdown"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "ran")
			trap := "#!/bin/sh\ntouch '" + marker + "'\necho 'Usage: " + name + " [command]'\n"
			if err := os.WriteFile(filepath.Join(dir, name), []byte(trap), 0o755); err != nil {
				t.Fatal(err)
			}
			for _, arg := range []string{name, filepath.Join(dir, name)} {
				r := run(t, limit, []string{"PATH=" + dir + string(os.PathListSeparator) + "/usr/bin:/bin"}, "--no-cache", arg)
				if r.exit != 1 {
					t.Errorf("treemand %s: exit %d, want 1", arg, r.exit)
				}
				mustContain(t, "stderr", r.stderr, "never runs "+name, "--stdin")
				if _, err := os.Stat(marker); err == nil {
					t.Fatalf("treemand %s ran the binary", arg)
				}
			}
		})
	}
}

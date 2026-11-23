package e2e

import (
	"regexp"
	"strings"
	"testing"
)

// externalRef matches anything that would make the page fetch another file.
var externalRef = regexp.MustCompile(`(?i)(src|href)\s*=|@import|url\(|https?://`)

func TestHTML_selfContainedPage(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=html", "fakecobra")
	if r.exit != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr: %q", r.exit, r.stderr)
	}
	mustContain(t, "html page", r.stdout,
		"<!doctype html>", "<title>fakecobra command tree</title>",
		`<label for="q"`, `<input id="q" type="search"`, "<details",
		">build<", ">deploy<", "Build things", "--output", "--dry-run")
	if m := externalRef.FindString(r.stdout); m != "" {
		t.Errorf("html page loads an external resource (%q)", m)
	}
}

func TestHTML_honoursDepthFilterAndCommandsOnly(t *testing.T) {
	r := run(t, limit, nil, "--no-cache", "--output=html", "--commands-only", "--filter=build", "fakecobra")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	if !strings.Contains(r.stdout, ">build<") || strings.Contains(r.stdout, ">deploy<") {
		t.Errorf("--filter=build should keep build and drop deploy:\n%s", r.stdout)
	}
	if strings.Contains(r.stdout, "--output") {
		t.Errorf("--commands-only should hide flags:\n%s", r.stdout)
	}
	root := run(t, limit, nil, "--no-cache", "--output=html", "--depth=0", "fakecobra")
	if strings.Contains(root.stdout, ">build<") {
		t.Errorf("--depth=0 should keep only the root:\n%s", root.stdout)
	}
}

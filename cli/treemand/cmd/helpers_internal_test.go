package cmd

import (
	"bytes"
	"context"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// syncBuffer is a bytes.Buffer safe to write from the spinner goroutine.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestSpinner_onATerminalAnimatesThenClearsTheLine(t *testing.T) {
	var buf syncBuffer
	s := &Spinner{w: &buf, isTTY: true}
	s.Start("discovering git…")
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(buf.String(), "discovering git…") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	s.Stop()
	out := buf.String()
	if !strings.Contains(out, "\r⠋ discovering git…") {
		t.Errorf("no first frame with the label: %q", out)
	}
	if !strings.HasSuffix(out, "\r\033[K") {
		t.Errorf("Stop did not clear the line: %q", out)
	}
	s.Stop() // a second Stop is a no-op
	if buf.String() != out {
		t.Errorf("a second Stop wrote more output")
	}
}

func TestFormatAge(t *testing.T) {
	for d, want := range map[time.Duration]string{
		10 * time.Second: "just now",
		5 * time.Minute:  "5m ago",
		3 * time.Hour:    "3h ago",
		50 * time.Hour:   "2d ago",
	} {
		if got := formatAge(d); got != want {
			t.Errorf("formatAge(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	for n, want := range map[int]string{
		512:             "512B",
		1536:            "1.5KB",
		3 * 1024 * 1024: "3.0MB",
	} {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestVersionString_namesCommitAndBuildDate(t *testing.T) {
	defer func(v, c, b string) { Version, Commit, BuildDate = v, c, b }(Version, Commit, BuildDate)
	Version, Commit, BuildDate = "1.2.3", "abc1234", "2026-11-22"
	if got, want := versionString(), "treemand 1.2.3 (abc1234) built 2026-11-22"; got != want {
		t.Errorf("versionString() = %q, want %q", got, want)
	}
}

func TestResolveVersion_fallsBackToTheModuleVersion(t *testing.T) {
	info := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: v}}, true
		}
	}
	none := func() (*debug.BuildInfo, bool) { return nil, false }
	cases := []struct {
		name, ldflag string
		read         func() (*debug.BuildInfo, bool)
		want         string
	}{
		{"ldflags win", "v1.2.3", info("v0.9.0"), "v1.2.3"},
		{"go install from the proxy", "dev", info("v0.20.3"), "v0.20.3"},
		{"local build", "dev", info("(devel)"), "dev"},
		{"no build info", "dev", none, "dev"},
		{"empty module version", "dev", info(""), "dev"},
	}
	for _, c := range cases {
		if got := resolveVersion(c.ldflag, c.read); got != c.want {
			t.Errorf("%s: resolveVersion(%q) = %q, want %q", c.name, c.ldflag, got, c.want)
		}
	}
}

func TestMCPExpand_leavesTheRootAlone(t *testing.T) {
	root := &models.Node{Name: "tool", FullPath: []string{"tool"}, Description: "kept"}
	if err := mcpExpand(context.Background(), "tool", root, root); err != nil {
		t.Fatal(err)
	}
	if root.Description != "kept" || root.Discovered {
		t.Errorf("the root was changed: %+v", root)
	}
}

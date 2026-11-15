//go:build !windows

package e2e

import (
	"testing"
	"time"
)

type jsonFlag struct {
	Name string `json:"name"`
}

type jsonFull struct {
	Name     string      `json:"name"`
	Stub     bool        `json:"stub"`
	Flags    []jsonFlag  `json:"flags"`
	Children []*jsonFull `json:"children"`
}

// TestStream_theTUIOpensBeforeSlowSubcommandsAnswer is issue #68: with every
// subcommand taking 2s, an up-front crawl keeps the screen blank for at least
// 4s (two levels). The TUI must instead be usable as soon as the root is read
// and fill the rest in.
func TestStream_theTUIOpensBeforeSlowSubcommandsAnswer(t *testing.T) {
	envs, logPath, _ := cacheEnv(t)
	envs = append(envs, "FAKE_DELAY=2")
	start := time.Now()
	s := startWithEnv(t, envs, "-i", "fakeslow")
	s.waitFor("beta", 10*time.Second)
	if opened := time.Since(start); opened > 1500*time.Millisecond {
		t.Errorf("the TUI took %v to show the tree; it should not wait for subcommands", opened)
	}
	s.waitFor("discovering…", 2*time.Second) // background progress is shown
	s.waitFor("discovered", 15*time.Second)  // and finishes
	s.send("q")
	if code := s.exitCode(10 * time.Second); code != 0 {
		t.Fatalf("exit %d", code)
	}

	// Everything discovered in the background was cached: a non-interactive
	// run is complete and never touches the CLI.
	resetLog(t, logPath)
	res := run(t, 20*time.Second, envs, "--output=json", "fakeslow")
	if n := invocations(t, logPath); n != 0 {
		t.Errorf("the cached tree was incomplete: %d invocations on the next run", n)
	}
	assertComplete(t, res.stdout)
}

// TestStream_quittingMidCrawlStillGivesCompleteOutputLater: the partial tree
// saved by an interrupted session is finished on the next non-interactive run.
func TestStream_quittingMidCrawlStillGivesCompleteOutputLater(t *testing.T) {
	envs, logPath, _ := cacheEnv(t)
	s := startWithEnv(t, append(envs, "FAKE_DELAY=3"), "-i", "fakeslow")
	s.waitFor("beta", 10*time.Second)
	s.send("q") // long before the 3s subcommands answer
	if code := s.exitCode(10 * time.Second); code != 0 {
		t.Fatalf("exit %d", code)
	}
	resetLog(t, logPath)
	res := run(t, 30*time.Second, envs, "--output=json", "fakeslow")
	if invocations(t, logPath) == 0 {
		t.Errorf("the pending subcommands were never discovered")
	}
	assertComplete(t, res.stdout)
}

func assertComplete(t *testing.T, out string) {
	t.Helper()
	var root jsonFull
	if err := jsonUnmarshal(out, &root); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(root.Children) != 2 {
		t.Fatalf("want alpha and beta, got %+v", root.Children)
	}
	for _, c := range root.Children {
		if c.Stub || len(c.Flags) == 0 {
			t.Errorf("%s is incomplete (stub=%v, %d flags)", c.Name, c.Stub, len(c.Flags))
		}
		if len(c.Children) != 1 || c.Children[0].Stub || len(c.Children[0].Flags) == 0 {
			t.Errorf("%s deep is missing or incomplete: %+v", c.Name, c.Children)
		}
	}
}

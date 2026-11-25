package discovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestProbeCheck_onlySendsAWordTheCLIDocuments(t *testing.T) {
	cases := []struct {
		name string
		p    Probe
		ok   bool
	}{
		{"path and flags", Probe{Path: []string{"mod"}, Args: []string{"mod", "--help"}}, true},
		{"undocumented word", Probe{Args: []string{"list-commands"}}, false},
		{"documented in help", Probe{Args: []string{"list-commands"}, Basis: Documented, DocumentedIn: "use list-commands to see them"}, true},
		{"only part of a longer word", Probe{Args: []string{"list"}, Basis: Documented, DocumentedIn: "see list-commands"}, false},
		{"not in the text", Probe{Args: []string{"list-commands"}, Basis: Documented, DocumentedIn: "usage: touch FILE"}, false},
		{"help fallback", Probe{Path: []string{"mod"}, Args: []string{"help", "mod"}, Basis: HelpFallback}, true},
		{"help fallback sends only help", Probe{Args: []string{"prune"}, Basis: HelpFallback}, false},
		{"declared", Probe{Args: []string{"__complete", ""}, Basis: Declared}, true},
	}
	for _, c := range cases {
		if err := c.p.check(); (err == nil) != c.ok {
			t.Errorf("%s: check() = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestRunner_refusesAnUndocumentedWordWithoutLaunching(t *testing.T) {
	cli := &fakeCLI{answer: func([]string) reply { return reply{out: "acted\n"} }}
	r := Runner{Launcher: cli}.Run(context.Background(), "formcli", Probe{Args: []string{"list-commands"}})
	if !errors.Is(r.Err, ErrUndocumentedWord) || len(cli.calls()) != 0 {
		t.Errorf("Run = %+v, calls %q; want ErrUndocumentedWord and no launch", r, cli.calls())
	}
}

func TestRunner_eachLaunchHasItsOwnTimeout(t *testing.T) {
	cli := &fakeCLI{answer: func([]string) reply { return reply{delay: time.Minute} }}
	r := Runner{Launcher: cli, Timeout: 50 * time.Millisecond}.Run(context.Background(), "formcli", Probe{Args: []string{"--help"}})
	if !r.TimedOut || r.Cancelled {
		t.Errorf("Run = %+v, want TimedOut", r)
	}
}

func TestRunner_aCancelledCallerIsNoTimeout(t *testing.T) {
	cli := &fakeCLI{answer: func([]string) reply { return reply{delay: time.Minute} }}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r := Runner{Launcher: cli}.Run(ctx, "formcli", Probe{Args: []string{"--help"}})
	if !r.Cancelled || r.TimedOut {
		t.Errorf("Run = %+v, want Cancelled", r)
	}
}

// envCLI records the environment each launch is given.
type envCLI struct{ env [][]string }

func (e *envCLI) Launch(_ context.Context, _ string, _, env []string) ([]byte, error) {
	e.env = append(e.env, env)
	return nil, nil
}

func TestRunner_everyLaunchGetsThePagerEnvironment(t *testing.T) {
	cli := &envCLI{}
	_ = Runner{Launcher: cli}.Run(context.Background(), "formcli", Probe{Args: []string{"--help"}})
	_, _ = (&CompletionsDiscoverer{Launcher: cli}).Discover(context.Background(), "formcli", nil)
	if len(cli.env) != 2 {
		t.Fatalf("%d launches, want 2", len(cli.env))
	}
	for i, env := range cli.env {
		if !slices.Contains(env, "PAGER=cat") {
			t.Errorf("launch %d env %q lacks PAGER=cat", i, env)
		}
	}
}

func TestRunner_holdsAProcessSlotPerLaunch(t *testing.T) {
	SetMaxConcurrentHelp(1)
	defer SetMaxConcurrentHelp(0)
	cli := &fakeCLI{answer: func([]string) reply { return reply{delay: 100 * time.Millisecond} }}
	done := make(chan struct{})
	for range 2 {
		go func() {
			Runner{Launcher: cli}.Run(context.Background(), "formcli", Probe{Args: []string{"--help"}})
			done <- struct{}{}
		}()
	}
	<-done
	<-done
	if ev := cli.events; len(ev) != 4 || !strings.HasPrefix(ev[1], "end") {
		t.Errorf("events %q: the second launch started before the first ended", ev)
	}
}

// helperCLIEnv makes this test binary act as a small CLI (see TestMain), so
// the exec launcher is exercised on every platform without a shell script.
const helperCLIEnv = "TREEMAND_TEST_HELPER_CLI"

func TestMain(m *testing.M) {
	if os.Getenv(helperCLIEnv) == "1" {
		helperCLI(os.Args[1:])
		return
	}
	os.Exit(m.Run())
}

func helperCLI(args []string) {
	switch strings.Join(args, " ") {
	case "--help":
		fmt.Print("helper does things\n\nUsage:\n  helper [command]\n\nAvailable Commands:\n  alpha  first\n\nFlags:\n  -v, --verbose  verbose\n")
	case "alpha --help":
		fmt.Printf("Usage:\n  helper alpha [flags]\n\nFlags:\n  -f, --force  force it\n\npager=%s\n", os.Getenv("PAGER"))
	default:
		fmt.Fprintln(os.Stderr, "error: unknown command")
		os.Exit(2)
	}
}

func TestExecLauncher_discoversARealProcess(t *testing.T) {
	t.Setenv(helperCLIEnv, "1")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root, err := NewHelpDiscoverer(2).Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 1 || root.Children[0].Name != "alpha" || len(root.Children[0].Flags) == 0 {
		t.Fatalf("children = %+v", root.Children)
	}
	if !strings.Contains(root.Children[0].HelpText, "pager=cat") {
		t.Errorf("alpha's help %q: the process did not get the pager environment", root.Children[0].HelpText)
	}
}

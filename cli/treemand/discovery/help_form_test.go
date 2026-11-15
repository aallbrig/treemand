package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// formCLI writes a fake CLI that logs each invocation to calls.log, sleeps
// delay, and answers help only in the shapes the body allows. It returns the
// executable path and the log path.
func formCLI(t *testing.T, delay time.Duration, body string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\n" +
		"echo \"$*\" >>'" + logPath + "'\n" +
		"sleep " + fmt.Sprintf("%.3f", delay.Seconds()) + "\n" +
		body
	bin := filepath.Join(dir, "formcli")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, logPath
}

func calls(t *testing.T, logPath string) []string {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// suffixOnly answers only "<path> help", like aws; --help and -h are errors.
const suffixOnly = `last=""; for a in "$@"; do last="$a"; done
if [ "$last" != help ]; then echo "error: unknown option $last" >&2; exit 2; fi
case "$1" in
alpha|beta|gamma) printf 'NAME\n  %s - does %s\n\nOPTIONS\n  --force  force it\n' "$1" "$1";;
*) printf 'formcli - a fake aws-style tool\n\nAvailable Commands:\n  alpha  first\n  beta   second\n  gamma  third\n';;
esac
`

// flagOnly answers --help everywhere, like cobra tools.
const flagOnly = `case "$*" in
*--help*) ;;
*) echo "error: unknown command" >&2; exit 2;;
esac
case "$1" in
alpha|beta) printf 'Usage:\n  formcli %s [flags]\n\nFlags:\n  -f, --force  force it\n' "$1";;
*) printf 'formcli does things\n\nUsage:\n  formcli [command]\n\nAvailable Commands:\n  alpha  first\n  beta   second\n\nFlags:\n  -v, --verbose  verbose\n';;
esac
`

// slowFlagWithSideEffects answers --help after 600ms (well past the hedge) and
// treats a positional "help" as an action with a side effect, the way
// "git branch help" creates a branch: it touches $MARKER.
const slowFlagWithSideEffects = `case "$*" in
*--help*) sleep 0.6;;
*-h*) echo "error: unknown shorthand flag" >&2; exit 2;;
*help*) touch "$MARKER"; echo "did something to help"; exit 0;;
*) echo "error" >&2; exit 2;;
esac
case "$1" in
alpha|beta) printf 'Usage:\n  formcli %s [flags]\n\nFlags:\n  -f, --force  force it\n' "$1";;
*) printf 'formcli does things\n\nUsage:\n  formcli [command]\n\nAvailable Commands:\n  alpha  first\n  beta   second\n\nFlags:\n  -v, --verbose  verbose\n';;
esac
`

func TestRunHelp_neverPassesHelpAsAnArgumentWhileAFlagFormWorks(t *testing.T) {
	bin, logPath := formCLI(t, 0, slowFlagWithSideEffects)
	marker := filepath.Join(t.TempDir(), "side-effect")
	t.Setenv("MARKER", marker)

	if _, err := NewHelpDiscoverer(2).Discover(context.Background(), bin, nil); err != nil {
		t.Fatal(err)
	}
	// The crawler starts many nodes at once before any form is learned.
	d := NewHelpDiscoverer(1)
	d.ChildrenAsStubs = true
	root, err := d.Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	CompletePending(context.Background(), root, bin, 2, 50, 8)

	if _, err := os.Stat(marker); err == nil {
		t.Errorf("a positional help form ran although --help works; calls: %q", calls(t, logPath))
	}
	for _, c := range calls(t, logPath) {
		if f := strings.Fields(c); len(f) > 0 && (f[len(f)-1] == "help" || f[0] == "help") {
			t.Errorf("positional help form invoked: %q", c)
		}
	}
}

func TestRunHelp_positionalFormsRunOnlyAfterBothFlagsFail(t *testing.T) {
	bin, logPath := formCLI(t, 0, suffixOnly)
	root, err := NewHelpDiscoverer(2).Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 3 || len(root.Children[0].Flags) == 0 {
		t.Fatalf("aws-style help was not discovered: %+v", root.Children)
	}
	// For each node, "help" must come after both flag forms in the log.
	seen := map[string]map[string]bool{}
	for _, c := range calls(t, logPath) {
		f := strings.Fields(c)
		node, form := strings.Join(f[:len(f)-1], " "), f[len(f)-1]
		if seen[node] == nil {
			seen[node] = map[string]bool{}
		}
		if form == "help" && (!seen[node]["--help"] || !seen[node]["-h"]) {
			t.Errorf("%q ran before both flag forms for %q: %q", c, node, calls(t, logPath))
		}
		seen[node][form] = true
	}
}

func TestRunHelp_fastPreferredFormStartsNoAlternatives(t *testing.T) {
	bin, logPath := formCLI(t, 0, flagOnly)
	if _, err := NewHelpDiscoverer(2).Discover(context.Background(), bin, nil); err != nil {
		t.Fatal(err)
	}
	got := calls(t, logPath)
	if len(got) != 3 {
		t.Errorf("a --help CLI should cost one process per node (3), got %d: %q", len(got), got)
	}
	for _, c := range got {
		if !strings.HasSuffix(c, "--help") {
			t.Errorf("unexpected help form %q", c)
		}
	}
}

// shortOnly answers only -h; --help is an error.
const shortOnly = `case "$*" in
*--help*) echo "error: unknown flag: --help" >&2; exit 2;;
*-h*) ;;
*) echo "error" >&2; exit 2;;
esac
case "$1" in
alpha|beta|gamma) printf 'Usage: formcli %s [-f]\n\nOptions:\n  -f, --force  force it\n' "$1";;
*) printf 'Usage: formcli <command>\n\nCommands:\n  alpha  first\n  beta   second\n  gamma  third\n'
esac
`

func TestRunHelp_learnedShortFlagIsTriedFirstAndFlagsRace(t *testing.T) {
	// Each call takes 600ms. Trying --help then -h in turn costs 1.2s per
	// node: 2.4s for the root and its children. Racing the flags at the root
	// and asking children -h first costs about 0.85s + 0.6s.
	bin, logPath := formCLI(t, 600*time.Millisecond, shortOnly)
	start := time.Now()
	root, err := NewHelpDiscoverer(2).Discover(context.Background(), bin, nil)
	elapsed := time.Since(start)
	if err != nil || len(root.Children) != 3 || len(root.Children[0].Flags) == 0 {
		t.Fatalf("discovery failed: %v %+v", err, root)
	}
	if elapsed > 2*time.Second {
		t.Errorf("root + children took %v; trying flags in turn takes 2.4s", elapsed)
	}
	// Children ask the learned -h first (a slow -h may still let --help join
	// it after the hedge delay, which is fine).
	for _, child := range []string{"alpha", "beta", "gamma"} {
		for _, c := range calls(t, logPath) {
			if strings.HasPrefix(c, child+" ") {
				if c != child+" -h" {
					t.Errorf("%s was first asked %q; want the learned -h", child, c)
				}
				break
			}
		}
	}
}

func TestRunHelp_childFallsBackWhenTheLearnedFormFails(t *testing.T) {
	// go-style: the root answers --help, but subcommands only answer
	// "help <sub>".
	bin, _ := formCLI(t, 0, `case "$*" in
"--help") printf 'Usage:\n  formcli <command>\n\nThe commands are:\n\n  mod  module maintenance\n';;
"help mod") printf 'usage: formcli mod <command>\n\nThe commands are:\n\n  tidy  add missing modules\n';;
"help mod tidy") printf 'usage: formcli mod tidy [-v]\n';;
*) echo "formcli: unknown command" >&2; exit 2;;
esac
`)
	root, err := NewHelpDiscoverer(3).Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 1 || root.Children[0].Name != "mod" || root.Children[0].HelpText == "" {
		t.Fatalf("mod was not discovered through the help-prefix form: %+v", root.Children)
	}
	mod := root.Children[0]
	if len(mod.Children) != 1 || mod.Children[0].Name != "tidy" {
		t.Errorf("mod's children not discovered: %+v", mod.Children)
	}
}

func TestRaceForms_noLoserOutlivesTheDiscovery(t *testing.T) {
	// --help wins after 600ms. The losing -h leaves a child holding its stdout,
	// so after it is killed its reader waits for the pipe (up to 1s). That
	// goroutine must be finished when Discover returns: one still running
	// outlived the discovery and reported into the next test's hooks.
	bin, _ := formCLI(t, 0, `case "$*" in
*--help*) sleep 0.6; printf 'Usage: formcli\n\nOptions:\n  -v, --verbose  verbose\n';;
*-h*) sleep 5 & sleep 5;;
esac
`)
	if _, err := NewHelpDiscoverer(1).Discover(context.Background(), bin, nil); err != nil {
		t.Fatal(err)
	}
	if n := runningForms(); n > 0 {
		t.Errorf("%d help form(s) still running when Discover returned", n)
	}
}

// runningForms counts goroutines inside tryForm, the race's worker.
func runningForms() int {
	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]
	return strings.Count(string(buf), ").tryForm(")
}

func TestRunHelp_aTimedOutFlagFormNeverLeadsToPositionalHelp(t *testing.T) {
	// --help and -h hang (an overloaded CLI); "help" as an argument would act,
	// as "npm prune help" prunes. A timeout is not a refusal.
	bin, logPath := formCLI(t, 0, `case "$*" in
*--help*|*-h*) sleep 5;;
*help*) touch "$MARKER";;
esac
`)
	marker := filepath.Join(t.TempDir(), "acted")
	t.Setenv("MARKER", marker)
	d := NewHelpDiscoverer(1)
	d.Timeout = 300 * time.Millisecond
	_, _ = d.Discover(context.Background(), bin, []string{"prune"})
	if _, err := os.Stat(marker); err == nil {
		t.Errorf("positional help ran after the flag forms timed out; calls: %q", calls(t, logPath))
	}
}

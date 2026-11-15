package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil { //nolint:gosec // test fixture must be executable
		t.Fatal(err)
	}
	return bin, logPath
}

func calls(t *testing.T, logPath string) []string {
	t.Helper()
	b, err := os.ReadFile(logPath) //nolint:gosec // test-owned path
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

func TestRunHelp_learnsTheRootFormSoChildrenCostOneProcess(t *testing.T) {
	bin, logPath := formCLI(t, 0, suffixOnly)
	d := NewHelpDiscoverer(2)
	root, err := d.Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 3 {
		t.Fatalf("want 3 children, got %d", len(root.Children))
	}
	for _, c := range root.Children {
		if len(c.Flags) == 0 {
			t.Errorf("%s: help was not parsed (no flags)", c.Name)
		}
	}
	got := calls(t, logPath)
	childCalls := 0
	for _, c := range got {
		if strings.HasPrefix(c, "alpha") || strings.HasPrefix(c, "beta") || strings.HasPrefix(c, "gamma") {
			childCalls++
			if c != strings.Fields(c)[0]+" help" {
				t.Errorf("child asked with %q; want the learned form only", c)
			}
		}
	}
	if childCalls != 3 {
		t.Errorf("want exactly 1 call per child (3), got %d: %q", childCalls, got)
	}
	// Root: at most the three root forms (--help, -h, help); before learning
	// it was 3 calls for every node, 12 in total.
	if len(got) > 6 {
		t.Errorf("want at most 6 invocations, got %d: %q", len(got), got)
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

func TestRunHelp_failingFormsDoNotRunSerially(t *testing.T) {
	// Each call takes 600ms. Asking --help, -h and help one after another, as
	// before, costs 1.8s per node: 3.6s for the root and one level of
	// children. Racing at the root and reusing the learned form costs about
	// one call per level plus the hedge delay.
	bin, _ := formCLI(t, 600*time.Millisecond, suffixOnly)
	start := time.Now()
	root, err := NewHelpDiscoverer(2).Discover(context.Background(), bin, nil)
	elapsed := time.Since(start)
	if err != nil || len(root.Children) != 3 {
		t.Fatalf("discovery failed: %v", err)
	}
	if elapsed > 2800*time.Millisecond {
		t.Errorf("root + children took %v; serial fallbacks take 3.6s", elapsed)
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

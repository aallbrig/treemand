package discovery

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// reply is what a fake CLI prints for one call, its exit code, and how long
// it takes (cut short when the call's context ends, as a killed process is).
type reply struct {
	out   string
	exit  int
	delay time.Duration
}

// fakeCLI is the in-memory Launcher the help-chain tests script: answer maps
// an argument list to a reply. It logs "start <args>" and "end <args>" for
// every call, so a test can check which forms ran and whether they overlapped.
type fakeCLI struct {
	answer func(args []string) reply

	mu     sync.Mutex
	events []string
}

type fakeExitError int

func (e fakeExitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

func (f *fakeCLI) note(e string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}

func (f *fakeCLI) Launch(ctx context.Context, _ string, args, _ []string) ([]byte, error) {
	call := strings.Join(args, " ")
	f.note("start " + call)
	defer f.note("end " + call)
	r := f.answer(slices.Clone(args))
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if r.exit != 0 {
		return []byte(r.out), fakeExitError(r.exit)
	}
	return []byte(r.out), nil
}

// calls returns the argument lists the CLI was started with, in order.
func (f *fakeCLI) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, e := range f.events {
		if c, ok := strings.CutPrefix(e, "start "); ok {
			out = append(out, c)
		}
	}
	return out
}

// sentPositionalHelp returns the calls that passed "help" as an argument.
func (f *fakeCLI) sentPositionalHelp() []string {
	var out []string
	for _, c := range f.calls() {
		if slices.Contains(strings.Fields(c), "help") {
			out = append(out, c)
		}
	}
	return out
}

// discoverer returns a HelpDiscoverer that launches this fake.
func (f *fakeCLI) discoverer(maxDepth int) *HelpDiscoverer {
	d := NewHelpDiscoverer(maxDepth)
	d.Launcher = f
	return d
}

func lastArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[len(args)-1]
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func refused(msg string) reply { return reply{out: msg + "\n", exit: 2} }

// cobraHelp is the help of a cobra-style CLI with commands alpha and beta.
func cobraHelp(args []string) reply {
	switch c := firstArg(args); c {
	case "alpha", "beta":
		return reply{out: "Usage:\n  formcli " + c + " [flags]\n\nFlags:\n  -f, --force  force it\n"}
	}
	return reply{out: "formcli does things\n\nUsage:\n  formcli [command]\n\nAvailable Commands:\n  alpha  first\n  beta   second\n\nFlags:\n  -v, --verbose  verbose\n"}
}

// flagOnly answers --help everywhere, like cobra tools.
func flagOnly(args []string) reply {
	if !slices.Contains(args, "--help") {
		return refused("error: unknown command")
	}
	return cobraHelp(args)
}

// suffixOnly answers only "<path> help", like aws; --help and -h are errors.
func suffixOnly(args []string) reply {
	if l := lastArg(args); l != "help" {
		return refused("error: unknown option " + l)
	}
	switch c := firstArg(args); c {
	case "alpha", "beta", "gamma":
		return reply{out: "NAME\n  " + c + " - does " + c + "\n\nOPTIONS\n  --force  force it\n"}
	}
	return reply{out: "formcli - a fake aws-style tool\n\nAvailable Commands:\n  alpha  first\n  beta   second\n  gamma  third\n"}
}

// shortOnly answers only -h; --help is an error.
func shortOnly(args []string) reply {
	switch {
	case slices.Contains(args, "--help"):
		return refused("error: unknown flag: --help")
	case !slices.Contains(args, "-h"):
		return refused("error")
	}
	switch c := firstArg(args); c {
	case "alpha", "beta", "gamma":
		return reply{out: "Usage: formcli " + c + " [-f]\n\nOptions:\n  -f, --force  force it\n"}
	}
	return reply{out: "Usage: formcli <command>\n\nCommands:\n  alpha  first\n  beta   second\n  gamma  third\n"}
}

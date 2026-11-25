package discovery

import (
	"context"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// slowFlagWithSideEffects answers --help after 600ms (well past the hedge)
// and treats a positional "help" as an action, the way "git branch help"
// creates a branch.
func slowFlagWithSideEffects(args []string) reply {
	switch {
	case slices.Contains(args, "--help"):
		r := cobraHelp(args)
		r.delay = 600 * time.Millisecond
		return r
	case slices.Contains(args, "-h"):
		return refused("error: unknown shorthand flag")
	case slices.Contains(args, "help"):
		return reply{out: "did something to help\n"}
	}
	return refused("error")
}

func TestRunHelp_neverPassesHelpAsAnArgumentWhileAFlagFormWorks(t *testing.T) {
	cli := &fakeCLI{answer: slowFlagWithSideEffects}
	if _, err := cli.discoverer(2).Discover(context.Background(), "formcli", nil); err != nil {
		t.Fatal(err)
	}
	// The crawler starts many nodes at once before any form is learned.
	d := cli.discoverer(1)
	d.ChildrenAsStubs = true
	root, err := d.Discover(context.Background(), "formcli", nil)
	if err != nil {
		t.Fatal(err)
	}
	CompletePending(context.Background(), root, "formcli", ExpandOptions{MaxDepth: 2, Launcher: cli}, 50, 8)

	if got := cli.sentPositionalHelp(); len(got) > 0 {
		t.Errorf("positional help form invoked although --help works: %q", got)
	}
}

func TestRunHelp_positionalFormsRunOnlyAfterBothFlagsFail(t *testing.T) {
	cli := &fakeCLI{answer: suffixOnly}
	root, err := cli.discoverer(2).Discover(context.Background(), "formcli", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 3 || len(root.Children[0].Flags) == 0 {
		t.Fatalf("aws-style help was not discovered: %+v", root.Children)
	}
	// For each node, "help" must come after both flag forms in the log.
	seen := map[string]map[string]bool{}
	for _, c := range cli.calls() {
		f := strings.Fields(c)
		node, form := strings.Join(f[:len(f)-1], " "), f[len(f)-1]
		if seen[node] == nil {
			seen[node] = map[string]bool{}
		}
		if form == "help" && (!seen[node]["--help"] || !seen[node]["-h"]) {
			t.Errorf("%q ran before both flag forms for %q: %q", c, node, cli.calls())
		}
		seen[node][form] = true
	}
}

func TestRunHelp_fastPreferredFormStartsNoAlternatives(t *testing.T) {
	cli := &fakeCLI{answer: flagOnly}
	if _, err := cli.discoverer(2).Discover(context.Background(), "formcli", nil); err != nil {
		t.Fatal(err)
	}
	got := cli.calls()
	if len(got) != 3 {
		t.Errorf("a --help CLI should cost one process per node (3), got %d: %q", len(got), got)
	}
	for _, c := range got {
		if !strings.HasSuffix(c, "--help") {
			t.Errorf("unexpected help form %q", c)
		}
	}
}

func TestRunHelp_learnedShortFlagIsTriedFirstAndFlagsRace(t *testing.T) {
	// The root's calls take 1s. Racing the flags starts -h while --help still
	// runs (the hedge fires at 250ms); trying them in turn would end --help
	// before -h starts.
	cli := &fakeCLI{answer: func(args []string) reply {
		r := shortOnly(args)
		if len(args) == 1 {
			r.delay = time.Second
		}
		return r
	}}
	root, err := cli.discoverer(2).Discover(context.Background(), "formcli", nil)
	if err != nil || len(root.Children) != 3 || len(root.Children[0].Flags) == 0 {
		t.Fatalf("discovery failed: %v %+v", err, root)
	}
	if !rootFlagsOverlap(cli.events) {
		t.Errorf("the root's --help and -h did not run at once; events: %q", cli.events)
	}
	// Children ask the learned -h first (a slow -h may still let --help join
	// it after the hedge delay, which is fine).
	for _, child := range []string{"alpha", "beta", "gamma"} {
		for _, c := range cli.calls() {
			if strings.HasPrefix(c, child+" ") {
				if c != child+" -h" {
					t.Errorf("%s was first asked %q; want the learned -h", child, c)
				}
				break
			}
		}
	}
}

// rootFlagsOverlap reports whether the root's --help and -h both started
// before either of them ended.
func rootFlagsOverlap(events []string) bool {
	started := 0
	for _, e := range events {
		switch e {
		case "start --help", "start -h":
			started++
		case "end --help", "end -h":
			return started == 2
		}
	}
	return false
}

func TestRunHelp_childFallsBackWhenTheLearnedFormFails(t *testing.T) {
	// go-style: the root answers --help, but subcommands only answer
	// "help <sub>".
	cli := &fakeCLI{answer: func(args []string) reply {
		switch strings.Join(args, " ") {
		case "--help":
			return reply{out: "Usage:\n  formcli <command>\n\nThe commands are:\n\n  mod  module maintenance\n"}
		case "help mod":
			return reply{out: "usage: formcli mod <command>\n\nThe commands are:\n\n  tidy  add missing modules\n"}
		case "help mod tidy":
			return reply{out: "usage: formcli mod tidy [-v]\n"}
		}
		return refused("formcli: unknown command")
	}}
	root, err := cli.discoverer(3).Discover(context.Background(), "formcli", nil)
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
	// --help wins after 600ms while -h would run for 5s. The losing form must
	// be finished when Discover returns: one still running outlived the
	// discovery and reported into the next test's hooks.
	cli := &fakeCLI{answer: func(args []string) reply {
		if slices.Contains(args, "--help") {
			return reply{out: "Usage: formcli\n\nOptions:\n  -v, --verbose  verbose\n", delay: 600 * time.Millisecond}
		}
		return reply{delay: 5 * time.Second}
	}}
	if _, err := cli.discoverer(1).Discover(context.Background(), "formcli", nil); err != nil {
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
	cli := &fakeCLI{answer: func(args []string) reply {
		if slices.Contains(args, "--help") || slices.Contains(args, "-h") {
			return reply{delay: 5 * time.Second}
		}
		return reply{out: "pruned\n"}
	}}
	d := cli.discoverer(1)
	d.Timeout = 300 * time.Millisecond
	_, _ = d.Discover(context.Background(), "formcli", []string{"prune"})
	if got := cli.sentPositionalHelp(); len(got) > 0 {
		t.Errorf("positional help ran after the flag forms timed out: %q", got)
	}
}

func TestRunHelp_aLearnedFormIsAskedAloneOnSlowChildren(t *testing.T) {
	// Each call takes 400ms, past the hedge delay. Once the root has shown
	// that --help works, a child must not also be asked -h: on a slow CLI
	// that doubled the processes on every node (gh: 457 for 236 nodes, #216).
	cli := &fakeCLI{answer: func(args []string) reply {
		r := flagOnly(args)
		r.delay = 400 * time.Millisecond
		return r
	}}
	if _, err := cli.discoverer(2).Discover(context.Background(), "formcli", nil); err != nil {
		t.Fatal(err)
	}
	for _, c := range cli.calls() {
		if f := strings.Fields(c); len(f) == 2 && f[1] == "-h" {
			t.Errorf("child asked %q although --help was learned; calls: %q", c, cli.calls())
		}
	}
}

// bsdOperandCLI behaves like BSD mkdir: every flag form is rejected with an
// "illegal option" line followed by the usage synopsis, exit 64, and every
// other argument is an operand it acts on (mkdir creates a directory named
// "help").
func bsdOperandCLI(args []string) reply {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return reply{out: "formcli: illegal option -- " + a[1:] + "\nusage: formcli [-pv] [-m mode] directory_name ...\n", exit: 64}
		}
	}
	return reply{}
}

func TestRunHelp_usageAfterAnIllegalOptionIsHelpAndNoPositionalFormRuns(t *testing.T) {
	for _, args := range [][]string{nil, {"sub"}} {
		cli := &fakeCLI{answer: bsdOperandCLI}
		node, err := cli.discoverer(1).Discover(context.Background(), "formcli", args)
		if got := cli.sentPositionalHelp(); len(got) > 0 {
			t.Errorf("args %q: a positional help form ran although the flag form printed a usage synopsis: %q", args, got)
		}
		if err != nil || node == nil || !strings.Contains(node.HelpText, "usage: formcli [-pv]") {
			t.Errorf("args %q: the usage synopsis was not used as help: err=%v node=%+v", args, err, node)
		}
	}
}

// goStyle answers "<cmd> --help" with only a usage line and a pointer to
// "help <cmd>", like the go command, and "help <cmd>" with the full text.
func goStyle(args []string) reply {
	switch strings.Join(args, " ") {
	case "help build":
		return reply{out: "usage: formcli build [-o output] [packages]\n\nBuild compiles packages.\n\n\t-race\n\t\tenable data race detection.\n"}
	case "build --help":
		return reply{out: "usage: formcli build [-o output] [packages]\nRun 'formcli help build' for details.\n"}
	case "help doc":
		return reply{out: "usage: formcli doc [doc flags]\n\n\t-all\n\t\tShow all the documentation.\n"}
	case "doc --help":
		return reply{out: "Usage of [formcli] doc:\n\tformcli doc <pkg>\nFor more information run\n\tformcli help doc\n\n"}
	}
	if firstArg(args) == "build" {
		return reply{out: "built " + strings.Join(args[1:], " ") + "\n"}
	}
	return reply{out: "Formcli manages source.\n\nUsage:\n\n\tformcli <command> [arguments]\n\nThe commands are:\n\n\tbuild       compile packages\n\tdoc         show documentation\n"}
}

func TestRunHelp_followsAPointerToTheFullHelp(t *testing.T) {
	cli := &fakeCLI{answer: goStyle}
	root, err := cli.discoverer(2).Discover(context.Background(), "formcli", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 2 {
		t.Fatalf("children: %+v", root.Children)
	}
	got := map[string]bool{}
	for _, c := range root.Children {
		for _, f := range c.Flags {
			got[c.Name+" "+f.Name] = true
		}
	}
	if !got["build -race"] || !got["build -o"] || !got["doc -all"] {
		t.Errorf("build flags %v, want build -race (from help build), build -o and doc -all (from help doc); calls: %q", got, cli.calls())
	}
	for _, c := range cli.calls() {
		if c == "build" || strings.HasPrefix(c, "build help") {
			t.Errorf("ran a form the CLI did not name: %q", c)
		}
	}
}

// aws documents "aws <path> help" as its help and answers neither --help nor
// -h (each fails after ~2.4 s). The aws preset says so, and discovery asks the
// documented form first: one call instead of three in a row (#215).
func TestRunHelp_presetDocumentedHelpSubcommandIsAskedFirst(t *testing.T) {
	cli := &fakeCLI{answer: func(args []string) reply {
		if lastArg(args) != "help" {
			return reply{out: "error: argument operation: Invalid choice\n", exit: 252, delay: 2 * time.Second}
		}
		return reply{out: "AWS()\n\nNAME\n       aws -\n\nDESCRIPTION\n       The AWS Command Line Interface.\n\nAVAILABLE SERVICES\n       o s3\n\n       o ec2\n"}
	}}
	d := cli.discoverer(1)
	d.ChildrenAsStubs = true
	d.HelpSubcommand = true
	root, err := d.Discover(context.Background(), "aws", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 2 {
		t.Errorf("services not discovered: %+v", root.Children)
	}
	if got := cli.calls(); len(got) != 1 || got[0] != "help" {
		t.Errorf("calls = %q, want one: the documented \"help\"", got)
	}
}

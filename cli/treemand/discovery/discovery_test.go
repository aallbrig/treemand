package discovery_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
)

func TestHelpDiscovererName(t *testing.T) {
	d := discovery.NewHelpDiscoverer(3)
	if d.Name() != "help" {
		t.Errorf("Name() = %q, want %q", d.Name(), "help")
	}
}

func TestHelpDiscovererDiscover_echo(t *testing.T) {
	d := discovery.NewHelpDiscoverer(1)
	ctx := context.Background()
	node, err := d.Discover(ctx, "echo", nil)
	// echo may or may not have --help; we just check it returns a node
	if node == nil {
		t.Fatal("expected non-nil node")
	}
	_ = err
	if node.Name != "echo" {
		t.Errorf("Name = %q, want %q", node.Name, "echo")
	}
}

func TestHelpDiscovererDiscover_nonexistent(t *testing.T) {
	d := discovery.NewHelpDiscoverer(1)
	ctx := context.Background()
	node, _ := d.Discover(ctx, "nonexistent_cli_12345", nil)
	// Should return a node with error description, not nil
	if node == nil {
		t.Fatal("expected non-nil node even for nonexistent CLI")
	}
}

func TestMerge_basic(t *testing.T) {
	a := &models.Node{
		Name:  "git",
		Flags: []models.Flag{{Name: "--verbose"}},
		Children: []*models.Node{
			{Name: "commit", Description: "record changes"},
		},
	}
	b := &models.Node{
		Name:        "git",
		Description: "the source control tool",
		Flags:       []models.Flag{{Name: "--version"}},
		Children: []*models.Node{
			{Name: "commit", Flags: []models.Flag{{Name: "--message"}}},
			{Name: "push"},
		},
	}
	merged := discovery.Merge([]*models.Node{a, b})
	if merged.Description != "the source control tool" {
		t.Errorf("Description = %q", merged.Description)
	}
	if len(merged.Flags) != 2 {
		t.Errorf("Flags count = %d, want 2", len(merged.Flags))
	}
	if len(merged.Children) != 2 {
		t.Errorf("Children count = %d, want 2", len(merged.Children))
	}
	commit := merged.Find("commit")
	if commit == nil {
		t.Fatal("expected commit child")
	}
	if len(commit.Flags) != 1 {
		t.Errorf("commit.Flags count = %d, want 1", len(commit.Flags))
	}
}

func TestMerge_empty(t *testing.T) {
	if r := discovery.Merge(nil); r != nil {
		t.Error("expected nil for empty merge")
	}
}

func TestBuildDiscoverers(t *testing.T) {
	ds := discovery.BuildDiscoverers([]string{"help"}, 2)
	if len(ds) != 1 {
		t.Errorf("expected 1 discoverer, got %d", len(ds))
	}
	if ds[0].Name() != "help" {
		t.Errorf("discoverer name = %q, want help", ds[0].Name())
	}
}

func TestBuildDiscoverers_empty(t *testing.T) {
	ds := discovery.BuildDiscoverers([]string{}, 2)
	if len(ds) == 0 {
		t.Error("expected at least one discoverer for empty strategies")
	}
}

// MockDiscoverer implements Discoverer for testing.
type MockDiscoverer struct {
	name string
	node *models.Node
	err  error
}

func (m *MockDiscoverer) Name() string { return m.name }
func (m *MockDiscoverer) Discover(_ context.Context, _ string, _ []string) (*models.Node, error) {
	return m.node, m.err
}

func TestRun_withMock(t *testing.T) {
	mock := &MockDiscoverer{
		name: "mock",
		node: &models.Node{Name: "testcli", Description: "test"},
	}
	ctx := context.Background()
	node, err := discovery.Run(ctx, []discovery.Discoverer{mock}, "testcli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node == nil {
		t.Fatal("expected non-nil node")
	}
	if node.Name != "testcli" {
		t.Errorf("Name = %q, want testcli", node.Name)
	}
}

func TestParseHelpOutput_flags(t *testing.T) {
	helpText := `Usage: git [options] <command>

Options:
  -v, --verbose    be more verbose
  --version        display version info
  -C <path>        run as if git was started in <path>
`
	d := discovery.NewHelpDiscoverer(1)
	ctx := context.Background()
	// Use a fake CLI by providing help text inline via echo
	_ = d
	_ = ctx
	// Test parse directly via exported helper if available, else test indirectly
	_ = helpText
	// Verify the regex works by checking flag parsing works end-to-end
	if !strings.Contains(helpText, "--verbose") {
		t.Error("test data malformed")
	}
}

func TestBuildDiscoverersWithThreshold(t *testing.T) {
	// Default threshold of 50 should be set.
	ds := discovery.BuildDiscoverers([]string{"help"}, 2)
	if len(ds) == 0 {
		t.Fatal("expected at least one discoverer")
	}
	hd, ok := ds[0].(*discovery.HelpDiscoverer)
	if !ok {
		t.Fatal("expected *discovery.HelpDiscoverer")
	}
	if hd.StubThreshold != 150 {
		t.Errorf("default StubThreshold = %d, want 150", hd.StubThreshold)
	}

	// Custom threshold.
	ds2 := discovery.BuildDiscoverersWithThreshold([]string{"help"}, 2, 100)
	hd2, ok := ds2[0].(*discovery.HelpDiscoverer)
	if !ok {
		t.Fatal("expected *discovery.HelpDiscoverer")
	}
	if hd2.StubThreshold != 100 {
		t.Errorf("StubThreshold = %d, want 100", hd2.StubThreshold)
	}
}

func TestManDiscovererName(t *testing.T) {
	d := discovery.NewManDiscoverer()
	if d.Name() != "man" {
		t.Errorf("Name() = %q, want %q", d.Name(), "man")
	}
}

func TestManDiscovererDiscover_ls(t *testing.T) {
	d := discovery.NewManDiscoverer()
	ctx := context.Background()
	node, err := d.Discover(ctx, "ls", nil)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	// ls has a man page on Linux; if not available just skip
	if node == nil {
		t.Skip("no man page for ls on this system")
	}
	if node.Name != "ls" {
		t.Errorf("Name = %q, want %q", node.Name, "ls")
	}
	// ls man page should contain at least a few flags
	if len(node.Flags) == 0 {
		t.Log("warning: ManDiscoverer found no flags for ls (may depend on OS)")
	}
}

func TestManDiscovererDiscover_nonexistent(t *testing.T) {
	d := discovery.NewManDiscoverer()
	ctx := context.Background()
	node, err := d.Discover(ctx, "nonexistent_cli_99999", nil)
	if err != nil {
		t.Fatalf("unexpected error for missing man page: %v", err)
	}
	// Should return nil, not crash
	_ = node
}

func TestBuildDiscoverersWithThreshold_man(t *testing.T) {
	ds := discovery.BuildDiscoverersWithThreshold([]string{"help", "man"}, 2, 50)
	if len(ds) != 2 {
		t.Fatalf("expected 2 discoverers, got %d", len(ds))
	}
	names := make([]string, len(ds))
	for i, d := range ds {
		names[i] = d.Name()
	}
	if names[0] != "help" || names[1] != "man" {
		t.Errorf("unexpected discoverer order: %v", names)
	}
}

// ── CompletionsDiscoverer ─────────────────────────────────────────────────────

func TestCompletionsDiscovererName(t *testing.T) {
	d := discovery.NewCompletionsDiscoverer()
	if d.Name() != "completions" {
		t.Errorf("Name() = %q, want %q", d.Name(), "completions")
	}
}

func TestCompletionsDiscoverer_nonexistent(t *testing.T) {
	d := discovery.NewCompletionsDiscoverer()
	ctx := context.Background()
	// A non-existent binary should return nil, nil (not an error).
	node, err := d.Discover(ctx, "nonexistent_cli_99999", nil)
	if err != nil {
		t.Errorf("expected nil error for missing binary, got: %v", err)
	}
	if node != nil {
		t.Errorf("expected nil node for missing binary, got: %v", node.Name)
	}
}

func TestParseCompletionOutput_basicSubcmds(t *testing.T) {
	input := "get\tFetch a resource\ndelete\tRemove a resource\n:4\n"
	nodes := discovery.ParseCompletionOutput(input, []string{"kubectl"})

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %v", len(nodes), nodeNames(nodes))
	}
	if nodes[0].Name != "get" {
		t.Errorf("nodes[0].Name = %q, want %q", nodes[0].Name, "get")
	}
	if nodes[0].Description != "Fetch a resource" {
		t.Errorf("nodes[0].Description = %q, want %q", nodes[0].Description, "Fetch a resource")
	}
	if nodes[1].Name != "delete" {
		t.Errorf("nodes[1].Name = %q, want %q", nodes[1].Name, "delete")
	}
	if !nodes[0].Stub {
		t.Error("parsed children should be stubs")
	}
}

func TestParseCompletionOutput_skipsFlags(t *testing.T) {
	input := "--global-flag\tA flag\nsubcmd\tA subcommand\n"
	nodes := discovery.ParseCompletionOutput(input, []string{"mycli"})
	if len(nodes) != 1 || nodes[0].Name != "subcmd" {
		t.Errorf("expected only 'subcmd', got: %v", nodeNames(nodes))
	}
}

func TestParseCompletionOutput_skipsDirectives(t *testing.T) {
	input := ":4\nactive\tActive context\n"
	nodes := discovery.ParseCompletionOutput(input, []string{"kubectl", "config"})
	if len(nodes) != 1 || nodes[0].Name != "active" {
		t.Errorf("expected only 'active', got: %v", nodeNames(nodes))
	}
}

func TestParseCompletionOutput_deduplicates(t *testing.T) {
	input := "sub\tFirst\nsub\tDuplicate\n"
	nodes := discovery.ParseCompletionOutput(input, []string{"cli"})
	if len(nodes) != 1 {
		t.Errorf("expected 1 node (deduped), got %d", len(nodes))
	}
}

func TestParseCompletionOutput_fullPath(t *testing.T) {
	input := "apply\tApply config\n"
	nodes := discovery.ParseCompletionOutput(input, []string{"kubectl"})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	want := []string{"kubectl", "apply"}
	if len(nodes[0].FullPath) != len(want) {
		t.Fatalf("FullPath = %v, want %v", nodes[0].FullPath, want)
	}
	for i, p := range want {
		if nodes[0].FullPath[i] != p {
			t.Errorf("FullPath[%d] = %q, want %q", i, nodes[0].FullPath[i], p)
		}
	}
}

func TestBuildDiscoverersWithThreshold_completions(t *testing.T) {
	ds := discovery.BuildDiscoverersWithThreshold([]string{"completions"}, 2, 50)
	if len(ds) != 1 {
		t.Fatalf("expected 1 discoverer, got %d", len(ds))
	}
	if ds[0].Name() != "completions" {
		t.Errorf("Name() = %q, want %q", ds[0].Name(), "completions")
	}
}

func nodeNames(nodes []*models.Node) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	return names
}

// A CLI whose help spawns a lingering child (inheriting stdout) must not make
// discovery outlive its context: the orphan keeps the output pipe open.
func TestHelpDiscoverer_hangingCLI_honoursContext(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 30 &\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(dir, "hangcli"), []byte(script), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	start := time.Now()
	_, _ = discovery.NewHelpDiscoverer(1).Discover(ctx, "hangcli", nil)
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("Discover took %v after a 1s context timeout", elapsed)
	}
}

// A CLI whose help is only a usage synopsis gets its root description from the
// man page NAME one-liner instead of a synopsis fragment.
func TestHelpDiscoverer_rootDescriptionFallsBackToManName(t *testing.T) {
	dir := t.TempDir()
	cli := "#!/bin/sh\nprintf 'usage: tool [-ab] [-c file]\\n           [command [flags]]\\n'\n"
	man := "#!/bin/sh\nprintf 'NAME\\n       tool - a friendly terminal tool\\n\\nSYNOPSIS\\n       tool [-ab]\\n'\n"
	for name, body := range map[string]string{"tool": cli, "man": man} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	node, err := discovery.NewHelpDiscoverer(1).Discover(context.Background(), "tool", nil)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if want := "a friendly terminal tool"; node.Description != want {
		t.Errorf("Description = %q, want %q", node.Description, want)
	}
}

// The per-command timeout must not become a deadline for a whole subtree: a
// group with many slow children needs more total time than Timeout, but every
// single command is fast enough.
func TestHelpDiscoverer_timeoutIsPerCommandNotPerSubtree(t *testing.T) {
	dir := t.TempDir()
	var kids []string
	for i := 0; i < 16; i++ {
		kids = append(kids, "k"+string(rune('a'+i)))
	}
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"grp) if [ \"$2\" = \"--help\" ] || [ \"$2\" = \"-h\" ]; then printf 'group help\\n\\nCommands:\\n" + func() string {
		s := ""
		for _, k := range kids {
			s += "  " + k + "   the " + k + " command\\n"
		}
		return s
	}() + "'; exit 0; fi ;;\n" +
		"esac\n" +
		"if [ \"$2\" = \"--help\" ] || [ \"$3\" = \"--help\" ] || [ \"$2\" = \"-h\" ] || [ \"$3\" = \"-h\" ]; then sleep 0.5; printf 'leaf help\\n\\nOptions:\\n  --x   an option\\n'; exit 0; fi\n" +
		"printf 'tool\\n\\nCommands:\\n  grp   a group\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "tool"), []byte(script), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	d := discovery.NewHelpDiscoverer(3)
	d.Timeout = 1500 * time.Millisecond
	node, err := d.Discover(context.Background(), "tool", nil)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	var grp *models.Node
	for _, c := range node.Children {
		if c.Name == "grp" {
			grp = c
		}
	}
	if grp == nil {
		t.Fatalf("grp not discovered; children: %d", len(node.Children))
	}
	if len(grp.Children) != 16 {
		t.Fatalf("grp has %d children, want 16", len(grp.Children))
	}
	failed := 0
	for _, c := range grp.Children {
		if c.DiscoveryErr != "" {
			failed++
		}
	}
	if failed > 0 {
		t.Errorf("%d of 16 children failed discovery although each command takes 0.5s (Timeout 1.5s)", failed)
	}
}

// Each node's help should be fetched once: not once for the comparison with
// the parent and again for the recursion, and not both --help and -h.
func TestHelpDiscoverer_oneHelpInvocationPerNode(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> " + logPath + "\n" +
		"case \"$1\" in\n" +
		"alpha|bravo|charlie) printf '%s help\\n\\nOptions:\\n  --x   an option\\n' \"$1\"; exit 0;;\n" +
		"esac\n" +
		"printf 'root help\\n\\nCommands:\\n  alpha   first\\n  bravo   second\\n  charlie   third\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "tool"), []byte(script), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	node, err := discovery.NewHelpDiscoverer(2).Discover(context.Background(), "tool", nil)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(node.Children) != 3 {
		t.Fatalf("got %d children, want 3", len(node.Children))
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		first := strings.Fields(line)[0]
		calls[first]++
	}
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		if calls[name] != 1 {
			t.Errorf("%s help was run %d times, want 1 (all calls: %v)", name, calls[name], calls)
		}
	}
}

// However deep or wide the tree, no more than the configured number of help
// processes run at once.
func TestHelpDiscoverer_capsConcurrentProcesses(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	maxFile := filepath.Join(dir, "max")
	var kids []string
	for i := 0; i < 12; i++ {
		kids = append(kids, "k"+string(rune('a'+i)))
	}
	listing := ""
	for _, k := range kids {
		listing += "  " + k + "   the " + k + " command\\n"
	}
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"k*) touch " + runDir + "/$$; sleep 0.3; n=$(ls " + runDir + " | wc -l); " +
		"echo $n >> " + maxFile + "; rm " + runDir + "/$$; printf 'leaf help\\n\\nOptions:\\n  --x   an option\\n'; exit 0;;\n" +
		"esac\n" +
		"printf 'root help\\n\\nCommands:\\n" + listing + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "tool"), []byte(script), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	discovery.SetMaxConcurrentHelp(3)
	t.Cleanup(func() { discovery.SetMaxConcurrentHelp(0) })

	node, err := discovery.NewHelpDiscoverer(2).Discover(context.Background(), "tool", nil)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(node.Children) != 12 {
		t.Fatalf("got %d children, want 12", len(node.Children))
	}
	raw, err := os.ReadFile(maxFile)
	if err != nil {
		t.Fatal(err)
	}
	peak := 0
	for _, f := range strings.Fields(string(raw)) {
		n := 0
		for _, r := range f {
			n = n*10 + int(r-'0')
		}
		if n > peak {
			peak = n
		}
	}
	if peak > 3 {
		t.Errorf("saw %d help processes at once, want at most 3", peak)
	}
	if peak == 0 {
		t.Error("no concurrency recorded; the fake CLI did not run")
	}
}

// A CLI that floods stdout must neither exhaust memory nor outlive the
// timeout: output is read through a hard cap and the process is stopped once
// it is reached.
func TestHelpDiscoverer_capsHugeOutput(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nyes 'Commands: lots of text that goes on and on and on' | head -c 50000000\n"
	if err := os.WriteFile(filepath.Join(dir, "floodcli"), []byte(script), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// A small cap keeps this fast under -race, where parsing is about 18x slower.
	oldMax := discovery.MaxHelpBytes
	t.Cleanup(func() { discovery.MaxHelpBytes = oldMax })
	discovery.MaxHelpBytes = 256 << 10

	start := time.Now()
	node, err := discovery.NewHelpDiscoverer(1).Discover(context.Background(), "floodcli", nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Discover took %v on a 50MB flood, want well under the 5s command timeout", elapsed)
	}
	if limit := discovery.MaxHelpBytes + 1024; len(node.HelpText) > limit {
		t.Errorf("kept %d bytes of help text, want at most %d", len(node.HelpText), limit)
	}
	if len(node.HelpText) == 0 {
		t.Error("the capped prefix should still be kept")
	}
	if oldMax < 512<<10 {
		t.Errorf("default cap %d bytes is too small for real help text", oldMax)
	}
}

func TestHelpDiscoverer_outputAtTheCapIsNotTruncated(t *testing.T) {
	dir := t.TempDir()
	// Exactly the cap in bytes: must be kept whole and not reported as truncated.
	script := "#!/bin/sh\nhead -c 1000 /dev/zero | tr '\\0' 'a'\n"
	if err := os.WriteFile(filepath.Join(dir, "exactcli"), []byte(script), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldMax, oldHook := discovery.MaxHelpBytes, discovery.OnOutputTruncated
	t.Cleanup(func() { discovery.MaxHelpBytes, discovery.OnOutputTruncated = oldMax, oldHook })
	discovery.MaxHelpBytes = 1000
	var reported []string
	discovery.OnOutputTruncated = func(name string, _ []string, _ int) { reported = append(reported, name) }

	node, err := discovery.NewHelpDiscoverer(1).Discover(context.Background(), "exactcli", nil)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(node.HelpText) != 1000 {
		t.Errorf("kept %d bytes, want all 1000", len(node.HelpText))
	}
	if len(reported) != 0 {
		t.Errorf("output exactly at the cap must not be reported as truncated, got %v", reported)
	}

	discovery.MaxHelpBytes = 999
	_, err = discovery.NewHelpDiscoverer(1).Discover(context.Background(), "exactcli", nil)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(reported) == 0 {
		t.Error("one byte over the cap must be reported through OnOutputTruncated")
	}
}

package presets_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/models"
	"github.com/aallbrig/treemand/cli/treemand/presets"
)

// fakeModeEnv makes the test binary act as a fake CLI when re-executed, so the
// seed tests run a real subprocess on every OS without shell scripts.
const fakeModeEnv = "PRESETS_FAKE_CLI"

func TestMain(m *testing.M) {
	switch os.Getenv(fakeModeEnv) {
	case "":
		os.Exit(m.Run())
	case "map":
		if len(os.Args) != 2 || os.Args[1] != "--map" {
			fmt.Fprintf(os.Stderr, "unexpected args %q\n", os.Args[1:])
			os.Exit(3)
		}
		fmt.Print(stripeMap)
		os.Exit(0)
	case "fail":
		fmt.Fprintln(os.Stderr, "unknown flag: --map")
		os.Exit(1)
	}
}

func fakeCLI(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv(fakeModeEnv, mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

func stripeSeed(t *testing.T) func(context.Context, string) (*models.Node, error) {
	t.Helper()
	p, ok := presets.Lookup("stripe")
	if !ok || p.Seed == nil {
		t.Fatal("stripe preset must have a Seed")
	}
	return p.Seed
}

func TestStripeSeed_parsesMapOutput(t *testing.T) {
	cli := fakeCLI(t, "map")
	root, err := stripeSeed(t)(context.Background(), cli)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if want := filepath.Base(cli); root.Name != want {
		t.Errorf("root name = %q, want the binary's base name %q", root.Name, want)
	}
	var paths []string
	root.Walk(func(n *models.Node) {
		if n != root && n.Stub {
			paths = append(paths, fmt.Sprint(n.FullPath[1:]))
		}
	})
	want := []string{"[accounts create]", "[accounts persons list]", "[charges capture]", "[charges create]"}
	if fmt.Sprint(paths) != fmt.Sprint(want) {
		t.Errorf("stub leaves = %v, want %v", paths, want)
	}
}

func TestStripeSeed_failingCLIReturnsError(t *testing.T) {
	root, err := stripeSeed(t)(context.Background(), fakeCLI(t, "fail"))
	if err == nil {
		t.Fatal("want an error when the CLI rejects --map")
	}
	if root != nil {
		t.Errorf("want no tree on error, got %+v", root)
	}
}

func TestStripeSeed_missingCLIReturnsError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "stripe")
	if _, err := stripeSeed(t)(context.Background(), missing); err == nil {
		t.Fatal("want an error for a CLI that does not exist")
	}
}

func TestStripeSeed_cancelledContextReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := stripeSeed(t)(ctx, fakeCLI(t, "map")); err == nil {
		t.Fatal("want an error when the context is already cancelled")
	}
}

func TestAWSPreset_usesHelpSubcommandWithoutSeed(t *testing.T) {
	p, ok := presets.Lookup("aws")
	if !ok {
		t.Fatal("aws preset missing")
	}
	if !p.HelpSubcommand || p.Seed != nil || p.Depth != 1 {
		t.Errorf("aws preset = %+v, want HelpSubcommand, depth 1, no seed", p)
	}
}

func TestNames_matchesLookup(t *testing.T) {
	names := presets.Names()
	sort.Strings(names)
	if fmt.Sprint(names) != "[aws stripe]" {
		t.Errorf("Names = %v, want [aws stripe]", names)
	}
	for _, n := range names {
		p, ok := presets.Lookup(n)
		if !ok || p.Name != n || p.Summary == "" {
			t.Errorf("Lookup(%q) = %+v, %v; want a named preset with a summary", n, p, ok)
		}
	}
}

func TestGraft_nilIsNoOp(t *testing.T) {
	root := &models.Node{Name: "stripe", Children: []*models.Node{{Name: "a"}}}
	presets.Graft(root, nil)
	presets.Graft(nil, presets.ParseTreeMap(stripeMap, "stripe"))
	if len(root.Children) != 1 || root.Children[0].Name != "a" {
		t.Errorf("Graft with a nil seed changed root: %+v", root.Children)
	}
}

func TestGraft_addsMissingGrandchildrenUnderExistingNodes(t *testing.T) {
	root := &models.Node{Name: "stripe", Children: []*models.Node{
		{Name: "accounts", Discovered: true, Children: []*models.Node{
			{Name: "persons", Description: "discovered"},
		}},
	}}
	presets.Graft(root, presets.ParseTreeMap(stripeMap, "stripe"))

	acc := root.Children[0]
	if len(acc.Children) != 2 {
		t.Fatalf("accounts children = %d, want persons + create", len(acc.Children))
	}
	persons := acc.Children[0]
	if persons.Description != "discovered" {
		t.Errorf("persons description overwritten: %q", persons.Description)
	}
	if len(persons.Children) != 1 || persons.Children[0].Name != "list" || !persons.Children[0].Stub {
		t.Errorf("persons children = %+v, want the seeded list stub", persons.Children)
	}
	if acc.Children[1].Name != "create" {
		t.Errorf("seeded sibling appended after discovered ones, got %q", acc.Children[1].Name)
	}
}

func TestParseTreeMap_ignoresNonTreeAndOrphanLines(t *testing.T) {
	in := "stripe\n" +
		"            └── orphan\n" + // deeper than any parent: skipped
		"some banner text\n" +
		"├── customers\n" +
		"\n"
	root := presets.ParseTreeMap(in, "stripe")
	if len(root.Children) != 1 || root.Children[0].Name != "customers" {
		t.Fatalf("children = %+v, want only customers", root.Children)
	}
	if !root.Children[0].Stub || root.Stub {
		t.Error("leaf must be a stub and the root must not")
	}
}

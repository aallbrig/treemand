package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

var updateTreeStylesGolden = flag.Bool("update-tree-styles", false, "rewrite testdata/tree_styles.golden")

// styleFixture exercises every branch a row renderer takes: descriptions
// long enough to cut, typed and bool flags, more own flags than fit inline,
// an active flag, a discovery error, required and optional positionals.
func styleFixture() *models.Node {
	long := "a description long enough that every style has to cut it somewhere before the edge"
	many := []models.Flag{{Name: "--verbose", Inherited: true, Description: long}}
	for i, vt := range []string{"", "string", "int", "duration", "bool", "stringArray", "count"} {
		many = append(many, models.Flag{Name: fmt.Sprintf("--f%d", i), ValueType: vt, Description: "flag " + vt})
	}
	few := []models.Flag{{Name: "--force", ShortName: "f", Description: long}, {Name: "--out", ValueType: "string"}}
	broken := &models.Node{Name: "broken", FullPath: []string{"app", "sub", "broken"}, DiscoveryErr: "timed out", Description: "never answered"}
	leaf := &models.Node{
		Name: "leaf", FullPath: []string{"app", "sub", "leaf"}, Description: long,
		Positionals: []models.Positional{{Name: "file", Required: true, Description: long}, {Name: "rest"}},
	}
	sub := &models.Node{Name: "sub", FullPath: []string{"app", "sub"}, Description: long, Flags: few, Children: []*models.Node{broken, leaf}}
	other := &models.Node{Name: "other", FullPath: []string{"app", "other"}, Description: "short one", Flags: many}
	return &models.Node{
		Name: "app", FullPath: []string{"app"}, Description: long,
		Flags:    []models.Flag{{Name: "--help"}},
		Children: []*models.Node{sub, other},
	}
}

// TestTreeStylesGolden draws the fixture in every display style, collapsed
// and fully expanded, focused or not, with a command line that activates
// flags, so any change to how a row looks shows up as a golden diff.
func TestTreeStylesGolden(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	var b strings.Builder
	for style, name := range config.DisplayStyleNames {
		for _, expandAll := range []bool{false, true} {
			tm := NewTreeModel(styleFixture(), config.DefaultConfig())
			tm.SetDisplayStyle(config.DisplayStyle(style))
			tm.SetCmdTokens([]string{"app", "other", "--f1=x", "-f"})
			tm.SetFocused(expandAll)
			if expandAll {
				tm.ExpandAll()
			} else {
				tm.Right() // expand the root's subcommands section
				tm.Right()
			}
			tm.Down()
			tm.Down()
			fmt.Fprintf(&b, "== %s expandAll=%v\n", name, expandAll)
			for _, line := range strings.Split(tm.ViewSized(70, 40), "\n") {
				fmt.Fprintf(&b, "%q\n", line)
			}
		}
	}
	checkTreeStylesGolden(t, b.String())
}

func checkTreeStylesGolden(t *testing.T, got string) {
	t.Helper()
	path := filepath.Join("testdata", "tree_styles.golden")
	if *updateTreeStylesGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update-tree-styles to create it): %v", err)
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
		if gotLines[i] != wantLines[i] {
			t.Fatalf("tree differs at golden line %d:\n got: %s\nwant: %s", i+1, gotLines[i], wantLines[i])
		}
	}
	if len(gotLines) != len(wantLines) {
		t.Fatalf("tree differs in length: got %d lines, want %d", len(gotLines), len(wantLines))
	}
}

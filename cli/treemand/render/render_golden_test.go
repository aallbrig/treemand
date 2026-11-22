package render_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/render"
)

var updateTextGolden = flag.Bool("update-text-golden", false, "rewrite testdata/text_render.golden")

// goldenTree reaches every branch of the text renderer: required and optional
// positionals, inherited flags, an inline flag list with every value type, a
// flag count, a virtual node, stub and error suffixes, a truncated
// description, and names the filter and exclude cases select on.
func goldenTree() *models.Node {
	long := strings.Repeat("a long description ", 6)
	many := make([]models.Flag, 0, 7)
	for i := range 7 {
		many = append(many, models.Flag{Name: fmt.Sprintf("--opt%d", i), ValueType: "string"})
	}
	return &models.Node{
		Name:        "tool",
		Description: "the root",
		Flags:       []models.Flag{{Name: "--help", ValueType: "bool"}},
		Children: []*models.Node{
			{
				Name:        "build",
				Description: long,
				Positionals: []models.Positional{{Name: "target", Required: true}, {Name: "extra"}},
				Flags: []models.Flag{
					{Name: "--help", ValueType: "bool", Inherited: true},
					{Name: "--force", ValueType: "bool"},
					{Name: "--name", ValueType: "string"},
					{Name: "--jobs", ValueType: "int"},
					{Name: "--wait", ValueType: "duration"},
					{Name: "--dry-run"},
				},
				Children: []*models.Node{
					{Name: "image", Flags: many},
					{Name: "cache", Stub: true},
				},
			},
			{Name: "plugins", Virtual: true, Children: []*models.Node{
				{Name: "broken", DiscoveryErr: "exit 1", Description: "fails"},
				{Name: "deep", Children: []*models.Node{{Name: "deeper-image"}}},
			}},
			{Name: "version"},
		},
	}
}

func TestRenderTextGolden(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	cases := []struct {
		name string
		edit func(*render.Options)
	}{
		{"default", func(*render.Options) {}},
		{"no color", func(o *render.Options) { o.NoColor = true }},
		{"max depth 1", func(o *render.Options) { o.MaxDepth = 1 }},
		{"filter image", func(o *render.Options) { o.Filter = "image" }},
		{"exclude plugins", func(o *render.Options) { o.Exclude = "plugins" }},
		{"commands only", func(o *render.Options) { o.CommandsOnly = true }},
		{"desc limit 10", func(o *render.Options) { o.DescLineLength = 10 }},
		{"desc limit 0", func(o *render.Options) { o.DescLineLength = 0 }},
	}
	var b strings.Builder
	for _, c := range cases {
		opts := render.DefaultOptions()
		c.edit(&opts)
		out, err := render.ToString(goldenTree(), opts)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "== %s\n", c.name)
		for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
			fmt.Fprintf(&b, "%q\n", line)
		}
	}
	checkTextGolden(t, b.String())
}

// checkTextGolden compares got with testdata/text_render.golden, or rewrites
// the golden when -update-text-golden is set.
func checkTextGolden(t *testing.T, got string) {
	t.Helper()
	path := filepath.Join("testdata", "text_render.golden")
	if *updateTextGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update-text-golden to create it): %v", err)
	}
	if got == string(want) {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
		if gotLines[i] != wantLines[i] {
			t.Fatalf("text render differs at golden line %d:\n got: %s\nwant: %s", i+1, gotLines[i], wantLines[i])
		}
	}
	t.Fatalf("text render differs in length: got %d lines, want %d", len(gotLines), len(wantLines))
}

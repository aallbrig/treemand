package render_test

import (
	"strings"
	"testing"

	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/render"
)

func sampleTree() *models.Node {
	return &models.Node{
		Name:        "git",
		FullPath:    []string{"git"},
		Description: "the version control system",
		Flags: []models.Flag{
			{Name: "--version", ValueType: "bool"},
			{Name: "--verbose", ShortName: "v", ValueType: "bool"},
		},
		Children: []*models.Node{
			{
				Name:        "commit",
				FullPath:    []string{"git", "commit"},
				Description: "record changes to the repository",
				Flags:       []models.Flag{{Name: "--message", ShortName: "m", ValueType: "string"}},
				Positionals: []models.Positional{{Name: "file", Required: false, Variadic: true}},
			},
			{
				Name:     "remote",
				FullPath: []string{"git", "remote"},
				Children: []*models.Node{
					{Name: "add", FullPath: []string{"git", "remote", "add"},
						Positionals: []models.Positional{{Name: "name", Required: true}, {Name: "url", Required: true}}},
					{Name: "remove", FullPath: []string{"git", "remote", "remove"}},
				},
			},
		},
	}
}

func TestRenderToString_text(t *testing.T) {
	opts := render.DefaultOptions()
	opts.NoColor = true
	got, err := render.RenderToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("RenderToString error: %v", err)
	}
	if !strings.Contains(got, "git") {
		t.Error("expected 'git' in output")
	}
	if !strings.Contains(got, "commit") {
		t.Error("expected 'commit' in output")
	}
	if !strings.Contains(got, "remote") {
		t.Error("expected 'remote' in output")
	}
	if !strings.Contains(got, "add") {
		t.Error("expected 'add' in output")
	}
}

func TestRenderToString_json(t *testing.T) {
	opts := render.DefaultOptions()
	opts.Output = "json"
	got, err := render.RenderToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("RenderToString error: %v", err)
	}
	if !strings.Contains(got, `"name"`) {
		t.Error("expected JSON output with name field")
	}
	if !strings.Contains(got, `"git"`) {
		t.Error("expected 'git' in JSON output")
	}
}

func TestRenderToString_maxDepth(t *testing.T) {
	opts := render.DefaultOptions()
	opts.NoColor = true
	opts.MaxDepth = 1
	got, err := render.RenderToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("RenderToString error: %v", err)
	}
	// "add" is at depth 2 (git=0, remote=1, add=2), should not appear
	if strings.Contains(got, "└── ▼ add") || strings.Contains(got, "├── • add") {
		t.Error("depth-2 node 'add' should not appear at MaxDepth=1")
	}
}

func TestRenderToString_filter(t *testing.T) {
	opts := render.DefaultOptions()
	opts.NoColor = true
	opts.Filter = "remote"
	got, err := render.RenderToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("RenderToString error: %v", err)
	}
	if !strings.Contains(got, "remote") {
		t.Error("expected 'remote' in filtered output")
	}
}

func TestRenderToString_unknownFormat(t *testing.T) {
	opts := render.DefaultOptions()
	opts.Output = "yaml"
	_, err := render.RenderToString(sampleTree(), opts)
	if err == nil {
		t.Error("expected error for unknown output format")
	}
}

func TestCollect(t *testing.T) {
	stats := render.Collect(sampleTree())
	if stats.Commands == 0 {
		t.Error("expected non-zero command count")
	}
	if stats.MaxDepth < 2 {
		t.Errorf("MaxDepth = %d, want >= 2", stats.MaxDepth)
	}
}

func TestRenderToString_icons(t *testing.T) {
	opts := render.DefaultOptions()
	opts.NoColor = true
	got, err := render.RenderToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("RenderToString error: %v", err)
	}
	if !strings.Contains(got, "▼") {
		t.Error("expected branch icon ▼ in output")
	}
	if !strings.Contains(got, "•") {
		t.Error("expected leaf icon • in output")
	}
}

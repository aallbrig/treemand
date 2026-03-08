package render_test

import (
	"strings"
	"testing"

	"github.com/aallbrig/treemand/config"
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
	got, err := render.ToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("ToString error: %v", err)
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
	got, err := render.ToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("ToString error: %v", err)
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
	got, err := render.ToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("ToString error: %v", err)
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
	got, err := render.ToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("ToString error: %v", err)
	}
	if !strings.Contains(got, "remote") {
		t.Error("expected 'remote' in filtered output")
	}
}

func TestRenderToString_unknownFormat(t *testing.T) {
	opts := render.DefaultOptions()
	opts.Output = "toml"
	_, err := render.ToString(sampleTree(), opts)
	if err == nil {
		t.Error("expected error for unknown output format")
	}
}

func TestRenderToString_yaml(t *testing.T) {
	opts := render.DefaultOptions()
	opts.Output = "yaml"
	out, err := render.ToString(sampleTree(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "name:") {
		t.Errorf("expected YAML output to contain 'name:', got: %s", out)
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
	got, err := render.ToString(sampleTree(), opts)
	if err != nil {
		t.Fatalf("ToString error: %v", err)
	}
	if !strings.Contains(got, "▼") {
		t.Error("expected branch icon ▼ in output")
	}
	if !strings.Contains(got, "•") {
		t.Error("expected leaf icon • in output")
	}
}

func TestRenderNode_flagStyles(t *testing.T) {
// Exercise flagStyle branches: bool, string, int, other.
root := &models.Node{
Name:     "tool",
FullPath: []string{"tool"},
Flags: []models.Flag{
{Name: "--flag-bool", ValueType: "bool"},
{Name: "--flag-str", ValueType: "string"},
{Name: "--flag-int", ValueType: "int"},
{Name: "--flag-other", ValueType: "duration"},
{Name: "--flag-empty", ValueType: ""},
},
}
opts := render.DefaultOptions()
opts.NoColor = true
got, err := render.ToString(root, opts)
if err != nil {
t.Fatalf("ToString error: %v", err)
}
for _, name := range []string{"--flag-bool", "--flag-str", "--flag-int", "--flag-other"} {
if !strings.Contains(got, name) {
t.Errorf("expected flag %q in output", name)
}
}
}

func TestRenderNode_discoveryErr(t *testing.T) {
root := &models.Node{
Name:     "aws",
FullPath: []string{"aws"},
Children: []*models.Node{
{
Name:         "s3",
FullPath:     []string{"aws", "s3"},
DiscoveryErr: "could not get help: timeout",
},
},
}
opts := render.DefaultOptions()
opts.NoColor = true
got, err := render.ToString(root, opts)
if err != nil {
t.Fatalf("ToString error: %v", err)
}
// Should show subtle (?) indicator, NOT the full error text.
if strings.Contains(got, "could not get help") {
t.Error("full error text should not appear in rendered output")
}
if !strings.Contains(got, "(?)") {
t.Error("expected (?) indicator for discovery error node")
}
}

func TestRenderNode_stub(t *testing.T) {
root := &models.Node{
Name:     "aws",
FullPath: []string{"aws"},
Children: []*models.Node{
{Name: "s3", FullPath: []string{"aws", "s3"}, Stub: true},
},
}
opts := render.DefaultOptions()
opts.NoColor = true
got, err := render.ToString(root, opts)
if err != nil {
t.Fatalf("ToString error: %v", err)
}
if !strings.Contains(got, "(…)") {
t.Error("expected (…) indicator for stub node")
}
}

func TestRenderNode_commandsOnly_noPositionals(t *testing.T) {
root := &models.Node{
Name:     "tool",
FullPath: []string{"tool"},
Children: []*models.Node{
{
Name:        "run",
FullPath:    []string{"tool", "run"},
Flags:       []models.Flag{{Name: "--verbose", ValueType: "bool"}},
Positionals: []models.Positional{{Name: "file", Required: true}},
},
},
}
opts := render.DefaultOptions()
opts.NoColor = true
opts.CommandsOnly = true
got, err := render.ToString(root, opts)
if err != nil {
t.Fatalf("ToString error: %v", err)
}
// Positionals and flags should be absent.
if strings.Contains(got, "<file>") || strings.Contains(got, "--verbose") {
t.Error("commands-only mode should not show flags or positionals")
}
}

func TestRenderNode_hasPositionals(t *testing.T) {
n := &models.Node{
Name:        "cmd",
FullPath:    []string{"cmd"},
Positionals: []models.Positional{{Name: "arg", Required: true}},
}
if !n.HasPositionals() {
t.Error("HasPositionals() should return true")
}
n2 := &models.Node{Name: "cmd2", FullPath: []string{"cmd2"}}
if n2.HasPositionals() {
t.Error("HasPositionals() should return false when empty")
}
}

func TestRenderNode_asciiIconSet(t *testing.T) {
root := &models.Node{
Name:     "git",
FullPath: []string{"git"},
Children: []*models.Node{
{Name: "commit", FullPath: []string{"git", "commit"}},
},
}
opts := render.DefaultOptions()
opts.NoColor = true
opts.Icons = config.IconSetForPreset(config.IconPresetASCII)
got, err := render.ToString(root, opts)
if err != nil {
t.Fatalf("ToString error: %v", err)
}
if !strings.Contains(got, "v ") {
t.Errorf("ascii branch icon 'v ' not found in output:\n%s", got)
}
// Should NOT contain unicode icons.
if strings.Contains(got, "▼") || strings.Contains(got, "•") {
t.Errorf("unicode icons should not appear with ascii preset:\n%s", got)
}
}

func TestRenderNode_descLineLength(t *testing.T) {
longDesc := strings.Repeat("x", 100)
root := &models.Node{
Name:        "cmd",
FullPath:    []string{"cmd"},
Description: longDesc,
}
opts := render.DefaultOptions()
opts.NoColor = true
opts.DescLineLength = 20
got, err := render.ToString(root, opts)
if err != nil {
t.Fatalf("ToString error: %v", err)
}
// Description should be truncated at 20 chars + ellipsis.
if strings.Contains(got, strings.Repeat("x", 21)) {
t.Errorf("description was not truncated at 20 chars:\n%s", got)
}
if !strings.Contains(got, "…") {
t.Errorf("truncation ellipsis not found:\n%s", got)
}
}

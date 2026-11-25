package tui

import (
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// TestHelpPaneNodeLines pins the help pane lines for a node that reaches every
// branch of the node view: a description, flags whose short name has or lacks
// its dash, bool and typed values, flags with and without a description,
// required and optional positionals, children with and without a description,
// and raw help.
func TestHelpPaneNodeLines(t *testing.T) {
	node := &models.Node{
		Name:        "deploy",
		Description: "Deploy the app",
		Flags: []models.Flag{
			{Name: "--verbose", ShortName: "v", Description: "print more"},
			{Name: "--force", ShortName: "-f", ValueType: "bool"},
			{Name: "--name", ShortName: "n", ValueType: "string", Description: "the name"},
			{Name: "--count", ValueType: "int"},
		},
		Positionals: []models.Positional{
			{Name: "target", Required: true},
			{Name: "extra"},
		},
		Children: []*models.Node{
			{Name: "status", Description: "Show status"},
			{Name: "logs"},
		},
		HelpText: "Usage: deploy [flags] <target>\n\nraw body\n",
	}
	want := []string{
		"Deploy the app",
		"",
		"Flags:",
		"  --verbose, -v",
		"      print more",
		"  --force, -f",
		"  --name, -n <string>",
		"      the name",
		"  --count <int>",
		"",
		"Positionals:",
		"  <target>",
		"  [extra]",
		"",
		"Subcommands:",
		"  status  Show status",
		"  logs",
		"",
		"Raw help:",
		"Usage: deploy [flags] <target>",
		"",
		"raw body",
	}
	h := NewHelpPaneModel(nil)
	h.SetNode(node)
	if got := strings.Join(h.lines, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("help pane lines:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

// TestHelpPaneNodeLinesSparse pins the node view when every section is empty
// except one, and when there is no node at all.
func TestHelpPaneNodeLinesSparse(t *testing.T) {
	h := NewHelpPaneModel(nil)
	h.SetNode(&models.Node{Name: "bare", HelpText: "only raw"})
	if got := strings.Join(h.lines, "|"); got != "Raw help:|only raw" {
		t.Fatalf("raw-only node lines = %q", got)
	}
	h.SetNode(&models.Node{Name: "empty"})
	if got := strings.Join(h.lines, "|"); got != "" {
		t.Fatalf("empty node lines = %q, want one empty line", got)
	}
	h.SetNode(nil)
	if h.lines != nil {
		t.Fatalf("nil node lines = %q, want nil", h.lines)
	}
}

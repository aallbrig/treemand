package overrides

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/models"
)

// tree is mytool with remote (add, prune) and status; remote add has --tags.
func tree() *models.Node {
	add := &models.Node{Name: "add", FullPath: []string{"mytool", "remote", "add"}, Discovered: true,
		Flags: []models.Flag{{Name: "--tags", Description: "fetch tags"}}}
	prune := &models.Node{Name: "prune", FullPath: []string{"mytool", "remote", "prune"}, Discovered: true}
	remote := &models.Node{Name: "remote", FullPath: []string{"mytool", "remote"}, Discovered: true,
		Children: []*models.Node{add, prune}}
	status := &models.Node{Name: "status", FullPath: []string{"mytool", "status"}, Description: "Show stauts", Discovered: true}
	return &models.Node{Name: "mytool", FullPath: []string{"mytool"}, Discovered: true,
		Flags:    []models.Flag{{Name: "--verbose"}, {Name: "--noise"}},
		Children: []*models.Node{remote, status}}
}

func find(root *models.Node, path ...string) *models.Node {
	n := root
	for _, p := range path {
		var next *models.Node
		for _, c := range n.Children {
			if c.Name == p {
				next = c
			}
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

func flagNames(n *models.Node) []string {
	var out []string
	for _, f := range n.Flags {
		out = append(out, f.Name)
	}
	return out
}

func mustParse(t *testing.T, src string) *Override {
	t.Helper()
	o, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return o
}

func TestApplyReplacesDescriptions(t *testing.T) {
	root := tree()
	n := mustParse(t, `
commands:
  "":
    description: My tool does things
  status:
    description: Show status
`).Apply(root)
	if root.Description != "My tool does things" || find(root, "status").Description != "Show status" {
		t.Errorf("descriptions = %q, %q", root.Description, find(root, "status").Description)
	}
	if n != 2 {
		t.Errorf("changes = %d, want 2", n)
	}
}

func TestApplyAddsAndReplacesFlags(t *testing.T) {
	root := tree()
	mustParse(t, `
commands:
  remote add:
    flags:
      - name: --tags
        description: Fetch every tag
      - name: --track
        short: -t
        value_type: string
        description: Track one branch
`).Apply(root)
	add := find(root, "remote", "add")
	if got := flagNames(add); !slices.Equal(got, []string{"--tags", "--track"}) {
		t.Fatalf("flags = %v, want [--tags --track]", got)
	}
	if add.Flags[0].Description != "Fetch every tag" {
		t.Errorf("--tags description = %q, want it replaced", add.Flags[0].Description)
	}
	if f := add.Flags[1]; f.ShortName != "-t" || f.ValueType != "string" {
		t.Errorf("--track = %+v", f)
	}
}

func TestApplyAddsMissingCommandsWithTheirParents(t *testing.T) {
	root := tree()
	mustParse(t, `
commands:
  config set:
    description: Set a value
`).Apply(root)
	cfg := find(root, "config")
	set := find(root, "config", "set")
	if cfg == nil || set == nil {
		t.Fatalf("config set was not added: %v", root.Children)
	}
	if !slices.Equal(set.FullPath, []string{"mytool", "config", "set"}) || set.Description != "Set a value" {
		t.Errorf("set = %+v", set)
	}
	// An added command is never probed: treemand has nothing to discover.
	if !set.Discovered || set.Stub || !cfg.Discovered || cfg.Stub {
		t.Errorf("added commands must be discovered, not stubs: config %+v, set %+v", cfg, set)
	}
}

func TestApplyHidesCommandsAndFlags(t *testing.T) {
	root := tree()
	mustParse(t, `
commands:
  "":
    hide_flags: [--noise]
  remote prune:
    hide: true
`).Apply(root)
	if find(root, "remote", "prune") != nil {
		t.Error("remote prune is still in the tree")
	}
	if got := flagNames(root); !slices.Equal(got, []string{"--verbose"}) {
		t.Errorf("root flags = %v, want [--verbose]", got)
	}
}

// A hidden command that is not in the tree is not added.
func TestApplyDoesNotAddAHiddenCommand(t *testing.T) {
	root := tree()
	if n := mustParse(t, "commands:\n  ghost:\n    hide: true\n").Apply(root); n != 0 {
		t.Errorf("changes = %d, want 0", n)
	}
	if find(root, "ghost") != nil {
		t.Error("a hidden, missing command was added")
	}
}

// Applying twice changes nothing more: a cached tree that already carries the
// override comes out the same.
func TestApplyIsIdempotent(t *testing.T) {
	o := mustParse(t, `
commands:
  status:
    description: Show status
  remote add:
    flags: [{name: --track}]
  remote prune:
    hide: true
  config set:
    description: Set a value
`)
	root := tree()
	o.Apply(root)
	once := render(root)
	o.Apply(root)
	if twice := render(root); twice != once {
		t.Errorf("second Apply changed the tree:\n%s\nvs\n%s", once, twice)
	}
}

func render(n *models.Node) string {
	var b strings.Builder
	n.Walk(func(x *models.Node) {
		b.WriteString(x.FullCommand() + "|" + x.Description + "|" + strings.Join(flagNames(x), ",") + "\n")
	})
	return b.String()
}

func TestParseRejectsUnknownFields(t *testing.T) {
	_, err := Parse([]byte("commands:\n  status:\n    colour: typo\n"))
	if err == nil || !strings.Contains(err.Error(), "colour") {
		t.Errorf("err = %v, want one naming the unknown field", err)
	}
}

func TestParseRejectsAFlagWithoutAName(t *testing.T) {
	_, err := Parse([]byte("commands:\n  status:\n    flags: [{description: nameless}]\n"))
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Errorf("err = %v, want one naming the command", err)
	}
}

func TestLoadReadsTheCLIsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mytool.yaml"), []byte("commands:\n  \"\":\n    description: hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	o, err := LoadFrom(dir, "/usr/local/bin/mytool")
	if err != nil || o == nil {
		t.Fatalf("LoadFrom = %v, %v", o, err)
	}
	if o.Path != filepath.Join(dir, "mytool.yaml") || o.Hash == "" {
		t.Errorf("path %q, hash %q", o.Path, o.Hash)
	}
	root := tree()
	o.Apply(root)
	if root.Description != "hi" {
		t.Errorf("description = %q", root.Description)
	}
}

func TestLoadWithoutAFileIsNil(t *testing.T) {
	o, err := LoadFrom(t.TempDir(), "mytool")
	if o != nil || err != nil {
		t.Errorf("LoadFrom = %v, %v; want nil, nil", o, err)
	}
}

func TestLoadNamesTheFileInAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mytool.yaml"), []byte("commands: [oops\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(dir, "mytool")
	if err == nil || !strings.Contains(err.Error(), "mytool.yaml") {
		t.Errorf("err = %v, want one naming the file", err)
	}
}

func TestApplyOnANilOverrideOrTreeIsANoOp(t *testing.T) {
	var o *Override
	if o.Apply(tree()) != 0 {
		t.Error("nil override changed the tree")
	}
	if mustParse(t, "commands:\n  status: {description: x}\n").Apply(nil) != 0 {
		t.Error("nil tree reported changes")
	}
}

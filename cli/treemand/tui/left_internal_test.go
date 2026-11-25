package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// rowLabel names a row in one short token, for the Left characterization.
func rowLabel(r treeRow) string {
	switch r.kind {
	case rowKindCommand:
		return fmt.Sprintf("cmd:%s@%d", r.node.Name, r.depth)
	case rowKindSection:
		return "section:" + r.sectionKey
	case rowKindFlag:
		return fmt.Sprintf("flag:%s@%s", r.flag.Name, r.owner.Name)
	default:
		return fmt.Sprintf("pos:%s@%s", r.positional.Name, r.owner.Name)
	}
}

// leftDump presses Left once from every row of a freshly set-up model and
// records where the cursor lands and how many rows remain, so a refactor of
// Left that changes any landing spot or collapse shows up as a diff.
func leftDump(newModel func() *treeRows) string {
	var b strings.Builder
	base := newModel()
	for i := range base.rows {
		tm := newModel()
		tm.cursor = i
		from := rowLabel(tm.rows[i])
		tm.Left()
		fmt.Fprintf(&b, "%s -> %s rows=%d\n", from, rowLabel(tm.rows[tm.cursor]), len(tm.rows))
	}
	return b.String()
}

func TestLeftLandings(t *testing.T) {
	// openLeaves leaves every leaf collapsed, so Left on a leaf walks to its
	// parent: only the root's subcommands section and sub are opened.
	openLeaves := func(tm *treeRows) {
		tm.sectionExpanded["app@0/subcommands"] = true
		tm.nodeExpanded[nodeKey(tm.root.Children[0], 1)] = true
		tm.rebuild()
	}
	cases := []struct {
		name    string
		fixture func() *models.Node
		setup   func(*treeRows)
		want    string
	}{
		{name: "everything expanded", fixture: flattenFixture, setup: func(tm *treeRows) { tm.ExpandAll() }, want: `cmd:app@0 -> cmd:app@0 rows=1
section:app@0/flags -> section:app@0/flags rows=22
flag:--verbose@app -> cmd:app@0 rows=24
flag:--help@app -> cmd:app@0 rows=24
section:app@0/subcommands -> section:app@0/subcommands rows=7
cmd:sub@1 -> cmd:sub@1 rows=14
section:app/sub@1/inherited -> section:app/sub@1/inherited rows=23
flag:--verbose@sub -> cmd:sub@1 rows=24
section:app/sub@1/flags -> section:app/sub@1/flags rows=23
flag:--force@sub -> cmd:sub@1 rows=24
section:app/sub@1/subcommands -> section:app/sub@1/subcommands rows=19
cmd:deep@2 -> cmd:deep@2 rows=21
section:app/sub/deep@2/positionals -> section:app/sub/deep@2/positionals rows=22
pos:file@deep -> cmd:deep@2 rows=24
pos:rest@deep -> cmd:deep@2 rows=24
cmd:other@2 -> cmd:other@2 rows=24
cmd:c0@1 -> cmd:c0@1 rows=24
cmd:c1@1 -> cmd:c1@1 rows=24
cmd:c2@1 -> cmd:c2@1 rows=24
cmd:c3@1 -> cmd:c3@1 rows=24
cmd:c4@1 -> cmd:c4@1 rows=24
cmd:c5@1 -> cmd:c5@1 rows=24
section:app@0/positionals -> section:app@0/positionals rows=23
pos:target@app -> cmd:app@0 rows=24
`},
		{name: "sections hidden", fixture: flattenFixture, setup: func(tm *treeRows) {
			tm.ExpandAll()
			tm.ToggleSections()
		}, want: `cmd:app@0 -> cmd:app@0 rows=1
flag:--verbose@app -> cmd:app@0 rows=17
flag:--help@app -> cmd:app@0 rows=17
cmd:sub@1 -> cmd:sub@1 rows=11
flag:--verbose@sub -> cmd:sub@1 rows=17
flag:--force@sub -> cmd:sub@1 rows=17
cmd:deep@2 -> cmd:deep@2 rows=15
pos:file@deep -> cmd:deep@2 rows=17
pos:rest@deep -> cmd:deep@2 rows=17
cmd:other@2 -> cmd:other@2 rows=17
cmd:c0@1 -> cmd:c0@1 rows=17
cmd:c1@1 -> cmd:c1@1 rows=17
cmd:c2@1 -> cmd:c2@1 rows=17
cmd:c3@1 -> cmd:c3@1 rows=17
cmd:c4@1 -> cmd:c4@1 rows=17
cmd:c5@1 -> cmd:c5@1 rows=17
pos:target@app -> cmd:app@0 rows=17
`},
		{name: "leaves collapsed", fixture: flattenFixture, setup: openLeaves, want: `cmd:app@0 -> cmd:app@0 rows=1
section:app@0/flags -> section:app@0/flags rows=18
flag:--verbose@app -> cmd:app@0 rows=20
flag:--help@app -> cmd:app@0 rows=20
section:app@0/subcommands -> section:app@0/subcommands rows=7
cmd:sub@1 -> cmd:sub@1 rows=14
section:app/sub@1/inherited -> section:app/sub@1/inherited rows=20
section:app/sub@1/flags -> section:app/sub@1/flags rows=19
flag:--force@sub -> cmd:sub@1 rows=20
section:app/sub@1/subcommands -> section:app/sub@1/subcommands rows=18
cmd:deep@2 -> cmd:sub@1 rows=20
cmd:other@2 -> cmd:sub@1 rows=20
cmd:c0@1 -> cmd:app@0 rows=20
cmd:c1@1 -> cmd:app@0 rows=20
cmd:c2@1 -> cmd:app@0 rows=20
cmd:c3@1 -> cmd:app@0 rows=20
cmd:c4@1 -> cmd:app@0 rows=20
cmd:c5@1 -> cmd:app@0 rows=20
section:app@0/positionals -> section:app@0/positionals rows=19
pos:target@app -> cmd:app@0 rows=20
`},
		{name: "filtered", fixture: flattenFixture, setup: func(tm *treeRows) { tm.SetFilter("deep") }, want: `cmd:app@0 -> cmd:app@0 rows=3
cmd:sub@1 -> cmd:app@0 rows=3
cmd:deep@2 -> cmd:sub@1 rows=3
`},
		{
			name: "no parent path falls back to the nearest shallower command",
			fixture: func() *models.Node {
				root := flattenFixture()
				root.Children[1].FullPath = nil // c0
				root.Children[0].Children[1].FullPath = []string{"missing", "other"}
				return root
			},
			setup: openLeaves,
			want: `cmd:app@0 -> cmd:app@0 rows=1
section:app@0/flags -> section:app@0/flags rows=18
flag:--verbose@app -> cmd:app@0 rows=20
flag:--help@app -> cmd:app@0 rows=20
section:app@0/subcommands -> section:app@0/subcommands rows=7
cmd:sub@1 -> cmd:sub@1 rows=14
section:app/sub@1/inherited -> section:app/sub@1/inherited rows=20
section:app/sub@1/flags -> section:app/sub@1/flags rows=19
flag:--force@sub -> cmd:sub@1 rows=20
section:app/sub@1/subcommands -> section:app/sub@1/subcommands rows=18
cmd:deep@2 -> cmd:sub@1 rows=20
cmd:other@2 -> cmd:sub@1 rows=20
cmd:c0@1 -> cmd:app@0 rows=20
cmd:c1@1 -> cmd:app@0 rows=20
cmd:c2@1 -> cmd:app@0 rows=20
cmd:c3@1 -> cmd:app@0 rows=20
cmd:c4@1 -> cmd:app@0 rows=20
cmd:c5@1 -> cmd:app@0 rows=20
section:app@0/positionals -> section:app@0/positionals rows=19
pos:target@app -> cmd:app@0 rows=20
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newModel := func() *treeRows {
				tm := newTreeRows(tc.fixture())
				tc.setup(tm)
				return tm
			}
			if got := leftDump(newModel); got != tc.want {
				t.Errorf("landings:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

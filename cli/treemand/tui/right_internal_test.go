package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// rightDump presses Right once from every row of a freshly set-up model and
// records where the cursor lands and how many rows remain, so a refactor of
// Right that changes any landing spot or expansion shows up as a diff.
func rightDump(newModel func() *treeRows) string {
	var b strings.Builder
	base := newModel()
	for i := range base.rows {
		tm := newModel()
		tm.cursor = i
		from := rowLabel(tm.rows[i])
		tm.Right()
		fmt.Fprintf(&b, "%s -> %s rows=%d\n", from, rowLabel(tm.rows[tm.cursor]), len(tm.rows))
	}
	return b.String()
}

func TestRightLandings(t *testing.T) {
	cases := []struct {
		name    string
		fixture func() *models.Node
		setup   func(*treeRows)
		want    string
	}{
		{name: "fresh", fixture: flattenFixture, setup: func(*treeRows) {}, want: `cmd:app@0 -> flag:--verbose@app rows=7
section:app@0/flags -> flag:--verbose@app rows=7
flag:--verbose@app -> flag:--verbose@app rows=7
flag:--help@app -> flag:--help@app rows=7
section:app@0/subcommands -> section:app@0/subcommands rows=14
section:app@0/positionals -> pos:target@app rows=7
pos:target@app -> pos:target@app rows=7
`},
		{name: "everything expanded", fixture: flattenFixture, setup: func(tm *treeRows) { tm.ExpandAll() }, want: `cmd:app@0 -> cmd:sub@1 rows=24
section:app@0/flags -> flag:--verbose@app rows=24
flag:--verbose@app -> flag:--verbose@app rows=24
flag:--help@app -> flag:--help@app rows=24
section:app@0/subcommands -> cmd:sub@1 rows=24
cmd:sub@1 -> cmd:deep@2 rows=24
section:app/sub@1/inherited -> flag:--verbose@sub rows=24
flag:--verbose@sub -> flag:--verbose@sub rows=24
section:app/sub@1/flags -> flag:--force@sub rows=24
flag:--force@sub -> flag:--force@sub rows=24
section:app/sub@1/subcommands -> cmd:deep@2 rows=24
cmd:deep@2 -> pos:file@deep rows=24
section:app/sub/deep@2/positionals -> pos:file@deep rows=24
pos:file@deep -> pos:file@deep rows=24
pos:rest@deep -> pos:rest@deep rows=24
cmd:other@2 -> cmd:other@2 rows=24
cmd:c0@1 -> cmd:c0@1 rows=24
cmd:c1@1 -> cmd:c1@1 rows=24
cmd:c2@1 -> cmd:c2@1 rows=24
cmd:c3@1 -> cmd:c3@1 rows=24
cmd:c4@1 -> cmd:c4@1 rows=24
cmd:c5@1 -> cmd:c5@1 rows=24
section:app@0/positionals -> pos:target@app rows=24
pos:target@app -> pos:target@app rows=24
`},
		{name: "sections hidden", fixture: flattenFixture, setup: func(tm *treeRows) {
			tm.ExpandAll()
			tm.ToggleSections()
		}, want: `cmd:app@0 -> cmd:sub@1 rows=17
flag:--verbose@app -> flag:--verbose@app rows=17
flag:--help@app -> flag:--help@app rows=17
cmd:sub@1 -> cmd:deep@2 rows=17
flag:--verbose@sub -> flag:--verbose@sub rows=17
flag:--force@sub -> flag:--force@sub rows=17
cmd:deep@2 -> pos:file@deep rows=17
pos:file@deep -> pos:file@deep rows=17
pos:rest@deep -> pos:rest@deep rows=17
cmd:other@2 -> cmd:other@2 rows=17
cmd:c0@1 -> cmd:c0@1 rows=17
cmd:c1@1 -> cmd:c1@1 rows=17
cmd:c2@1 -> cmd:c2@1 rows=17
cmd:c3@1 -> cmd:c3@1 rows=17
cmd:c4@1 -> cmd:c4@1 rows=17
cmd:c5@1 -> pos:target@app rows=17
pos:target@app -> pos:target@app rows=17
`},
		{name: "nodes expanded, sections collapsed", fixture: flattenFixture, setup: func(tm *treeRows) {
			tm.nodeExpanded[nodeKey(tm.root, 0)] = true
			tm.nodeExpanded[nodeKey(tm.root.Children[0], 1)] = true
			tm.nodeExpanded[nodeKey(tm.root.Children[0].Children[0], 2)] = true
			tm.rebuild()
		}, want: `cmd:app@0 -> flag:--verbose@app rows=7
section:app@0/flags -> flag:--verbose@app rows=7
flag:--verbose@app -> flag:--verbose@app rows=7
flag:--help@app -> flag:--help@app rows=7
section:app@0/subcommands -> section:app@0/subcommands rows=23
section:app@0/positionals -> pos:target@app rows=7
pos:target@app -> pos:target@app rows=7
`},
		{name: "subcommands open, leaves expanded", fixture: flattenFixture, setup: func(tm *treeRows) {
			tm.sectionExpanded["app@0/subcommands"] = true
			tm.nodeExpanded[nodeKey(tm.root.Children[0], 1)] = true
			tm.nodeExpanded[nodeKey(tm.root.Children[0].Children[0], 2)] = true
			tm.nodeExpanded[nodeKey(tm.root.Children[1], 1)] = true
			tm.rebuild()
		}, want: `cmd:app@0 -> cmd:sub@1 rows=23
section:app@0/flags -> flag:--verbose@app rows=23
flag:--verbose@app -> flag:--verbose@app rows=23
flag:--help@app -> flag:--help@app rows=23
section:app@0/subcommands -> cmd:sub@1 rows=23
cmd:sub@1 -> cmd:deep@2 rows=23
section:app/sub@1/inherited -> section:app/sub@1/inherited rows=24
section:app/sub@1/flags -> flag:--force@sub rows=23
flag:--force@sub -> flag:--force@sub rows=23
section:app/sub@1/subcommands -> cmd:deep@2 rows=23
cmd:deep@2 -> pos:file@deep rows=23
section:app/sub/deep@2/positionals -> pos:file@deep rows=23
pos:file@deep -> pos:file@deep rows=23
pos:rest@deep -> pos:rest@deep rows=23
cmd:other@2 -> cmd:other@2 rows=23
cmd:c0@1 -> cmd:c0@1 rows=23
cmd:c1@1 -> cmd:c1@1 rows=23
cmd:c2@1 -> cmd:c2@1 rows=23
cmd:c3@1 -> cmd:c3@1 rows=23
cmd:c4@1 -> cmd:c4@1 rows=23
cmd:c5@1 -> cmd:c5@1 rows=23
section:app@0/positionals -> pos:target@app rows=23
pos:target@app -> pos:target@app rows=23
`},
		{name: "filtered", fixture: flattenFixture, setup: func(tm *treeRows) { tm.SetFilter("deep") }, want: `cmd:app@0 -> cmd:sub@1 rows=3
cmd:sub@1 -> cmd:sub@1 rows=3
cmd:deep@2 -> cmd:deep@2 rows=3
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newModel := func() *treeRows {
				tm := newTreeRows(tc.fixture())
				tc.setup(tm)
				return tm
			}
			if got := rightDump(newModel); got != tc.want {
				t.Errorf("landings:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

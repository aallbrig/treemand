package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

// flattenFixture is a tree that exercises every section flattenNode emits:
// inherited and own flags, subcommands (with a virtual child that must be
// skipped), positionals, a section over its default-expanded size, and nesting
// deep enough for graph prefixes.
func flattenFixture() *models.Node {
	leaf := func(name string, path ...string) *models.Node {
		return &models.Node{Name: name, FullPath: path}
	}
	deep := &models.Node{
		Name: "deep", FullPath: []string{"app", "sub", "deep"},
		Positionals: []models.Positional{{Name: "file", Required: true}, {Name: "rest"}},
	}
	sub := &models.Node{
		Name: "sub", FullPath: []string{"app", "sub"},
		Flags: []models.Flag{
			{Name: "--verbose", Inherited: true},
			{Name: "--force"},
		},
		Children: []*models.Node{deep, leaf("other", "app", "sub", "other")},
	}
	children := []*models.Node{sub}
	for i := 0; i < 6; i++ {
		children = append(children, leaf(fmt.Sprintf("c%d", i), "app", fmt.Sprintf("c%d", i)))
	}
	children = append(children, &models.Node{Name: "hidden", FullPath: []string{"app", "hidden"}, Virtual: true})
	return &models.Node{
		Name: "app", FullPath: []string{"app"},
		Flags:       []models.Flag{{Name: "--verbose"}, {Name: "--help"}},
		Positionals: []models.Positional{{Name: "target"}},
		Children:    children,
	}
}

// rowDump renders the rows as one line each, so a refactor that changes
// order, depth, section state or graph prefixes shows up as a diff.
func rowDump(rows []treeRow) string {
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%d %s", r.depth, strings.Repeat(" ", r.depth))
		switch r.kind {
		case rowKindCommand:
			fmt.Fprintf(&b, "cmd %s prefix=%q last=%v", r.node.Name, r.graphPrefix, r.isLast)
		case rowKindSection:
			fmt.Fprintf(&b, "section %s (%d) default=%v key=%s", r.sectionName, r.sectionCount, r.sectionDefault, r.sectionKey)
		case rowKindFlag:
			fmt.Fprintf(&b, "flag %s owner=%s@%d in=%s", r.flag.Name, r.owner.Name, r.ownerDepth, r.sectionRef)
		case rowKindPositional:
			fmt.Fprintf(&b, "pos %s owner=%s@%d in=%s", r.positional.Name, r.owner.Name, r.ownerDepth, r.sectionRef)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestFlattenRows(t *testing.T) {
	expandAll := func(tm *TreeModel) { tm.ExpandAll() }
	cases := []struct {
		name  string
		setup func(*TreeModel)
		want  string
	}{
		{
			name:  "root expanded by default",
			setup: func(*TreeModel) {},
			want: `0 cmd app prefix="" last=true
1  section Flags (2) default=true key=app@0/flags
2   flag --verbose owner=app@0 in=app@0/flags
2   flag --help owner=app@0 in=app@0/flags
1  section Subcommands (7) default=false key=app@0/subcommands
1  section Positional arguments (1) default=true key=app@0/positionals
2   pos target owner=app@0 in=app@0/positionals
`,
		},
		{
			name:  "everything expanded",
			setup: expandAll,
			want: `0 cmd app prefix="" last=true
1  section Flags (2) default=true key=app@0/flags
2   flag --verbose owner=app@0 in=app@0/flags
2   flag --help owner=app@0 in=app@0/flags
1  section Subcommands (7) default=false key=app@0/subcommands
1  cmd sub prefix="" last=false
2   section Inherited flags (1) default=false key=app/sub@1/inherited
3    flag --verbose owner=sub@1 in=app/sub@1/inherited
2   section Flags (1) default=true key=app/sub@1/flags
3    flag --force owner=sub@1 in=app/sub@1/flags
2   section Subcommands (2) default=true key=app/sub@1/subcommands
2   cmd deep prefix="│   " last=false
3    section Positional arguments (2) default=true key=app/sub/deep@2/positionals
4     pos file owner=deep@2 in=app/sub/deep@2/positionals
4     pos rest owner=deep@2 in=app/sub/deep@2/positionals
2   cmd other prefix="│   " last=true
1  cmd c0 prefix="" last=false
1  cmd c1 prefix="" last=false
1  cmd c2 prefix="" last=false
1  cmd c3 prefix="" last=false
1  cmd c4 prefix="" last=false
1  cmd c5 prefix="" last=true
1  section Positional arguments (1) default=true key=app@0/positionals
2   pos target owner=app@0 in=app@0/positionals
`,
		},
		{
			name: "sections hidden",
			setup: func(tm *TreeModel) {
				tm.ExpandAll()
				tm.ToggleSections()
			},
			want: `0 cmd app prefix="" last=true
2   flag --verbose owner=app@0 in=app@0/flags
2   flag --help owner=app@0 in=app@0/flags
1  cmd sub prefix="" last=false
3    flag --verbose owner=sub@1 in=app/sub@1/inherited
3    flag --force owner=sub@1 in=app/sub@1/flags
2   cmd deep prefix="│   " last=false
4     pos file owner=deep@2 in=app/sub/deep@2/positionals
4     pos rest owner=deep@2 in=app/sub/deep@2/positionals
2   cmd other prefix="│   " last=true
1  cmd c0 prefix="" last=false
1  cmd c1 prefix="" last=false
1  cmd c2 prefix="" last=false
1  cmd c3 prefix="" last=false
1  cmd c4 prefix="" last=false
1  cmd c5 prefix="" last=true
2   pos target owner=app@0 in=app@0/positionals
`,
		},
		{
			name:  "filtered keeps matches and their ancestors",
			setup: func(tm *TreeModel) { tm.SetFilter("deep") },
			want: `0 cmd app prefix="" last=false
1  cmd sub prefix="" last=false
2   cmd deep prefix="" last=false
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tm := NewTreeModel(flattenFixture(), config.DefaultConfig())
			tc.setup(tm)
			if got := rowDump(tm.rows); got != tc.want {
				t.Errorf("rows:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

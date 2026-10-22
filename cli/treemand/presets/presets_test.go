package presets_test

import (
	"testing"

	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/presets"
)

const stripeMap = `stripe
├── accounts
│   ├── create
│   └── persons
│       └── list
└── charges
    ├── capture
    └── create
`

func TestParseTreeMap(t *testing.T) {
	root := presets.ParseTreeMap(stripeMap, "stripe")
	if len(root.Children) != 2 {
		t.Fatalf("root children = %d, want 2", len(root.Children))
	}
	acc := root.Children[0]
	if acc.Name != "accounts" || len(acc.Children) != 2 {
		t.Fatalf("accounts = %+v", acc)
	}
	list := acc.Children[1].Children[0]
	if got := list.FullCommand(); got != "stripe accounts persons list" {
		t.Errorf("FullCommand = %q", got)
	}
	if !list.Stub || acc.Stub {
		t.Errorf("leaf must be a stub, branch must not (leaf=%v branch=%v)", list.Stub, acc.Stub)
	}
}

func TestGraft_keepsDiscoveredAndAddsMissing(t *testing.T) {
	discovered := &models.Node{Name: "stripe", FullPath: []string{"stripe"}, Children: []*models.Node{
		{Name: "charges", FullPath: []string{"stripe", "charges"}, Description: "real", Discovered: true,
			Children: []*models.Node{{Name: "create", Description: "kept"}}},
	}}
	graft := presets.ParseTreeMap(stripeMap, "stripe")
	presets.Graft(discovered, graft)

	if len(discovered.Children) != 2 {
		t.Fatalf("children = %d, want 2 (charges + accounts)", len(discovered.Children))
	}
	ch := discovered.Children[0]
	if ch.Description != "real" || len(ch.Children) != 2 {
		t.Fatalf("charges = %+v", ch)
	}
	if ch.Children[0].Description != "kept" {
		t.Error("discovered child was overwritten")
	}
}

func TestLookup(t *testing.T) {
	for _, n := range []string{"stripe", "/usr/bin/stripe", "AWS"} {
		if _, ok := presets.Lookup(n); !ok {
			t.Errorf("Lookup(%q) missed", n)
		}
	}
	if _, ok := presets.Lookup("git"); ok {
		t.Error("git should have no preset")
	}
}

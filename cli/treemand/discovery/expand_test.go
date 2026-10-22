package discovery_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
)

func TestExpandStubs_fillsStubFromHelp(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$*" in
  "sub --help") printf 'Do the sub thing.\n\nUsage:\n  fakecli sub [flags]\n\nFlags:\n      --loud   be loud\n' ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "fakecli"), []byte(script), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	stub := &models.Node{Name: "sub", FullPath: []string{"fakecli", "sub"}, Stub: true}
	root := &models.Node{Name: "fakecli", FullPath: []string{"fakecli"}, Children: []*models.Node{stub}}

	discovery.ExpandStubs(context.Background(), root, "fakecli", 1, 2, 5*time.Second, nil)

	if stub.Stub || !stub.Discovered {
		t.Fatalf("stub not expanded: %+v", stub)
	}
	if stub.Description != "Do the sub thing." {
		t.Errorf("description = %q", stub.Description)
	}
	if len(stub.Flags) != 1 || stub.Flags[0].Name != "--loud" {
		t.Errorf("flags = %+v", stub.Flags)
	}
}

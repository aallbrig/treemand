package discovery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aws documents "aws <path> help" as its help and answers neither --help nor
// -h (each fails after ~2.4 s). The aws preset says so, and discovery asks the
// documented form first: one call instead of three in a row (#215).
func TestRunHelp_presetDocumentedHelpSubcommandIsAskedFirst(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\necho \"$*\" >>'" + logPath + "'\n" +
		"last=\"\"; for a in \"$@\"; do last=\"$a\"; done\n" +
		"if [ \"$last\" != help ]; then sleep 2; echo 'error: argument operation: Invalid choice' >&2; exit 252; fi\n" +
		"printf 'AWS()\\n\\nNAME\\n       aws -\\n\\nDESCRIPTION\\n       The AWS Command Line Interface.\\n\\nAVAILABLE SERVICES\\n       o s3\\n\\n       o ec2\\n'\n"
	bin := filepath.Join(dir, "aws")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	d := NewHelpDiscoverer(1)
	d.ChildrenAsStubs = true
	root, err := d.Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 2 {
		t.Errorf("services not discovered: %+v", root.Children)
	}
	b, _ := os.ReadFile(logPath)
	if got := strings.Split(strings.TrimSpace(string(b)), "\n"); len(got) != 1 || got[0] != "help" {
		t.Errorf("calls = %q, want one: the documented \"help\"", got)
	}
}

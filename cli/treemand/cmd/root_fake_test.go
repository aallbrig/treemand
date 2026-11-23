package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cmd"
)

// runCmdSplit runs treemand and returns stdout and stderr separately.
func runCmdSplit(args ...string) (string, string, error) {
	c := cmd.NewRootCmd()
	var out, errb bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errb)
	c.SetArgs(args)
	err := c.Execute()
	return out.String(), errb.String(), err
}

type jsonNode struct {
	Name     string      `json:"name"`
	Stub     bool        `json:"stub"`
	Children []*jsonNode `json:"children"`
	Flags    []struct {
		Name string `json:"name"`
	} `json:"flags"`
}

func (n *jsonNode) child(name string) *jsonNode {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func decodeTree(t *testing.T, out string) *jsonNode {
	t.Helper()
	var n jsonNode
	if err := json.Unmarshal([]byte(out), &n); err != nil {
		t.Fatalf("stdout is not a JSON tree: %v\n%s", err, out)
	}
	return &n
}

func TestRoot_stripePresetSeedsTheMapAndLowersDepth(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "stripe")
	out, _, err := runCmdSplit("--output=json", "--no-cache", "stripe")
	if err != nil {
		t.Fatal(err)
	}
	root := decodeTree(t, out)
	cust := root.child("customers")
	if cust == nil || cust.child("create") == nil || cust.child("list") == nil {
		t.Fatalf("the --map seed was not grafted: %+v", root.Children)
	}
	// alpha was discovered from help and also appears in the map: the
	// discovered node keeps its data and gains the map's children.
	alpha := root.child("alpha")
	if alpha == nil || alpha.child("rotate") == nil {
		t.Fatalf("alpha did not gain the seeded rotate: %+v", alpha)
	}
	calls := e.calls(t)
	if !strings.Contains(calls, "--map") {
		t.Errorf("the seed did not run stripe --map: %q", calls)
	}
	// The preset's depth 1 stops at the root's own subcommands.
	if strings.Contains(calls, "alpha deep") {
		t.Errorf("preset depth 1 should not probe nested commands: %q", calls)
	}
}

func TestRoot_explicitDepthOverridesThePreset(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "stripe")
	if _, _, err := runCmdSplit("--output=json", "--no-cache", "--depth=2", "stripe"); err != nil {
		t.Fatal(err)
	}
	if calls := e.calls(t); !strings.Contains(calls, "alpha deep") {
		t.Errorf("--depth=2 should probe nested commands despite the preset: %q", calls)
	}
}

func TestRoot_aFailingSeedStillPrintsTheDiscoveredTree(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "stripe")
	t.Setenv(fakeMapEnv, "fail")
	out, _, err := runCmdSplit("--output=json", "--no-cache", "stripe")
	if err != nil {
		t.Fatal(err)
	}
	root := decodeTree(t, out)
	if root.child("alpha") == nil || root.child("beta") == nil {
		t.Fatalf("discovered commands missing: %+v", root.Children)
	}
	if root.child("customers") != nil {
		t.Errorf("a failed seed grafted nodes")
	}
}

func TestRoot_cacheHitSkipsDiscoveryAndRefreshBypassesIt(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "faketool")
	first, _, err := runCmdSplit("--output=json", "faketool")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.calls(t), "alpha --help") {
		t.Fatal("the cold run did not discover subcommands")
	}
	second, _, err := runCmdSplit("--output=json", "faketool")
	if err != nil {
		t.Fatal(err)
	}
	if calls := e.calls(t); strings.Contains(calls, "--help") {
		t.Errorf("a cache hit ran help again: %q", calls)
	}
	if first != second {
		t.Errorf("cached output differs from the cold run:\n%s\n---\n%s", first, second)
	}
	if _, _, err := runCmdSplit("--output=json", "--refresh", "faketool"); err != nil {
		t.Fatal(err)
	}
	if calls := e.calls(t); !strings.Contains(calls, "alpha --help") {
		t.Errorf("--refresh did not re-discover: %q", calls)
	}
}

func TestRoot_anUnopenableCacheRunsWithout(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "faketool")
	blocker := filepath.Join(e.dir, "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TREEMAND_CACHE_DIR", filepath.Join(blocker, "cache"))
	out, _, err := runCmdSplit("--output=json", "--timeout=0", "faketool")
	if err != nil {
		t.Fatal(err)
	}
	if decodeTree(t, out).child("alpha") == nil {
		t.Errorf("no tree without a cache:\n%s", out)
	}
}

func TestRoot_expandStubsDiscoversStubsBeforeOutput(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "faketool")
	out, stderr, err := runCmdSplit("--no-color", "--stub-threshold=1", "--expand-stubs=1", "faketool")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "--alpha-flag") || !strings.Contains(out, "--beta-flag") {
		t.Errorf("stubs were not expanded:\n%s", out)
	}
	if strings.Contains(stderr, "undiscovered") {
		t.Errorf("an expanded tree still reports stubs: %s", stderr)
	}
}

func TestRoot_debugLogsGoToStderr(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "faketool")
	out, _, err := runCmdSplit("--output=json", "--no-cache", "--debug", "faketool")
	if err != nil {
		t.Fatal(err)
	}
	// stdout stays a clean JSON document with debug logging on.
	decodeTree(t, out)
}

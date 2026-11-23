package cmd_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDoctor_reportsAPresetTreeAndItsProbes(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "stripe")
	out, _, err := runCmdSplit("doctor", "--output=json", "stripe")
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Preset   string `json:"preset"`
		HelpForm string `json:"help_form"`
		Nodes    int    `json:"nodes"`
		Probes   struct {
			Run int `json:"run"`
		} `json:"probes"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	// root, alpha, beta, the seeded customers (create, list) and alpha rotate.
	if rep.Preset != "stripe" || rep.HelpForm != "--help" || rep.Nodes < 6 || rep.Probes.Run < 3 {
		t.Errorf("report = %+v, want preset stripe, --help, at least 6 nodes and 3 probes", rep)
	}
	// doctor never reads or writes the cache: running it twice probes twice.
	e.calls(t)
	if _, _, err := runCmdSplit("doctor", "stripe"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.calls(t), "--help") {
		t.Error("a second doctor run did not probe the CLI again")
	}
}

func TestDoctor_textReport(t *testing.T) {
	e := newFakeEnv(t)
	e.tool(t, "fakedoc")
	out, _, err := runCmdSplit("doctor", "fakedoc")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"treemand doctor: fakedoc", "help form              --help", "nodes                  4", "slowest probes", "fakedoc alpha --help"} {
		if !strings.Contains(out, w) {
			t.Errorf("report is missing %q:\n%s", w, out)
		}
	}
}

func TestDoctor_usageErrors(t *testing.T) {
	newFakeEnv(t)
	for _, args := range [][]string{
		{"doctor", "--output=xml", "git"},
		{"doctor", "--timeout=-1", "git"},
	} {
		if _, _, err := runCmdSplit(args...); err == nil {
			t.Errorf("%v: want a usage error", args)
		}
	}
}

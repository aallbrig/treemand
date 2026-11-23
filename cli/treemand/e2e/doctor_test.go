package e2e

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// doctorReport is the part of `treemand doctor --output=json` the tests read.
type doctorReport struct {
	CLI             string   `json:"cli"`
	TreemandVersion string   `json:"treemand_version"`
	Strategies      []string `json:"strategies"`
	HelpForm        string   `json:"help_form"`
	Nodes           int      `json:"nodes"`
	NoDescription   []string `json:"no_description"`
	NoFlags         []string `json:"no_flags"`
	Stubs           []string `json:"stubs"`
	DiscoveryErrors []struct {
		Command string `json:"command"`
		Error   string `json:"error"`
	} `json:"discovery_errors"`
	Probes struct {
		Run         int `json:"run"`
		TimedOut    int `json:"timed_out"`
		NonZeroExit int `json:"non_zero_exit"`
	} `json:"probes"`
	SlowestProbes []struct {
		Command    string `json:"command"`
		DurationMS int64  `json:"duration_ms"`
		Outcome    string `json:"outcome"`
	} `json:"slowest_probes"`
}

// treemand doctor reports how a CLI was parsed: the strategy and help form,
// what the tree lacks, which commands failed and which probes were slowest
// (#94). It exits 0 even when some commands could not be read.
func TestDoctorJSONReportsHowTheCLIWasParsed(t *testing.T) {
	r := run(t, limit, nil, "doctor", "--output=json", "fakedoctor")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	var rep doctorReport
	if err := json.Unmarshal([]byte(r.stdout), &rep); err != nil {
		t.Fatalf("doctor --output=json is not JSON: %v\n%s", err, r.stdout)
	}
	if rep.CLI != "fakedoctor" || rep.TreemandVersion == "" {
		t.Errorf("cli %q, version %q; want fakedoctor and a version", rep.CLI, rep.TreemandVersion)
	}
	if !slices.Equal(rep.Strategies, []string{"help"}) || rep.HelpForm != "--help" {
		t.Errorf("strategies %v, help form %q; want [help] and --help", rep.Strategies, rep.HelpForm)
	}
	if rep.Nodes != 5 {
		t.Errorf("nodes = %d, want 5 (root and four commands)", rep.Nodes)
	}
	if !slices.Contains(rep.NoFlags, "fakedoctor bare") || slices.Contains(rep.NoFlags, "fakedoctor good") {
		t.Errorf("no_flags = %v, want bare and not good", rep.NoFlags)
	}
	if len(rep.Stubs) != 0 {
		t.Errorf("stubs = %v, want none", rep.Stubs)
	}
	checkDoctorFailures(t, rep)
	checkDoctorProbes(t, rep)
}

// checkDoctorFailures checks that fakedoctor's slow and broken commands are
// the report's discovery errors.
func checkDoctorFailures(t *testing.T, rep doctorReport) {
	t.Helper()
	errs := map[string]string{}
	for _, e := range rep.DiscoveryErrors {
		errs[e.Command] = e.Error
	}
	if !strings.Contains(errs["fakedoctor slow"], "timed out") || errs["fakedoctor broken"] == "" || len(errs) != 2 {
		t.Errorf("discovery_errors = %v, want slow (timed out) and broken", errs)
	}
}

// checkDoctorProbes checks the probe counts and that the slow command's
// timed-out probe leads the slowest.
func checkDoctorProbes(t *testing.T, rep doctorReport) {
	t.Helper()
	if rep.Probes.Run < 6 || rep.Probes.TimedOut < 1 || rep.Probes.NonZeroExit < 1 {
		t.Errorf("probes = %+v, want at least 6 run, 1 timed out, 1 non-zero exit", rep.Probes)
	}
	if len(rep.SlowestProbes) == 0 || len(rep.SlowestProbes) > 5 {
		t.Fatalf("slowest_probes has %d entries, want 1 to 5", len(rep.SlowestProbes))
	}
	if top := rep.SlowestProbes[0]; top.Command != "fakedoctor slow --help" || top.Outcome != "timeout" || top.DurationMS < 4000 {
		t.Errorf("slowest probe = %+v, want fakedoctor slow --help, timeout, about 5s", top)
	}
}

// The text report is meant to be pasted into a bug report: it names the
// version and the CLI, and lists each problem by command.
func TestDoctorTextIsAPasteableReport(t *testing.T) {
	r := run(t, limit, nil, "doctor", "fakedoctor")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	mustContain(t, "doctor report", r.stdout,
		"treemand doctor: fakedoctor",
		"strategy",
		"help form",
		"nodes",
		"without flags",
		"fakedoctor bare",
		"discovery errors",
		"fakedoctor slow",
		"timed out",
		"slowest probes",
		"fakedoctor slow --help",
	)
	if strings.Contains(r.stdout, "\x1b[") {
		t.Errorf("doctor text written to a pipe has escape codes:\n%q", r.stdout)
	}
}

// A CLI that does not exist is still an error, not a report.
func TestDoctorMissingCLIFails(t *testing.T) {
	r := run(t, limit, nil, "doctor", "no-such-cli-anywhere")
	if r.exit != 1 {
		t.Errorf("exit %d, want 1; stderr: %s", r.exit, r.stderr)
	}
}

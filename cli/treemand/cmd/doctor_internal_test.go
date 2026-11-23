package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
)

func TestDoctorAddTreeSkipsVirtualAndExcusesStubs(t *testing.T) {
	root := &models.Node{Name: "x", FullPath: []string{"x"}, Description: "root", Flags: []models.Flag{{Name: "--v"}}, Children: []*models.Node{
		{Name: "group", Virtual: true},
		{Name: "lazy", FullPath: []string{"x", "lazy"}, Stub: true},
		{Name: "bad", FullPath: []string{"x", "bad"}, DiscoveryErr: "could not get help: timed out"},
	}}
	var r doctorReport
	r.addTree(root)
	if r.Nodes != 3 {
		t.Errorf("nodes = %d, want 3 (virtual nodes are not counted)", r.Nodes)
	}
	if len(r.Stubs) != 1 || r.Stubs[0] != "x lazy" {
		t.Errorf("stubs = %v", r.Stubs)
	}
	if len(r.NoDescription) != 1 || r.NoDescription[0] != "x bad" || len(r.NoFlags) != 1 {
		t.Errorf("no_description %v, no_flags %v; want only x bad (a stub was never asked)", r.NoDescription, r.NoFlags)
	}
	if len(r.DiscoveryErrors) != 1 || r.DiscoveryErrors[0].Command != "x bad" {
		t.Errorf("discovery_errors = %v", r.DiscoveryErrors)
	}

	var empty doctorReport
	empty.addTree(nil)
	if empty.Nodes != 0 || empty.NoFlags == nil {
		t.Errorf("a failed discovery should report 0 nodes and empty (not null) lists: %+v", empty)
	}
}

func TestDoctorAddProbesCountsAndKeepsTheSlowest(t *testing.T) {
	var recs []discovery.ProbeRecord
	for i, o := range []string{discovery.ProbeOK, discovery.ProbeNonZero, discovery.ProbeTimeout, discovery.ProbeCancelled, discovery.ProbeFailed, discovery.ProbeOK, discovery.ProbeOK} {
		recs = append(recs, discovery.ProbeRecord{Name: `C:\bin\x.exe`, Args: []string{fmt.Sprint(i)}, Duration: time.Duration(i) * time.Second, Outcome: o, ExitCode: 2, Truncated: i == 6})
	}
	var r doctorReport
	r.addProbes(recs)
	want := probeCounts{Run: 7, OK: 3, NonZeroExit: 1, TimedOut: 1, Cancelled: 1, Failed: 1, Truncated: 1}
	if r.Probes != want {
		t.Errorf("probes = %+v, want %+v", r.Probes, want)
	}
	if len(r.SlowestProbes) != slowestProbeCount || r.SlowestProbes[0].Command != "x 6" || r.SlowestProbes[0].DurationMS != 6000 {
		t.Errorf("slowest = %+v", r.SlowestProbes)
	}
	if got := r.SlowestProbes[slowestProbeCount-1].Outcome; got != discovery.ProbeTimeout {
		t.Errorf("fifth slowest outcome = %q, want timeout", got)
	}
	if got := probeOutcomeLabel(discovery.ProbeRecord{Outcome: discovery.ProbeNonZero, ExitCode: 2}); got != "exit 2" {
		t.Errorf("non-zero outcome label = %q, want exit 2", got)
	}
}

func TestDoctorTextCapsLongLists(t *testing.T) {
	r := doctorReport{CLI: "x", Strategies: []string{"help"}}
	for i := range doctorListLimit + 3 {
		r.NoFlags = append(r.NoFlags, fmt.Sprintf("x c%d", i))
	}
	r.Error = "boom"
	txt := r.text()
	for _, w := range []string{"without flags          13", "x c9\n", "... and 3 more", "none answered", "error                  boom"} {
		if !strings.Contains(txt, w) {
			t.Errorf("text is missing %q:\n%s", w, txt)
		}
	}
	if strings.Contains(txt, "x c10") {
		t.Errorf("text lists more than %d items:\n%s", doctorListLimit, txt)
	}
}

func TestDoctorYAMLAndHomeRelative(t *testing.T) {
	var buf bytes.Buffer
	if err := writeDoctorReport(&buf, doctorReport{CLI: "x", Preset: "aws"}, "yaml"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "cli: x") || !strings.Contains(buf.String(), "preset: aws") {
		t.Errorf("yaml report:\n%s", buf.String())
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	if got := homeRelative(filepath.Join(home, "bin", "x")); got != "~"+string(filepath.Separator)+filepath.Join("bin", "x") {
		t.Errorf("homeRelative = %q", got)
	}
	if got := homeRelative("/usr/bin/x"); got != "/usr/bin/x" {
		t.Errorf("homeRelative outside home = %q", got)
	}
}

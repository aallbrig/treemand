package discovery

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestProbeLogRecordsEachOutcome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	var log ProbeLog
	ctx := WithProbeLog(context.Background(), &log)

	_, _, _ = runCapped(ctx, "/bin/sh", []string{"-c", "echo hi"}, nil, MaxHelpBytes)
	_, _, _ = runCapped(ctx, "/bin/sh", []string{"-c", "exit 3"}, nil, MaxHelpBytes)
	tctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	_, _, _ = runCapped(tctx, "/bin/sh", []string{"-c", "sleep 5"}, nil, MaxHelpBytes)
	cancel()
	cctx, cancel2 := context.WithCancel(ctx)
	cancel2()
	_, _, _ = runCapped(cctx, "/bin/sh", []string{"-c", "true"}, nil, MaxHelpBytes)
	_, _, _ = runCapped(ctx, "/no/such/binary", nil, nil, MaxHelpBytes)
	_, _, _ = runCapped(ctx, "/bin/sh", []string{"-c", "yes"}, nil, 1024)

	got := log.Records()
	want := []struct {
		outcome   string
		exit      int
		truncated bool
	}{{ProbeOK, 0, false}, {ProbeNonZero, 3, false}, {ProbeTimeout, 0, false}, {ProbeCancelled, 0, false}, {ProbeFailed, 0, false}, {ProbeOK, 0, true}}
	if len(got) != len(want) {
		t.Fatalf("recorded %d probes, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Outcome != w.outcome || got[i].ExitCode != w.exit || got[i].Truncated != w.truncated {
			t.Errorf("probe %d (%v) = %s exit %d truncated %v, want %s exit %d truncated %v",
				i, got[i].Args, got[i].Outcome, got[i].ExitCode, got[i].Truncated, w.outcome, w.exit, w.truncated)
		}
	}
	//nolint:gocritic // timing: a lower bound; a probe killed at its timeout cannot take less time, however loaded the host
	if got[2].Duration < 50*time.Millisecond {
		t.Errorf("timed-out probe took %v, want at least its 50ms timeout", got[2].Duration)
	}
}

func TestProbeLogIsOptional(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("no true binary")
	}
	// No log in the context: nothing is recorded, nothing panics.
	if _, _, err := runCapped(context.Background(), "true", nil, nil, MaxHelpBytes); err != nil {
		t.Fatal(err)
	}
}

package discovery

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

// Probe outcomes, as ProbeRecord.Outcome reports them.
const (
	ProbeOK        = "ok"        // the process exited 0
	ProbeNonZero   = "exit"      // the process exited non-zero (help often does)
	ProbeTimeout   = "timeout"   // the probe ran out of its time
	ProbeCancelled = "cancelled" // stopped early: another help form won, or the run ended
	ProbeFailed    = "failed"    // the process could not be started
)

// ProbeRecord is one process discovery ran against the CLI.
type ProbeRecord struct {
	Name      string // the executable, as resolved
	Args      []string
	Duration  time.Duration
	Outcome   string // one of the Probe* constants
	ExitCode  int    // set when Outcome is ProbeNonZero
	Truncated bool   // the output hit MaxHelpBytes
}

// ProbeLog collects the probes run under a context made by WithProbeLog. It
// is safe for concurrent use: discovery runs many probes at once.
type ProbeLog struct {
	mu      sync.Mutex
	records []ProbeRecord
}

// Records returns a copy of the probes recorded so far, in finishing order.
func (l *ProbeLog) Records() []ProbeRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]ProbeRecord(nil), l.records...)
}

func (l *ProbeLog) add(r ProbeRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, r)
}

type probeLogKey struct{}

// WithProbeLog returns a context under which every probe discovery runs is
// recorded in log. Runs without one record nothing.
func WithProbeLog(ctx context.Context, log *ProbeLog) context.Context {
	return context.WithValue(ctx, probeLogKey{}, log)
}

// recordProbe adds one finished probe to the context's log, if it has one.
// ctx is the probe's own context (with its timeout); err is what running the
// process returned.
func recordProbe(ctx context.Context, name string, args []string, start time.Time, truncated bool, err error) {
	log, _ := ctx.Value(probeLogKey{}).(*ProbeLog)
	if log == nil {
		return
	}
	r := ProbeRecord{Name: name, Args: append([]string(nil), args...), Duration: time.Since(start), Truncated: truncated}
	r.Outcome, r.ExitCode = probeOutcome(ctx, truncated, err)
	log.add(r)
}

// probeOutcome classifies a finished probe. A probe stopped at the output cap
// is killed on purpose, so it counts by what it printed, not how it ended.
func probeOutcome(ctx context.Context, truncated bool, err error) (string, int) {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return ProbeTimeout, 0
	case ctx.Err() != nil:
		return ProbeCancelled, 0
	case err == nil || truncated:
		return ProbeOK, 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return ProbeNonZero, exitErr.ExitCode()
	}
	return ProbeFailed, 0
}

---
name: treemand-tui-responsiveness
description: Measure how fast treemand's TUI reacts on huge trees and decide what to optimize. Use for TUI lag, rendering speed, or tui frame findings.
---

# TUI responsiveness

Follow **treemand-measure** for running, recording and filing; this skill is the TUI speed playbook.

**Measure**: `task measure -- tui` runs `cli/treemand/tui/bench_test.go` (best of 5) on an aws-sized tree. `frame_ms.<case>` is one key press, Update plus View; one frame at 60 Hz is 16.7 ms. `allocs.<case>` is allocations per press, which is load-independent: trust it when timings are noisy.

**Act**: profile the slow case before changing anything:

    cd cli/treemand && go test -run '^$' -bench <Case> -cpuprofile /tmp/cpu.out ./tui && go tool pprof -top /tmp/cpu.out

- Rebuild cost (expand-all, filter) scales with rows: reuse buffers, avoid rebuilding unchanged rows, keep work proportional to what changed.
- View cost scales with what is drawn: render only visible rows; cache styled strings that do not change per frame.
- Allocations per key press: look for per-row string building and lipgloss style construction in loops.

Add a benchmark case to `bench_test.go` for any new interaction worth guarding; the probe picks it up by name.

**Prove it**: the probe shows **improved**, and the TUI tests (`go test ./tui`, the e2e pty tests) still pass.

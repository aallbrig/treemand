---
name: treemand-perf-diagnose
description: Measure treemand's speed on real CLIs and decide what to optimize. Use for any performance question, a slow CLI report, or a perf budget or regression finding.
---

# Diagnosing performance

Follow **treemand-measure** for running, recording and filing; this skill is the perf playbook. Real CLIs run, so **treemand-safe-real-cli-run** applies (the probe already uses a scratch directory and fails on side effects).

**Measure**: `task measure -- perf` (all corpus CLIs, a few minutes; `PERF_CLIS="gh docker"` narrows it). Metrics per CLI: `cold_s` (uncached full discovery), `cpu_s` (user+sys CPU including the CLI's own help processes), `warm_s` (cache hit), `tti_s` (until `-i` shows the tree), `rss_mb`, `nodes`.

**Read the cost before optimizing.** `details.per_cli.cpu_per_wall` says where the time goes:

- **cpu_per_wall well above 1** (gh at ~8): many help processes run in parallel and the CLIs themselves are the cost. Look for fewer processes, not a faster parser: duplicate help texts (aliases), help forms that fail before one succeeds (count invocations with a logging PATH shim, as `help_form_test.go` does), children that need no help call (stubs past the depth the user needs).
- **cpu_per_wall near 1 and high cold_s**: a serial chain. Find it with the help-form tests' timing shape: hedge delay, fallbacks, one slow root.
- **High warm_s**: the cache path. It must never run the CLI (`task ab` shows invocations), and decoding should be the whole cost.
- **High tti_s**: the TUI waits for something before drawing; the root-only streaming path (`streamsDiscovery`) should make it about one help call.
- **rss_mb**: help text kept per node (`HelpText`) dominates large trees.

**Prove it**: an optimization is done when the same probe, interleaved with the old binary on the same host (`task ab` for tree equality, `task measure -- perf` for numbers), shows **improved** with the tree unchanged. cpu_s is the load-resistant number; wall time on a busy host is not.

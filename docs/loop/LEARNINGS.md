# Dev loop learnings

Rolling log appended by the dev loop's Retro phase. This file is an exception to
the timestamped-filename convention: one file that grows is easier for the loop
to read than hundreds of per-cycle files. Entries older than 30 cycles are folded
into the `## Summary` section by the loop.

## Summary

_Nothing yet._

## Recent cycles

<!-- Retro appends below: ### Cycle N — YYYY-MM-DD — outcome — #issue vX.Y.Z -->

### Cycle 1 - 2026-11-09 - released - #6 v0.3.3
Footer hints now drop whole tokens when narrow. Release runs that fail with no jobs and no logs are GitHub startup failures: rerun before rolling back or escalating (release.sh now does).

### Cycle 2 - 2026-11-09 - released - #7 v0.3.4
Hardcoded /treemand/ paths broke under baseURL=/; use the demo shortcode (relURL). A red main CI run whose jobs are all green is the same startup glitch: rerun it.

### Cycle 3 - 2026-11-09 - released - #14 v0.3.5
Idle audit of discovery-quality: parents list a one-line description per subcommand that was being dropped (gh 137 to 0 descriptionless nodes). Audit with --no-cache; edit only in a worktree. Release binaries have no cache (CGO_ENABLED=0), filed as an issue.

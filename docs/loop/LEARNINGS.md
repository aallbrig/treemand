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

### Cycle 4 - 2026-11-09 - released - #16 v0.3.6
Replaced cgo go-sqlite3 with pure-Go modernc.org/sqlite so CGO_ENABLED=0 release binaries keep the cache; smoke.sh now fails on "could not open cache". Existing tests run under CGO_ENABLED=0 made the red test.

### Cycle 5 - 2026-11-09 - released - #20 v0.3.7
Idle audit of docs-try-page: all 15 sitemap URLs return 200 but the logo linked to the github.io root (404) because relURL of "/" ignores baseURL. Added site:check (in task ci) to catch root-relative links outside /treemand/. GitHub CI does not run task ci or Hugo, so this gate is local only.

### Cycle 6 - 2026-11-09 - released - #24 v0.3.8
Perf audit: cached runs 0.1-0.3s, cold gh 23s (224 nodes), no races or flakes. Found a hang: an orphan child holding stdout blocked CombinedOutput past --timeout; fixed with cmd.WaitDelay. Check labels exist before gh issue create (area:perf, not area:performance) and verify the issue number is non-empty before using it.

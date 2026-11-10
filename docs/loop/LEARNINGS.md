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

### Cycle 7 - 2026-11-10 - released - #26 v0.3.9
Security/test audit: coverage 64-100% (cache lowest); govulncheck flagged two x/text advisories, not called by our code. Bumped x/text and added a ListEntries test (green on first run, so it only locks behaviour in). Daily release cap of 6 resets at UTC midnight; the loop correctly idled until then.

### Cycle 8 - 2026-11-10 - idle - #30
mcp-api-exploration is propose-only: filed #30 (stable JSON schema, then an MCP subcommand) as loop:proposed and waiting for a human thumbs-up. JSON output currently has no schema_version.

### Cycle 10 - 2026-11-10 - released - #31 v0.3.10
Root description was a usage-block continuation line for git/tmux/ssh. Fix: skip usage blocks, list headings and list rows; root-only man NAME fallback; NAME lines accept em/en dash. Tightening listRowRe was needed after the aws fixture regressed (justified man text has double spaces), so run the fixtures after every parser heuristic. Untrusted open issues: 0.

### Cycle 11 - 2026-11-10 - parked - #32
Parked: aws root help lists services as bare bullets, so descriptions cannot be carried from the parent; per-service help is about 3s each (20 min cold). Stub marking (discovered:false, (...) in text) already exists. Verify an issue premise against real output before planning; filed findings on the issue with three re-scope options.

### Cycle 12 - 2026-11-10 - released - #32 v0.3.11
Re-scoped #32 instead of leaving it parked (new rule: decide, then ship): stubs get an honest stderr footer plus a public --expand-stubs; descriptions stay lazy because aws help has none and per-service help is about 3s. Under zsh, `2>&1 >/dev/null | wc` tees stdout (multios): redirect to files when checking stream separation.

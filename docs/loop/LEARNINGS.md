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

### Cycle 13 - 2026-11-10 - released - #33 v0.4.0
Added a root-only list-commands fallback (tmux: 1 node to 91, all with man-derived descriptions and synopsis flags). make and other target runners are out of scope and skipped, because probing a target can execute project build rules; documented in the fallback-chain ADR. Minor bump for a new capability.

### Cycle 14 - 2026-11-10 - released - #34 v0.4.1
#34 had three causes: a per-child 5s deadline shared by a whole subtree (docker/gh), kubectl flag syntax (=false: with the next-line description), and each node running help up to 4 times. Fixing the last cut cold times 2-4x (gh 23s to 7s, npm 30s to 9s), so #35 is mostly addressed. perf:compare flagged apt memory 18 to 118MB, but that is a child process (apt changelog) and the main binary measures the same on this host; history swings 118/17/18, so the apt rss gate is noisy. Never `cd` out of a worktree between commands: edits once landed in the main checkout and had to be moved.

### Cycle 15 - 2026-11-10 - released - #35 v0.4.2
v0.4.1 had already met #35 cold-time bar (all corpus CLIs under 10s), so I re-scoped: a global slot pool caps concurrent help processes (--jobs, default 2 x CPUs within 4..16) and TUI streaming split to #68. Take the slot before the per-command timeout starts, or waiting counts against the command. Host load average was 48 from other sessions, so timings vary by 2x between runs: compare A/B in the same minute and repeat three times.

### Cycle 16 - 2026-11-10 - released - #36 v0.4.3
Committed a 16-CLI help-fixture corpus with a baseline gate in task ci. The first CI run caught real nondeterminism: detectSection ranged over a map, so "Environment Commands:" (matches commands and environment) parsed as commands or free text at random (systemctl 49 or 53 subcommands). Fixed with a ranked, deterministic choice. A regression corpus pays for itself immediately: run any new parse path 25 times. release.sh smoke hit a transient api.github.com error after a good release; re-running smoke.sh alone is the right recovery, not a rollback. Untested here: cargo and terraform fixtures (not installed).

### Cycle 17 - 2026-11-10 - released - #37 v0.4.4
Re-scoped #37 into floor + fuzz + cache/config now and cmd/tui/presets as #74-#76. A coverage floor must hold on GitHub CI, not just locally: cmd measured 67.9% here but 64.2% on the runner (tests use CLIs installed on a dev box), so the first push failed CI; floors are now the lower of the two environments. Fuzzing the parser (about 83k execs) and timing 800KB adversarial inputs found nothing, so the corpus-time determinism fix last cycle was the real find. After pushing, checks take a moment to register: pr:merge reports "no checks reported" if run too early.

### Cycle 18 - 2026-11-10 - released - #53 v0.4.5
Help, completion and list-commands probes now read through a 1MB cap and kill the process when it is hit: chaos-huge went from 1898MB and 25s to 21MB and 2.4s, and its xfail line is gone. The cap size mattered: sanitize+parse of 2MB is 0.9s normally but 18s under -race (instrumented regexes), so the default is 1MB and the flood test uses 256KB; always run a new slow-path test under -race before pushing. Largest real help seen is 14KB, so 1MB is generous.

### Cycle 19 - 2026-11-10 - released - #38 v0.4.6
Added an e2e package (built binary, fake cobra/argparse/clap CLIs, real pty). It found a parser bug on its first run: detectSection read flag lines of 30 chars or less whose description ends in a section keyword (output, cache, config) as a header, so the flag and the rest of its section vanished (curl +20 flags, systemctl +8 recovered). Fake CLIs also exposed two gaps, filed as #81 (argparse subcommands) and #82 (clap child descriptions) with skipped, linked tests. A pty harness must answer cursor and color queries or every start waits about 5s. apt cold time flagged again (5s to 17s) but main measures the same on this host: apt speed tracks machine state, not code.

### Cycle 20 - 2026-11-10 - released - #39 v0.5.0
First-run audit: usage errors now exit 2 (runtime failures 1), with a hint on every usage error; README gained Scripting, Exit codes, Shell completion and Man page sections, each claim backed by a unit or e2e test (including NO_COLOR on a real pty). Minor bump because the exit code of usage errors changed. Walk the real paths with the built binary before writing docs: the audit found only 3 real defects and 5 undocumented behaviours that already worked. perf:compare apt memory flaps 17-118MB between runs (child process), so judge it by a main-vs-branch A/B, not one reading. Filed #85 (--timeout=0).

### Cycle 21 - 2026-11-10 - merged, no release - #54
The nondeterminism in #54 was already fixed by v0.4.1 (per-command timeout) and v0.4.3 (map-order section detection); this cycle proved it (10 cold runs each of npm, gh, docker identical) and guarded it with a jitter test plus tighter perf thresholds for the deterministic metrics (nodes, no_desc, no_flags), validated by replaying the recorded history. No release: a test and a loop script do not change the binary, and a no-op tag only burns the daily cap. When an issue is already fixed, verify with the issue acceptance command first and spend the cycle on the guard.

### Cycle 22 - 2026-11-10 - released - #40 v0.6.0
TUI at scale: measured before changing. Real aws (422 stubs) was already fast (every key about 5ms); a fully expanded 60k-row tree had one slow key (expand-all 295ms to 42ms by reusing the row buffer and dropping fmt.Sprintf). The bigger finds were not latency: the tree pane had no PgUp/PgDn at all, and the key bindings overlay overflowed the terminal (long lines wrapped inside a box sized by source-line count). Render-fit tests at 3 sizes across 33 UI states are cheap and caught it; so does scanning model_keys.go against the overlay text, which fails for any undocumented new key. A zsh pitfall cost a capture run: an unmatched glob in rm aborted an && chain before a variable was set, so use find -delete and set variables on their own line.

### Cycle 23 - 2026-11-10 - merged, no release - #42
Competitive teardown from the peers published pages only: the permission policy denied installing and running third-party code, so nothing was run and every peer claim is labelled as theirs (hands-on follow-up filed as #101, loop:blocked). I did not look for a way around the denial. Reading the pages still paid off: it corrected stale versions (mandible is 0.9.0, not the linked 0.2.0) and produced 9 deduped gap issues (#92-#100) plus an honest README section. When a tool is denied, do the part that needs no new permission, say plainly what was not done, and file the rest for a human. Run and cold-start comparisons remain open.

### Cycle 24 - 2026-11-10 - released - #55 v0.6.1
Warm runs were slow because the cache key ran `<cli> --version` (aws 2.5s, pip 0.9s start an interpreter), not because of the cache read (4.9ms). Keyed on the executable on disk (resolved path, size, mtime) instead: warm aws 2.9s to 0.03s, pip 0.98s to 0.03s. It also fixed a latent staleness bug (a CLI without --version never invalidated). Profile by timing each phase before assuming where the time goes: the obvious suspect (sqlite and JSON decode) cost 4ms. The version string is now display-only and read concurrently with discovery.

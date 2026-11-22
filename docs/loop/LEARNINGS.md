# Dev loop learnings

Rolling log appended by the dev loop's Retro phase. This file is an exception to
the timestamped-filename convention: one file that grows is easier for the loop
to read than hundreds of per-cycle files. Entries older than 30 cycles are folded
into the `## Summary` section by the loop.

## Summary

- Cycle 1 (#6, v0.3.3): a release run that fails with no jobs and no logs is a GitHub startup failure. Rerun it before rolling back or escalating; `release.sh` does this now.
- Cycles 2-13 (v0.3.4 to v0.4.0): a red CI run whose jobs are all green is a startup glitch, so rerun it. Site links go through relURL or the demo shortcode, never a hardcoded `/treemand/`. Release binaries are `CGO_ENABLED=0`, so the cache uses pure-Go sqlite. Child processes need `cmd.WaitDelay` or an orphan holding stdout hangs past `--timeout`. Check that labels exist before `gh issue create`. Run every parser fixture after each parser heuristic change (aws's justified man text broke a tightened row regex). Verify an issue premise against real output before planning (cycle 11, aws service descriptions). Re-scope an infeasible issue and ship rather than park it; under zsh, `2>&1 >/dev/null | wc` tees stdout, so redirect to files when checking stream separation. Never probe make-style target runners: running a target can execute build rules (cycle 13).

## Recent cycles

<!-- Retro appends below: ### Cycle N — YYYY-MM-DD — outcome — #issue vX.Y.Z -->

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

### Cycle 25 - 2026-11-10 - merged and deployed, no release - #43
Lighthouse could not be run (third-party download denied), so no scores were claimed (#106, blocked). Built `task site:audit` instead: a dependency-free Node script that drives the installed Chrome over CDP against the production-path build and fails on the rule families Lighthouse scores; in task ci, and the GitHub runner has Chrome so it gates there too. It found missing meta/canonical/OG/favicon, 138 contrast failures that were one Monokai comment color, 17px nav targets, 8 h1 on a list page, unsized images. Third-party fetch failures (shields.io 408 mid-run) must be warnings, not failures. Lazy-loaded images look blank in full-page screenshots unless you wait. The site deploys from main via Pages (gh-pages then GitHub pages-build), so no tag needed; verify production with a cache-busting URL after the second build finishes. Before polling a command in a loop, run it once: I spent minutes polling a workflow name gh could not resolve. The audit then flaked once on the GitHub runner (PASS printed, then ENOTEMPTY exit 1): cleanup deleted Chrome's profile while Chrome was still writing; fixed by waiting for the exit and making cleanup unable to change the verdict (PR #110). Check a script's real exit status, not the status of a pipe (`| tail` hid it for me).

### Cycle 26 - 2026-11-10 - merged and deployed, no release - #44, #107
Added install.sh (checksum-verified, no privilege escalation unless --system), a hermetic 34-check test against a local fake release, clean-container (debian, ubuntu, alpine) and macOS CI jobs against the real latest release, and an OS-aware, no-JS-safe Installation page whose detection, override, hash and copy buttons are driven in a real browser by site:audit. Verified the published one-liner on production after deploy. Process slips: a guard hook blocks any shell command containing the word for privilege escalation, which also blocked the command that created the worktree, so my Write calls created plain directories at that path and I worked in a non-worktree until missing files showed it; check `git worktree list` before the first edit, and write such files with the Write tool (it is only allowed inside the repo, so PR bodies go under .loop/). A recording shim tests the privileged path without ever elevating.

### Cycle 28 - 2026-11-16 - released - #220 v0.8.1
First cycle driven by the measurement engine (`task measure`). Expand-all allocations 38,935 to 12,346 (91,669 at the finding) by keying the row-key memo with an int section instead of a string and building "Flags (N)" labels only for drawn rows. This host's load swings from 0.6 to 2.5 per CPU within minutes (other projects' sessions), so wall-time verdicts are mostly unreliable here: allocations are the proof that survives, and an interleaved old-vs-new run is the only fair timing comparison (both binaries slowed together under load). Leave a timing issue open rather than close it on busy-host numbers.

### Cycle 29 - 2026-11-16 - merged, no release - #45
Load was 54 on 16 CPUs, so the user's timing issues (#220, #216) were skipped in favor of the oldest ready issue. Distribution audit: the site already listed only working channels (an earlier cycle had pulled the unpublished ones), so the work was a guard: `scripts/channels_check.sh` fails on an unpublished channel being documented or a Homebrew formula more than one release behind, run daily and after every package publish, with hermetic tests in `task ci`. Account-gated steps went into one checklist issue (#224) instead of stalling. A `workflow_run` trigger names the workflow's `name:`, not its file: read it before wiring (my first draft named one that does not exist).

### Cycle 30 - 2026-11-16 - released - #216 (open), #220 closed v0.8.2
First cycle under measured-first selection. The new help_calls count found the waste at once: the hedged race ran both `--help` and `-h` on every slow child, even after the root had shown which works (gh 457 processes for 236 nodes). Asking the learned form alone: gh 457 to 237 locally, 284 to 243 on the runner; pip cold 3.3 to 1.5-1.9 s; trees identical by `task ab`. The first runner run also closed #220 (expand-all 7.76 ms). Two lessons. One runner sample is noise too: the same commit gave gh 3.8 vs 5.4 s, so timing needs a median (#230) before a wall-clock regression means anything. And a flaky one-node kubectl tree on the runner exposed a silent failure (#229): rerun before blaming the change, then file what the flake reveals. `task ci` cannot pass here at load 80 (a wall-clock assertion under -race fails on main too); the PR's GitHub CI is the gate.

### Cycle 31 - 2026-11-22 - merged, no release - #47
Community health 57 to 100 with SECURITY.md, a code of conduct, issue forms, a PR template, grouped Dependabot and release-note categories, kept honest by `scripts/community_check.sh` in `task docs:check`. Check each badge an issue asks for before adding it: pkg.go.dev 404s because the module lives in `cli/treemand` (proxy latest v0.3.1), and the Go Report Card badge now renders "retired", so both were left out with reasons in the PR. Repo settings an issue asks to "enable if permitted" (private vulnerability reporting) are not permitted to the loop: say so in the PR and write the docs with a fallback. The guard blocks writes to the session scratchpad; draft PR bodies under `.loop/`.

### Cycle 32 - 2026-11-22 - released - #48 v0.9.0
A versioned JSON Schema (`treemand schema`, served at treemand.dev/schema/tree.v1.json) with a frozen v1 contract test. The first real check found that YAML had never matched JSON: it wrote Go-derived keys (`fullpath`) and no omitempty. Validating every corpus fixture through a fake CLI is cheap, but cap a fan-out e2e test: 152 unbounded runs made discovery's wall-clock hedge tests flaky while `go test` ran both packages together. The CI complexity ratchet counts `_test.go` functions too, so run gocognit `-over 15` on new tests before pushing. Locally, `site:audit` needs Node 24 from nvm on PATH.

### Cycle 33 - 2026-11-22 - released - #214 (open) v0.9.1
`flattenNode` went from cognitive 57 to under 15 with no row changes: one helper per section, sharing `appendSection`. `functions_over_30` dropped from 8 to 7 and TUI allocations stayed flat. A characterization test that dumps every row field as one line per row, committed before the refactor, made the change safe and quick to review. Pin its golden from the real output, not from a guess: section keys look like `path@depth/flags`, and with sections hidden the flat rows keep their depth+2. Selection is not the work file: `.loop/work` named #51, but select.md puts measure:finding issues first. Budget issues like #214 ship one function per release, say "Part of", and go back to ready.

### Cycle 34 - 2026-11-22 - released - #214 (open) v0.9.2
`runRoot` went from cognitive 58 to 11 with one helper per stage; `functions_over_30` 7 to 6. Host load blocked the pre-commit hook on discovery wall-clock tests: confirm a timing failure on main once, then retry in a quiet minute.

### Cycle 35 - 2026-11-22 - released - #214 (open) v0.9.3
`discoverWithHelp` went from cognitive 56 to under 15; `functions_over_30` 6 to 5. Pin the uncovered branches with a characterization test before refactoring (here a child whose help fails and a child that echoes its parent's help). `fuzz:smoke` can trip on the known #338 crasher by chance, so re-run before you suspect the change.

### Cycle 36 - 2026-11-22 - released - #214 (open) v0.9.4
`Left` went from cognitive 43 to under 10; `functions_over_30` 5 to 4. `ExpandAll` marks leaves expanded too, so Left on a leaf collapses in place there; a characterization test needs a layout with collapsed leaves to reach the parent-jump path. `task pr:merge` can outlast a 10-minute foreground call (`task ci` takes about 10 minutes), so run it in the background.

### Cycle 37 - 2026-11-22 - released - #214 (open) v0.9.6
`Right` went from cognitive 33 to under 15; `functions_over_30` 4 to 3. PR bodies now need a `## Release note` section (`none` for a refactor) or the PR Body check fails; run `scripts/release/check_note.sh` on the body before `pr:merge`. `wait.sh` can print `none` even when the release succeeds, because the tag lands on the changelog commit after the merge; confirm with `gh release list` and `git merge-base --is-ancestor`.

### Cycle 38 - 2026-11-22 - released - #214 (open) v0.9.7
`renderFlagModal` went from cognitive 54 to under 15; `functions_over_30` 3 to 2. A golden file that records each rendered line with %q under a forced TrueColor profile pins styling as well as layout, so a renderer refactor can be checked byte for byte. The loop guard blocks writes to the scratchpad, so put new code straight into a package file. The `fuzz:smoke` gate counted the #338 crasher twice on one run; a re-run counted it once and passed.

### Cycle 39 - 2026-11-22 - released - #214 (open) v0.9.8
`renderNode` went from cognitive 49 to under 15; `functions_over_30` 2 to 1. Label the PR `release:patch` when you open it: #363 merged without the label, so Auto Release planned `none`; adding the label after the merge and dispatching `auto-release.yml` cut the release, because `plan.sh` counts every PR merged since the last tag. Run `check_note.sh` with the body on stdin, not as an argument.

### Cycle 40 - 2026-11-22 - released - #214 v0.9.9
`rebuildNodeLines` went from cognitive 31 to under 15; `functions_over_30` 1 to 0 closes #214 after eight releases, one function each. Locally, run `task ci` with Node 24 first on PATH (`PATH=~/.nvm/versions/node/v24.*/bin:$PATH`) or `site:audit` fails at the very end. The #335 wall-clock scale test failed the PR's macOS job by 3 ms under `-race`: confirm the failing test from the log, `gh run rerun --failed`, then merge.

### Cycle 41 - 2026-11-22 - released - #46 v0.10.0
Releases now build with GoReleaser as a draft that `release.yml` publishes only after cosign verifies; v0.10.0 shipped the same asset names plus .deb/.rpm/.apk and SBOMs, with release-health and install probes unchanged. The PR's `Release snapshot` job caught a relative-path bug in the contract test that local runs hid, so keep release changes behind that job. `fuzz:smoke` double-counted the #338 crasher again: rerun the failed job.

### Cycle 42 - 2026-11-22 - released - #229 v0.10.1
A root whose help times out is retried once at 3x the per-command budget, then fails with exit 1 instead of printing a one-node tree; stdout stays empty and nothing is cached. The new branch first pushed `discoverWithHelp` over cognitive 15 (`functions_over_15` 32 to 33, caught only by the PR's ratchet gate): run `measure.py complexity --gate` locally before pushing. `fuzz:smoke` writes its crasher into `discovery/testdata/fuzz`, which then fails `coverage:check`: delete that untracked folder before re-running.

### Cycle 43 - 2026-11-22 - released - #51 v0.10.2
All third-party actions are now pinned by SHA, every workflow defaults to `contents: read` with writes granted per job, and releases carry build provenance that `gh attestation verify` accepted on the v0.10.2 asset (infra-security: 46/1/1 to 0/0/0). Resolve tags to SHAs with `gh api repos/<o>/<r>/commits/<tag>`, and rewrite refs with Python, not BSD sed (the slashes in `owner/repo` break `s///`). The guard blocks Write to the scratchpad, so keep PR bodies under `.loop/`.

### Cycle 44 - 2026-11-22 - released - #49 v0.11.0
`treemand mcp` ships a dependency-free stdio MCP server with `discover`, `search` and `assemble`, tested end to end over stdio against the fake CLIs. A timed-out discovery returns a partial tree without failing, so the server labels it partial instead of passing it off as complete. `assemble` keeps flags the help does not list, with a warning, because help text misses flags (`go test`). In site markdown, a heading named `search` gets `id="search"` and fails the CI site audit's duplicate-id check, so give such headings explicit `{#ids}`. Run `task site:audit` after any docs change, not only before it.

# Contributing to treemand

## Development Setup

```bash
git clone https://github.com/aallbrig/treemand.git
cd treemand
task dev:install    # runs tests + installs binary
```

### Dev host setup (macOS or Linux)

`scripts/dev/setup.sh` checks a host for every tool `task ci` and the dev
session need (the list is `scripts/dev/tools.txt`) and for the git hooks.
It is a dry run by default: it prints a table of present and missing tools
with the command that would install each, and changes nothing.

```bash
sh scripts/dev/setup.sh            # report only (or: task setup)
sh scripts/dev/setup.sh --install  # install what is missing, then the hooks (or: task setup -- --install)
```

`--install` uses the host's package manager (Homebrew, apt or dnf), falls
back to `go install`, `pipx` or nvm (Node from `.nvmrc`), and lists anything
that still needs a manual step. It exits non-zero while a required tool is
missing or the hooks are not active.

### Git Hooks

`setup.sh --install` (or `task hooks`) installs:

- `pre-commit`: the [pre-commit](https://pre-commit.com) framework
  (`.pre-commit-config.yaml`: gitleaks, tmuxinator schema, shellcheck), which
  first runs `pre-commit.legacy`.
- `pre-commit.legacy`: `task precommit` (gitleaks on staged changes, lint, tests).
- `pre-push`: `task precommit` before a push to `main` or `develop`.

Hooks live in the shared git dir, so every worktree uses them.

### Host-local files

`.loop/config.env` (loop settings) and `.loop/autostart/` (generated launchd
plists and systemd units) are host-local and gitignored. They never hold secrets.

## Development Loop

```bash
task dev:install  # run tests + install binary (recommended)
task build        # compile local binary only
task test         # run all tests
task lint         # run golangci-lint
task precommit    # full hygiene gate: fmt, vet, lint, vuln, test
task fix          # auto-fix formatting and lint issues
```

Makefile targets mirror the Taskfile and remain available (`make dev`, `make test`, etc.).

## Recording Demo Videos

The project uses [VHS](https://github.com/charmbracelet/vhs) to produce demo GIFs.
Tape files live in `demos/`; the CI workflow re-records when relevant source files change.

### Prerequisites

- [VHS](https://github.com/charmbracelet/vhs) (`go install github.com/charmbracelet/vhs@latest`)
- `ffmpeg` (bundled with most VHS installs, or install separately)
- `treemand` on `$PATH` (run `task install` first)
- `git` on `$PATH` (used as the target CLI in most demos)

### Recording All Features

```bash
task install         # ensure treemand is up to date
task demo            # records all segments and produces dist/demo.mp4
task demo-subcmds    # records per-subcommand GIFs → www/treemand/static/demos/
task demo-all        # records both the full video and all subcmd GIFs
```

The full video is written to `dist/demo.mp4`. Per-subcommand GIFs land in
`www/treemand/static/demos/` and are served by the Hugo documentation site.

### Re-recording a Single Segment

```bash
vhs demos/03_filter_exclude.tape
```

This writes the segment to `demos/segments/03_filter_exclude.mp4`. Re-run `task demo` to rebuild the full video.

### Adding a New Feature Demo

1. Create `demos/NN_<name>.tape` following the numbering and pattern of
   existing tapes (source `_settings.tape`, print an ANSI banner label, run demo commands)
2. Add the feature to `docs/FEATURES.md`
3. Run `task demo` to verify the segment records correctly

### Cleaning Up

```bash
task demo-clean    # removes demos/segments/, dist/demo.mp4, and subcmd GIFs
```

### CI

The demo GitHub Actions workflow (`.github/workflows/demo.yml`) triggers on
pushes that touch `cli/**`, `demos/**`, `scripts/record-demo.sh`, `Makefile`,
or `Taskfile.yml`. It can also be triggered manually via `workflow_dispatch`.

## Project Structure

```
cli/treemand/       Go source (commands, TUI, rendering, config, cache)
demos/              VHS tape files for demo video
  _settings.tape    Shared VHS settings
  01_*.tape         Feature segment tapes (numbered for ordering)
docs/               Feature docs and design documents
scripts/            Utility scripts (hooks, demo recording)
www/treemand/       Hugo documentation site
```

## Code Style

- Go code is linted with `golangci-lint`
- Run `task lint` before submitting PRs
- Tests live alongside source files (`*_test.go`)

## Dev session and the autonomous loop

```bash
task dev            # tmuxinator session treemand_dev (attaches if running)
task dev:recycle    # kill and restart the session
task dev:bootstrap  # host readiness report: tools task ci needs, with install hints
task loop:start     # start the autonomous dev loop: adds an agent pane to the loops window
task loop:stop      # stop it at the next phase boundary (-- --now kills it immediately); the pane closes
task loop:status    # state, daily caps, failure streak
task visual:check   # TUI + site screenshots into .loop/shots/
```

The session's windows are `bootstrap` (readiness report, then opens the local and
production sites; set `TREEMAND_NO_BROWSER=1` to skip the browser), `dev` (shell and
TUI), `watch` (fmt, test and lint on change), `site` (`task site:dev`), `prod` and
`loops` (a dev shell showing `task loop:status`). The loop's agent pane exists only
while the loop runs.

The loop is the `treemand-loop` skill (`.claude/skills/treemand-loop/`). One cycle
ships one issue as one release. File issues labelled `loop:ready` to feed it, or
`human:wip` to keep it away from something you are touching.

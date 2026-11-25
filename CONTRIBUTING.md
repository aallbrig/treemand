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

Agent tooling (gopls, dlv and the Claude Code Go LSP plugin) is provisioned
the same way; [docs/agents/dev-tooling.md](docs/agents/dev-tooling.md) covers
when to use each.

### Git Hooks

`setup.sh --install` (or `task hooks`) installs:

- `pre-commit`: the [pre-commit](https://pre-commit.com) framework
  (`.pre-commit-config.yaml`: tmuxinator schema), which first runs
  `pre-commit.legacy`.
- `pre-commit.legacy`: `task precommit`, scoped to what is staged so it takes
  seconds: gitleaks always; gofmt, `go vet` and golangci-lint on the packages
  with staged Go files; shellcheck on staged shell scripts; infra lint only
  when `infra/` is staged.
- `commit-msg`: the subject is at most 72 characters with a blank line after
  it; a commit made from a Claude Code session needs a `Co-Authored-By`
  trailer.
- `pre-push`, on every branch: `task measure:gate` (the ratchets CI's
  measurement gate runs), the tests with coverage floors (`task
  coverage:check`) and the race tests (`task test-race`). When `task ci`
  passes on a clean tree it stamps that tree (`scripts/dev/proof.sh`), and
  pre-push skips the steps already proven for the pushed tree.

A hook fails when `task` is missing, so a missing tool never turns a gate off.

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
task precommit    # pre-commit gate: gitleaks, lint scoped to staged files
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
task loop:start     # enable and start the autonomous dev loop: adds an agent pane to the loops window
task loop:stop      # disable it; the cycle in flight ends, then the pane closes (-- --now kills it immediately)
task loop:resume    # start the loop only if it is enabled (the loops window runs this when the session starts)
task loop:status    # enabled?, supervisor, last work check, daily caps, failure streak
task visual:check   # TUI + site screenshots into .loop/shots/
task autostart:install    # optional: start the session at login (launchd or systemd --user)
task autostart:status     # installed? loaded?
task autostart:uninstall  # remove it and every file it wrote
```

The session's windows are `bootstrap` (readiness report, then opens the local and
production sites; set `TREEMAND_NO_BROWSER=1` to skip the browser), `dev` (shell and
TUI), `watch` (fmt, test and lint on change), `site` (`task site:dev`), `prod` and
`loops` (a dev shell that runs `task loop:resume` and `task loop:status`). The
loop's agent pane exists only while the loop runs.

The `prod` window shows read-only production status in four panes, each
refreshing every `PROD_REFRESH_SECONDS` (default 300): site health (key pages,
TLS expiry, served release vs latest release), pipelines (latest workflow runs,
open PRs, `loop:in-progress` issues), AWS (CloudFront status and the monthly
budget, through free APIs only) and release downloads. The AWS pane uses
`AWS_PROFILE`; without access it shows a hint instead of failing. `task
prod:status` prints all four once.

The loop runs from its own worktree beside your checkout (`../treemand.loop`,
detached at `origin/main` and fast-forwarded every cycle), so it never touches
your checkout; its state is in that worktree's `.loop/`. `loop:start` writes
`.loop/ENABLED` there and `loop:stop` removes it, so an enabled loop comes back
after a reboot. The agent pane runs `scripts/loop/supervisor.sh`, a token-free
POSIX `sh` loop: when `scripts/loop/work.sh` finds work it starts one interactive
Remote Control session ("treemand.dev dev loop") in the pane, and after each
cycle (`state.py end-cycle` writes `.loop/cycle-done`) it types `/clear` and
`/treemand-loop` while work remains. After `LOOP_IDLE_EXIT_MINUTES` without work
it types `/exit` (stands down) and keeps polling every `LOOP_POLL_SECONDS`. On
start it recovers from a crash: a claimed issue with a clean worktree is resumed,
anything else goes back to `loop:ready`.

Autostart is optional. `task autostart:install` writes a LaunchAgent
(`~/Library/LaunchAgents/dev.treemand.session.plist`, log in
`~/Library/Logs/treemand-session.log`) on macOS, or a `systemd --user` unit
(`~/.config/systemd/user/treemand-session.service`) on Linux; a copy of the
generated file is kept in `.loop/autostart/`. At login it runs
`scripts/dev_session.sh start` from the checkout you installed from. That only
creates the detached tmux session (a no-op if it is already running), so it does
not need the network or the login keychain. The loops window then runs
`task loop:resume`, so the loop comes back only if `loop:stop` was not the last
command. The file holds the repo path and a PATH, never credentials. Install
copies `TREEMAND_SESSION` and `TREEMAND_NO_BROWSER` into it when they are set.

The loop is the `treemand-loop` skill (`.claude/skills/treemand-loop/`). One cycle
ships one issue as one release. File issues labelled `loop:ready` to feed it, or
`human:wip` to keep it away from something you are touching.

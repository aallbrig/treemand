# Contributing to treemand

## Development Setup

```bash
git clone https://github.com/aallbrig/treemand.git
cd treemand
task dev    # runs tests + installs binary
```

Requires Go 1.25+ and [go-task](https://taskfile.dev) (`brew install go-task` or `go install github.com/go-task/task/v3/cmd/task@latest`).

### Git Hooks

Install pre-commit and pre-push hooks that run `task precommit` automatically:

```bash
bash scripts/setup-hooks.sh
```

## Development Loop

```bash
task dev          # run tests + install binary (recommended)
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

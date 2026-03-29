# Contributing to treemand

## Development Setup

```bash
git clone https://github.com/aallbrig/treemand.git
cd treemand
make dev    # runs tests + installs binary
```

### Git Hooks

Install the pre-push hook to catch lint and test failures before pushing:

```bash
bash scripts/setup-hooks.sh
```

## Development Loop

```bash
make dev      # run tests + install binary (recommended)
make build    # compile local binary only
make test     # run all tests
make lint     # run golangci-lint
```

## Recording Demo Videos

The project uses [VHS](https://github.com/charmbracelet/vhs) to produce a
single MP4 showcasing all CLI features. Each feature has its own `.tape` file
in `demos/`, and they are stitched into one video via `ffmpeg`.

### Prerequisites

- [VHS](https://github.com/charmbracelet/vhs) (`go install github.com/charmbracelet/vhs@latest`)
- `ffmpeg` (bundled with most VHS installs, or install separately)
- `treemand` on `$PATH` (run `make install` first)
- `git` on `$PATH` (used as the target CLI in most demos)

### Recording All Features

```bash
make install   # ensure treemand is up to date
make demo      # records all segments and produces dist/demo.mp4
```

The final video is written to `dist/demo.mp4`.

### Re-recording a Single Segment

```bash
vhs demos/03_filter_exclude.tape
```

This writes the segment to `demos/segments/03_filter_exclude.mp4`. You can
then re-run `make demo` to rebuild the full video, or view the segment on its
own.

### Adding a New Feature Demo

1. Create `demos/NN_<name>.tape` following the numbering and pattern of
   existing tapes (source `_settings.tape`, print an ANSI banner label, run
   demo commands)
2. Add the feature to `docs/FEATURES.md`
3. Run `make demo` to verify the segment records correctly and the full video
   looks good

### Cleaning Up

```bash
make demo-clean   # removes demos/segments/ and dist/demo.mp4
```

### CI

A nightly GitHub Actions workflow (`.github/workflows/demo.yml`) checks for
source changes since the last recording and regenerates the video automatically.
It can also be triggered manually via `workflow_dispatch`.

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
- Run `make lint` before submitting PRs
- Tests live alongside source files (`*_test.go`)

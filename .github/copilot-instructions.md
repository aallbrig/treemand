# Copilot Instructions for treemand

## Project Overview
treemand is a Go CLI tool that visualizes arbitrary CLI command hierarchies as a tree.
- Non-interactive mode: colored ASCII/Unicode tree output
- Interactive mode (`-i`): Bubble Tea TUI with live preview, modals, keyboard/mouse controls
- Website: Hugo static site at `www/treemand/`
- Module root: `cli/treemand/` (`github.com/aallbrig/treemand`)

## Dev Loop
After making code changes, use **`task dev`** (from the repo root) to run tests and install the binary in one step:
```
task dev   # runs: go test ./cli/treemand/... && go install ./cli/treemand
```
This ensures the system-wide `treemand` binary stays in sync with the latest code. Do **not** assume the installed binary is current unless `task dev` (or `task install`) was run recently.

Key Taskfile targets (Makefile mirrors these):
- `task build`      — compile to `./treemand` (local binary, not installed)
- `task install`    — install to `$GOPATH/bin` without running tests
- `task test`       — run all tests
- `task lint`       — run golangci-lint
- `task precommit`  — full hygiene gate: fmt, vet, lint, vuln, test
- `task fix`        — auto-fix formatting and fixable lint issues

## Repository Layout
```
cli/treemand/        Go module (main CLI + TUI)
  cmd/               Cobra root command, subcommands (cache, completion, gendocs)
  tui/               Bubble Tea models: model.go (main), tree.go, preview.go, help.go
  discovery/         CLI introspection strategies (help, completions, man, error-mining)
  render/            ASCII/JSON/YAML tree output
  cache/             SQLite-backed discovery cache (~/.treemand/cache.db)
  config/            Color scheme, icon sets, DisplayStyle
  models/            Node, Flag, Positional structs
www/treemand/        Hugo static site
.github/workflows/   CI (ci.yml), release (release.yml, release-packages.yml)
```

## Key Conventions
- Go 1.25.8 (toolchain pinned in go.mod); all packages under `github.com/aallbrig/treemand`
- TUI uses Bubble Tea (bubbletea + bubbles + lipgloss)
- `tui/model.go` is large (1200+ lines) — prefer Python inline scripts for edits to that file to avoid tool timeouts
- `config.DisplayStyle` controls TUI tree presentation: `StyleDefault`, `StyleColumns`, `StyleCompact`, `StyleGraph`; cycle with `T` key or `--tree-style` flag
- Cache keys are SHA-256 of `cli|version|strategies|schemaVersion`
- Test coverage targets: cmd/ ≥ 78%, tui/ ≥ 73%, render/ ≥ 80%

## Release Process
- Tag `vX.Y.Z` → triggers `release.yml` (GoReleaser multi-platform binaries) + `update-homebrew` job (updates `aallbrig/homebrew-tap`) + `deploy-site` job (Hugo → gh-pages)
- Homebrew tap: `brew tap aallbrig/tap && brew install treemand`
- After releasing, run `task dev` locally to update the dev binary

## GitHub Pages
https://aallbrig.github.io/treemand

## Documentation

When creating documentation files:

- Place them in the `docs/` directory or a subdirectory. Typical subdirectories include `docs/design/` and `docs/research/`.
- Prepend a UTC timestamp to the `.md` filename in the format `YYYYMMDD_HHMMSS_UTC_` so files sort chronologically (e.g., `docs/design/20260329_150800_UTC_architecture_overview.md`).


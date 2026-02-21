# 🌲 Treemand

> Visualize and interact with any CLI command hierarchy as a beautiful tree.

[![CI](https://github.com/aallbrig/treemand/actions/workflows/ci.yml/badge.svg)](https://github.com/aallbrig/treemand/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/aallbrig/treemand)](https://goreportcard.com/report/github.com/aallbrig/treemand)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

<!-- TODO: Add animated GIF demo here -->

## Features

- 🔍 **Auto-discovery** — introspects `--help` output recursively (no config needed)
- 🎨 **Colored output** — syntax-highlighted tree with configurable color schemes
- 🖥️ **Interactive TUI** (`-i`) — k9s-inspired explorer with live preview, keyboard + mouse
- 💾 **SQLite cache** — instant repeat lookups, keyed on CLI + version + strategy
- 🌐 **Multiple output formats** — `text`, `json`
- 🔄 **Self-dogfooding** — `treemand treemand` works!

## Example Output

```
▼ git  the version control system
├── ▼ remote  [2 flags]
│   ├── • add <name> <url>
│   ├── • remove <name>
│   └── • get-url <name>
├── • commit  [--message=<string> --all]
├── • status  [--short]
└── • push  [--force --set-upstream]
```

## Installation

### Go Install (recommended)

```bash
go install github.com/aallbrig/treemand@latest
```

Requires **Go 1.22+**.

### Pre-built Binaries

Download from [Releases](https://github.com/aallbrig/treemand/releases) for Linux, macOS, and Windows (amd64/arm64).

### Build from Source

```bash
git clone https://github.com/aallbrig/treemand.git
cd treemand/cli/treemand
go build -o treemand .
```

## Usage

### Non-interactive (default)

```bash
# Any installed CLI
treemand git
treemand kubectl
treemand docker
treemand aws

# Limit depth
treemand --depth=2 git

# Filter nodes
treemand --filter=remote git

# Exclude nodes
treemand --exclude=help kubectl

# Commands only (hide flags & positionals)
treemand --commands-only aws

# JSON output
treemand --output=json docker

# Disable color (or set NO_COLOR=1)
treemand --no-color git

# Skip cache
treemand --no-cache kubectl
```

### Interactive TUI (`-i`)

```bash
treemand -i git
```

```
┌─ Preview ───────────────────────────────────────────────────────┐
│ git remote add <name> <url>                                     │
└─────────────────────────────────────────────────────────────────┘
┌─ Tree: git ──────────────────┐ ┌─ Help: remote ───────────────┐
│ ▼ git                        │ │ Manage set of tracked repos  │
│   ▼ remote                   │ │                              │
│ ▶ ● add <name> <url>         │ │ Subcommands:                 │
│   • remove <name>            │ │   add, remove, get-url       │
│   • get-url <name>           │ └──────────────────────────────┘
└──────────────────────────────┘
  git remote   nav:arrows  ?:help  /:filter  H:help-pane  q:quit
```

#### Keyboard Controls

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate tree |
| `→` / `Space` / `Enter` | Expand node |
| `←` | Collapse node |
| `Ctrl+S` | Cycle nav scheme (arrows → vim → WASD) |
| `/` | Fuzzy filter |
| `H` | Toggle help pane |
| `?` | Show key bindings |
| `Ctrl+P` | Toggle panes |
| `R` | Refresh node |
| `q` / `Esc` | Quit |

**Vim scheme** (`Ctrl+S` once): `k`/`j`/`h`/`l`  
**WASD scheme** (`Ctrl+S` twice): `w`/`s`/`a`/`d`

## Development

### Run from source

```bash
# From repo root:
go run cli/treemand/main.go git

# Or from the module directory:
cd cli/treemand
go run . git
```

### Build

```bash
cd cli/treemand
go build -o treemand .
```

### Test

```bash
cd cli/treemand
go test ./...                        # all tests
go test ./... -cover                 # with coverage
go test ./... -v -race               # verbose + race detector
```

### Lint

```bash
# Install golangci-lint (if needed)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

cd cli/treemand
golangci-lint run
```

## Website (Hugo)

```bash
cd www/treemand
hugo server --watch          # dev server at http://localhost:1313/treemand/
hugo --minify                # production build → public/
```

## Project Structure

```
treemand/
├── .github/workflows/       # CI, release, pages Actions
├── cli/treemand/            # Go CLI module
│   ├── main.go
│   ├── cmd/                 # Cobra root command + flags
│   ├── tui/                 # Bubble Tea interactive TUI
│   ├── discovery/           # CLI introspection strategies
│   ├── models/              # Node/Flag/Positional structs
│   ├── render/              # ASCII/Unicode tree renderer
│   ├── cache/               # SQLite result cache
│   └── config/              # Color scheme + config
├── www/treemand/            # Hugo static site
└── .golangci.yml            # Linter config
```

## Configuration

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--no-color` | `NO_COLOR` | `false` | Disable color output |
| `--no-cache` | — | `false` | Skip SQLite cache |
| — | `TREEMAND_CACHE_DIR` | `~/.treemand` | Cache directory |
| — | `TREEMAND_STRATEGIES` | `help` | Default discovery strategies |

## License

[MIT](LICENSE) © aallbrig

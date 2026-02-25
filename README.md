# 🌲 treemand — visualize and build CLI commands interactively

[![Go version](https://img.shields.io/badge/go-1.22%2B-blue)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

treemand introspects any CLI tool's `--help` output and renders its full command hierarchy as a navigable tree. Use it statically to explore unfamiliar CLIs, or interactively to build and execute commands with a keyboard-driven TUI.

## Install

```bash
# Via go install
go install github.com/aallbrig/treemand/cli/treemand@latest

# From source
git clone https://github.com/aallbrig/treemand.git
cd treemand
make install
```

## Quick Start

```bash
treemand git              # static tree view
treemand -i kubectl       # interactive TUI
treemand version          # show version info
```

## TUI Controls

| Key | Action |
|-----|--------|
| ↓/j/s | Next item (auto-expands) |
| ↑/k/w | Previous item |
| →/l/d | Enter node (expand + move in) |
| ←/h/a | Exit node (move to parent) |
| Enter | Set command / add flag to preview |
| f | Open flag picker modal |
| Ctrl+E | Execute or copy command |
| Tab | Cycle pane focus (tree → help → preview) |
| / | Fuzzy filter |
| h | Toggle help pane |
| q/Esc | Quit |

## Discovery

treemand uses `--help` recursively to build the tree. The `--strategy` flag selects the discovery strategy (default: `help`). Pass `--depth=N` to limit recursion.

## Cache

Discovered trees are cached in SQLite (`~/.treemand/cache.db`). To clear stale entries:

```bash
treemand cache clear
```

## Development

```bash
make build   # build binary
make test    # run tests
make install # install to $GOPATH/bin
```

## License

[MIT](LICENSE) © aallbrig

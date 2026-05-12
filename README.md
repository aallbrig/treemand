# 🌲 treemand — explore CLIs as trees, build commands interactively

[![Go version](https://img.shields.io/badge/go-1.22%2B-blue)](https://go.dev/dl/)
[![CI](https://img.shields.io/github/actions/workflow/status/aallbrig/treemand/ci.yml?branch=main)](https://github.com/aallbrig/treemand/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

🌐 **Website & docs:** https://aallbrig.github.io/treemand

treemand introspects any CLI tool's `--help` output and maps its entire
command hierarchy into a navigable tree. Two modes:

- **Static** — print a colored ASCII tree to your terminal (or pipe JSON/YAML for scripting)
- **Interactive** (`-i`) — keyboard-driven TUI to explore commands, pick flags, and assemble + copy/run a full CLI invocation

![treemand demo](https://aallbrig.github.io/treemand/demo.gif)

## Install

```bash
# Homebrew (macOS / Linux) — tap: https://github.com/aallbrig/homebrew-tap
brew tap aallbrig/tap && brew install treemand

# Via go install
go install github.com/aallbrig/treemand/cli/treemand@latest

# Pre-built binaries: https://github.com/aallbrig/treemand/releases/latest

# From source
git clone https://github.com/aallbrig/treemand.git && cd treemand && make install
```

## Quick Start

```bash
treemand git                       # colored ASCII tree
treemand -i kubectl                # interactive TUI
treemand --output=json docker      # machine-readable JSON
treemand --output=yaml gh          # YAML (same structure as JSON)
treemand --tree-style=graph git    # classic ├──/└── connectors
```

## Output Formats

treemand supports three output formats — a human-readable tree, plus JSON and
YAML for scripting and tooling integration:

```bash
treemand git                       # default colored tree
treemand --output=json git         # JSON — pipe to jq, store, diff
treemand --output=yaml git         # YAML — same structure, friendlier to read
```

JSON/YAML output includes the full tree: subcommand names, descriptions, flags
(with types), positional arguments, and children — everything treemand discovers.

## Tree Display Styles

Cycle through styles interactively with **T** inside the TUI, or set one from
the command line:

```bash
treemand --tree-style=default git     # icon-prefixed tree with inline flag pills (baseline)
treemand --tree-style=columns git     # name · description — table-like alignment
treemand --tree-style=compact git     # no icons, no flags — maximum density
treemand --tree-style=graph git       # ├──/└── connectors like the `tree` command
```

## Interactive TUI (`-i`)

```bash
treemand -i git
```

The TUI lets you **browse a CLI's command tree and assemble a specific command**
step by step. Here's the workflow:

1. **Navigate** — use `↓`/`↑` (or `j`/`k` in vim mode) to browse subcommands
2. **Expand / Collapse** — `→` expands a node; press again to enter its children. `←` collapses.
3. **Pick a command** — press `Enter` on any subcommand to set it in the preview bar
4. **Add flags** — press `f` to open the flag picker, or navigate to a flag row and press `Enter`
5. **Add positionals** — press `Enter` on a positional argument row to fill in a value
6. **Execute or copy** — press `Ctrl+E` to copy the assembled command or run it directly

The preview bar at the top updates live as you build the command.

### Key Bindings

| Key | Action |
|-----|--------|
| `↓`/`↑` or `j`/`k` | Navigate (never auto-expands — you control what opens) |
| `→` or `l` | Expand node (first press); enter first child (second press) |
| `←` or `h` | Collapse node (first press); go to parent (second press) |
| `Shift+→`/`Shift+←` | Expand / collapse entire subtree |
| `gg` / `G` | Jump to top / bottom of tree |
| `Enter` | Set command in preview / add flag / fill positional |
| `f` / `F` | Open flag picker modal |
| `e` / `E` | Expand all / collapse all |
| `S` | Toggle section headers (Sub commands, Flags, etc.) |
| `T` | Cycle display style (default → columns → compact → graph) |
| `H` / `Ctrl+P` | Toggle help pane (uppercase `H` only; lowercase `h` is Left in vim mode) |
| `Tab` / `Shift+Tab` | Cycle pane focus |
| `/` | Fuzzy filter |
| `n` / `N` | Next / previous search match |
| `d` / `D` | Open docs URL in browser |
| `Ctrl+E` | Copy or execute the assembled command |
| `Ctrl+S` | Cycle navigation scheme (arrows → vim → WASD) |
| `?` | Show all key bindings |
| `q` / `Esc` | Quit |

### Mouse Support

Click a node to select it, click `▶`/`▼` to toggle, scroll to navigate.

## Useful Flags

```bash
treemand --depth=2 kubectl         # limit recursion depth
treemand --filter=remote git       # only nodes matching pattern
treemand --exclude=help git        # exclude nodes matching pattern
treemand --commands-only kubectl   # hide flags and positionals
treemand --icons=ascii git         # ASCII-safe icons (▼ → v, • → -)
treemand --icons=nerd git          # Nerd Font glyphs (requires patched font)
treemand --no-color git            # disable color output
treemand --no-cache git            # bypass the discovery cache
```

## Cache

Discovered trees are cached in SQLite (`~/.treemand/cache.db`).

```bash
treemand cache list                # show cached CLIs
treemand cache clear git           # clear one entry
treemand cache clear               # clear all entries
```

## Configuration

treemand uses an optional YAML config file. Settings cascade:
**CLI flags > environment variables (`TREEMAND_*`) > config file > defaults**.

```bash
treemand config                    # show current config with file location
treemand config view               # same as above
treemand config validate           # check for errors/warnings
treemand config validate --strict  # treat warnings as errors
treemand config set icons nerd     # set a value
treemand config set depth 5        # set tree depth
treemand config init               # create default config with comments
treemand config path               # print config file path
treemand config edit               # open in $EDITOR
```

Config file locations (searched in order):
1. `$XDG_CONFIG_HOME/treemand/config.yaml` (typically `~/.config/treemand/`)
2. `$HOME/.treemand/config.yaml`

If no config file exists, built-in defaults are used. Run `treemand config init`
to create a commented default config.

See [`docs/features/config.md`](docs/features/config.md) for full documentation.

## Development

```bash
task dev          # run tests + install binary (recommended dev loop)
task test         # run all tests
task lint         # run golangci-lint
task precommit    # full hygiene gate: fmt, vet, lint, vuln, test
task fix          # auto-fix formatting and lint issues
task build        # compile local binary
```

Make targets mirror the Taskfile and remain available:
`make dev`, `make test`, `make lint`, `make build`.

### Git Hooks

Install pre-commit and pre-push hooks (run `task precommit` automatically):

```bash
bash scripts/setup-hooks.sh
```

The pre-commit hook runs the full hygiene suite before every commit.
The pre-push hook runs the same suite before pushing to `main` or `develop`.

## License

[MIT](LICENSE) © aallbrig

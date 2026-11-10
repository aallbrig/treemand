# 🌲 treemand — explore CLIs as trees, build commands interactively

[![Go version](https://img.shields.io/badge/go-1.25%2B-blue)](https://go.dev/dl/)
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
# One line (Linux, macOS): checksum-verified, installs to ~/.local/bin, no sudo.
# Read it first: https://aallbrig.github.io/treemand/install.sh  (options: --help)
curl -fsSL https://aallbrig.github.io/treemand/install.sh | sh

# Homebrew (macOS / Linux) — tap: https://github.com/aallbrig/homebrew-tap
brew tap aallbrig/tap && brew install treemand

# Via go install
go install github.com/aallbrig/treemand/cli/treemand@latest

# Pre-built binaries (and Windows): https://github.com/aallbrig/treemand/releases/latest
# Step-by-step for your OS, with copy buttons: https://aallbrig.github.io/treemand/installation/

# From source
git clone https://github.com/aallbrig/treemand.git && cd treemand && task install
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
| `PgUp`/`PgDn` or `Ctrl+U`/`Ctrl+D` | Move up / down a screenful (handy on 400-command CLIs like `aws`) |
| `gg` / `G` | Jump to top / bottom of tree |
| `Space` | Toggle expand / collapse |
| `Enter` | Set command in preview / add flag / fill positional |
| `f` / `F` | Open flag picker modal |
| `e` / `E` | Expand all / collapse all |
| `S` | Toggle section headers (Sub commands, Flags, etc.) |
| `t` / `T` | Cycle display style (default → columns → compact → graph) |
| `H` / `Ctrl+P` | Toggle help pane (uppercase `H` only; lowercase `h` is Left in vim mode) |
| `Tab` / `Shift+Tab` | Cycle pane focus |
| `/` | Fuzzy filter |
| `n` / `N` | Next / previous search match |
| `d` / `D` | Open docs URL in browser |
| `Ctrl+K` | Clear the preview bar |
| `Ctrl+E` | Copy or execute the assembled command |
| `Ctrl+S` | Cycle navigation scheme (arrows → vim → WASD) |
| `r` / `R` | Re-discover / refresh children of selected node |
| `?` | Show all key bindings (also named in the footer: `?:keys`) |
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

## Scripting

treemand is safe to use in pipelines.

```bash
treemand --output=json git | jq '.children[].name'   # machine-readable tree (json or yaml)
treemand git | less -R                               # plain text when piped
```

- **Output goes to stdout, messages to stderr.** Errors never write to stdout, so a pipe only ever
  sees a complete result. The progress spinner and the "undiscovered subcommands" note use stderr.
- **Color is on only when stdout is a terminal.** Piped output has no escape codes. Set
  [`NO_COLOR`](https://no-color.org) (to any value) or pass `--no-color` to turn color off on a
  terminal too.
- **Bounded runs.** `--timeout=SECONDS` limits discovery of a CLI that hangs, and a CLI that prints
  a flood of help text is read only up to 1MB.

### Exit codes

| Code | Meaning | Examples |
|------|---------|----------|
| `0` | Success | the tree was printed |
| `1` | Runtime failure | the CLI is not on `PATH`; discovery could not run |
| `2` | Usage error | no CLI name, more than one, an unknown flag, a bad flag value, an unknown `--output` format or shell |

A usage error prints what was wrong plus a hint, for example `treemand` alone prints
`Error: missing CLI name` and shows `treemand git` as an example.

## Shell completion

One step per shell (open a new shell afterwards):

```bash
# bash
mkdir -p ~/.local/share/bash-completion/completions
treemand completion bash > ~/.local/share/bash-completion/completions/treemand
# zsh: add this line to ~/.zshrc (after compinit)
source <(treemand completion zsh)
# fish
treemand completion fish > ~/.config/fish/completions/treemand.fish
# PowerShell
treemand completion powershell | Out-String | Invoke-Expression
```

`treemand completion <shell> --help` shows the same steps, and `treemand completion` with no shell
lists the supported ones. Completion also suggests CLI names from your `PATH`.

## Man page

Release archives and the Homebrew formula install `treemand.1` (`man treemand`). From a source
checkout, generate it with `treemand gendocs -o ./docs` and read it with `man ./docs/man/treemand.1`.

## Tested CLIs

Parser quality is guarded by a regression corpus: sanitized `--help` captures of real CLIs
live in `cli/treemand/discovery/testdata/corpus/`, and `task ci` fails if a change loses
subcommands, flags or the description of any of them. Currently covered:
`apt`, `aws`, `curl`, `docker`, `gh`, `git`, `go`, `helm`, `jq`, `kubectl`, `npm`, `pip`,
`ssh`, `systemctl`, `tar`, `tmux`. (`cargo` and `terraform` are planned: capture them on a
machine that has them with `task corpus:capture -- cargo terraform`.)

## How treemand compares

Other tools turn `--help` into a tree: [helptree](https://pkg.go.dev/github.com/kwrkb/helptree) (Go),
[cli-tree](https://www.pkgstats.com/pkg:@crafter/cli-tree) (JS), [mandible](https://docs.rs/crate/mandible/0.2.0)
and [te-cli](https://docs.rs/te-cli/0.1.0) (Rust). A fair summary, from their published documentation
(we have not run them side by side yet, so these are their claims):

- **Where treemand is ahead:** it both browses the tree *and* assembles and runs the command, works
  in a browser (the Try page), caches results so repeat runs are instant, prints JSON and YAML, runs on
  Windows, and publishes its quality evidence (the [tested CLIs](#tested-clis) corpus, fuzzing and
  end-to-end tests).
- **Where peers are ahead:** mandible reads more sources (completion scripts, native protocols, user
  overrides), has a `--doctor` report and never probes destructive tools; helptree and mandible open
  instantly and fill the tree in (treemand waits for discovery); cli-tree reads help text from stdin and
  can write HTML; te-cli keeps command history and presets; mandible ships `.deb` and `.rpm` packages.

Each of those gaps is a tracked issue. The full comparison with sources is in
[docs/research](docs/research/20261110_002254_UTC_landscape.md).

## Cache

Discovered trees are cached in SQLite (`~/.treemand/cache.db`). A cache hit is near-instant (about
30ms for `aws`, 422 services) because it never starts the CLI: entries are keyed on the executable on
disk (its path, size and modification time), so upgrading a tool, by replacing the file or repointing a
symlink, refreshes its entry on the next run. Entries also expire after 24 hours.

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

---
title: "Reference"
weight: 4
---

## Command Syntax

```
treemand <cli> [flags]
treemand version
treemand cache [clear|list]
```

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--interactive` | `-i` | false | Launch interactive TUI explorer |
| `--strategy` | `-s` | `help` | Discovery strategies: `help`, `man`, `completions` (comma-separated) |
| `--depth` | | `-1` | Max tree depth (-1 = unlimited) |
| `--filter` | | | Only show nodes whose name matches pattern |
| `--exclude` | | | Exclude nodes whose name matches pattern |
| `--commands-only` | | false | Hide flags and positional arguments |
| `--full-path` | | false | Show full command paths in tree |
| `--output` | | `text` | Output format: `text` or `json` |
| `--no-color` | | false | Disable color output |
| `--no-cache` | | false | Skip cache lookup and write |
| `--timeout` | | `30` | Discovery timeout in seconds |
| `--debug` | | false | Enable debug logging to stderr |

## Subcommands

### `version`

Print version, git commit, and build date.

```bash
treemand version
# treemand v1.0.0 (abc1234) built 2025-01-01T00:00:00Z
```

Also available as a flag:

```bash
treemand --version
```

### `cache`

Manage the discovery cache (`~/.treemand/cache.db`).

```bash
treemand cache list           # List cached entries
treemand cache clear <cli>    # Remove one CLI's cached entry
treemand cache clear-all      # Remove all cached entries
```

## Non-Interactive Output

The default output is a Unicode tree:

```
▼ git  the stupid content tracker
├── ▼ remote [--verbose]  Manage set of tracked repositories
│   ├── • add <name> <url>  Add a remote
│   └── • remove <name>  Remove a remote
├── • commit [--message=<string>, --all]  Record changes
└── • status [--short, --branch]  Show working tree status
```

### Icons

| Icon | Meaning |
|------|---------|
| `▼` | Command with children (expanded) |
| `▶` | Command with children (collapsed) |
| `•` | Leaf command (no subcommands) |

### Output Formats

```bash
treemand --output=json git     # JSON tree structure
treemand --output=text git     # Default colored tree
```

JSON output follows this schema:

```json
{
  "name": "git",
  "description": "the stupid content tracker",
  "flags": [{"name": "--version", "value_type": "bool"}],
  "positionals": [],
  "children": [...]
}
```

## Interactive TUI (`-i`)

```bash
treemand -i git
```

### Layout

```
┌─ ► git remote add ────────────────────────────────────┐
│   (live command preview)                              │
└───────────────────────────────────────────────────────┘
┌─ Tree: git ───────────────┐┌─ Help: remote ───────────┐
│ ▼ git                     ││ Manage set of tracked    │
│   ▼ remote                ││ repositories.            │
│ ► • add <name> <url>      ││                          │
│   • remove <name>         ││ --verbose (-v)           │
│   • get-url <name>        ││   Be verbose             │
└───────────────────────────┘└──────────────────────────┘
  git remote add  [arrows]  /:filter  H:help  ?:keys  q:quit
```

### Keyboard Controls

#### Navigation

| Keys (arrows) | Keys (vim) | Keys (WASD) | Action |
|---------------|------------|-------------|--------|
| `↑` / `↓` | `k` / `j` | `w` / `s` | Move up / down through siblings and section items |
| `→` | `l` | `d` | Enter node (expand + move to first child) |
| `←` | `h` | `a` | Exit node (go to parent; collapse if at root) |
| `Space` | | | Toggle expand/collapse on a command node |
| `Enter` | | | Execute highlighted command |

Toggle navigation scheme with **Ctrl+S** (cycles: arrows → vim → WASD).

#### Tree Operations

| Key | Action |
|-----|--------|
| `/` | Fuzzy filter tree nodes |
| `R` | Refresh / re-discover current node |
| `F` | Open flags modal for current node |
| `P` | Open positionals modal for current node |

#### View Controls

| Key | Action |
|-----|--------|
| `H` | Toggle help pane |
| `Ctrl+P` | Toggle panes |
| `E` | Full preview edit |
| `?` | Show all key bindings modal |
| `Ctrl+S` | Cycle navigation scheme |

#### Actions

| Key | Action |
|-----|--------|
| `Enter` | Execute command (shows confirmation) |
| `Ctrl+E` | Execute or copy command to clipboard |
| `Esc` / `q` | Quit (copies command to clipboard as fallback) |
| `Ctrl+Z` / `Ctrl+Y` | Undo / redo command edits |

#### Mouse

| Interaction | Action |
|-------------|--------|
| Click node | Select node |
| Click `▶`/`▼` | Toggle expand/collapse |
| Scroll | Scroll the focused pane |

### Flags Modal (`F`)

Press `F` on any command node to open an interactive flag selector:

- Checkboxes for boolean flags
- Text inputs for value flags with type hints
- Tab completion on flag names
- Green border = valid selection, red = invalid

### Positionals Modal (`P`)

Press `P` on a command node to fill in positional arguments:

- Labeled input fields matching the command signature
- `<required>` shown in red if left empty

## Caching

Discovery results are cached in an SQLite database:

| Location | `~/.treemand/cache.db` |
|----------|------------------------|
| TTL | 24 hours |
| Key | CLI name + version + strategies |
| Schema | `v8` |

```bash
# Skip the cache for this run
treemand --no-cache docker

# Inspect or clear the cache
treemand cache list
treemand cache clear
```

## Discovery Strategies

### `help` (default)

Recursively runs `<cli> --help` / `<cli> <subcmd> --help` to build the tree.
Falls back to `<cli> help <subcmd>`, man page lookup, and error output mining.

### `man`

Parses the `man` page for the CLI (if available) using `man <cli>` and stripping
groff formatting. Provides richer descriptions than `--help` for many Unix tools.

### `completions`

Uses shell completion output (`<cli> __complete`, `<cli> completion`) to
enumerate subcommands without executing `--help` for every node.

## Color Configuration

Colors follow the `config.ColorScheme`:

| Element | Default |
|---------|---------|
| Base command | bold white `#FFFFFF` |
| Subcommand | blue `#0000FF` |
| Flag | green `#00FF00` |
| Positional | yellow `#FFFF00` |
| Value type | magenta `#FF00FF` |
| Invalid/error | red `#FF0000` |
| Highlight background | cyan `#00FFFF` |
| Highlight text | black `#000000` |

Override via environment variables or a config file (see `--config`).

## Self-Dogfooding

```bash
treemand treemand
```

treemand introspects its own cobra command tree, so you can explore its
own flags and subcommands interactively:

```bash
treemand -i treemand
```

## Man Page

If treemand was installed from a release tarball, a man page is included:

```bash
man treemand
```

Or generate docs locally:

```bash
treemand gendocs --output-dir ./docs
```

This writes `docs/man/treemand.1` and `docs/md/treemand.md`.

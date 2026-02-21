---
title: "Getting Started"
---

## Basic Usage

Discover and visualize any CLI command hierarchy:

```bash
# Explore git
treemand git

# Explore kubectl
treemand kubectl

# Explore the AWS CLI
treemand aws

# Explore treemand itself
treemand treemand
```

## Non-Interactive Mode

```
▼ git  the version control system
├── ▼ remote
│   ├── • add <name> <url>
│   ├── • remove <name>
│   └── • get-url <name>
├── • commit [-m <message>] [file]
├── • status
└── • push [--force]
```

### Flags

```bash
# Limit tree depth
treemand --depth=2 git

# Filter to matching nodes
treemand --filter=remote git

# Exclude certain nodes
treemand --exclude=help git

# Show commands only (no flags/positionals)
treemand --commands-only kubectl

# Output as JSON
treemand --output=json git > git-tree.json

# Disable color
treemand --no-color git
```

## Interactive TUI (`-i`)

```bash
treemand -i git
```

The TUI gives you a full explorer:

```
┌─ Preview ─────────────────────────────────────────────┐
│ git remote add <name> <url>                           │
└───────────────────────────────────────────────────────┘
┌─ Tree: git ──────────────┐ ┌─ Help: remote ──────────┐
│ ▼ git                    │ │ Manage remotes           │
│   ▼ remote               │ │                          │
│ ▶ ● add <name> <url>     │ │ Subcommands:             │
│   • remove <name>        │ │   add, remove, get-url   │
└──────────────────────────┘ └─────────────────────────┘
  git remote add  nav:arrows  ?:help  /:filter  q:quit
```

### Keyboard Controls

| Key | Action |
|-----|--------|
| `↑`/`↓` | Navigate tree |
| `→`/`Space`/`Enter` | Expand node |
| `←` | Collapse node |
| `Ctrl+S` | Cycle nav scheme (arrows → vim → WASD) |
| `/` | Fuzzy filter |
| `H` | Toggle help pane |
| `?` | Show key bindings |
| `Ctrl+P` | Toggle panes |
| `R` | Refresh node |
| `q`/`Esc` | Quit |

### Navigation Schemes

Toggle between three schemes with **Ctrl+S**:
- **Arrows** — `↑↓←→`
- **Vim** — `hjkl`
- **WASD** — `wasd`

## Caching

Discovery results are cached in `~/.treemand/cache.db` for 24 hours.

```bash
# Bypass cache
treemand --no-cache git

# Cache location can be changed via environment variable
TREEMAND_CACHE_DIR=/tmp/treemand treemand git
```

## Discovery Strategies

```bash
# Default: --help recursion
treemand -s help git

# Future: shell completions (coming soon)
treemand -s completions kubectl
```

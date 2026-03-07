---
title: "Getting Started"
---

## Basic Usage

Discover and visualize any CLI command hierarchy:

```bash
treemand git          # explore git
treemand kubectl      # explore kubectl
treemand aws          # explore the AWS CLI
treemand treemand     # introspect treemand itself
```

## Non-Interactive Output

```
▼ git  [--version --verbose]
├── ▼ remote
│   ├── • add <name> <url>
│   ├── • remove <name>
│   └── • get-url <name>
├── • commit [--message=<string>]
├── • status
└── • push [--force]
```

### Useful Flags

```bash
treemand --depth=2 kubectl          # limit tree depth
treemand --filter=remote git        # only show matching nodes
treemand --exclude=help git         # exclude nodes by name
treemand --commands-only kubectl    # subcommands only, no flags
treemand --output=json git          # machine-readable JSON
treemand --no-color git             # disable color
treemand --icons=ascii git          # ASCII-safe icon set (no Unicode)
treemand --icons=nerd git           # Nerd Font icons (requires patched font)
```

### Stub Nodes

Large CLIs (aws: 400+ services, gcloud: 300+ groups) would take minutes to
fully explore. treemand creates **stub nodes** `(…)` for commands with many
children, then discovers them lazily.

```bash
# Explore a specific service directly instead of the full tree
treemand aws s3

# Force full eager discovery (slow — use with care)
treemand --stub-threshold=500 aws
```

## Interactive TUI (`-i`)

```bash
treemand -i git
```

The TUI gives you a live explorer with a preview bar, tree pane, and help pane.

### Keyboard Controls

| Key | Action |
|-----|--------|
| `↑`/`↓` or `j`/`k` | Navigate tree |
| `→`/`Space`/`Enter` | Expand / add to command |
| `←` | Collapse |
| `Ctrl+S` | Cycle nav scheme (arrows → vim → WASD) |
| `/` | Fuzzy filter |
| `H` | Toggle help pane |
| `f` / `F` | Open flags modal |
| `p` / `P` | Open positionals modal |
| `Ctrl+E` | Copy or execute command |
| `?` | Show all key bindings |
| `q` / `Esc` | Quit |

## Configuration

treemand reads `~/.config/treemand/config.yaml` (XDG standard) or
`~/.treemand/config.yaml` as a fallback.

```yaml
# ~/.config/treemand/config.yaml
icons: ascii          # unicode (default) | ascii | nerd
desc_line_length: 80  # max chars before description is truncated
stub_threshold: 50    # subcommand count before switching to stub nodes

colors:
  subcmd: "#5EA4F5"
  flag: "#50FA7B"
```

Precedence: **CLI flags > environment variables > config file > defaults**

## Caching

Results are cached in `~/.treemand/cache.db` for 24 hours.

```bash
treemand --no-cache git                        # bypass cache
TREEMAND_CACHE_DIR=/tmp/treemand treemand git  # custom cache dir
treemand cache list                            # show cached CLIs
treemand cache clear git                       # clear one entry
```

## Discovery Strategies

```bash
treemand -s help git          # default: --help recursion
treemand -s man git           # man page parser (richer descriptions)
treemand -s help,man git      # combine strategies, merge results
```

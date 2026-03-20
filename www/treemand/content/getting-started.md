---
title: "Getting Started"
---

## Explore Any CLI

Point treemand at a CLI tool and it maps out the full command hierarchy:

```bash
treemand git          # explore git
treemand kubectl      # explore kubectl
treemand aws          # explore the AWS CLI
treemand treemand     # introspect treemand itself
```

## Output Formats

treemand produces three output formats — a human-readable tree for your
terminal, plus JSON and YAML for piping into scripts and tools:

```bash
treemand git                       # colored ASCII tree (default)
treemand --output=json git         # JSON — full tree with flags, descriptions
treemand --output=yaml git         # YAML — same structure, easier to scan
```

JSON/YAML include the complete tree: subcommand names, descriptions, flags
with types, and positional arguments. Pipe JSON to `jq`:

```bash
treemand --output=json git | jq '.children[].name'    # list all subcommands
```

## Tree Display Styles

treemand offers four visual styles. Use `--tree-style` or press **T** inside
the interactive TUI to cycle through them:

```bash
treemand --tree-style=default git     # ▼/▶/• icons with inline flag pills
treemand --tree-style=columns git     # name · description — table alignment
treemand --tree-style=compact git     # no icons, no flags — maximum density
treemand --tree-style=graph git       # ├──/└── connectors like `tree`
```

## Useful Flags

```bash
treemand --depth=2 kubectl          # limit tree depth (default: unlimited)
treemand --filter=remote git        # only show matching nodes
treemand --exclude=help git         # exclude nodes by name
treemand --commands-only kubectl    # subcommands only, no flags
treemand --no-color git             # disable color
treemand --icons=ascii git          # ASCII-safe icons (no Unicode)
treemand --icons=nerd git           # Nerd Font icons (requires patched font)
treemand --no-cache git             # bypass the discovery cache
```

## Stub Nodes

Large CLIs (aws: 400+ services, gcloud: 300+ groups) would take minutes to
fully explore. treemand creates **stub nodes** `(…)` for commands with many
children, then discovers them lazily when you expand them in the TUI.

```bash
treemand aws s3                     # explore a specific service directly
treemand --stub-threshold=500 aws   # force full eager discovery (slow)
```

## Interactive TUI (`-i`)

```bash
treemand -i git
```

The TUI gives you a live explorer with three panes:

- **Preview bar** (top) — shows the command you're building, updated live
- **Tree pane** (left) — the command hierarchy you navigate
- **Help pane** (right, toggle with `H`) — shows `--help` output for the
  selected node

### Tutorial: Build `git commit --message="fix bug" --all`

Here's a step-by-step walkthrough of assembling a command interactively:

**1. Launch the TUI:**

```bash
treemand -i git
```

You see the git command tree. The root `git` node is selected.

**2. Navigate to `commit`:**

Press `↓` (or `j`) to move down to the `commit` subcommand.

**3. Pick the command:**

Press `Enter` on `commit`. The preview bar at the top now reads:
`► git commit`

**4. Add the `--message` flag:**

Press `f` to open the **flag picker**. You see all flags for `git commit`.
Type `mess` to filter, then press `Enter` on `--message`. Since `--message`
takes a string value, an **input prompt** appears. Type `fix bug` and
press `Enter`.

Preview bar now reads: `► git commit --message=fix bug`

**5. Add the `--all` flag:**

Press `f` again, find `--all` (a boolean flag), and press `Enter`. It gets
added directly — no input prompt needed for boolean flags.

Preview bar: `► git commit --message=fix bug --all`

**6. Copy or run:**

Press `Ctrl+E`. A confirmation modal appears:

- **Copy** — copies the command to your clipboard
- **Run** — executes the command in your shell

Press `c` to copy or `r` to run.

### Keyboard Reference

| Key | Action |
|-----|--------|
| `↓`/`↑` or `j`/`k` | Navigate tree (cursor only — never auto-expands) |
| `→` or `l` | Expand a node and stay on it; press again to enter children |
| `←` or `h` | Collapse a node and stay on it; press again to go to parent |
| `Shift+→` / `Shift+←` | Expand / collapse entire subtree |
| `Enter` | Pick a command / add a flag / fill a positional |
| `f` | Open flag picker (with search) |
| `S` | Toggle section headers (Sub commands, Flags, Inherited flags) |
| `T` | Cycle display style (default → columns → compact → graph) |
| `H` | Toggle help pane |
| `/` | Fuzzy filter |
| `Backspace` | Remove last token from preview |
| `Ctrl+E` | Copy or execute the assembled command |
| `Ctrl+S` | Cycle navigation scheme (arrows → vim → WASD) |
| `?` | Show all key bindings |
| `q` / `Esc` | Quit |

> **Tip:** Press `?` inside the TUI to see the full key binding reference
> at any time.

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

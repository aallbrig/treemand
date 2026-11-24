---
title: "Reference"
description: "Every treemand flag, output format, key binding and configuration option in one place."
weight: 4
---

# Reference

## Command Syntax

```
treemand <cli> [flags]
treemand version
treemand schema
treemand doctor <cli>
treemand mcp
treemand cache [clear|list]
```

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--interactive` | `-i` | false | Launch interactive TUI explorer |
| `--strategy` | `-s` | `help` | Discovery strategies: `help`, `man`, `completions`, `scripts` (comma-separated) |
| `--depth` | | `3` | Max tree depth (default 3; -1 = unlimited) |
| `--filter` | | | Only show nodes whose name matches pattern |
| `--exclude` | | | Exclude nodes whose name matches pattern |
| `--commands-only` | | false | Hide flags and positional arguments |
| `--full-path` | | false | Show full command paths in tree |
| `--output` | | `text` | Output format: `text`, `json`, `yaml`, or `html` (a self-contained page) |
| `--tree-style` | | `default` | Tree presentation: `default`, `columns`, `compact`, `graph` |
| `--icons` | | `unicode` | Icon preset: `unicode`, `ascii`, `nerd` |
| `--line-length` | | `80` | Max description chars before truncation |
| `--no-color` | | false | Disable color output |
| `--no-cache` | | false | Skip cache lookup and write |
| `--refresh` | | false | Re-discover even if cached, and update the cache entry |
| `--stdin` | | false | Read the CLI's help text from stdin instead of running it; subcommands stay undiscovered (`docker --help \| treemand --stdin docker`) |
| `--timeout` | | `30` | Discovery timeout in seconds (0 = no limit) |
| `--debug` | | false | Enable debug logging to stderr |

## Subcommands

### `version`

Print version, git commit, and build date.

```bash
treemand version
# treemand v0.3.0 (abc1234) built 2026-01-01
```

### `schema`

Print the JSON Schema that `--output=json` and `--output=yaml` follow (see
[JSON / YAML Schema](#json--yaml-schema)).

```bash
treemand schema | jq '.["$defs"] | keys'
# ["flag", "node", "positional"]
```

### `doctor`

Discover a CLI afresh and report how it was parsed: strategy and help form,
node count, commands without a description or flags, undiscovered stubs,
commands that failed, probe outcomes and the five slowest probes. Paste the
text report into a bug report; `--output=json` prints the same report for
scripts. See [doctor](../commands/doctor/).

```bash
treemand doctor git
treemand doctor --output=json kubectl | jq .slowest_probes
```

### `cache`

Manage the discovery cache (`~/.treemand/cache.db`).

```bash
treemand cache list           # List cached entries with age and size
treemand cache clear <cli>    # Remove one CLI's cached entry
treemand cache clear          # Remove all cached entries
```

## Output Formats

treemand supports three output modes. The default is a colored tree for
terminals; JSON and YAML are intended for scripting, diffing, and tool
integration.

```bash
treemand git                     # colored text tree (default)
treemand --output=json git       # full tree as JSON
treemand --output=yaml git       # full tree as YAML (same structure)
treemand --output=html git > git.html  # one shareable page: collapsible tree, search
```

### JSON / YAML Schema

JSON and YAML output share one structure, described by a versioned
[JSON Schema](https://treemand.dev/schema/tree.v1.json) (draft 2020-12). Print it with:

```bash
treemand schema > tree.schema.json
```

A trimmed example:

```json
{
  "schema_version": 1,
  "name": "git",
  "full_path": ["git"],
  "description": "the stupid content tracker",
  "flags": [
    {"name": "--version", "value_type": "bool", "description": "Print version"}
  ],
  "children": [
    {
      "name": "commit",
      "full_path": ["git", "commit"],
      "description": "Record changes to the repository",
      "flags": [
        {"name": "--message", "short_name": "-m", "value_type": "string", "description": "Commit message"}
      ],
      "positionals": [
        {"name": "pathspec", "required": false}
      ],
      "discovered": true
    }
  ],
  "discovered": true
}
```

Every node has `name`, `full_path` and `discovered`; every flag has `name`;
every positional has `name` and `required`. Other properties are omitted when
empty or false. `schema_version` appears on the root only.

### Compatibility promise

- `schema_version` names the schema the output follows. It is `1` today.
- Within a version, output only **gains optional properties**. Nothing is
  removed, renamed or retyped, and a property that is always present stays
  always present.
- Anything else is a new version with a new schema file
  (`tree.v2.json`), announced in the release notes. Old schema files stay
  published.
- Consumers should ignore properties they do not recognise.
- CI enforces this: every corpus CLI's output is validated against the schema,
  and a test fails when a version-1 property changes without a version bump.

Pipe JSON to `jq` for extraction:

```bash
treemand --output=json git | jq '.children[].name'       # list subcommands
treemand --output=json git | jq '.children[] | select(.name == "commit") | .flags[].name'
```

## Tree Display Styles

treemand supports four presentation styles. In the TUI, press **T** to cycle
through them; from the command line, use `--tree-style`:

### `default` — icon-prefixed tree with inline flag pills

```
▼ git  the stupid content tracker
├── ▼ remote [--verbose]  Manage set of tracked repositories
│   ├── • add <name> <url>  Add a remote
│   └── • remove <name>  Remove a remote
├── • commit [--message=<string>, --all]  Record changes
└── • status [--short, --branch]  Show working tree status
```

### `columns` — name · description alignment

```
  git                · the stupid content tracker
    remote           · Manage set of tracked repositories
      add            · Add a remote
      remove         · Remove a remote
    commit           · Record changes
    status           · Show working tree status
```

### `compact` — maximum density (no icons, no flags)

```
  git
    remote
      add
      remove
    commit
    status
```

### `graph` — classic tree connectors

```
└── git
    ├── remote
    │   ├── add
    │   └── remove
    ├── commit
    └── status
```

## Non-Interactive Output

The default output is a Unicode tree with icons:

| Icon | Meaning |
|------|---------|
| `▼` | Command with children (expanded) |
| `▶` | Command with children (collapsed, TUI only) |
| `•` | Leaf command (no subcommands) |

Use `--icons=ascii` for terminals without Unicode, or `--icons=nerd` for
Nerd Font glyphs.

## Interactive TUI (`-i`)

```bash
treemand -i git
```

### What the TUI Does

The TUI lets you **explore a CLI's command tree and assemble a specific
command** interactively. The workflow:

1. **Browse** — navigate subcommands with `↓`/`↑` (or `j`/`k`)
2. **Expand** — press `→` to open a node; press again to enter children
3. **Pick a command** — press `Enter` to set it in the preview bar
4. **Add flags** — press `f` to open the flag picker, or `Enter` on a flag row
5. **Fill positionals** — press `Enter` on a positional to open an input prompt
6. **Copy or run** — press `Ctrl+E` to copy the assembled command or run it

The **preview bar** at the top updates live as you build the command.

### Layout

```
┌─ ► git remote add ────────────────────────────────────┐
│   (live command preview — updates as you pick items)  │
└───────────────────────────────────────────────────────┘
┌─ Tree: git ───────────────┐┌─ Help: remote ───────────┐
│ ▼ git                     ││ Manage set of tracked    │
│   ▼ remote                ││ repositories.            │
│ ► • add <name> <url>      ││                          │
│   • remove <name>         ││ --verbose (-v)           │
│   • get-url <name>        ││   Be verbose             │
└───────────────────────────┘└──────────────────────────┘
  git remote add  [arrows]  ←:collapse  →:expand  H:help  q:quit
```

### Keyboard Controls

#### Navigation

| Keys (arrows) | Keys (vim) | Keys (WASD) | Action |
|---------------|------------|-------------|--------|
| `↑` / `↓` | `k` / `j` | `w` / `s` | Move up / down (cursor only — never auto-expands) |
| `→` | `l` | `d` | Expand node and stay (1st); enter first child (2nd) |
| `←` | `h` | `a` | Collapse node and stay (1st); go to parent (2nd) |
| `Shift+→` | `Shift+→` | `Shift+→` | Expand entire subtree (at root = expand all) |
| `Shift+←` | `Shift+←` | `Shift+←` | Collapse entire subtree (at root = collapse all) |

This matches the VS Code / macOS Finder tree model. To collapse a node and
move to its sibling: press `←` (collapse), then `↓` (next sibling).

Toggle navigation scheme with **Ctrl+S** (cycles: arrows → vim → WASD).

#### Tree Operations

| Key | Action |
|-----|--------|
| `/` | Fuzzy filter tree nodes |
| `n` / `N` | Jump to next / previous search match (after `/` search) |
| `gg` | Jump to top of tree |
| `G` | Jump to bottom of tree |
| `e` | Expand all nodes |
| `E` | Collapse all nodes |
| `f` / `F` | Open flags modal for current node |
| `R` | Re-discover / refresh children of selected node |
| `S` | Toggle section headers (Sub commands, Flags, Inherited flags) |
| `T` | Cycle display style (default → columns → compact → graph) |

#### Building Commands

| Key | Action |
|-----|--------|
| `Enter` | On a command: set it in the preview. On a flag: add it. On a positional: open input prompt. |
| `f` | Open flag picker — browse all flags for the current command with search |
| `Backspace` | Remove last token from the preview |
| `Ctrl+K` | Clear the entire preview bar |
| `Ctrl+E` | **Copy** the assembled command to your clipboard, or **run** it (confirmation prompt) |
| `Ctrl+R` | Recent commands you copied or ran for this CLI; `Enter` loads one into the preview (see [`treemand history`](../commands/history/)) |
| `Ctrl+O` | Saved presets for this CLI: `Enter` loads one into the preview, `a` saves the assembled command under a name, `x` or `Delete` deletes one (see [`treemand presets`](../commands/presets/)) |
| `Esc` / `q` | Quit |

#### View Controls

| Key | Action |
|-----|--------|
| `H` / `Ctrl+P` | Toggle help pane (uppercase `H` only — lowercase `h` is Left navigation in vim mode) |
| `Tab` / `Shift+Tab` | Cycle pane focus forward / backward (tree → help → preview) |
| `?` | Show all key bindings in a scrollable overlay |
| `d` / `D` | Open docs URL in browser (if detected in help text; `d` conflicts with Right in WASD mode) |

#### Mouse

| Interaction | Action |
|-------------|--------|
| Click node | Select node |
| Click `▶`/`▼` | Toggle expand/collapse |
| Scroll | Scroll the focused pane |

### Flags Modal (`f`)

Press `f` on any command node to open an interactive flag selector:

- Browse all flags for the current command (own + inherited)
- Search by typing to filter the flag list
- Press `Enter` on a boolean flag to add it directly
- Press `Enter` on a value flag (e.g. `--message=<string>`) to open an input prompt
- Already-added flags are marked with a checkmark

### Positionals

When a command has positional arguments (e.g. `git remote add <name> <url>`),
navigate to the positional row and press `Enter` to open an input prompt.
The value is appended to the preview bar.

## Caching

Discovery results are cached in an SQLite database:

| Property | Value |
|----------|-------|
| Location | `~/.treemand/cache.db` |
| Invalidation | the CLI's executable changes (path, size or modification time) |
| TTL | 30 days (`cache_ttl_hours`; `0` = until the CLI changes) |
| TUI expansions | saved back to the entry on exit |
| Key | CLI binary fingerprint + strategies + depth/stub settings |
| Schema | `v8` |

```bash
treemand --no-cache docker           # skip the cache for this run
treemand --refresh docker            # re-crawl now and update the entry
treemand cache list                  # show cached CLIs
treemand cache clear git             # clear one entry
```

## Discovery Strategies

### `help` (default)

Recursively runs `<cli> --help` / `<cli> <subcmd> --help` to build the tree.
Falls back to `<cli> help <subcmd>`, man page lookup, and error output mining.

### `man`

Parses the `man` page for the CLI (if available) using `man <cli>` and stripping
groff formatting. Provides richer descriptions than `--help` for many Unix tools.

### `completions`

Runs `<cli> __complete ""` (Cobra's built-in completion protocol) to enumerate
top-level subcommands without executing `--help` for every node. Results are
stub nodes expanded lazily on demand. Falls back gracefully when the CLI does
not support `__complete`.

### `scripts`

Reads the shell completion script the CLI installed (fish, then zsh, then
bash) from the standard directories: `fish/vendor_completions.d`,
`zsh/site-functions` and `bash-completion/completions` under your data
directory, `$XDG_DATA_DIRS` and the Homebrew and system prefixes, plus
`bash_completion.d`. The script is only read, never sourced, and the CLI is
never run. fish scripts give nested subcommands and flags with descriptions;
zsh scripts give subcommands and root flags; bash scripts give the first
argument's words. Scripts that ask the CLI at completion time (cobra,
argcomplete) hold nothing to read. Put it after `help` so help output stays
authoritative and the script fills the gaps.

```bash
treemand -s help git          # default
treemand -s man git           # man page parser
treemand -s help,man git      # combine strategies, merge results
treemand -s help,scripts ruff  # fill gaps from the installed completion script
```

## Configuration

treemand reads `~/.config/treemand/config.yaml` or `~/.treemand/config.yaml`:

```yaml
icons: ascii          # unicode (default) | ascii | nerd
desc_line_length: 80  # max chars before description is truncated
stub_threshold: 150   # subcommand count before switching to stub nodes

colors:
  subcmd: "#5EA4F5"
  flag: "#50FA7B"
```

Precedence: **CLI flags > environment variables > config file > defaults**.

### Overrides

A file at `~/.config/treemand/overrides/<cli>.yaml` corrects how treemand reads
one CLI: it replaces descriptions, adds commands and flags, and hides commands
or flags, with no new release. See
[overrides](../commands/config/#overrides-for-clis-treemand-misreads) for the
format.

### Color Scheme

| Element | Default |
|---------|---------|
| Base command | bold white `#FFFFFF` |
| Subcommand | blue `#5EA4F5` |
| Flag (bool) | green `#50FA7B` |
| Flag (string) | cyan `#8BE9FD` |
| Flag (int) | orange `#FFB86C` |
| Flag (other) | purple `#BD93F9` |
| Positional | yellow `#F1FA8C` |
| Invalid/error | red `#FF5555` |
| Selected bg | cyan `#00BFFF` |
| Selected text | black `#000000` |

## Self-Dogfooding

```bash
treemand treemand          # explore treemand's own command tree
treemand -i treemand       # interactively explore treemand itself
```

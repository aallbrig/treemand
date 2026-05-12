# treemand Features

Canonical list of features. Each feature maps to a demo segment in the
[CLI video demo system](design/20260329_144628_UTC-cli-video-demo.md).

## Static Output

### 1. Basic Tree Output
Discover any CLI's command hierarchy and render it as a colored ASCII tree.
```bash
treemand git
treemand --depth=2 git
```

### 2. Tree Display Styles
Four rendering styles, switchable via `--tree-style` or `T` key in TUI:
- **default** — icon-prefixed tree with inline flag pills
- **columns** — aligned name/description table
- **compact** — no icons, no flags, maximum density
- **graph** — classic `├──`/`└──` connectors
```bash
treemand --tree-style=graph --depth=1 git
treemand --tree-style=columns --depth=1 git
```

### 3. Filter & Exclude
Regex-based filtering to show or hide specific nodes.
```bash
treemand --filter=remote git
treemand --exclude=help git
```

### 4. JSON & YAML Output
Machine-readable output for scripting, diffing, and tooling integration.
```bash
treemand --output=json git | head -20
treemand --output=yaml git | head -20
```

### 5. Icon Presets
Three icon sets: `unicode` (default), `ascii` (safe for all terminals), `nerd`
(requires patched font).
```bash
treemand --icons=ascii --depth=1 git
treemand --icons=nerd --depth=1 git
```

### 6. Commands-Only Mode
Hide flags and positional arguments to show only the subcommand hierarchy.
```bash
treemand --commands-only --depth=2 git
```

## Cache Management

### 7. Cache List & Clear
Discovered trees are cached in SQLite. Manage cached entries:
```bash
treemand cache list
treemand cache clear git
treemand cache clear
```

## Configuration

### 8. Config Subcommand
Full configuration management via subcommands:
```bash
treemand config view
treemand config set icons nerd
treemand config validate
treemand config init
treemand config path
treemand config edit
```

See [features/config.md](features/config.md) for full documentation.

## Interactive TUI

### 9. Interactive TUI Mode
Keyboard-driven TUI to explore commands, pick flags, and build CLI invocations:
- Navigate with arrows or vim keys (`j`/`k`/`h`/`l`) or WASD; cycle scheme with `Ctrl+S`
- Expand/collapse nodes (`→`/`←`) and entire subtrees (`Shift+→`/`Shift+←`)
- Expand all / collapse all with `e` / `E`
- Jump to top / bottom with `gg` / `G`
- Fuzzy filter with `/`; cycle matches with `n` / `N`
- Flag picker modal (`f`/`F`)
- Live preview bar showing assembled command
- Clear preview bar with `Ctrl+K`
- Execute or copy built command (`Ctrl+E`)
- Re-discover / refresh selected node's children with `R`
- Toggle help pane with `H` or `Ctrl+P` (uppercase only — lowercase `h` is Left in vim mode)
- Toggle section headers with `S`
- Display style cycling with `T`
- Cycle pane focus with `Tab` / `Shift+Tab`
- Open docs URL in browser with `d` / `D`
- Show all key bindings with `?` (scrollable overlay)
- Mouse support (click, scroll)
- `⚠` indicator on nodes where discovery partially failed
```bash
treemand -i git
```

### 13. Discovery Progress Spinner
A braille-frame spinner is shown on stderr while discovery runs (TTY only —
no output when piped or in CI).

### 14. Discovery Error Indicator
Nodes whose children could not be fully discovered display a `⚠` prefix (styled
with `colors.invalid`) so failures are visible without expanding the node.

## Misc

### 10. Self-Introspection
treemand can analyze its own command tree (dogfooding).
```bash
treemand treemand
```

### 11. Shell Completion
Generate completion scripts for bash, zsh, fish, or powershell.
```bash
treemand completion bash
source <(treemand completion zsh)
```

### 12. Version Info
Print version, git commit, and build date.
```bash
treemand version
```

## Global Flags

These flags apply to the root command and control discovery behavior:

| Flag | Description |
|------|-------------|
| `--depth=N` | Limit tree recursion depth (default 3; -1 = unlimited) |
| `--filter=<regex>` | Show only matching nodes |
| `--exclude=<regex>` | Hide matching nodes |
| `--commands-only` | Hide flags and positionals |
| `--full-path` | Show full command paths |
| `--output=<format>` | Output format: text, json, yaml |
| `--tree-style=<style>` | Tree style: default, columns, compact, graph |
| `--icons=<preset>` | Icon set: unicode, ascii, nerd |
| `--strategy=<list>` | Discovery strategies: help, completions, man |
| `--no-color` | Disable colored output |
| `--no-cache` | Bypass discovery cache |
| `--timeout=<secs>` | Discovery timeout (default 30) |
| `--debug` | Enable debug logging |

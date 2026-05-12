---
title: "Tree Output"
weight: 1
---

# Tree Output

`treemand <cli>` discovers any CLI's full command hierarchy and renders it as a
color-coded tree. Point it at anything: `git`, `kubectl`, `docker`, `aws`, your
own tools.

<img src="/treemand/demos/cmd_tree.gif" alt="treemand tree demo" width="100%">

## Basic usage

```bash
treemand git                        # full tree at default depth (3)
treemand --depth=2 kubectl          # limit recursion depth
treemand --filter=remote git        # only show nodes matching pattern
treemand --exclude=help git         # hide nodes matching pattern
treemand --commands-only kubectl    # subcommands only — no flags or positionals
```

## Display styles

Four styles, switchable with `--tree-style` or **T** inside the TUI:

```bash
treemand --tree-style=default git   # ▼/▶/• icons with inline flag pills
treemand --tree-style=columns git   # name · description alignment
treemand --tree-style=compact git   # maximum density — names only
treemand --tree-style=graph git     # ├──/└── connectors like the `tree` command
```

## Icon presets

```bash
treemand --icons=unicode git        # default (▼ ▶ •)
treemand --icons=ascii git          # safe for all terminals (v > -)
treemand --icons=nerd git           # Nerd Font glyphs (requires patched font)
```

## Useful flags

| Flag | Description |
|------|-------------|
| `--depth=N` | Limit recursion depth (default 3; -1 = unlimited) |
| `--filter=<pattern>` | Show only nodes whose name matches |
| `--exclude=<pattern>` | Hide nodes whose name matches |
| `--commands-only` | Hide flags and positional arguments |
| `--full-path` | Show full command paths instead of just names |
| `--no-color` | Disable colored output |
| `--no-cache` | Skip the discovery cache for this run |
| `--timeout=N` | Discovery timeout in seconds (default 30) |
| `--strategy=<list>` | Discovery strategies: `help` (default), `man`, `completions` |

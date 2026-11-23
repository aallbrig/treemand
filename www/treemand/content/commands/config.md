---
title: "config"
description: "treemand config: view and edit the config file, icons, colors, strategies and defaults."
weight: 5
---

# `treemand config`

Manage treemand's optional YAML configuration file without opening it manually.

{{< demo src="demos/cmd_config.gif" alt="treemand config demo" width="900" height="420" >}}

## Commands

```bash
treemand config view               # show merged config + file path
treemand config validate           # check for errors or unknown keys
treemand config validate --strict  # treat warnings as errors
treemand config set <key> <value>  # write a value to the config file
treemand config init               # create a commented default config
treemand config init --force       # overwrite an existing config
treemand config path               # print the config file path
treemand config edit               # open config in $EDITOR
```

## Config file locations

treemand searches for the config file in this order:

1. `$XDG_CONFIG_HOME/treemand/config.yaml` (typically `~/.config/treemand/`)
2. `$HOME/.treemand/config.yaml`

## Precedence

**CLI flags > environment variables (`TREEMAND_*`) > config file > built-in defaults**

## Available keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `icons` | string | `unicode` | Icon preset: `unicode`, `ascii`, `nerd` |
| `desc_line_length` | int | `80` | Max description chars before truncation |
| `stub_threshold` | int | `150` | Subcommand count before switching to stub nodes |
| `tree_style` | string | `default` | TUI tree style: `default`, `columns`, `compact`, `graph` |
| `no_color` | bool | `false` | Disable colored output |
| `depth` | int | `3` | Max tree depth (default 3; -1 = unlimited) |
| `no_cache` | bool | `false` | Disable discovery cache |
| `strategies` | string | `help` | Comma-separated discovery strategies |
| `colors.base` | hex | `#FFFFFF` | Root command color |
| `colors.subcmd` | hex | `#5EA4F5` | Subcommand color |
| `colors.flag` | hex | `#50FA7B` | Flag color (fallback) |
| `colors.selected` | hex | `#00BFFF` | TUI selection background |

## Overrides for CLIs treemand misreads

When treemand reads a CLI's help wrongly, you can correct the tree yourself
without waiting for a release. Put a YAML file named after the CLI in
`overrides/` beside the config file, for example
`~/.config/treemand/overrides/mytool.yaml`:

```yaml
commands:
  "":                    # the CLI itself
    description: My tool does things
    hide_flags: [--debug-internal]
  remote add:            # a command path below the CLI
    description: Add a remote
    flags:
      - name: --track    # added, or updates the flag of the same name
        short: -t
        value_type: string
        description: Track one branch
  remote prune:
    hide: true           # removed from the tree
  config set:            # not in the tree: added, with its parents
    description: Set a value
```

Each key under `commands` is a command path without the CLI's name (`""` is
the CLI itself). A command can set:

| Key | Effect |
|-----|--------|
| `description` | Replaces the description |
| `flags` | Adds each flag, or updates the fields it sets on an existing flag of that `name` (`short`, `value_type`, `description`) |
| `hide_flags` | Removes flags by name |
| `hide` | Removes the command and everything below it |

A command the tree lacks is added, and treemand never runs it to read its
help. The override applies to every tree treemand shows for that CLI: text and
JSON output, the TUI, `--stdin`, `treemand mcp` and `treemand doctor`. Editing
the file takes effect on the next run. An unknown key or invalid YAML is an
error that names the file, so a typo never goes unnoticed.

An override is also the most useful thing to attach to a
[bug report](https://github.com/aallbrig/treemand/issues): it shows exactly
what treemand got wrong.

## Example config

```yaml
# ~/.config/treemand/config.yaml
icons: nerd
depth: 5
stub_threshold: 100

colors:
  subcmd: "#5EA4F5"
  flag: "#50FA7B"
  selected: "#00BFFF"
```

Run `treemand config validate` after editing to catch typos before they affect behavior.

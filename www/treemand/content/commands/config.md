---
title: "config"
weight: 5
---

# `treemand config`

Manage treemand's optional YAML configuration file without opening it manually.

<img src="/treemand/demos/cmd_config.gif" alt="treemand config demo" width="100%">

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
| `stub_threshold` | int | `50` | Subcommand count before switching to stub nodes |
| `tree_style` | string | `default` | TUI tree style: `default`, `columns`, `compact`, `graph` |
| `no_color` | bool | `false` | Disable colored output |
| `depth` | int | `3` | Max tree depth (default 3; -1 = unlimited) |
| `no_cache` | bool | `false` | Disable discovery cache |
| `strategies` | string | `help` | Comma-separated discovery strategies |
| `colors.base` | hex | `#FFFFFF` | Root command color |
| `colors.subcmd` | hex | `#5EA4F5` | Subcommand color |
| `colors.flag` | hex | `#50FA7B` | Flag color (fallback) |
| `colors.selected` | hex | `#00BFFF` | TUI selection background |

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

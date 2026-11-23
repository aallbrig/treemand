---
title: "presets"
description: "treemand presets: save assembled commands under a name in the interactive TUI, list them, and reopen the TUI with one loaded using --preset."
weight: 11
---

# `treemand presets`

A saved preset is a command you built in the [interactive TUI](../interactive/)
and saved under a short name, so you can come back to it. Each CLI has its own
presets.

- In the TUI, press `Ctrl+O` to open the saved presets for the CLI you are
  exploring. `a` saves the assembled command under a name you type, `Enter`
  loads the selected preset into the preview bar, and `x` deletes it.
- `treemand --preset <name> <cli>` opens the TUI with that preset already
  loaded.
- `treemand presets <cli>` lists them, one `name  command` per line:

```sh
treemand presets git
treemand --preset amend git
treemand presets git --output json
```

Names use letters, digits, `.`, `_` and `-`. Saving under a name that exists
replaces that preset.

## Where they live

Presets are a plain-text file per CLI in the config directory:
`~/.config/treemand/presets/<cli>.txt` (or `$XDG_CONFIG_HOME/treemand/presets/`;
`%AppData%\treemand\presets\` on Windows). Each line is a name, then the
command. Lines starting with `#` are comments. Edit the file by hand if you
like: the TUI keeps your comments when it saves.

```text
# git presets
amend   git commit --amend --no-edit
graph   git log --oneline --graph --all
```

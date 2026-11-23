---
title: "history"
description: "treemand history: list the commands you copied or ran from the interactive TUI for a CLI, and reload them with Ctrl+R."
weight: 10
---

# `treemand history`

treemand remembers the commands you copy or run from the
[interactive TUI](../interactive/) (`Ctrl+E`), one list per CLI. Use them two
ways:

- In the TUI, press `Ctrl+R` to see the recent commands for the CLI you are
  exploring. `Enter` loads one into the preview bar, ready to edit, copy or
  run again.
- From the shell, `treemand history <cli>` prints them, most recent first, one
  per line:

```sh
treemand history git
treemand history kubectl --output json   # with use counts and timestamps
```

## Where it lives

History stays on your machine. Each CLI has a small JSON file under
`~/.treemand/history/` (or `$TREEMAND_CACHE_DIR/history/`), capped at the 100
most recently used commands. A command used again moves to the top and its
count goes up. Nothing is sent anywhere.

## Turning it off

Set `no_history: true` in the [config file](../config/), or
`TREEMAND_NO_HISTORY=1` in the environment. treemand then records nothing and
`Ctrl+R` says history is off. To forget what is already there, delete the
`history` directory.

---
title: "doctor"
description: "treemand doctor: see how treemand parsed a CLI, what the tree is missing and which help probes were slow or failed."
weight: 9
---

# `treemand doctor`

When a tree looks wrong or a run is slow, `treemand doctor <cli>` shows what
happened. It discovers the CLI afresh (the cache is neither read nor written)
and reports:

- the treemand version, platform, binary, strategy and the help form the CLI
  answered (`--help` or `-h`)
- how many commands were found, and which have no description, no flags, or
  were left undiscovered (stubs)
- commands whose help could not be read, and why
- the [override file](../config/#overrides-for-clis-treemand-misreads), if you
  have one for the CLI, and how many changes it made (the counts above include
  them)
- every help probe by outcome (ok, non-zero exit, timed out, cancelled, failed,
  truncated) and the five slowest

## Usage

```bash
treemand doctor git
```

```
treemand doctor: git

  treemand               v0.14.0 (darwin/arm64)
  binary                 /opt/homebrew/bin/git
  strategy               help
  help form              --help
  depth                  3
  elapsed                1.928s

Tree
  nodes                  25
  without description    0
  without flags          0
  undiscovered stubs     0
  discovery errors       0

Probes
  run                    25 (ok 25, non-zero exit 0, timed out 0, cancelled 0, failed 0, truncated 0)
  slowest probes
      1.476s  ok         git reset --help
      1.303s  ok         git status --help
       463ms  ok         git branch --help
```

The text report is plain and meant to be pasted into a
[bug report](https://github.com/aallbrig/treemand/issues). A binary under your
home directory is shown as `~/...`. Lists longer than ten commands are cut short;
`--output=json` (or `yaml`) prints the full report for scripts:

```bash
treemand doctor --output=json kubectl | jq '.probes, .slowest_probes'
```

`doctor` takes the same discovery flags as a tree run (`--depth`, `--strategy`,
`--timeout`, `--jobs`). It exits 0 whenever it prints a report, even when some
commands could not be read; a CLI that is not on your PATH is still an error.

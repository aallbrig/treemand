---
name: treemand-safe-real-cli-run
description: Guardrails for running treemand (or its perf, corpus or capture scripts) against real installed CLIs. Use before any real-CLI measurement, A/B comparison, fixture capture, or change to which arguments discovery passes to a CLI.
---

# Running treemand against real CLIs

Discovery executes the user's tools with arguments. A stray argument is an action: `git branch help` created a branch, `docker context use help` switched the user's Docker context (#149), `touch list-commands` created a file (#153). This machine also has an authenticated `gh` and a kubectl pointed at a real k3s cluster. Treat every real-CLI run as a run with side effects.

## Where to run

- **Prefer the zoo**: `task zoo:smoke` and `task zoo:capture` run inside the disposable `test/cli-zoo` container. Use it for any sweep across many CLIs.
- **On the host, from an empty directory** made with `mktemp -d`, never the repo, with `TREEMAND_CACHE_DIR` pointed at a throwaway directory too. `scripts/loop/perf.py` and `scripts/capture_corpus.sh` already do this; a new script must as well.

## Comparing against main

`task ab -- <cli>...` (`--base REF` to compare another ref) builds origin/main's treemand and the working tree's, runs full discovery of each CLI from an empty scratch directory, and compares the trees: nodes, flags and descriptions gained or lost. It also audits this machine before and after (git branches and status, Docker context, builders, volumes, networks, the scratch directory) and **fails on any side effect**. If it fails, restore what the run created before anything else (timestamps show what is yours).

For speed, wall-clock numbers alone prove nothing: other projects load this host (`uptime` shows the load average on Linux and macOS). Alternate old and new binaries run by run, and prefer load-independent evidence: invocation counts from a logging fake CLI, CPU seconds (`python3 -I scripts/metrics/timed.py OUT CWD -- CMD...` writes wall, user, sys and max RSS on both platforms; GNU `/usr/bin/time -f` does not exist on macOS).

## Changing what discovery sends to a CLI

Help is asked with `--help` / `-h` first. Positional forms (`<cli> <sub> help`, `help <cli> <sub>`, `list-commands`) run only after the flag forms fail or when the CLI documents them. Any change to that ordering ships with a marker test: a fake CLI that touches a file if it is given the argument, asserted untouched (see `TestRunHelp_neverPassesHelpAsAnArgumentWhileAFlagFormWorks`).

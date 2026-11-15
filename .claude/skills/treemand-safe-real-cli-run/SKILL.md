---
name: treemand-safe-real-cli-run
description: Guardrails for running treemand (or its perf, corpus or capture scripts) against real installed CLIs. Use before any real-CLI measurement, A/B comparison, fixture capture, or change to which arguments discovery passes to a CLI.
---

# Running treemand against real CLIs

Discovery executes the user's tools with arguments. A stray argument is an action: `git branch help` created a branch, `docker context use help` switched the user's Docker context (#149), `touch list-commands` created a file (#153). This machine also has an authenticated `gh` and a kubectl pointed at a real k3s cluster. Treat every real-CLI run as a run with side effects.

## Where to run

- **Prefer the zoo**: `task zoo:smoke` and `task zoo:capture` run inside the disposable `test/cli-zoo` container. Use it for any sweep across many CLIs.
- **On the host, from an empty directory** made with `mktemp -d`, never the repo, with `TREEMAND_CACHE_DIR` pointed at a throwaway directory too. `scripts/loop/perf.py` and `scripts/capture_corpus.sh` already do this; a new script must as well.

## Measuring

- Compare against `main` **A/B, interleaved**, alternating old and new binaries run by run: other projects load this host (check `cat /proc/loadavg`), so a wall-clock number alone proves nothing. Build the old binary from `git archive main cli/treemand`.
- Prefer load-independent evidence: invocation counts from a logging fake CLI, CPU seconds (`/usr/bin/time -f "%e %U %S"`), and whole-tree equality (every path, description and flag) between the two binaries.

## Audit after every run

Snapshot before, compare after, and restore anything the run created (the timestamps show what is yours):

- `git branch --list` and `git status -s` in the repo
- `docker context show` (must be `default`), `docker buildx ls`, `docker volume ls`
- the scratch directory: it should still be empty

## Changing what discovery sends to a CLI

Help is asked with `--help` / `-h` first. Positional forms (`<cli> <sub> help`, `help <cli> <sub>`, `list-commands`) run only after the flag forms fail or when the CLI documents them. Any change to that ordering ships with a marker test: a fake CLI that touches a file if it is given the argument, asserted untouched (see `TestRunHelp_neverPassesHelpAsAnArgumentWhileAFlagFormWorks`).

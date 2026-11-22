---
name: treemand-cli-ux
description: Judge treemand's own command-line experience (help, error copy, exit codes, stdout vs stderr, color and TTY, --output, config, flag naming) against clig.dev and produce a ranked gap list. Use for CLI usability questions, a CLI UX review, or cli-ux findings.
---

# CLI UX

treemand's promise is "explore any CLI as a tree", so its own CLI is the first one a user explores: it must read as well as the trees it draws. Follow **treemand-measure** for running, recording and filing; this skill is the CLI review.

## 1. Measure

Run `task measure -- cli-ux --findings`. The probe (`scripts/metrics/probes/cli-ux.sh`) builds treemand, runs it from an empty directory with a throwaway HOME and cache, and checks one rule per `check.<rule>` metric (1 holds, 0 does not): help, version, unknown flag and subcommand errors and suggestions, usage exit code 2, no color or progress when piped, `NO_COLOR` and `--no-color` on a TTY (via `script(1)`), clean `--output=json`, the missing-CLI error, config discoverability, subcommand descriptions, and completion for bash, zsh and fish. `details[0]` holds each failing check with its evidence.

Done when you have the findings JSON. Each failing check is **evidence**: it files as a `[measure] cli-ux: check.<rule> = 0 (1)` issue, `area:cli`, `measure:finding`, `loop:ready`, per treemand-measure. Never re-judge a failing check by eye.

## 2. Judge

Read what the probe cannot score: `treemand --help`, every `treemand <sub> --help` (the probe's `details[1]` lists them), the error text in `cli/treemand/cmd/exitcodes.go` and `cmd/*.go`, and the output of `--output=json` against `--output=yaml` for one tree. Run treemand only on itself or under **treemand-safe-real-cli-run**.

Apply every rule below to every help page and every error path you read. A gap is a specific place where a rule fails, with the **evidence** (the command and the quoted text, or the `file:line`).

- **Help leads with examples**: every command's help shows usage, at least one runnable example, and the most common flags first. Long help that buries the one-line answer is a gap.
- **Help is precise**: each flag says what it does, its default once (cobra appends `(default …)`; a description that also states it says it twice), and its allowed values.
- **Errors are for humans**: one line saying what went wrong in the user's words, then one hint saying what to do. Library text (`strconv.ParseInt`, `accepts 2 arg(s)`) is a gap.
- **Suggest, don't guess**: a typo'd flag or subcommand names the closest match; a group command given an unknown subcommand is an error, not its help page with exit 0.
- **Exit codes mean something**: 0 success, 1 runtime failure, 2 usage mistake, the same on every command.
- **stdout is for data**: the tree, JSON or YAML goes to stdout; hints, warnings and progress go to stderr; piped output is plain.
- **Machine output is a contract**: `--output=json` and `--output=yaml` carry the same fields under the same names, stable across releases; scripts and agents are users (vision, "Who it's for").
- **Flags follow convention**: standard names (`-h`, `--help`, `--version`, `-o/--output`, `--no-color`, `-q`, `--debug`), short forms for the common ones, kebab-case, no two flags for one idea.
- **Config is discoverable**: from `--help` a user finds the config file, where it lives, its precedence (flags > env > file), and how to see the effective value.
- **Tree-first**: what treemand prints about itself reads like a good tree (`treemand treemand` shows treemand's real commands and flags, nothing spurious). A gap here is both a CLI and a parser gap.

Done when every rule has been applied to every help page and error path, and each gap has evidence.

## 3. Rank

Rank gaps by **reach** (how many users or scripts hit it: root help and the default path beat a rarely used subcommand), then **harm** (breaks a script or misleads, over merely untidy), then **cost** (a one-line copy change before a redesign). Write one numbered list, highest first; each entry is the rule, the evidence, and the smallest fix.

The ranked list is **opinion** until the maintainer triages it: post it as one comment on the tracking issue, "for maintainer triage". File issues only for failing probe checks (step 1).

## Prove it

A fix re-runs `task measure -- cli-ux`: its check reads **improved**, and the PR records it with `--record`. A judgement fix quotes the before and after text.

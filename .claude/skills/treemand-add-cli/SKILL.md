---
name: treemand-add-cli
description: Add a CLI to treemand's test corpus with ground truth. Use when asked to support, test or cover a new CLI, or when a user reports treemand mis-parsing a specific tool.
---

# Adding a CLI to the corpus

The corpus (`cli/treemand/discovery/testdata/corpus/`) is how treemand proves it parses a CLI. Each CLI there has a root-help fixture, a regression baseline entry, and ground truth in `expect.json`. Read `docs/research/20261115_194227_UTC_cli_cross_section.md` for the help-text families and why the sample is chosen by family.

Follow **treemand-safe-real-cli-run** for every step that runs the CLI.

1. **Classify the family** from the CLI's real `--help` output (header shape, section names, flag layout), not from what you assume its framework is. Done when you can name the family from the table in the research doc, or name it as new.
2. **Capture the fixture.** Installed here: `task corpus:capture -- <cli>`. A second help text of a tool already in the corpus (BSD `tar` next to GNU `tar`) is a variant, `<cli>@<variant>` (`tar@bsd`): capture it on a host where that CLI is that variant. Not installed: add it to `test/cli-zoo/Dockerfile` (pinned version at least two weeks old, a SHA-256 per architecture, amd64 and arm64, for raw downloads), add it to `ZOO` in `test/cli-zoo/run.sh`, then `task zoo:capture`. Done when `<cli>.txt` is the CLI's actual help: read it; an error message or a different command's help is a bug to fix first (see #164).
3. **Write ground truth** in `expect.json` from the fixture and the tool's own docs: `subcommands_include` (3–4 real commands), `max_subcommands: 0` for a CLI with none, `flags_include` (3–4 real flags, long form), `description_include` when the description matters. Ground truth is what the CLI offers, never what the parser printed. Done when every value is checked against the fixture text.
4. **Run** `go test ./discovery -run TestCorpus_` in `cli/treemand` (`task corpus:show -- <cli>` shows what the parser extracted). For each mismatch, confirm in the fixture that the expectation is right, then mark it: add the issue tracking that gap to the CLI's `gaps`, filing a new `area:discovery` issue when no open one fits. Done when the test passes and every gap names an issue.
5. **Baseline and README**: `task corpus:update-baseline` (the diff must only add the new CLI), and list the CLI under its family in README's `## Tested CLIs`.
6. Ship it: `task ship:start -- <branch>` before you start, `task ship:pr -- …` to commit, open the PR and merge.

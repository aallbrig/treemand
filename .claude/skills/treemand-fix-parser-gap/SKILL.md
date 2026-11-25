---
name: treemand-fix-parser-gap
description: Fix a treemand parser gap tracked in the corpus (an issue named in testdata/corpus/expect.json `gaps`). Use when working an area:discovery parser issue or making a CLI's ground truth pass.
---

# Fixing a parser gap

A gap is a mismatch between a CLI's ground truth (`cli/treemand/discovery/testdata/corpus/expect.json`) and what the parser extracts from its fixture, marked with the issue that tracks it. The fix is done when the marker is gone and the corpus enforces it.

When the parser's output is a surprise, use LSP references and `dlv` breakpoints rather than prints: [docs/agents/dev-tooling.md](../../../docs/agents/dev-tooling.md).

1. **Go red.** List every CLI whose `gaps` names the issue (`grep '"gaps": \[.*<N>' expect.json`), see what the parser makes of each with `task corpus:show -- <cli>`, and write a focused unit test in `discovery/` with the smallest help excerpt that shows the shape, copied from the fixture. Done when that test fails for the reason the issue describes.
2. **Fix the parser** (`discovery/parse.go`, `parse_flags.go`, `parse_text.go`) for the shape, not for one CLI: other families print similar text, so state the rule in a comment.
3. **Check the whole corpus.** `go test ./discovery` must pass: the baseline (`TestCorpus_noRegressionAgainstBaseline`) fails on any subcommand, flag or description lost in any corpus CLI. Then `task corpus:diff` lists every subcommand, flag and description gained or lost against origin/main, with the fixture line it comes from. Done when every line of that diff is one you meant: a surprise gain is a change to inspect, and a loss is a bug unless it removes garbage.
4. **Remove the marker** from each CLI it fixes. `TestCorpus_matchesGroundTruth` fails while a fixed gap is still marked, and passes once it is gone; a CLI still failing for another reason keeps its other gaps. Then `task corpus:update-baseline` to lock in the gains (the diff may only increase numbers).
5. **Beyond root help**: the shape also appears in subcommand help, so run `task ab -- <cli>...` on a few real CLIs (git, kubectl, docker, gh, apt, systemctl) and `task zoo:smoke`. Done when no node or flag is lost and the side-effect audit is clean.
6. Ship with `task ship:pr -- …` (branch with `task ship:start -- <branch>` first); the PR body lists the CLIs fixed. Close the issue when no CLI still carries its marker.

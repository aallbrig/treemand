---
name: treemand-fix-parser-gap
description: Fix a treemand parser gap tracked in the corpus (an issue named in testdata/corpus/expect.json `gaps`). Use when working an area:discovery parser issue or making a CLI's ground truth pass.
---

# Fixing a parser gap

A gap is a mismatch between a CLI's ground truth (`cli/treemand/discovery/testdata/corpus/expect.json`) and what the parser extracts from its fixture, marked with the issue that tracks it. The fix is done when the marker is gone and the corpus enforces it.

1. **Go red.** List every CLI whose `gaps` names the issue (`grep '"gaps": \[.*<N>' expect.json`) and write a focused unit test in `discovery/` with the smallest help excerpt that shows the shape, copied from the fixture. Done when that test fails for the reason the issue describes.
2. **Fix the parser** (`discovery/help.go` and friends) for the shape, not for one CLI: other families print similar text, so state the rule in a comment.
3. **Check the whole corpus.** `go test ./discovery` must pass: the baseline (`TestCorpus_noRegressionAgainstBaseline`) fails on any subcommand, flag or description lost in any of the 63 CLIs. Done when it is green and the improvement log lines name only the CLIs you meant to improve; a surprise improvement is a change to inspect.
4. **Remove the marker** from each CLI it fixes. `TestCorpus_matchesGroundTruth` fails while a fixed gap is still marked, and passes once it is gone; a CLI still failing for another reason keeps its other gaps. Then `task corpus:update-baseline` to lock in the gains (the diff may only increase numbers).
5. **Beyond root help**: when the shape also appears in subcommand help, run `task zoo:smoke` (and the real CLI, per **treemand-safe-real-cli-run**) to see the fix through full discovery.
6. Ship through a PR whose body lists the CLIs fixed; close the issue when no CLI still carries its marker.

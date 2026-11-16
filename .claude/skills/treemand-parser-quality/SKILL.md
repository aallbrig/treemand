---
name: treemand-parser-quality
description: Measure how well treemand parses real CLI help (corpus ground truth, the cli-zoo sweep) and turn misses into fixes. Use for parser quality questions or parser and zoo findings.
---

# Parser quality

Follow **treemand-measure** for running, recording and filing; this skill is the parser playbook.

**Measure**:

- `task measure -- parser` (seconds, no CLIs run): `truth_pass_pct` (corpus CLIs meeting `testdata/corpus/expect.json`), `unexpected_misses` (misses with no gap marker), `clis_with_gaps`, `subcommands_total`, `flags_total`, `described_pct`. `details` lists every miss.
- `task measure -- zoo` (heavy, a few minutes, Docker): the sweep of every executable in the cli-zoo container (`sweep_errors`, `sweep_timeouts`, `sweep_panics`) and full discovery of each zoo CLI (`nodes.*`, `flags.*`).

**Act**:

- **unexpected_misses > 0** is a regression: `task corpus:diff` against the last good ref finds the change.
- **A gap-marked miss** is worked with **treemand-fix-parser-gap**.
- **described_pct**: inspect the undescribed CLIs with `task corpus:show -- <cli>`. A CLI with prose but no description is a gap; one with none should assert `description_exclude` so garbage never appears.
- **Sweep panics or timeouts** are reliability bugs: reproduce in the container (`docker run --rm --entrypoint sh treemand-cli-zoo -c 'treemand --depth=1 <prog>'`), then add the shape to `task chaos` or a unit test.
- **A CLI to add** comes from **treemand-add-cli**.

---
name: treemand-test-effectiveness
description: Measure how well treemand's tests catch bugs (mutation score of the core packages, coverage per package, flaky tests) and turn surviving mutants into missing assertions. Use for test-quality questions, a flaky test, or test-effectiveness findings.
---

# Test effectiveness

Follow **treemand-measure** for running, recording and filing; this skill is the test-quality playbook.

## Measure

`task test:effectiveness -- --findings` (the same as `task measure -- test-effectiveness --findings`). About ten minutes on a dev host; it runs weekly in `measure-weekly.yml`, never per PR. It needs gremlins at the version the probe pins: `go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`. Without it, mutation is reported **skipped** and only coverage and flakes are measured. The probe's docstring (`scripts/metrics/test_effectiveness.py`) defines every metric; `scripts/metrics/rules.json` holds the rules.

- **mutation_score.<pkg>** (`discovery`, `render`, `models`, `config`): of a stable sample of at most 60 covered mutants, the percent the tests detect. Ratchet with a 5-point tolerance: the sample is fixed while the code is unchanged, but timeouts under load move it a little. `mutants_lived.<pkg>` and `mutants_not_covered.<pkg>` carry the counts.
- **coverage_pct.<pkg>**: statement coverage, host-bound (cmd measures lower on CI). The hard floor is `scripts/coverage_floor.tsv` in `task ci`; this metric is its history.
- **flaky_tests** (budget 0, ratchet): tests whose result differs across `go test -count=3`. **failing_tests**: failed every run.

Run it on a quiet machine when you can. A loaded host makes timing-sensitive tests flake (`TestRunHelp_fastPreferredFormStartsNoAlternatives`, #288); the probe skips the tests it found flaky or failing during mutation runs, and confirms every kill: the killing test, run alone, must fail with the mutant and pass without it. A kill that fails that check is judged again without the test and counted in the package's `unconfirmed_kills`. A test high in `top_killers` across unrelated files is a flake suspect, not a strong test.

## Act on survived mutants

A survivor (`details`, kind `survived`) is a change to the code that every test still passes: a behaviour nobody asserts. Read `at` (file:line:column), `source` and `mutated`, then decide:

1. **Missing assertion** (most survivors): name the input that tells the two versions apart and the observable result that differs. A survived `a > b` -> `a >= b` means no test asks the equal case; a survived `n + 1` -> `n - 1` means no test checks that value. The fix is one test that asserts it at the package's public boundary.
2. **Equivalent mutant**: both versions behave the same for every input (for example a capacity hint, `make([]T, 0, n*2)` -> `n/2`, or a boundary on a value that can never equal the bound). Note it on the issue and move on; never write a test that pins implementation detail to kill it.
3. **Dead or redundant code**: the branch can never matter. Delete it; that is the better fix.

Pick findings from the package with the lowest score first, and the survivors whose behaviour a user would notice (parsing, rendering, config values) before internal bookkeeping.

## File

Survivors are filed by name, not by metric: title `[measure] test-effectiveness: <pkg> survivor <file>:<line> (<mutator>): Assert <behaviour>`, labels `area:quality`, `measure:finding`, `loop:ready`. The body says what to assert: "add an assertion that <input> gives <result>; today `<source>` -> `<mutated>` passes every test", plus the probe command and the `survived` entry. Several survivors in one function are one issue. Deduplicate first against `[measure] test-effectiveness` titles and the coverage issues (`gh issue list --search "test coverage in:title" --state open`): a survivor on a line already covered by an open coverage issue goes there as a comment.

A **budget** or **regression** finding on `flaky_tests` names the test in `details`: search open issues for the test name and comment if it is tracked; otherwise file `Flaky test: <name> ...` with labels `bug`, `loop:ready` and the package's `area:`. Fix flakes by removing the dependence on time or ordering, never by retrying. A `mutation_score` regression means a recent change added untested behaviour: find the commit and file its survivors.

## Prove it

The PR that adds the assertion re-runs the probe: the survivor is gone from `details`, the package's score reads **improved** or holds, and the new value is recorded with `--record`.

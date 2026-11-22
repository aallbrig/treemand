---
name: treemand-fuzz-campaign
description: Fuzz every treemand parser hot path with native Go fuzzing (help text, flag rows, section headers, usage lines, config YAML), turn crashers into failing unit tests and fixes. Use for fuzz findings, a panic on odd help output or config, or when adding a parser that needs a Fuzz target.
---

# Fuzz campaign

Follow **treemand-measure** for running, recording and filing; this skill is the fuzzing playbook.

**Measure**: `task fuzz:campaign` (= `task measure -- fuzz`) runs `scripts/metrics/fuzz_campaign.py`: every `func Fuzz*` in `cli/treemand`, one `go test -fuzz` after another, `FUZZ_SECONDS` each (default 60). `task fuzz:smoke` is the same at 5s per target inside `task ci`: it fails (`--gate`) only when `crashers` rises above the last record, and it deletes the crasher files it finds (`FUZZ_KEEP_CRASHERS=0`), keeping their inputs in the output. The weekly `Measure (weekly)` workflow runs 60s per target and uploads any crasher with the findings.

Metrics:

- `crashers` (budget 0): targets that failed, on a new input or on a committed seed. Each one's `details.crasher` has `rerun`, `input_file`, `failure` and `input`.
- `new_coverage_per_minute`: inputs that reached new coverage, per minute fuzzed. Informational: it falls as the corpus saturates; a jump means new code to explore.
- `corpus_size`: generated entries in `$GOCACHE/fuzz` plus committed seeds. Informational and host-bound: `go clean -fuzzcache` resets it.

Targets: `discovery` `FuzzParseHelpOutput`, `FuzzSanitizeHelp` (whole help text), `FuzzParseFlag`, `FuzzDetectSection`, `FuzzParsePositionals` (single lines; `fuzz_internal_test.go`); `config` `FuzzValidateYAML`, `FuzzLoadConfig` (viper load onto the defaults). Each asserts properties, not exact output: no panic, deterministic, and names that are well formed.

## Corpus

The generated corpus lives in `$GOCACHE/fuzz/<import path>/<target>` and persists on a host, so each campaign continues the last. `<package>/testdata/fuzz/<target>/` is the **committed seed corpus**, replayed by every `go test`. Go writes a failing input there. Commit it **only with its fix**: a committed seed that still fails breaks `task ci` for everyone. A campaign that finds a crasher leaves it there: move it out of the tree (or file it, below) before you ship unrelated work.

## Act on a crasher

1. **Reproduce**: run the finding's `rerun` from `cli/treemand` (`go test -run=FuzzParsePositionals/26f3a56c61e90ed2 ./discovery`). It must fail the same way.
2. **Minimize**: Go has already shrunk the input before writing it (for up to `-fuzzminimizetime`, default 60s). If it is still large, shrink it by hand: delete bytes from the `string(...)`/`[]byte(...)` line of the `go test fuzz v1` file while the re-run still fails with the same message. Decode the input (`strconv.Unquote` of the literal) to see the real bytes.
3. **File** (when not fixing now): one issue per root cause, titled `[measure] fuzz: crashers = N (<= 0)` or naming the target, labels `loop:ready`, `measure:finding` and the package's area (`area:discovery` for `discovery`, `area:cli` for `config`). The body carries the target, the input file's contents, the failure and the stack frame in our code. Do not commit the seed with the issue.
4. **Failing unit test first**: write an ordinary test next to the code with the decoded input and the behaviour you expect (`TestParsePositionalsInvalidUTF8`: no panic, no positional). It is red.
5. **Fix** the root cause, not the input: for example, index the string you searched, not a transformed copy of it. Keep the fix within the function's contract; `task corpus:diff` must show no parse change on real help.
6. **Commit the seed** (`testdata/fuzz/<target>/<id>`) with the test and the fix, so the fuzzer replays it forever. Re-run the probe; `crashers` reads **improved**, and record it.

## Add a target

A new parser, or one that panicked through a path no target reaches, gets a `Fuzz*` in its package. Seed it from the committed fixtures (`testdata/*.txt`, `testdata/corpus/*.txt`) and a few hand-written edge shapes; assert invariants a correct parser always keeps (determinism, no empty or malformed names, no line breaks in single-line fields) and skip inputs the caller can never pass (a line with a newline). Every seed must pass under `go test` before you ship. The probe picks a new target up on its own.

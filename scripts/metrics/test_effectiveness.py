#!/usr/bin/env python3
"""test_effectiveness.py: the test-effectiveness probe's measurements.

    test_effectiveness.py [--module DIR]

Prints one probe result (see scripts/metrics/measure.py). DIR (default:
cli/treemand) is the Go module measured. Three questions, one metric family each:

- mutation_score.<pkg>: of the sampled, covered mutants of a core package
  (MUTATION_PACKAGES, the package directory with its sub-packages), the percent
  the tests detect: (killed + timed out) / (killed + timed out + lived). A mutant
  that does not compile (not viable) is left out, as are mutants on lines no test
  runs (counted in mutants_not_covered.<pkg>; coverage_pct is their measure).
  mutants_lived.<pkg> and mutants_tested.<pkg> carry the counts behind the score.
- coverage_pct.<pkg>: statement coverage per package, from the same
  `go test -count=1 -cover ./...` run the coverage floor uses
  (scripts/check_coverage.sh); <pkg> is the import path inside the module.
- flaky_tests: tests (in FLAKE_PACKAGES) whose result differs across
  `go test -count=3` runs; failing_tests: tests that failed in every run.
  A failing run is reported, never fatal: the probe exits 0.

Mutation: gremlins (pinned GREMLINS_VERSION) enumerates each package's mutants
with `gremlins unleash --dry-run`, which also marks the ones on uncovered lines.
gremlins has no sampling, and the discovery tests take about 25 seconds a run,
so the probe applies the mutants itself: a stable sample of at most SAMPLE
covered mutants per package (ordered by a hash of the file, mutator and source
line, so the same mutants are tested run after run while the code is unchanged),
each swapped in with `go test -overlay` and run in parallel. The tests found
flaky or failing are skipped during mutation runs (and gremlins' coverage run),
so a flake never counts as a kill and a failing test never blocks a package. A
kill counts when the killing test, run alone, fails with the mutant and passes
without it; otherwise the mutant is judged again without that test
(unconfirmed_kills counts these).

Details: one "mutation" entry per package (the counts, the sample size, and
top_killers: the tests that killed most mutants), one "survived" entry per lived
mutant (file:line:column, mutator, the source line and its mutation), one "flaky"
or "failing" entry per test, and "skipped" reasons.

Environment: TEST_EFFECTIVENESS_SAMPLE (mutants per package, default 60),
TEST_EFFECTIVENESS_WORKERS (parallel mutant runs, default half the CPUs, at most
6), TEST_EFFECTIVENESS_PACKAGES and TEST_EFFECTIVENESS_FLAKE_PACKAGES (space
separated directories, overriding the defaults), GREMLINS (the gremlins binary;
default: gremlins on PATH, then $(go env GOPATH)/bin/gremlins).
"""

import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
from collections import Counter, defaultdict
from concurrent.futures import ThreadPoolExecutor
from functools import partial
from pathlib import Path
from typing import Any

type Obj = dict[str, Any]

REPO = Path(__file__).resolve().parents[2]
GREMLINS_VERSION = "v0.6.0"
GREMLINS_INSTALL = f"go install github.com/go-gremlins/gremlins/cmd/gremlins@{GREMLINS_VERSION}"
MUTATION_PACKAGES = ("discovery", "render", "models", "config")
# The packages whose tests are fast enough to run three times. cmd and e2e
# build binaries and run real CLIs; they are covered by the coverage run only.
FLAKE_PACKAGES = ("cache", "config", "discovery", "fuzzy", "models", "presets", "render", "tui")
FLAKE_RUNS = 3
SAMPLE = int(os.environ.get("TEST_EFFECTIVENESS_SAMPLE", "60"))
WORKERS = int(os.environ.get("TEST_EFFECTIVENESS_WORKERS", str(min(6, max(1, (os.cpu_count() or 2) // 2)))))
# A mutant run may take this many times the package's clean test run (plus
# MIN_TIMEOUT for compiling) before it counts as timed out (detected).
TIMEOUT_FACTOR = 4
MIN_TIMEOUT = 60.0
DRY_RUN_TIMEOUT = 900
DRY_RUN_ATTEMPTS = 3
TEST_TIMEOUT = 1200

# gremlins' mutators (internal/engine/mappings.go at GREMLINS_VERSION): the
# operator found at a mutant's position -> what that mutator turns it into.
MUTATIONS: dict[str, dict[str, str]] = {
    "ARITHMETIC_BASE": {"+": "-", "*": "/", "/": "*", "%": "*", "-": "+"},
    "CONDITIONALS_BOUNDARY": {">=": ">", ">": ">=", "<=": "<", "<": "<="},
    "CONDITIONALS_NEGATION": {"==": "!=", ">=": "<", ">": "<=", "<=": ">", "<": ">=", "!=": "=="},
    "INCREMENT_DECREMENT": {"--": "++", "++": "--"},
    "INVERT_LOGICAL": {"&&": "||", "||": "&&"},
    "INVERT_NEGATIVES": {"-": "+"},
    "INVERT_LOOPCTRL": {"break": "continue", "continue": "break"},
}
FAILED_TEST = re.compile(r"^--- FAIL: (\S+)", re.MULTILINE)
COVERAGE_LINE = re.compile(r"^ok\s+(\S+)\s.*coverage:\s+([0-9.]+)% of statements")


def run(cmd: list[str], cwd: Path, timeout: float, env: dict[str, str] | None = None) -> tuple[int, str, float]:
    """Run cmd; return (exit status, combined output, seconds). Exit -1 means it timed out."""
    start = time.monotonic()
    try:
        p = subprocess.run(
            cmd,
            check=False,
            cwd=cwd,
            capture_output=True,
            text=True,
            timeout=timeout,
            env=env,
            stdin=subprocess.DEVNULL,
        )
    except subprocess.TimeoutExpired as e:
        out = e.stdout if isinstance(e.stdout, str) else (e.stdout or b"").decode(errors="replace")
        return -1, out, time.monotonic() - start
    return p.returncode, p.stdout + p.stderr, time.monotonic() - start


def module_path(module: Path) -> str:
    for line in (module / "go.mod").read_text().splitlines():
        if line.startswith("module "):
            return line.split()[1]
    return ""


def short(pkg: str, mod: str) -> str:
    return pkg[len(mod) + 1 :] if pkg.startswith(mod + "/") else ("." if pkg == mod else pkg)


# ── coverage ──────────────────────────────────────────────────────────────


def coverage(module: Path, mod: str) -> tuple[dict[str, float], list[Obj]]:
    code, out, _ = run(["go", "test", "-count=1", "-cover", "./..."], module, TEST_TIMEOUT)
    pct: dict[str, float] = {}
    for line in out.splitlines():
        m = COVERAGE_LINE.match(line)
        if m:
            pct[short(m.group(1), mod)] = float(m.group(2))
    details: list[Obj] = []
    if code != 0:
        failed = sorted({short(ln.split()[1], mod) for ln in out.splitlines() if ln.startswith("FAIL\t")})
        details.append({"kind": "coverage", "note": "some packages failed; their coverage is absent", "failed": failed})
    return pct, details


# ── flaky tests ───────────────────────────────────────────────────────────


def test_outcomes(module: Path, mod: str, pkgs: list[str]) -> tuple[dict[tuple[str, str], list[str]], str]:
    """(package, top-level test) -> its result in each run, from `go test -json -count=N`."""
    _, out, _ = run(
        ["go", "test", f"-count={FLAKE_RUNS}", "-json", *[f"./{p}" for p in pkgs]], module, TEST_TIMEOUT * FLAKE_RUNS
    )
    results: dict[tuple[str, str], list[str]] = defaultdict(list)
    for line in out.splitlines():
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            continue
        test = ev.get("Test") or ""
        if ev.get("Action") in {"pass", "fail"} and test and "/" not in test:
            results[(short(ev.get("Package", ""), mod), test)].append(ev["Action"])
    return results, out


def flakes(module: Path, mod: str, pkgs: list[str]) -> tuple[int, int, list[Obj], set[str]]:
    results, _ = test_outcomes(module, mod, pkgs)
    flaky = failing = 0
    details: list[Obj] = []
    unreliable: set[str] = set()
    for (pkg, test), runs in sorted(results.items()):
        passed = runs.count("pass")
        if "fail" not in runs:
            continue
        if passed:
            flaky += 1
            unreliable.add(test)
            details.append({"kind": "flaky", "package": pkg, "test": test, "passed": passed, "runs": len(runs)})
        else:
            failing += 1
            unreliable.add(test)
            details.append({"kind": "failing", "package": pkg, "test": test, "runs": len(runs)})
    return flaky, failing, details, unreliable


# ── mutation ──────────────────────────────────────────────────────────────


def gremlins_bin() -> str | None:
    if os.environ.get("GREMLINS"):
        return os.environ["GREMLINS"] if shutil.which(os.environ["GREMLINS"]) else None
    found = shutil.which("gremlins")
    if found:
        return found
    gopath = subprocess.run(["go", "env", "GOPATH"], check=False, capture_output=True, text=True).stdout.strip()
    candidate = Path(gopath) / "bin" / "gremlins"
    return str(candidate) if gopath and candidate.exists() else None


def enumerate_mutants(gremlins: str, module: Path, pkg: str, work: Path, skip: list[str]) -> tuple[list[Obj], int, str]:
    """Return (covered mutants, not-covered count, error). Each mutant: file (module-relative), line, column, type."""
    out_file = work / f"{pkg.replace('/', '_')}-dry.json"
    # gremlins' coverage run fails if any test fails: skip the unreliable
    # tests there too (GOFLAGS carries go test's -skip).
    env = dict(os.environ)
    if skip:
        env["GOFLAGS"] = (env.get("GOFLAGS", "") + " -skip=" + skip[1]).strip()
    # The dry run only lists mutants, but its coverage run is a full test run:
    # on a loaded host a timing test can fail it, so it gets DRY_RUN_ATTEMPTS.
    for _ in range(DRY_RUN_ATTEMPTS):
        code, out, _ = run(
            [gremlins, "unleash", "--dry-run", "-s", "-o", str(out_file), f"./{pkg}"], module, DRY_RUN_TIMEOUT, env
        )
        if code == 0 and out_file.exists():
            break
    if code != 0 or not out_file.exists():
        return [], 0, f"gremlins dry run failed (exit {code}): {out.strip()[-300:]}"
    data = json.loads(out_file.read_text())
    mutants: list[Obj] = []
    not_covered = 0
    for f in data.get("files", []):
        for m in f.get("mutations", []):
            if m.get("status") == "NOT COVERED":
                not_covered += 1
            elif m.get("status") == "RUNNABLE":
                mutants.append(
                    {"file": f"{pkg}/{f['file_name']}", "line": m["line"], "column": m["column"], "type": m["type"]}
                )
    return mutants, not_covered, ""


def operator_at(text: str, column: int, candidates: dict[str, str]) -> str | None:
    """Return the longest operator of candidates starting at the 1-based byte column of text."""
    raw = text.encode()
    for op in sorted(candidates, key=len, reverse=True):
        if raw[column - 1 : column - 1 + len(op.encode())] == op.encode():
            return op
    return None


def mutate(source: str, m: Obj) -> tuple[str, str, str] | None:
    """Apply mutant m to source; return (mutated source, original line, mutated line), or None if it does not apply."""
    table = MUTATIONS.get(m["type"])
    lines = source.split("\n")
    if table is None or m["line"] > len(lines):
        return None
    line = lines[m["line"] - 1]
    op = operator_at(line, m["column"], table)
    if op is None:
        return None
    raw = line.encode()
    start = m["column"] - 1
    new = (raw[:start] + table[op].encode() + raw[start + len(op.encode()) :]).decode()
    lines[m["line"] - 1] = new
    return "\n".join(lines), line.strip(), new.strip()


def sample_key(m: Obj, source_lines: list[str]) -> str:
    # The source line's text, not its number, so an edit elsewhere in the file
    # leaves the sample unchanged.
    text = source_lines[m["line"] - 1].strip() if m["line"] <= len(source_lines) else ""
    return hashlib.sha256(f"{m['file']}|{m['type']}|{text}|{m['column']}".encode()).hexdigest()


def sample(mutants: list[Obj], module: Path) -> list[Obj]:
    lines: dict[str, list[str]] = {}
    for m in mutants:
        if m["file"] not in lines:
            lines[m["file"]] = (module / m["file"]).read_text().split("\n")
    ranked = sorted(mutants, key=lambda m: (sample_key(m, lines[m["file"]]), m["line"], m["column"]))
    return sorted(ranked[:SAMPLE], key=lambda m: (m["file"], m["line"], m["column"], m["type"]))


def skip_flag(tests: set[str]) -> list[str]:
    return ["-skip", "^(" + "|".join(re.escape(t) for t in sorted(tests)) + ")$"] if tests else []


def classify(code: int, out: str) -> str:
    if code == -1:
        return "timed_out"
    if code == 0:
        return "lived"
    if "[build failed]" in out or "[setup failed]" in out:
        return "not_viable"
    return "killed"


def run_mutant(m: Obj, module: Path, work: Path, timeout: float, skip: set[str]) -> Obj:
    path = module / m["file"]
    applied = mutate(path.read_text(), m)
    if applied is None:
        return {**m, "status": "not_viable", "why": "mutation did not apply"}
    mutated, before, after = applied
    tag = hashlib.sha256(f"{m['file']}:{m['line']}:{m['column']}:{m['type']}".encode()).hexdigest()[:16]
    mutant_file = work / f"{tag}.go"
    overlay = work / f"{tag}.json"
    mutant_file.write_text(mutated)
    overlay.write_text(json.dumps({"Replace": {str(path.resolve()): str(mutant_file)}}))
    test = ["go", "test", "-count=1", "-failfast", f"-overlay={overlay}"]
    pkg_dir = "./" + str(Path(m["file"]).parent)
    code, out, _ = run([*test, *skip_flag(skip), pkg_dir], module, timeout)
    status = classify(code, out)
    killer = FAILED_TEST.search(out) if status == "killed" else None
    unconfirmed = ""
    if killer:
        # A real kill is the killing test failing with the mutant and passing
        # without it. Check both, the test alone: a timing test that fails on a
        # busy host fails one of them, and the mutant is judged again without it.
        only = ["-run", f"^{re.escape(killer.group(1))}$", pkg_dir]
        clean, _, _ = run(["go", "test", "-count=1", *only], module, timeout)
        again, _, _ = run([*test, *only], module, timeout)
        if clean != 0 or again == 0:
            unconfirmed = killer.group(1)
            code, out, _ = run([*test, *skip_flag(skip | {unconfirmed}), pkg_dir], module, timeout)
            status = classify(code, out)
            killer = FAILED_TEST.search(out) if status == "killed" else None
    return {
        **m,
        "status": status,
        "source": before,
        "mutated": after,
        "killed_by": killer.group(1) if killer else "",
        "unconfirmed_kill": unconfirmed,
    }


def mutation(
    module: Path, pkgs: list[str], unreliable: set[str], work: Path
) -> tuple[dict[str, float], list[Obj], Obj]:
    metrics: dict[str, float] = {}
    details: list[Obj] = []
    skipped: Obj = {}
    gremlins = gremlins_bin()
    if gremlins is None:
        for pkg in pkgs:
            skipped[pkg] = f"gremlins not installed: {GREMLINS_INSTALL}"
        return metrics, details, skipped
    skip = skip_flag(unreliable)
    for pkg in pkgs:
        code, out, clean_s = run(["go", "test", "-count=1", *skip, f"./{pkg}/..."], module, TEST_TIMEOUT)
        if code != 0:
            skipped[pkg] = f"tests fail without mutation: {out.strip()[-300:]}"
            continue
        mutants, not_covered, err = enumerate_mutants(gremlins, module, pkg, work, skip)
        if err:
            skipped[pkg] = err
            continue
        chosen = sample(mutants, module)
        timeout = MIN_TIMEOUT + TIMEOUT_FACTOR * clean_s
        with ThreadPoolExecutor(max_workers=WORKERS) as pool:
            results = list(
                pool.map(partial(run_mutant, module=module, work=work, timeout=timeout, skip=unreliable), chosen)
            )
        count = {s: sum(r["status"] == s for r in results) for s in ("killed", "timed_out", "lived", "not_viable")}
        tested = count["killed"] + count["timed_out"] + count["lived"]
        if tested:
            metrics[f"mutation_score.{pkg}"] = round(100 * (count["killed"] + count["timed_out"]) / tested, 1)
        metrics[f"mutants_tested.{pkg}"] = tested
        metrics[f"mutants_lived.{pkg}"] = count["lived"]
        metrics[f"mutants_not_covered.{pkg}"] = not_covered
        details.append(
            {
                "kind": "mutation",
                "package": pkg,
                "covered_mutants": len(mutants),
                "sampled": len(chosen),
                **count,
                "not_covered": not_covered,
                "clean_test_s": round(clean_s, 1),
                # A test that kills many unrelated mutants may be failing on
                # timing (a busy host), not on the mutation.
                "unconfirmed_kills": sum(bool(r.get("unconfirmed_kill")) for r in results),
                "top_killers": Counter(r["killed_by"] for r in results if r.get("killed_by")).most_common(5),
            }
        )
        details.extend(
            {
                "kind": "survived",
                "package": pkg,
                "at": f"{r['file']}:{r['line']}:{r['column']}",
                "mutator": r["type"],
                "source": r["source"],
                "mutated": r["mutated"],
            }
            for r in results
            if r["status"] == "lived"
        )
    return metrics, details, skipped


def main(argv: list[str]) -> int:
    module = Path(argv[argv.index("--module") + 1]) if "--module" in argv else REPO / "cli" / "treemand"
    module = module.resolve()
    mod = module_path(module)
    pkgs = os.environ.get("TEST_EFFECTIVENESS_PACKAGES", " ".join(MUTATION_PACKAGES)).split()
    flake_pkgs = os.environ.get("TEST_EFFECTIVENESS_FLAKE_PACKAGES", " ".join(FLAKE_PACKAGES)).split()
    metrics: dict[str, float] = {}
    details: list[Obj] = []

    cov, cov_details = coverage(module, mod)
    metrics.update({f"coverage_pct.{p}": v for p, v in cov.items()})
    details.extend(cov_details)

    flaky, failing, flake_details, unreliable = flakes(module, mod, flake_pkgs)
    metrics["flaky_tests"] = flaky
    metrics["failing_tests"] = failing
    details.extend(flake_details)

    with tempfile.TemporaryDirectory(prefix="test-effectiveness-") as tmp:
        mut_metrics, mut_details, skipped = mutation(module, pkgs, unreliable, Path(tmp))
    metrics.update(mut_metrics)
    details.extend(mut_details)
    details.append({"kind": "config", "sample": SAMPLE, "workers": WORKERS, "gremlins": GREMLINS_VERSION})

    result: Obj = {"probe": "test-effectiveness", "metrics": metrics, "details": details}
    if skipped:
        result["skipped"] = skipped
    print(json.dumps(result, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

#!/usr/bin/env python3
"""fuzz_campaign.py: the fuzz probe's measurements (skill: treemand-fuzz-campaign).

    fuzz_campaign.py [MODULE_DIR]

Runs every native Go fuzz target (func Fuzz* in a *_test.go file) under
MODULE_DIR (default: the repo's cli/treemand) for FUZZ_SECONDS each (default
60), one `go test -fuzz` at a time, and prints one probe result (see
scripts/metrics/measure.py).

Corpus: Go keeps what it generates in $GOCACHE/fuzz/<import path>/<target>,
which persists between runs on a host, so each campaign starts where the last
one stopped. Inputs that fail are written to <package>/testdata/fuzz/<target>/;
that directory is the committed seed corpus `go test` replays, and an entry is
committed only together with the fix that makes it pass.

Metrics:

- crashers: targets that failed, either on a new input (left in testdata for
  the fix) or on a committed seed. Each one's details carry the re-run command,
  the failure and the input.
- new_coverage_per_minute: corpus entries Go added (inputs that reached new
  coverage) per minute fuzzed. It falls as the corpus saturates.
- corpus_size: cache entries plus committed seeds, over every target.

FUZZ_KEEP_CRASHERS=0 (task fuzz:smoke) deletes a new crasher's input file
after copying it into details, so a smoke run never leaves an unfixed seed in
the tree for a commit to pick up.

A build failure, or a failure that is not a fuzz failure, fails the probe.
"""

import json
import os
import re
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

type Obj = dict[str, Any]

REPO = Path(__file__).resolve().parents[2]
FUZZ_FUNC = re.compile(r"^func (Fuzz\w+)\(\w+ \*testing\.F\)", re.MULTILINE)
ELAPSED = re.compile(r"^fuzz: elapsed: (\d+)s", re.MULTILINE)
WRITTEN = re.compile(r"Failing input written to (\S+)")
SEED_FAIL = re.compile(r"failure while testing seed corpus entry: (\S+)")
RERUN = re.compile(r"go test -run=(\S+)")
MAX_TEXT = 2000
# Stack lines of the Go runtime and test harness: dropped from failure excerpts.
STDLIB_FRAME = re.compile(
    r"^\s*(?:runtime[/.]|testing\.(?!go:)|reflect\.|panic\(|goroutine \d|created by )|/src/(?:runtime|testing|reflect)/"
)


def go(args: list[str], cwd: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["go", *args], check=False, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True
    )


def module_path(mod: Path) -> str:
    for line in (mod / "go.mod").read_text().splitlines():
        if line.startswith("module "):
            return line.split()[1]
    sys.exit(f"fuzz: no module line in {mod / 'go.mod'}")


def fuzz_packages(mod: Path) -> list[str]:
    """List the package dirs (relative, ./x) holding a Fuzz target, skipping testdata and vendored code."""
    dirs = set()
    for f in mod.rglob("*_test.go"):
        rel = f.relative_to(mod)
        if any(p in {"testdata", "vendor"} or p.startswith(".") for p in rel.parts[:-1]):
            continue
        if FUZZ_FUNC.search(f.read_text(errors="replace")):
            dirs.add("./" + rel.parent.as_posix() if rel.parent.parts else ".")
    return sorted(dirs)


def list_targets(mod: Path, pkg: str) -> list[str]:
    p = go(["test", "-list", "^Fuzz", pkg], mod)
    if p.returncode != 0:
        sys.stderr.write(p.stdout)
        sys.exit(f"fuzz: listing the fuzz targets of {pkg} failed (exit {p.returncode})")
    return [line for line in p.stdout.splitlines() if re.fullmatch(r"Fuzz\w+", line)]


def count_files(d: Path) -> int:
    return sum(1 for f in d.iterdir() if f.is_file()) if d.is_dir() else 0


def failure_excerpt(out: str, mod: Path) -> str:
    """Return the failure and the stack frames in our own code, with repo-relative paths."""
    lines = out.splitlines()
    start = next((i for i, line in enumerate(lines) if line.startswith("--- FAIL")), 0)
    stop = next((i for i, line in enumerate(lines) if "Failing input written" in line), len(lines))
    root = str(mod) + "/"
    kept = [line.replace(root, "") for line in lines[start:stop] if line.strip() and not STDLIB_FRAME.search(line)]
    return "\n".join(kept[:40])[:MAX_TEXT]


def crasher(mod: Path, pkg: str, target: str, out: str) -> Obj | None:
    """Return the crasher a failed run reports, or None when the failure is not a fuzz failure."""
    if m := WRITTEN.search(out):
        rel = Path(pkg) / m.group(1)
        rerun = RERUN.search(out)
        name = rerun.group(1) if rerun else f"{target}/{Path(m.group(1)).name}"
        f = mod / rel
        return {
            "committed": False,
            "input_file": rel.as_posix(),
            "rerun": f"go test -run={name} {pkg}",
            "failure": failure_excerpt(out, mod),
            "input": f.read_text(errors="replace")[:MAX_TEXT] if f.is_file() else "",
        }
    if m := SEED_FAIL.search(out):
        entry = m.group(1)
        f = mod / pkg / "testdata" / "fuzz" / entry
        return {
            "committed": True,
            "input_file": (Path(pkg) / "testdata" / "fuzz" / entry).as_posix() if f.is_file() else "",
            "rerun": f"go test -run={entry} {pkg}",
            "failure": failure_excerpt(out, mod),
            "input": f.read_text(errors="replace")[:MAX_TEXT] if f.is_file() else "",
        }
    if out.lstrip().startswith("--- FAIL") or "\n--- FAIL: " + target in out:
        return {
            "committed": False,
            "input_file": "",
            "rerun": f"go test -run={target} {pkg}",
            "failure": failure_excerpt(out, mod),
            "input": "",
        }
    return None


def run_target(mod: Path, cache: Path, pkg: str, target: str, seconds: int) -> Obj:
    cache_dir = cache / "fuzz" / module_path(mod) / Path(pkg).as_posix().removeprefix("./") / target
    cache_dir = Path(os.path.normpath(cache_dir))
    seed_dir = mod / pkg / "testdata" / "fuzz" / target
    before = count_files(cache_dir)
    t0 = time.monotonic()
    p = subprocess.run(
        ["go", "test", "-run", "^$", "-fuzz", f"^{target}$", "-fuzztime", f"{seconds}s",
         "-timeout", f"{seconds + 300}s", "."],
        check=False, cwd=mod / pkg, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
    )  # fmt: skip
    wall = time.monotonic() - t0
    detail: Obj = {"target": target, "package": pkg}
    if p.returncode == 0:
        fuzzed = seconds
    else:
        found = crasher(mod, pkg, target, p.stdout)
        if found is None:
            sys.stderr.write(p.stdout)
            sys.exit(f"fuzz: {target} in {pkg} failed without a fuzz failure (exit {p.returncode})")
        detail["crasher"] = found
        if not found["committed"] and found["input_file"] and os.environ.get("FUZZ_KEEP_CRASHERS", "1") == "0":
            (mod / found["input_file"]).unlink(missing_ok=True)  # its input is in details
            found["removed"] = True
        elapsed = ELAPSED.findall(p.stdout)
        fuzzed = int(elapsed[-1]) if elapsed else 0
    after = count_files(cache_dir)
    detail.update({
        "seconds": fuzzed,
        "wall_s": round(wall, 1),
        "new_entries": max(0, after - before),
        "corpus_size": after + count_files(seed_dir),
    })  # fmt: skip
    return detail


def main(argv: list[str]) -> int:
    mod = Path(argv[0]).resolve() if argv else REPO / "cli" / "treemand"
    seconds = int(os.environ.get("FUZZ_SECONDS", "60"))
    env = go(["env", "GOCACHE"], mod)
    cache = Path(env.stdout.strip())
    targets = [(pkg, t) for pkg in fuzz_packages(mod) for t in list_targets(mod, pkg)]
    details: list[Obj] = [
        {"source": f"go test -fuzz, {seconds}s per target, {len(targets)} targets, corpus in $GOCACHE/fuzz"}
    ]
    for pkg, target in targets:
        details.append(run_target(mod, cache, pkg, target, seconds))
    runs = [d for d in details if "target" in d]
    minutes = sum(d["seconds"] for d in runs) / 60
    new = sum(d["new_entries"] for d in runs)
    metrics = {
        "crashers": sum(1 for d in runs if "crasher" in d),
        "new_coverage_per_minute": round(new / minutes, 1) if minutes else 0.0,
        "corpus_size": sum(d["corpus_size"] for d in runs),
    }
    print(json.dumps({"probe": "fuzz", "metrics": metrics, "details": details}))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

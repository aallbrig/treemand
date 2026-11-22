#!/usr/bin/env python3
"""dependency_health.py: the dependency-health probe's measurements.

    dependency_health.py [--root DIR]

Prints one probe result (see scripts/metrics/measure.py). DIR (default: the
repo) holds cli/treemand/go.mod, .github/workflows and Taskfile.yml.

Metrics (each is its own subject; one that cannot be measured, for example
offline, is reported under "skipped" with the reason, never as zero):

- vulns_reachable: govulncheck OSV entries whose vulnerable symbol the code calls.
- vulns_imported_unreachable: OSV entries in a package the code imports but
  whose vulnerable symbol it never calls. Entries in a module that is required
  but not imported are listed in details only.
- go_modules_behind: direct modules (go.mod require, not // indirect) that
  `go list -m -u` says have a newer minor or patch release.
- go_modules_stale: direct modules whose latest release (the update's time, or
  the current version's when it is the latest) is older than STALE_DAYS.
- go_toolchain_behind: 1 when go.mod's toolchain (or go directive) is behind:
  a newer stable patch exists on its release line, or its line is no longer one
  of the stable releases go.dev lists (unsupported). Otherwise 0.
- tools_behind: tools pinned in .github/workflows/*.yml and Taskfile.yml
  (TOOLS) whose pinned version is older than the latest release. Tools pinned
  to "latest" are listed in details as unpinned and not counted.

Network: one go.dev download-list fetch, one module-proxy reachability check,
the proxy queries `go list -m -u` makes for the direct modules, govulncheck's
database fetch, and one latest-version lookup per pinned tool (at most
MAX_TOOL_LOOKUPS). Every fetch is cached for the run. After the first
connection failure the probe treats the network as down and skips whatever
still needs it. DEPENDENCY_HEALTH_OFFLINE=1 skips the network outright.

Endpoints (overridable, for tests): DEPENDENCY_HEALTH_GO_DL (go.dev/dl JSON),
DEPENDENCY_HEALTH_PYPI (PyPI JSON API base), and the module proxy, the first
http(s) entry of GOPROXY (default proxy.golang.org).
"""

import json
import os
import re
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from collections.abc import Iterator
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

type Obj = dict[str, Any]

REPO = Path(__file__).resolve().parents[2]
GO_DL = os.environ.get("DEPENDENCY_HEALTH_GO_DL", "https://go.dev/dl/?mode=json")
PYPI = os.environ.get("DEPENDENCY_HEALTH_PYPI", "https://pypi.org/pypi").rstrip("/")
STALE_DAYS = 730
MAX_TOOL_LOOKUPS = 8
FETCH_TIMEOUT = 15
GO_LIST_TIMEOUT = 180
VULN_TIMEOUT = 600
METRICS = (
    "vulns_reachable",
    "vulns_imported_unreachable",
    "go_modules_behind",
    "go_modules_stale",
    "go_toolchain_behind",
    "tools_behind",
)
# Tool -> where its latest release is looked up: ("pypi", package) or ("proxy", module path).
TOOLS: dict[str, tuple[str, str]] = {
    "golangci-lint": ("proxy", "github.com/golangci/golangci-lint/v2"),
    "hugo": ("proxy", "github.com/gohugoio/hugo"),
    "cfn-lint": ("pypi", "cfn-lint"),
    "checkov": ("pypi", "checkov"),
    "ruff": ("pypi", "ruff"),
    "mypy": ("pypi", "mypy"),
}
PIP_PIN = re.compile(r"\bpipx? install\s+(?:-\S+\s+)*([A-Za-z0-9_.-]+)==([0-9][0-9A-Za-z.]*)")
HUGO_PIN = re.compile(r"""\bhugo-version:\s*["']?([^"'\s#]+)""")
GOLANGCI_INSTALL = re.compile(r"golangci-lint(?:/v\d+)?/cmd/golangci-lint@(\S+)")
GOLANGCI_ACTION = re.compile(r"uses:\s*golangci/golangci-lint-action@")
ACTION_VERSION = re.compile(r"""^\s*version:\s*["']?([^"'\s#]+)""")
ACTION_WINDOW = 8


def proxy_base() -> str:
    for entry in re.split(r"[,|]", os.environ.get("GOPROXY", "")):
        if entry.startswith(("http://", "https://")):
            return entry.rstrip("/")
    return "https://proxy.golang.org"


class Net:
    """HTTP GETs for one run: cached, and short-circuited once the network is down."""

    def __init__(self) -> None:
        self.down = os.environ.get("DEPENDENCY_HEALTH_OFFLINE") == "1"
        self.why = "offline (DEPENDENCY_HEALTH_OFFLINE=1)" if self.down else ""
        self.cache: dict[str, Obj | list[Any] | None] = {}
        self.calls = 0

    def json(self, url: str) -> Obj | list[Any] | None:
        """Return the parsed body, or None (HTTP error, bad JSON, or network down: see .down)."""
        if url in self.cache:
            return self.cache[url]
        if self.down:
            return None
        self.calls += 1
        got: Obj | list[Any] | None = None
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "dependency-health-probe"})  # noqa: S310
            with urllib.request.urlopen(req, timeout=FETCH_TIMEOUT) as resp:  # noqa: S310
                got = json.loads(resp.read())
        except urllib.error.HTTPError:
            got = None
        except (urllib.error.URLError, TimeoutError, OSError) as e:
            self.down = True
            self.why = f"offline ({type(e).__name__})"
        except ValueError:
            got = None
        self.cache[url] = got
        return got


def run(cmd: list[str], cwd: Path, timeout: int) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(cmd, cwd=cwd, check=False, capture_output=True, text=True, timeout=timeout)
    except subprocess.TimeoutExpired:
        return subprocess.CompletedProcess(cmd, 124, "", f"timed out after {timeout}s")
    except FileNotFoundError:
        return subprocess.CompletedProcess(cmd, 127, "", f"{cmd[0]}: not found")


def json_stream(text: str) -> Iterator[Obj]:
    """Concatenated JSON objects, as go list -json and govulncheck -format json print them."""
    dec = json.JSONDecoder()
    i = 0
    while True:
        while i < len(text) and text[i].isspace():
            i += 1
        if i >= len(text):
            return
        obj, i = dec.raw_decode(text, i)
        if isinstance(obj, dict):
            yield obj


def version_key(v: str) -> tuple[int, ...]:
    """go1.26.10, v0.157.0, 3.2.529 -> numeric parts; a pre-release tag ends the parse."""
    v = v.removeprefix("go").removeprefix("v")
    parts: list[int] = []
    for p in v.split("."):
        m = re.match(r"\d+", p)
        if not m:
            break
        parts.append(int(m.group()))
        if m.group() != p:
            break
    return tuple(parts)


# ── govulncheck ────────────────────────────────────────────────────────────


def vuln_levels(stream: str) -> tuple[dict[str, int], dict[str, str], dict[str, str], str]:
    """OSV id -> strongest level (1 module required, 2 package imported, 3 symbol called), plus where and summary."""
    level: dict[str, int] = {}
    where: dict[str, str] = {}
    summary: dict[str, str] = {}
    go_version = ""
    for obj in json_stream(stream):
        if "config" in obj:
            go_version = obj["config"].get("go_version", "")
        if "osv" in obj:
            summary[obj["osv"]["id"]] = obj["osv"].get("summary", "")
        if "finding" not in obj:
            continue
        f = obj["finding"]
        top = (f.get("trace") or [{}])[0]
        lvl = 3 if top.get("function") else 2 if top.get("package") else 1
        if lvl > level.get(f["osv"], 0):
            level[f["osv"]] = lvl
            where[f["osv"]] = top.get("module", "") + (" " + top.get("version", "") if top.get("version") else "")
    return level, where, summary, go_version


def vulns(gomod_dir: Path, pinned: str, out: Obj) -> None:
    if shutil.which("govulncheck") is None:
        skip(out, ("vulns_reachable", "vulns_imported_unreachable"), "govulncheck not installed (task setup)")
        return
    p = run(["govulncheck", "-format", "json", "./..."], gomod_dir, VULN_TIMEOUT)
    if p.returncode != 0:
        why = (p.stderr.strip().splitlines() or ["no output"])[-1][:200]
        skip(out, ("vulns_reachable", "vulns_imported_unreachable"), f"govulncheck failed: {why}")
        return
    level, where, summary, go_version = vuln_levels(p.stdout)
    if go_version and go_version != pinned:
        out["details"].append(
            {
                "check": "vulns_toolchain",
                "note": f"standard library judged as {go_version}, not the pinned {pinned}; "
                "the weekly workflow builds with the pinned toolchain",
            }
        )
    names = {3: "reachable", 2: "imported_unreachable", 1: "required_only"}
    for lvl, name in names.items():
        ids = sorted(i for i, v in level.items() if v == lvl)
        if lvl > 1:
            out["metrics"][f"vulns_{name}"] = len(ids)
        out["details"].append(
            {
                "check": f"vulns_{name}",
                "go_version": go_version,
                "items": [{"id": i, "module": where[i], "summary": summary.get(i, "")} for i in ids],
            }
        )


# ── Go modules ─────────────────────────────────────────────────────────────


def direct_modules(gomod_dir: Path) -> tuple[list[str], str, str]:
    """(direct module paths, go directive, toolchain directive) from go mod edit -json."""
    p = run(["go", "mod", "edit", "-json"], gomod_dir, 60)
    if p.returncode != 0:
        sys.exit(f"dependency-health: go mod edit -json failed: {p.stderr.strip()}")
    mod = json.loads(p.stdout)
    paths = [r["Path"] for r in mod.get("Require") or [] if not r.get("Indirect")]
    return paths, mod.get("Go", ""), mod.get("Toolchain", "")


def modules(gomod_dir: Path, direct: list[str], net: Net, out: Obj) -> None:
    names = ("go_modules_behind", "go_modules_stale")
    if not direct:
        out["metrics"].update(dict.fromkeys(names, 0))
        return
    # go list -u is silent when the proxy is unreachable (it reports no update),
    # so check the proxy is there before believing it.
    net.json(f"{proxy_base()}/{direct[0]}/@latest")
    if net.down:
        skip(out, names, net.why)
        return
    p = run(["go", "list", "-m", "-u", "-json", *direct], gomod_dir, GO_LIST_TIMEOUT)
    errors = [ln for ln in p.stderr.splitlines() if ln.startswith("go: ")]
    mods = list(json_stream(p.stdout)) if p.returncode == 0 else []
    errors += [m["Error"]["Err"] for m in mods if isinstance(m.get("Error"), dict)]
    if p.returncode != 0 or errors:
        why = (errors or p.stderr.strip().splitlines() or ["no output"])[0][:200]
        skip(out, names, f"go list -m -u failed: {why}")
        return
    now = datetime.now(UTC)
    behind: list[Obj] = []
    stale: list[Obj] = []
    for m in mods:
        upd = m.get("Update") or {}
        if upd.get("Version"):
            behind.append({"module": m["Path"], "version": m["Version"], "latest": upd["Version"]})
        released = upd.get("Time") or m.get("Time")
        if not released:
            continue
        when = datetime.fromisoformat(released)
        if now - when > timedelta(days=STALE_DAYS):
            stale.append({"module": m["Path"], "latest": upd.get("Version") or m["Version"], "released": released})
    out["metrics"]["go_modules_behind"] = len(behind)
    out["metrics"]["go_modules_stale"] = len(stale)
    out["details"].append({"check": "go_modules_behind", "direct": len(direct), "items": behind})
    out["details"].append({"check": "go_modules_stale", "older_than_days": STALE_DAYS, "items": stale})


def toolchain(go: str, tc: str, net: Net, out: Obj) -> None:
    pinned = tc or f"go{go}"
    releases = net.json(GO_DL)
    if not isinstance(releases, list):
        skip(out, ("go_toolchain_behind",), net.why or "go.dev download list unavailable")
        return
    stable = [r["version"] for r in releases if isinstance(r, dict) and r.get("stable")]
    if not stable:
        skip(out, ("go_toolchain_behind",), "go.dev download list has no stable release")
        return
    line = version_key(pinned)[:2]
    same_line = [v for v in stable if version_key(v)[:2] == line]
    newest_line = max(same_line, key=version_key) if same_line else ""
    behind = not same_line or version_key(newest_line) > version_key(pinned)
    out["metrics"]["go_toolchain_behind"] = int(behind)
    out["details"].append(
        {
            "check": "go_toolchain_behind",
            "pinned": pinned,
            "latest_on_line": newest_line or "line unsupported",
            "latest_stable": max(stable, key=version_key),
        }
    )


# ── pinned tools ───────────────────────────────────────────────────────────


def pins(root: Path) -> list[Obj]:
    """Every pinned version of a TOOLS entry: {tool, version, file, line}."""
    files = sorted((root / ".github" / "workflows").glob("*.y*ml"))
    if (root / "Taskfile.yml").is_file():
        files.append(root / "Taskfile.yml")
    found: list[Obj] = []
    for f in files:
        lines = f.read_text(encoding="utf-8").splitlines()
        rel = f.relative_to(root).as_posix()
        for n, text in enumerate(lines, 1):
            hits: list[tuple[str, str]] = [(m.group(1), m.group(2)) for m in PIP_PIN.finditer(text)]
            if m := HUGO_PIN.search(text):
                hits.append(("hugo", m.group(1)))
            if m := GOLANGCI_INSTALL.search(text):
                hits.append(("golangci-lint", m.group(1)))
            if GOLANGCI_ACTION.search(text):
                for later in lines[n : n + ACTION_WINDOW]:
                    if v := ACTION_VERSION.match(later):
                        hits.append(("golangci-lint", v.group(1)))
                        break
            found += [{"tool": t, "version": v, "file": rel, "line": n} for t, v in hits if t in TOOLS]
    return found


def latest_tool(tool: str, net: Net) -> str:
    kind, ref = TOOLS[tool]
    if kind == "pypi":
        got = net.json(f"{PYPI}/{ref}/json")
        info = got.get("info") if isinstance(got, dict) else None
        return str(info.get("version", "")) if isinstance(info, dict) else ""
    got = net.json(f"{proxy_base()}/{ref}/@latest")
    return str(got.get("Version", "")) if isinstance(got, dict) else ""


def tools(root: Path, net: Net, out: Obj) -> None:
    found = pins(root)
    unpinned = [p for p in found if p["version"] == "latest"]
    pinned: dict[str, list[Obj]] = {}
    for p in found:
        if p["version"] != "latest":
            pinned.setdefault(p["tool"], []).append(p)
    behind: list[Obj] = []
    unknown: list[str] = []
    for n, (tool, where) in enumerate(sorted(pinned.items())):
        latest = latest_tool(tool, net) if n < MAX_TOOL_LOOKUPS else ""
        if net.down:
            skip(out, ("tools_behind",), net.why)
            return
        if not latest:
            unknown.append(tool)
            continue
        old = [
            {k: v for k, v in p.items() if k != "tool"}
            for p in where
            if version_key(p["version"]) < version_key(latest)
        ]
        if old:
            behind.append({"tool": tool, "latest": latest.removeprefix("v"), "pins": old})
    out["metrics"]["tools_behind"] = len(behind)
    out["details"].append(
        {
            "check": "tools_behind",
            "items": behind,
            "unpinned": [f"{p['tool']} ({p['file']}:{p['line']})" for p in unpinned],
            "lookup_failed": unknown,
        }
    )


def skip(out: Obj, names: tuple[str, ...], why: str) -> None:
    for name in names:
        out["skipped"][name] = why


def main(argv: list[str]) -> int:
    root = Path(argv[argv.index("--root") + 1]) if "--root" in argv else REPO
    gomod_dir = root / "cli" / "treemand"
    out: Obj = {"probe": "dependency-health", "metrics": {}, "details": [], "skipped": {}}
    net = Net()
    direct, go, tc = direct_modules(gomod_dir)
    toolchain(go, tc, net, out)
    modules(gomod_dir, direct, net, out)
    tools(root, net, out)
    if net.down:
        skip(out, ("vulns_reachable", "vulns_imported_unreachable"), net.why)
    else:
        vulns(gomod_dir, tc or f"go{go}", out)
    out["details"].append({"check": "network", "fetches": net.calls, "down": net.down})
    if not out["skipped"]:
        del out["skipped"]
    print(json.dumps(out, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))

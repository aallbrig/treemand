#!/usr/bin/env python3
"""adoption_signals.py: the adoption-signals probe (skill: treemand-adoption-signals).

    adoption_signals.py

Prints one probe result (see scripts/metrics/measure.py) from free sources
only: the GitHub API through `gh api` (the repository from the current
checkout, or GH_REPO), the Go module proxy and the Homebrew formula API.
These are trend signals, not quality gates: the only rule is
latest_zero_downloads_7d. Aggregate numbers only; no user names are read
into the result.

Metrics:
- stars, forks, watchers: the repository's counts (watchers = subscribers).
- downloads_total: downloads of release archives (checksums, signatures and
  other side files left out: an install fetches those too) over every
  published release, prereleases included.
- downloads_latest: the same for the latest published non-prerelease.
- latest_release_age_days: days since the latest release was published.
- latest_zero_downloads_7d: 1 when the latest release is 7 or more days old
  and none of its assets was downloaded (a likely broken channel), else 0.
- views_14d, views_uniques_14d, clones_14d, clones_uniques_14d,
  referrers_14d (total referred views over the top referrers): GitHub
  traffic, which needs push access; skipped ("traffic") when forbidden.
- tap_clones_14d, tap_clones_uniques_14d: clones of the Homebrew tap
  repository (brew tap and brew update fetch it), the closest free signal of
  Homebrew installs; skipped ("tap_traffic") when forbidden.
- brew_installs_30d: formulae.brew.sh install analytics; that API covers
  homebrew/core and cask only, so a tap formula is skipped ("brew_installs").
- proxy_latest_is_release: 1 when the Go module proxy's @latest for the
  module path is the latest release tag, else 0 (the version seen is in
  details).
- external_open_issues: open issues (not pull requests) whose author is not
  the owner, a member or a collaborator, and not a bot. A count only.

Details carry the latest release's downloads per asset (side files too), the top referrers by
source name (sites, not people), the versions the proxy reports, and the
week-over-week delta of every metric against the newest record in the
history file that is at least WEEK_DAYS days old.

Environment: GH_REPO (owner/repo), ADOPTION_HISTORY (history file, default
docs/metrics/adoption-signals.jsonl), ADOPTION_NOW (ISO 8601 clock for
tests), ADOPTION_PROXY and ADOPTION_BREW_API (base URLs; tests use file://),
ADOPTION_TAP_REPO (default <owner>/homebrew-tap), ADOPTION_PER_PAGE (tests).
"""

import json
import os
import subprocess
import sys
import urllib.error
import urllib.request
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

type Obj = dict[str, Any]

ZERO_DOWNLOAD_DAYS = 7
WEEK_DAYS = 6.5  # a weekly cron drifts by minutes; accept a record from 6.5 days back
MAINTAINER = {"OWNER", "MEMBER", "COLLABORATOR"}
PROXY = os.environ.get("ADOPTION_PROXY", "https://proxy.golang.org")
BREW_API = os.environ.get("ADOPTION_BREW_API", "https://formulae.brew.sh/api")
HISTORY = Path(os.environ.get("ADOPTION_HISTORY", "docs/metrics/adoption-signals.jsonl"))
SIDE_FILES = (".txt", ".json", ".sig", ".pem", ".sbom")
TIMEOUT_S = 30
PER_PAGE = int(os.environ.get("ADOPTION_PER_PAGE", "100"))


class GhError(Exception):
    pass


def gh(path: str) -> Any:  # noqa: ANN401 # parsed JSON: an object or a list
    """GET a GitHub API path through gh; GhError carries gh's message."""
    p = subprocess.run(["gh", "api", "-X", "GET", path], check=False, capture_output=True, text=True)
    if p.returncode != 0:
        raise GhError(p.stderr.strip().splitlines()[-1] if p.stderr.strip() else f"exit {p.returncode}")
    return json.loads(p.stdout)


def gh_pages(path: str) -> list[Obj]:
    out: list[Obj] = []
    sep = "&" if "?" in path else "?"
    page = 1
    while True:
        items = gh(f"{path}{sep}per_page={PER_PAGE}&page={page}")
        out.extend(items)
        if len(items) < PER_PAGE:
            return out
        page += 1


def fetch_json(url: str) -> Obj | None:
    """GET JSON from a public URL; None when it does not exist."""
    req = urllib.request.Request(url, headers={"User-Agent": "treemand-adoption-signals"})  # noqa: S310 # http(s) or file base from constants/env
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT_S) as resp:  # noqa: S310
            data: Obj = json.loads(resp.read())
            return data
    except urllib.error.HTTPError as e:
        if e.code in {404, 410}:
            return None
        raise
    except urllib.error.URLError as e:
        if isinstance(e.reason, FileNotFoundError):
            return None
        raise


def now() -> datetime:
    fixed = os.environ.get("ADOPTION_NOW")
    return datetime.fromisoformat(fixed) if fixed else datetime.now(UTC)


def ts(value: str) -> datetime:
    return datetime.fromisoformat(value)


def repo_name() -> str:
    env = os.environ.get("GH_REPO")
    if env:
        return env
    data: Obj = gh("repos/{owner}/{repo}")
    name: str = data["full_name"]
    return name


def is_archive(name: str) -> bool:
    # One install fetches an archive plus checksums.txt and its bundle; count the archive.
    return not name.startswith("checksums") and not name.endswith(SIDE_FILES)


def releases(repo: str, metrics: Obj, details: list[Obj], clock: datetime) -> str | None:
    rels = [r for r in gh_pages(f"repos/{repo}/releases") if not r.get("draft") and r.get("published_at")]

    def count(r: Obj) -> int:
        return sum(a.get("download_count", 0) for a in r.get("assets", []) if is_archive(a["name"]))

    metrics["downloads_total"] = sum(count(r) for r in rels)
    stable = [r for r in rels if not r.get("prerelease")]
    if not stable:
        details.append({"kind": "releases", "note": "no published release"})
        return None
    latest = max(stable, key=lambda r: ts(r["published_at"]))
    age = (clock - ts(latest["published_at"])).total_seconds() / 86400
    metrics["downloads_latest"] = count(latest)
    metrics["latest_release_age_days"] = round(age, 1)
    metrics["latest_zero_downloads_7d"] = int(age >= ZERO_DOWNLOAD_DAYS and count(latest) == 0)
    details.append(
        {
            "kind": "latest_release",
            "tag": latest["tag_name"],
            "published": latest["published_at"],
            "assets": {a["name"]: a.get("download_count", 0) for a in latest.get("assets", [])},
            "releases_counted": len(rels),
        }
    )
    tag: str = latest["tag_name"]
    return tag


def traffic(repo: str, kinds: tuple[str, ...], skipped: Obj, subject: str) -> Obj | None:
    """Read traffic/<kind> for each kind; on a forbidden repo, skip the subject."""
    got: Obj = {}
    try:
        for kind in kinds:
            got[kind] = gh(f"repos/{repo}/traffic/{kind}")
    except GhError as e:
        skipped[subject] = f"traffic API not readable ({e}); it needs push access"
        return None
    return got


def counts(got: Obj, prefix: str, metrics: Obj) -> None:
    for kind in ("views", "clones"):
        if kind in got:
            metrics[f"{prefix}{kind}_14d"] = got[kind].get("count", 0)
            metrics[f"{prefix}{kind}_uniques_14d"] = got[kind].get("uniques", 0)


def proxy(module: str, tag: str | None, metrics: Obj, details: list[Obj], skipped: Obj) -> None:
    try:
        latest = fetch_json(f"{PROXY}/{module}/@latest")
    except (urllib.error.URLError, TimeoutError) as e:
        skipped["proxy"] = f"module proxy not reachable ({e})"
        return
    seen = latest.get("Version") if latest else None
    metrics["proxy_latest_is_release"] = int(seen is not None and seen == tag)
    details.append({"kind": "module_proxy", "module": module, "latest_seen": seen, "latest_release": tag})


def brew(formula: str, metrics: Obj, details: list[Obj], skipped: Obj) -> None:
    try:
        data = fetch_json(f"{BREW_API}/formula/{formula}.json")
    except (urllib.error.URLError, TimeoutError) as e:
        skipped["brew_installs"] = f"formulae.brew.sh not reachable ({e})"
        return
    if data is None:
        skipped["brew_installs"] = (
            "no public install analytics: formulae.brew.sh covers homebrew/core and cask only, "
            "and the formula ships from a tap (tap_clones_14d is the proxy signal)"
        )
        return
    installs = data.get("analytics", {}).get("install", {}).get("30d", {})
    metrics["brew_installs_30d"] = sum(installs.values())
    details.append({"kind": "homebrew_core", "installs_30d": installs})


def external_issues(repo: str, metrics: Obj) -> None:
    issues = gh_pages(f"repos/{repo}/issues?state=open")
    metrics["external_open_issues"] = sum(
        1
        for i in issues
        if "pull_request" not in i
        and i.get("author_association") not in MAINTAINER
        and (i.get("user") or {}).get("type") != "Bot"
    )


def week_over_week(metrics: Obj, clock: datetime) -> Obj:
    if not HISTORY.exists():
        return {"kind": "week_over_week", "note": f"no history yet ({HISTORY})"}
    base: Obj | None = None
    for line in HISTORY.read_text().splitlines():
        if not line.strip():
            continue
        rec: Obj = json.loads(line)
        if (clock - ts(rec["date"])).total_seconds() / 86400 >= WEEK_DAYS:
            base = rec
    if base is None:
        return {"kind": "week_over_week", "note": "no record a week old yet"}
    prev: Obj = base.get("metrics", {})
    deltas = {m: round(v - prev[m], 1) for m, v in sorted(metrics.items()) if m in prev}
    return {"kind": "week_over_week", "since": base["date"], "deltas": deltas}


def measure() -> Obj:
    clock = now()
    repo = repo_name()
    owner = repo.split("/", 1)[0]
    metrics: Obj = {}
    details: list[Obj] = []
    skipped: Obj = {}

    info: Obj = gh(f"repos/{repo}")
    metrics["stars"] = info.get("stargazers_count", 0)
    metrics["forks"] = info.get("forks_count", 0)
    metrics["watchers"] = info.get("subscribers_count", 0)

    tag = releases(repo, metrics, details, clock)

    got = traffic(repo, ("views", "clones", "popular/referrers"), skipped, "traffic")
    if got is not None:
        counts(got, "", metrics)
        refs = got["popular/referrers"]
        metrics["referrers_14d"] = sum(r.get("count", 0) for r in refs)
        details.append({"kind": "referrers", "top": {r["referrer"]: r.get("count", 0) for r in refs}})
    tap = os.environ.get("ADOPTION_TAP_REPO", f"{owner}/homebrew-tap")
    tap_got = traffic(tap, ("clones",), skipped, "tap_traffic")
    if tap_got is not None:
        counts(tap_got, "tap_", metrics)

    # The proxy's case encoding: an upper-case letter is "!" and its lower case.
    module = "".join(f"!{c.lower()}" if c.isupper() else c for c in f"github.com/{repo}")
    proxy(module, tag, metrics, details, skipped)
    brew(repo.split("/", 1)[1], metrics, details, skipped)
    external_issues(repo, metrics)

    details.append(week_over_week(metrics, clock))
    out: Obj = {"probe": "adoption-signals", "metrics": metrics, "details": details}
    if skipped:
        out["skipped"] = skipped
    return out


def main() -> None:
    try:
        print(json.dumps(measure(), indent=2))
    except GhError as e:
        sys.exit(f"adoption-signals: gh api failed: {e}")


if __name__ == "__main__":
    main()

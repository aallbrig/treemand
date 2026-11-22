#!/usr/bin/env python3
"""release_health.py: the release-health probe's measurements (skill: treemand-release-health).

    release_health.py

Prints one probe result (see scripts/metrics/measure.py), judged from the
GitHub API and public URLs only; nothing is installed.

Per release (the last RELEASES published, non-draft, non-prerelease):
- lead_time_min: median minutes from the tagged commit's merge (its committer
  date) to the release being published.
- assets_missing: expected assets (5 archives, checksums.txt and its sigstore
  bundle) absent from a release.
- checksum_mismatches: archives whose checksums.txt line is missing or differs
  from the SHA-256 GitHub reports for the uploaded asset (the asset is
  downloaded and hashed when GitHub reports none).
- signature_failures: bundles `cosign verify-blob` rejects for the identity
  install.sh pins. Skipped, not failed, when cosign is not installed.

Latest release:
- tap_lag, mirror_lag, mirror_assets_missing: the Homebrew tap formula, the
  treemand.dev mirror's latest.txt and the mirror's files for the latest tag
  are behind it more than LAG_GRACE after publishing (inside the grace they
  are reported as pending, not counted).
- install_script_missing: treemand.dev/install.sh is not served or does not
  default to the mirror whose latest.txt is checked above.
- package_publish_failed: the Publish to Package Managers run for the latest
  tag did not succeed (none started within LAG_GRACE counts as failed).
- auto_release_failure_pct: failed Auto Release runs among the last 20
  completed ones (cancelled runs are superseded, not failures, and excluded).

Environment: GH_TOKEN or GITHUB_TOKEN (optional; raises the API rate limit).
Tests: RELEASE_HEALTH_ROUTES is a file of "<url><TAB><local file>" lines that
replaces the network (an unlisted URL is a 404), RELEASE_HEALTH_NOW fixes the
clock (ISO 8601), and RELEASE_HEALTH_COSIGN names the cosign binary ("none":
as if it were not installed; default: cosign on PATH).
"""

import hashlib
import json
import os
import shutil
import statistics
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

type Obj = dict[str, Any]

REPO = "aallbrig/treemand"
API = f"https://api.github.com/repos/{REPO}"
SITE = "https://treemand.dev"
TAP_FORMULA = "https://raw.githubusercontent.com/aallbrig/homebrew-tap/main/Formula/treemand.rb"
RELEASES = 10
AUTO_RELEASE_RUNS = 20
LAG_GRACE = timedelta(hours=1)
ARCHIVES = [
    "treemand_darwin_amd64.tar.gz",
    "treemand_darwin_arm64.tar.gz",
    "treemand_linux_amd64.tar.gz",
    "treemand_linux_arm64.tar.gz",
    "treemand_windows_amd64.zip",
]
CHECKSUMS = "checksums.txt"
BUNDLE = "checksums.txt.sigstore.json"
EXPECTED = [*ARCHIVES, CHECKSUMS, BUNDLE]
OIDC_ISSUER = "https://token.actions.githubusercontent.com"
FAILED_RUN = {"failure", "timed_out", "startup_failure"}
TIMEOUT_S = 60


class NotFoundError(Exception):
    pass


def routes() -> dict[str, Path] | None:
    path = os.environ.get("RELEASE_HEALTH_ROUTES")
    if not path:
        return None
    out: dict[str, Path] = {}
    for line in Path(path).read_text().splitlines():
        url, _, file = line.partition("\t")
        if url:
            out[url] = Path(file)
    return out


ROUTES = routes()


def fetch(url: str) -> bytes:
    """GET a URL; NotFoundError on 404 (or an unrouted URL under test)."""
    if ROUTES is not None:
        if url not in ROUTES:
            raise NotFoundError(url)
        return ROUTES[url].read_bytes()
    headers = {"User-Agent": "treemand-release-health"}
    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if token and url.startswith("https://api.github.com/"):
        headers["Authorization"] = f"Bearer {token}"
        headers["Accept"] = "application/vnd.github+json"
    req = urllib.request.Request(url, headers=headers)  # noqa: S310 # https URLs built from constants
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT_S) as resp:  # noqa: S310
            body: bytes = resp.read()
            return body
    except urllib.error.HTTPError as e:
        if e.code == 404:  # noqa: PLR2004
            raise NotFoundError(url) from e
        raise


def exists(url: str) -> bool:
    if ROUTES is not None:
        return url in ROUTES
    req = urllib.request.Request(url, method="HEAD", headers={"User-Agent": "treemand-release-health"})  # noqa: S310
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT_S) as resp:  # noqa: S310
            ok: bool = resp.status == 200  # noqa: PLR2004
            return ok
    except urllib.error.HTTPError:
        return False


def api(path: str) -> Any:  # noqa: ANN401 # parsed JSON of whatever shape the endpoint returns
    return json.loads(fetch(f"{API}/{path}"))


def when(stamp: str) -> datetime:
    return datetime.fromisoformat(stamp)


def now() -> datetime:
    fixed = os.environ.get("RELEASE_HEALTH_NOW")
    return when(fixed) if fixed else datetime.now(UTC)


def download(tag: str, name: str) -> bytes:
    return fetch(f"https://github.com/{REPO}/releases/download/{tag}/{name}")


def parse_checksums(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in text.splitlines():
        parts = line.split()
        if len(parts) == 2:  # noqa: PLR2004
            out[parts[1].lstrip("*")] = parts[0].lower()
    return out


def check_checksums(tag: str, assets: dict[str, Obj]) -> list[str]:
    """List the archives whose checksums.txt entry does not match the published bytes."""
    if CHECKSUMS not in assets:
        return [a for a in ARCHIVES if a in assets]
    sums = parse_checksums(download(tag, CHECKSUMS).decode())
    bad: list[str] = []
    for name in ARCHIVES:
        if name not in assets:
            continue
        digest = str(assets[name].get("digest") or "")
        actual = digest.removeprefix("sha256:") if digest.startswith("sha256:") else ""
        if not actual:
            actual = hashlib.sha256(download(tag, name)).hexdigest()
        if sums.get(name) != actual:
            bad.append(name)
    return bad


def verify_signature(tag: str, cosign: str, workdir: Path) -> str:
    """Return "ok" or why the bundle does not verify."""
    d = workdir / tag
    d.mkdir()
    try:
        (d / CHECKSUMS).write_bytes(download(tag, CHECKSUMS))
        (d / BUNDLE).write_bytes(download(tag, BUNDLE))
    except NotFoundError as e:
        return f"not downloadable: {e}"
    identity = f"https://github.com/{REPO}/.github/workflows/release.yml@refs/tags/{tag}"
    proc = subprocess.run(
        [
            cosign,
            "verify-blob",
            "--bundle",
            str(d / BUNDLE),
            "--certificate-identity",
            identity,
            "--certificate-oidc-issuer",
            OIDC_ISSUER,
            str(d / CHECKSUMS),
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    if proc.returncode == 0:
        return "ok"
    lines = (proc.stderr or proc.stdout).strip().splitlines()
    return f"cosign exit {proc.returncode}: {lines[-1] if lines else ''}"


def lead_time_min(tag: str, published: datetime) -> float | None:
    try:
        commit = api(f"commits/{tag}")
    except NotFoundError:
        return None
    merged = when(commit["commit"]["committer"]["date"])
    return round((published - merged).total_seconds() / 60, 1)


def check_release(rel: Obj, cosign: str | None, workdir: Path) -> Obj:
    tag = str(rel["tag_name"])
    published = when(rel["published_at"])
    assets = {str(a["name"]): a for a in rel.get("assets", [])}
    row: Obj = {
        "tag": tag,
        "published_at": rel["published_at"],
        "lead_time_min": lead_time_min(tag, published),
        "assets_missing": [a for a in EXPECTED if a not in assets],
        "checksum_mismatches": check_checksums(tag, assets),
    }
    if cosign:
        row["signature"] = verify_signature(tag, cosign, workdir) if BUNDLE in assets else "no bundle"
    return row


def lagging(published: datetime, behind: object) -> tuple[int, str]:
    """Count a lag only once the grace period after publishing has passed."""
    if not behind:
        return 0, "current"
    if now() - published <= LAG_GRACE:
        return 0, "pending (inside the 1 hour grace)"
    return 1, "behind"


def tap_version() -> str:
    try:
        text = fetch(TAP_FORMULA).decode()
    except NotFoundError:
        return ""
    for line in text.splitlines():
        s = line.strip()
        if s.startswith('version "'):
            return s.split('"')[1]
    return ""


def mirror_latest() -> str:
    try:
        return fetch(f"{SITE}/releases/latest.txt").decode().strip()
    except NotFoundError:
        return ""


def install_script_ok() -> bool:
    try:
        text = fetch(f"{SITE}/install.sh").decode()
    except NotFoundError:
        return False
    return f"{SITE}/releases" in text and "latest.txt" in text


def package_run(tag: str, published: datetime) -> Obj:
    """Find the first Publish to Package Managers run started after the tag was published."""
    runs = api("actions/workflows/release-packages.yml/runs?per_page=30")["workflow_runs"]
    after = sorted((r for r in runs if when(r["created_at"]) >= published), key=lambda r: r["created_at"])
    tagged = [r for r in after if r.get("head_branch") == tag]
    run = (tagged or after or [None])[0]
    if run is None:
        failed, state = lagging(published, behind=True)
        return {"tag": tag, "run": None, "state": "no run" if failed else state, "failed": failed}
    if run["status"] != "completed":
        failed, state = lagging(published, behind=True)
        state = "still running past the grace" if failed else "running"
    else:
        failed = int(run["conclusion"] != "success")
        state = str(run["conclusion"])
    return {"tag": tag, "run": run["html_url"], "state": state, "failed": failed}


def auto_release_failures() -> Obj:
    path = f"actions/workflows/auto-release.yml/runs?status=completed&per_page={AUTO_RELEASE_RUNS}"
    runs = api(path)["workflow_runs"][:AUTO_RELEASE_RUNS]
    counted = [r for r in runs if r["conclusion"] != "cancelled"]
    failed = [r for r in counted if r["conclusion"] in FAILED_RUN]
    pct = round(100 * len(failed) / len(counted), 1) if counted else 0.0
    return {
        "runs": len(runs),
        "cancelled": len(runs) - len(counted),
        "failed": [r["html_url"] for r in failed],
        "failure_pct": pct,
    }


def main() -> int:
    rels = [r for r in api(f"releases?per_page={RELEASES + 5}") if not r["draft"] and not r["prerelease"]]
    rels = rels[:RELEASES]
    if not rels:
        print("release-health: no published releases", file=sys.stderr)
        return 1
    cosign = os.environ.get("RELEASE_HEALTH_COSIGN") or shutil.which("cosign")
    if cosign == "none":
        cosign = None
    with tempfile.TemporaryDirectory(prefix="treemand-release-health.") as tmp:
        rows = [check_release(r, cosign, Path(tmp)) for r in rels]

    latest = rows[0]["tag"]
    published = when(rows[0]["published_at"])
    tap = tap_version()
    mirror = mirror_latest()
    mirror_missing = [f for f in EXPECTED if not exists(f"{SITE}/releases/{latest}/{f}")]
    tap_lag, tap_state = lagging(published, tap != latest.removeprefix("v"))
    mirror_lag, mirror_state = lagging(published, mirror != latest)
    files_lag, files_state = lagging(published, mirror_missing)
    install_ok = install_script_ok()
    pkg = package_run(latest, published)
    auto = auto_release_failures()

    leads = [r["lead_time_min"] for r in rows if r["lead_time_min"] is not None]
    metrics: dict[str, float] = {
        "releases_checked": len(rows),
        "lead_time_min": statistics.median(leads) if leads else 0,
        "assets_missing": sum(len(r["assets_missing"]) for r in rows),
        "checksum_mismatches": sum(len(r["checksum_mismatches"]) for r in rows),
        "tap_lag": tap_lag,
        "mirror_lag": mirror_lag,
        "mirror_assets_missing": len(mirror_missing) if files_lag else 0,
        "install_script_missing": int(not install_ok),
        "package_publish_failed": pkg["failed"],
        "auto_release_failure_pct": auto["failure_pct"],
    }
    skipped: dict[str, str] = {}
    if cosign:
        metrics["signature_failures"] = sum(1 for r in rows if r["signature"] != "ok")
    else:
        skipped["signature_failures"] = "cosign is not installed"

    latest_row: Obj = {
        "latest": latest,
        "published_at": rows[0]["published_at"],
        "checked_at": now().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "tap": {"version": tap, "state": tap_state},
        "mirror": {"latest_txt": mirror, "state": mirror_state},
        "mirror_files": {"missing": mirror_missing, "state": files_state},
        "install_script": "ok" if install_ok else f"{SITE}/install.sh not served or not using {SITE}/releases",
        "package_publish": pkg,
        "auto_release": auto,
    }
    result: Obj = {"probe": "release-health", "metrics": metrics, "details": [latest_row, *rows]}
    if skipped:
        result["skipped"] = skipped
    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    sys.exit(main())

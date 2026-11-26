"""Write the worlds adoption_signals_test.sh's fake gh and file stand-ins serve.

Usage: python3 -I scripts/metrics/adoption_signals_test_world.py DIR

Writes into DIR:

- healthy.json: a GitHub API world (API path to response body) where traffic
  is readable and the latest release is two days old and downloaded.
- broken.json: the same world whose latest release is 8 days old with no
  downloads and whose traffic API is forbidden ({"__error": ...} bodies).
- proxy/: a module proxy stand-in whose @latest is v1.2.0.
- brew/formula/: an empty Homebrew API stand-in.
- history.jsonl: three earlier records for the week-over-week deltas.
"""

import json
import sys
from pathlib import Path
from typing import Any


def rel(tag: str, published: str, counts: list[int], *, draft: bool = False, pre: bool = False) -> dict[str, Any]:
    return {
        "tag_name": tag,
        "published_at": published,
        "draft": draft,
        "prerelease": pre,
        "assets": [{"name": f"a{i}.tar.gz", "download_count": c} for i, c in enumerate(counts)],
    }


def issue(n: int, assoc: str, kind: str = "User", *, pr: bool = False) -> dict[str, Any]:
    i: dict[str, Any] = {"number": n, "author_association": assoc, "user": {"login": f"person{n}", "type": kind}}
    if pr:
        i["pull_request"] = {}
    return i


def worlds() -> tuple[dict[str, Any], dict[str, Any]]:
    issues = [
        issue(1, "OWNER"),
        issue(2, "NONE"),
        issue(3, "CONTRIBUTOR"),
        issue(4, "NONE", kind="Bot"),
        issue(5, "NONE", pr=True),
        issue(6, "COLLABORATOR"),
    ]
    repo = {"full_name": "acme/tool", "stargazers_count": 12, "forks_count": 3, "subscribers_count": 2}
    healthy: dict[str, Any] = {
        "repos/acme/tool": repo,
        "repos/acme/tool/releases": [
            rel("v1.3.0-rc1", "2026-03-09T00:00:00Z", [50], pre=True),
            rel("v1.2.0", "2026-03-08T00:00:00Z", [4, 6])
            | {
                "assets": [
                    {"name": "a0.tar.gz", "download_count": 4},
                    {"name": "a1.zip", "download_count": 6},
                    {"name": "checksums.txt", "download_count": 9},
                    {"name": "checksums.txt.sigstore.json", "download_count": 9},
                ]
            },
            rel("v1.3.0", "2026-03-11T00:00:00Z", [99], draft=True),
            rel("v1.1.0", "2026-02-01T00:00:00Z", [30, 10]),
        ],
        "repos/acme/tool/issues?state=open": issues,
        "repos/acme/tool/traffic/views": {"count": 40, "uniques": 9},
        "repos/acme/tool/traffic/clones": {"count": 300, "uniques": 70},
        "repos/acme/tool/traffic/popular/referrers": [
            {"referrer": "news.example", "count": 7, "uniques": 5},
            {"referrer": "github.com", "count": 3, "uniques": 2},
        ],
        "repos/acme/homebrew-tap/traffic/clones": {"count": 25, "uniques": 11},
    }
    broken = dict(healthy)
    broken["repos/acme/tool/releases"] = [
        rel("v1.2.0", "2026-03-02T00:00:00Z", [0, 0]),
        rel("v1.1.0", "2026-02-01T00:00:00Z", [5]),
    ]
    for k in ("views", "clones", "popular/referrers"):
        broken[f"repos/acme/tool/traffic/{k}"] = {"__error": "gh: Must have push access to repository (HTTP 403)"}
    return healthy, broken


def main(tmp: Path) -> None:
    healthy, broken = worlds()
    (tmp / "healthy.json").write_text(json.dumps(healthy))
    (tmp / "broken.json").write_text(json.dumps(broken))
    (tmp / "proxy/github.com/acme/tool/@v").mkdir(parents=True)
    (tmp / "proxy/github.com/acme/tool/@latest").write_text(json.dumps({"Version": "v1.2.0"}))
    (tmp / "brew/formula").mkdir(parents=True)
    with (tmp / "history.jsonl").open("w") as f:
        for date, stars in (("2026-03-01T00:00:00Z", 8), ("2026-03-02T12:00:00Z", 10), ("2026-03-08T00:00:00Z", 11)):
            record = {"date": date, "host": "github-api", "metrics": {"stars": stars, "downloads_total": 20}}
            f.write(json.dumps(record) + "\n")


if __name__ == "__main__":
    main(Path(sys.argv[1]))

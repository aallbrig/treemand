"""Write the Actions API world that ci_health_test.sh's fake gh serves.

Usage: python3 -I scripts/metrics/ci_health_test_world.py WORLD_JSON

Writes WORLD_JSON: a map from API path (runs keyed with their ?branch= filter)
to the response body. The world holds one of each case the ci-health probe
counts: a re-run that turned a failure green, a main commit that failed then
passed in a later run, a startup failure, a run with no jobs, a cancelled run,
and a dynamic workflow that must be left out.
"""

import json
import sys
from pathlib import Path
from typing import Any


def run(  # noqa: PLR0913 - one field per argument keeps the world readable
    i: int,
    event: str,
    branch: str,
    conclusion: str,
    created: str,
    *,
    started: str | None = None,
    updated: str | None = None,
    attempt: int = 1,
    sha: str | None = None,
) -> dict[str, Any]:
    return {
        "id": i,
        "event": event,
        "head_branch": branch,
        "conclusion": conclusion,
        "run_attempt": attempt,
        "head_sha": sha or f"{i:040d}",
        "created_at": created,
        "run_started_at": started or created,
        "updated_at": updated or created,
    }


def job(  # noqa: PLR0913, PLR0917 - one field per argument keeps the world readable
    name: str, attempt: int, conclusion: str, created: str, started: str, completed: str
) -> dict[str, Any]:
    return {
        "name": name,
        "run_attempt": attempt,
        "conclusion": conclusion,
        "created_at": created,
        "started_at": started,
        "completed_at": completed,
    }


def world() -> dict[str, Any]:
    d = "2026-01-01T"
    same = "f" * 40
    r1 = run(1, "push", "main", "success", d + "00:00:00Z", updated=d + "00:10:00Z")
    r2 = run(2, "pull_request", "topic", "success", d + "01:00:00Z", updated=d + "01:05:00Z", attempt=2)
    r3 = run(3, "pull_request", "topic", "startup_failure", d + "01:30:00Z")
    r4 = run(4, "pull_request", "topic", "failure", d + "01:40:00Z")
    r5 = run(5, "push", "main", "failure", d + "02:00:00Z", updated=d + "02:03:00Z", sha=same)
    r6 = run(6, "push", "main", "success", d + "03:00:00Z", updated=d + "03:07:00Z", sha=same)
    r7 = run(7, "push", "main", "success", d + "04:00:00Z", updated=d + "04:08:00Z")
    r8 = run(8, "push", "main", "skipped", d + "05:00:00Z")
    demo = [
        run(10 + i, "push", "main", c, d + f"0{i}:30:00Z")
        for i, c in enumerate(["success", "success", "failure", "success", "cancelled"])
    ]
    api = "repos/{owner}/{repo}/actions"
    return {
        f"{api}/workflows": {
            "workflows": [
                {"id": 100, "path": ".github/workflows/ci.yml", "state": "active"},
                {"id": 200, "path": ".github/workflows/demo.yml", "state": "active"},
                {"id": 300, "path": "dynamic/dependabot/dependabot-updates", "state": "active"},
            ]
        },
        f"{api}/workflows/100/runs?branch=": {"workflow_runs": [r6, r5, r4, r3, r2, r1]},
        f"{api}/workflows/100/runs?branch=main": {"workflow_runs": [r8, r7, r6, r5, r1]},
        f"{api}/workflows/200/runs?branch=": {"workflow_runs": demo},
        f"{api}/runs/1/jobs": {
            "jobs": [
                job("Test", 1, "success", d + "00:00:00Z", d + "00:00:05Z", d + "00:07:00Z"),
                job("Build", 1, "success", d + "00:00:00Z", d + "00:00:20Z", d + "00:01:00Z"),
            ]
        },
        f"{api}/runs/2/jobs": {
            "jobs": [
                job("Test", 1, "failure", d + "00:50:00Z", d + "00:50:05Z", d + "00:55:00Z"),
                job("Test", 2, "success", d + "01:00:00Z", d + "01:00:05Z", d + "01:04:05Z"),
                job("Lint", 2, "skipped", d + "01:00:00Z", d + "01:00:00Z", d + "01:00:00Z"),
            ]
        },
        f"{api}/runs/3/jobs": {"jobs": []},
        f"{api}/runs/4/jobs": {"jobs": []},
        f"{api}/runs/5/jobs": {"jobs": [job("Test", 1, "failure", d + "02:00:00Z", d + "02:00:05Z", d + "02:03:00Z")]},
        f"{api}/runs/6/jobs": {"jobs": [job("Test", 1, "success", d + "03:00:00Z", d + "03:00:05Z", d + "03:06:05Z")]},
    }


if __name__ == "__main__":
    Path(sys.argv[1]).write_text(json.dumps(world()))

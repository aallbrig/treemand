"""Check the site-health probe's result on site_health_test.sh's fixture site.

Usage: python3 -I scripts/metrics/site_health_test_verify.py RESULT_JSON

RESULT_JSON is what scripts/metrics/probes/site-health.sh printed for the
fixture. Prints one ok/FAIL line per expected metric and for the broken links
named on the home page, and exits 1 when any check fails.
"""

import json
import sys
from pathlib import Path

WANT = {
    "pages": 3,
    "http_errors": 1,
    "console_errors": 1,
    "js_exceptions": 1,
    "broken_internal_links": 2,
    "broken_external_links": 1,
    "mixed_content": 1,
}


def main(result: Path) -> int:
    got = json.loads(result.read_text())
    fail = False
    for k, v in WANT.items():
        ok = got["metrics"].get(k + ".local") == v
        fail = fail or not ok
        print(f"  {'ok  ' if ok else 'FAIL'} {k}.local = {got['metrics'].get(k + '.local')} (want {v})")
    if got["probe"] != "site-health":
        print(f"  FAIL probe name {got['probe']!r}")
        return 1
    home = next(d for d in got["details"] if d.get("page") == "/")
    hrefs = sorted(b["href"] for b in home["broken_links"])
    ok = hrefs[:2] == ["/good/#nope", "/missing/"]
    fail = fail or not ok
    print(f"  {'ok  ' if ok else 'FAIL'} broken links named on the linking page: {hrefs}")
    return 1 if fail else 0


if __name__ == "__main__":
    sys.exit(main(Path(sys.argv[1])))

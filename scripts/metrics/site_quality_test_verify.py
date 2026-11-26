"""Check the site-quality probe's result on site_quality_test.sh's fixture site.

Usage: python3 -I scripts/metrics/site_quality_test_verify.py RESULT_JSON

RESULT_JSON is what scripts/metrics/probes/site-quality.sh printed for the
fixture's bad and clean pages with the stand-in Lighthouse. Prints one ok/FAIL
line per check; when any fails, prints the metrics to stderr and exits 1.
"""

import json
import sys
from pathlib import Path
from typing import Any


def checks_for(got: dict[str, Any]) -> list[tuple[str, bool]]:
    m = got["metrics"]
    checks: list[tuple[str, bool]] = []
    for ff in ("mobile", "desktop"):
        bad, clean = f"local.bad.{ff}", f"local.clean.{ff}"
        checks += [
            (f"axe_critical.{bad} == 2", m.get(f"axe_critical.{bad}") == 2),  # noqa: PLR2004
            (f"axe_serious.{bad} == 1", m.get(f"axe_serious.{bad}") == 1),
            (f"axe_critical.{clean} == 0", m.get(f"axe_critical.{clean}") == 0),
            (f"axe_serious.{clean} == 0", m.get(f"axe_serious.{clean}") == 0),
            (f"cls.{bad} > 0.05", m.get(f"cls.{bad}", 0) > 0.05),  # noqa: PLR2004
            (f"cls.{clean} == 0", m.get(f"cls.{clean}") == 0),
            (
                f"40000 <= weight_bytes.{bad} < 60000",
                40000 <= m.get(f"weight_bytes.{bad}", 0) < 60000,  # noqa: PLR2004
            ),
            (f"weight_bytes.{clean} < 2000", 0 < m.get(f"weight_bytes.{clean}", 0) < 2000),  # noqa: PLR2004
            (f"lh_accessibility.{bad} == 80", m.get(f"lh_accessibility.{bad}") == 80),  # noqa: PLR2004
            (f"lh_seo.{clean} == 92", m.get(f"lh_seo.{clean}") == 92),  # noqa: PLR2004
            (f"lh_best_practices.{clean} == 100", m.get(f"lh_best_practices.{clean}") == 100),  # noqa: PLR2004
        ]
    checks += [
        (
            "lh_performance.local.bad.mobile == 60 (median of 30, 90, 60)",
            m.get("lh_performance.local.bad.mobile") == 60,  # noqa: PLR2004
        ),
        ("lh_performance.local.bad.desktop == 99", m.get("lh_performance.local.bad.desktop") == 99),  # noqa: PLR2004
    ]
    bad_page = next(d for d in got["details"] if d.get("page") == "bad" and d.get("form_factor") == "mobile")
    ids = sorted(v["id"] for v in bad_page["axe"] if v["impact"] in ("critical", "serious"))
    failing = [a["id"] for a in bad_page["lighthouse"]["failing"].get("accessibility", [])]
    checks += [
        ("axe ids named in details", ids == ["button-name", "color-contrast", "image-alt"]),
        ("lighthouse failing audits named in details", failing == ["image-alt"]),
        ("probe name", got["probe"] == "site-quality"),
    ]
    return checks


def main(result: Path) -> int:
    got = json.loads(result.read_text())
    fail = False
    for name, ok in checks_for(got):
        fail = fail or not ok
        print(f"  {'ok  ' if ok else 'FAIL'} {name}")
    if fail:
        print(json.dumps(got["metrics"], indent=1), file=sys.stderr)
    return 1 if fail else 0


if __name__ == "__main__":
    sys.exit(main(Path(sys.argv[1])))

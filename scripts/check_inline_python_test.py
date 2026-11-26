"""Tests for check_inline_python.py: each planted form is caught, allowed forms pass.

python3 -I scripts/check_inline_python_test.py
"""

from __future__ import annotations

import importlib.util
import sys
import tempfile
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from types import ModuleType

CAUGHT = {
    "python3 -I - \"$work\" <<'PY'": "a heredoc fed to Python",
    'cat >"$tmp/x" <<PY': "a heredoc fed to Python",
    "python3 -c 'print(1)'": "python3 without -I",
    "python3 -m http.server 0": "python3 without -I",
    'python3 "$repo/x.py"': "python3 without -I",
    "python3 -I -c 'import json": "a -c program that spans lines",
    'x | python3 -I -u -c "import sys': "a -c program that spans lines",
    "python3 -I - a b": "Python read from stdin",
}
ALLOWED = [
    "python3 -I -c 'print(1)' arg",
    'python3 -I -c "import json,sys; print(json.load(sys.stdin)$1)"',
    "python3 -I -u -m http.server --bind 127.0.0.1 0",
    'python3 -I "$repo/scripts/metrics/perf_report.py" nodes "$1"',
    "command -v python3 >/dev/null",
    "python@3.12) n=python3 ;;",
    "# python3 - <<'PY' in a comment",
]


def load_check() -> ModuleType:
    """Import scripts/check_inline_python.py as a module."""
    spec = importlib.util.spec_from_file_location(
        "check_inline_python", Path(__file__).with_name("check_inline_python.py")
    )
    if spec is None or spec.loader is None:
        msg = "cannot load check_inline_python.py"
        raise SystemExit(msg)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def main() -> int:
    check = load_check()
    cases = [(line, [why]) for line, why in CAUGHT.items()] + [(line, []) for line in ALLOWED]
    failures = 0
    with tempfile.TemporaryDirectory() as d:
        path = Path(d) / "t.sh"
        for line, want in cases:
            path.write_text(line + "\n", encoding="utf-8")
            got = [p.split(": ", 1)[1] for p in check.problems(path)]
            if got != want:
                print(f"FAIL {line!r}: got {got}, want {want}")
                failures += 1
    print(f"check_inline_python_test: {len(cases) - failures}/{len(cases)} ok")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())

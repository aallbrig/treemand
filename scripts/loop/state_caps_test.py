"""Daily caps: 0 (the default) means no cap; a positive cap still halts the loop."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from types import ModuleType


def load_state() -> ModuleType:
    """Import scripts/loop/state.py as a module."""
    path = Path(__file__).with_name("state.py")
    spec = importlib.util.spec_from_file_location("loop_state", path)
    if spec is None or spec.loader is None:
        msg = "cannot load state.py"
        raise SystemExit(msg)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def main() -> int:
    """Check budget_hit and cap_label for capped and uncapped configs."""
    m = load_state()
    cases = [
        (0, 0, ""),
        (1, 0, "daily cycle cap reached (5)"),
        (0, 9, "daily release cap reached (9)"),
        (10, 10, ""),
    ]
    failures = 0
    for cycles_cap, releases_cap, want in cases:
        m.CFG["LOOP_MAX_CYCLES_PER_DAY"] = str(cycles_cap)
        m.CFG["LOOP_MAX_RELEASES_PER_DAY"] = str(releases_cap)
        state: dict[str, object] = {"days": {}}
        today = m.day(state)
        today["cycles"] = 5
        today["releases"] = 9
        got = m.budget_hit(state)
        if got != want:
            failures += 1
            print(f"  FAIL caps {cycles_cap}/{releases_cap}: want {want!r}, got {got!r}")
    m.CFG["LOOP_MAX_CYCLES_PER_DAY"] = "0"
    if m.cap_label("LOOP_MAX_CYCLES_PER_DAY") != "no cap":
        failures += 1
        print("  FAIL cap_label(0) should read 'no cap'")
    if failures:
        return 1
    print("state_caps_test: all passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())

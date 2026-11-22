#!/usr/bin/env python3
"""Machine state for the treemand dev loop: .loop/state.json.

Commands (all print to stdout; exit codes are documented per command):
  summary                  human-readable status
  budget                   exit 0 if under daily caps, 10 if a cap is hit (reason on stdout)
  breaker                  exit 0 if closed, 11 if open (reason on stdout)
  begin-cycle              start a cycle (increments counters), prints the cycle number
  end-cycle OUTCOME        released | idle | parked | failed | stopped; writes .loop/cycle-done
  pick-area                print the least-recently-touched focus area
  touch-area AREA          record that AREA was worked this cycle
  escalation               exit 0 and print reason if a phone escalation is due, else exit 1
  mark-escalated           record that the escalation was sent
  request-escalation MSG   flag an escalation (e.g. revert failed)
"""

import json
import re
import sys
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

type State = dict[str, Any]

ROOT = Path(__file__).resolve().parents[2]
LOOP_DIR = ROOT / ".loop"
STATE = LOOP_DIR / "state.json"
# The supervisor (scripts/loop/supervisor.sh) waits for this marker to start the next cycle.
CYCLE_DONE = LOOP_DIR / "cycle-done"

AREAS = [
    "tui-cli-ux",
    "discovery-quality",
    "website-design",
    "docs-try-page",
    "performance-reliability",
    "test-quality-security",
    "packaging-distribution",
    "mcp-api-exploration",
]


def load_config() -> dict[str, str]:
    cfg: dict[str, str] = {}
    for path in (ROOT / "scripts/loop/config.defaults.env", LOOP_DIR / "config.env"):
        if not path.exists():
            continue
        for line in path.read_text().splitlines():
            m = re.match(r"^\s*([A-Z_]+)=([^\s#]*)", line)
            if m:
                cfg[m.group(1)] = m.group(2).strip("\"'")
    return cfg


CFG = load_config()


def cfg_int(key: str) -> int:
    return int(CFG[key])


def today() -> str:
    return datetime.now(UTC).strftime("%Y-%m-%d")


def now() -> str:
    return datetime.now(UTC).strftime("%Y-%m-%dT%H:%M:%SZ")


def load() -> State:
    LOOP_DIR.mkdir(exist_ok=True)
    state: State = {}
    if STATE.exists():
        state = json.loads(STATE.read_text())
    state.setdefault("cycle", 0)
    state.setdefault("failure_streak", 0)
    state.setdefault("escalated_at_streak", 0)
    state.setdefault("pending_escalation", "")
    state.setdefault("last_outcome", "")
    state.setdefault("areas", {})
    state.setdefault("days", {})
    state.setdefault("last_release", "")
    for a in AREAS:
        state["areas"].setdefault(a, "")
    return state


def save(state: State) -> None:
    tmp = STATE.with_suffix(".tmp")
    tmp.write_text(json.dumps(state, indent=2, sort_keys=True) + "\n")
    tmp.replace(STATE)


def day(state: State) -> dict[str, int]:
    days: dict[str, dict[str, int]] = state["days"]
    return days.setdefault(today(), {"cycles": 0, "releases": 0})


def cmd_summary(state: State) -> int:
    d = day(state)
    print(f"cycle: {state['cycle']}  last outcome: {state['last_outcome'] or '-'}")
    print(
        f"today: {d['cycles']}/{cfg_int('LOOP_MAX_CYCLES_PER_DAY')} cycles, "
        f"{d['releases']}/{cfg_int('LOOP_MAX_RELEASES_PER_DAY')} releases"
    )
    print(
        f"failure streak: {state['failure_streak']} "
        f"(breaker at {cfg_int('LOOP_BREAKER_THRESHOLD')})  last release: {state['last_release'] or '-'}"
    )
    if state["pending_escalation"]:
        print(f"PENDING ESCALATION: {state['pending_escalation']}")
    return 0


def budget_hit(state: State) -> str:
    d = day(state)
    if d["cycles"] >= cfg_int("LOOP_MAX_CYCLES_PER_DAY"):
        return f"daily cycle cap reached ({d['cycles']})"
    if d["releases"] >= cfg_int("LOOP_MAX_RELEASES_PER_DAY"):
        return f"daily release cap reached ({d['releases']})"
    return ""


def breaker_open(state: State) -> str:
    if state["failure_streak"] >= cfg_int("LOOP_BREAKER_THRESHOLD"):
        return f"breaker open: {state['failure_streak']} consecutive failed cycles"
    return ""


def cmd_budget(state: State) -> int:
    why = budget_hit(state)
    if why:
        print(why)
        return 10
    return 0


def cmd_breaker(state: State) -> int:
    why = breaker_open(state)
    if why:
        print(why)
        return 11
    return 0


def cmd_begin(state: State) -> int:
    state["cycle"] += 1
    day(state)["cycles"] += 1
    state["cycle_started"] = now()
    save(state)
    print(state["cycle"])
    return 0


def cmd_end(state: State, outcome: str) -> int:
    if outcome not in {"released", "idle", "parked", "failed", "stopped"}:
        print("outcome must be released|idle|parked|failed|stopped", file=sys.stderr)
        return 2
    state["last_outcome"] = outcome
    if outcome == "failed":
        state["failure_streak"] += 1
    elif outcome == "released":
        state["failure_streak"] = 0
        state["escalated_at_streak"] = 0
        state["pending_escalation"] = ""
        day(state)["releases"] += 1
        state["last_release"] = now()
    # idle/parked/stopped neither help nor hurt the streak: nothing shipped, nothing broke.
    save(state)
    CYCLE_DONE.write_text(f"{outcome} {now()}\n")
    return 0


def cmd_pick(state: State) -> int:
    # Never-touched areas (empty string) sort first; then oldest touch.
    print(min(AREAS, key=lambda a: (state["areas"][a] != "", state["areas"][a])))
    return 0


def cmd_touch(state: State, area: str) -> int:
    if area not in AREAS:
        print(f"unknown area {area}; one of: {', '.join(AREAS)}", file=sys.stderr)
        return 2
    state["areas"][area] = now()
    save(state)
    return 0


def cmd_escalation(state: State) -> int:
    threshold = cfg_int("LOOP_BREAKER_THRESHOLD")
    streak = state["failure_streak"]
    # Three breaker trips in a row (threshold failures each) means the loop can no longer ship.
    if streak >= 3 * threshold and state["escalated_at_streak"] < streak:
        print(f"{streak} consecutive failed cycles: the loop cannot ship")
        return 0
    if state["pending_escalation"]:
        print(state["pending_escalation"])
        return 0
    return 1


def cmd_mark_escalated(state: State) -> int:
    state["escalated_at_streak"] = state["failure_streak"]
    state["pending_escalation"] = ""
    save(state)
    return 0


def cmd_request(state: State, msg: str) -> int:
    state["pending_escalation"] = msg
    save(state)
    return 0


def main(argv: list[str]) -> int:
    if not argv[1:]:
        print(__doc__)
        return 2
    state = load()
    cmd, args = argv[1], argv[2:]
    simple = {
        "summary": cmd_summary,
        "budget": cmd_budget,
        "breaker": cmd_breaker,
        "begin-cycle": cmd_begin,
        "pick-area": cmd_pick,
        "escalation": cmd_escalation,
        "mark-escalated": cmd_mark_escalated,
    }
    if cmd in simple:
        # `budget`/`breaker` may touch days{} via day(); persist only via explicit saves.
        return simple[cmd](state)
    with_arg = {
        "end-cycle": cmd_end,
        "touch-area": cmd_touch,
        "request-escalation": cmd_request,
    }
    if cmd in with_arg and args:
        return with_arg[cmd](state, " ".join(args))
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))

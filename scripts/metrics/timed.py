"""Run a command and report its cost, the same way on Linux and macOS.

    python3 -I scripts/metrics/timed.py OUT CWD -- CMD [ARG...]

Runs CMD in directory CWD with stdout and stderr inherited, then writes one
line to OUT: "<wall_s> <user_s> <sys_s> <maxrss_kb>". User and system time
include the command's own child processes. This replaces GNU
`/usr/bin/time -f "%e %U %S %M"` plus `env -C`, neither of which exists on
macOS. The exit status is CMD's, so callers can ignore failures with `|| true`.
"""

from __future__ import annotations

import resource
import subprocess
import sys
import time
from pathlib import Path

USAGE = "usage: timed.py OUT CWD -- CMD [ARG...]"
# ru_maxrss is kilobytes on Linux but bytes on macOS.
RSS_DIVISOR = 1024 if sys.platform == "darwin" else 1


def main(argv: list[str]) -> int:
    """Run the command in argv and write its cost line; return its exit code."""
    if len(argv) < 4 or argv[2] != "--":  # noqa: PLR2004 - OUT CWD -- CMD
        sys.stderr.write(USAGE + "\n")
        return 2
    out, cwd, cmd = argv[0], argv[1], argv[3:]
    before = resource.getrusage(resource.RUSAGE_CHILDREN)
    start = time.monotonic()
    code = subprocess.call(cmd, cwd=cwd)
    wall = time.monotonic() - start
    after = resource.getrusage(resource.RUSAGE_CHILDREN)
    user = after.ru_utime - before.ru_utime
    system = after.ru_stime - before.ru_stime
    rss_kb = after.ru_maxrss // RSS_DIVISOR
    Path(out).write_text(f"{wall:.2f} {user:.2f} {system:.2f} {rss_kb}\n", encoding="utf-8")
    return code


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

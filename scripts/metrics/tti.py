#!/usr/bin/env python3
"""tti.py <treemand-binary> <cli>: seconds until `treemand -i <cli>` shows its tree.

Drives the TUI on a pseudo-terminal (answering the terminal queries Bubble Tea
sends), with an empty cache from TREEMAND_CACHE_DIR, and prints the seconds
until the tree's "Subcommands" section is on screen, or -1 if it never was.
"""

import fcntl
import os
import pty
import re
import select
import signal
import struct
import sys
import termios
import time

# Give up waiting for the tree after this many seconds.
DEADLINE_S = 60
# Terminal query -> the reply a real terminal would send (cursor position,
# background colour, foreground colour).
REPLIES = {
    b"\x1b[6n": b"\x1b[1;1R",
    b"\x1b]11;?": b"\x1b]11;rgb:0000/0000/0000\x07",
    b"\x1b]10;?": b"\x1b]10;rgb:ffff/ffff/ffff\x07",
}
ANSI = re.compile(rb"\x1b\[[0-9;?]*[ -/]*[@-~]")


def wait_for_tree(fd: int) -> float:
    """Return seconds until the tree is on screen, or -1.0 if it never was."""
    t0, buf = time.time(), b""
    while time.time() - t0 < DEADLINE_S:
        ready, _, _ = select.select([fd], [], [], 0.05)
        if not ready:
            continue
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            break
        buf += chunk
        for query, reply in REPLIES.items():
            if query in chunk:
                os.write(fd, reply)
        if b"Subcommands" in ANSI.sub(b"", buf[-20000:]):
            return time.time() - t0
    return -1.0


def main(binp: str, cli: str) -> None:
    pid, fd = pty.fork()
    if pid == 0:
        os.execv(binp, [binp, "-i", "--timeout=120", cli])  # noqa: S606 - the child becomes treemand
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 140, 0, 0))
    tree = wait_for_tree(fd)
    try:
        os.write(fd, b"q")
        time.sleep(0.2)
        os.kill(pid, signal.SIGKILL)
    except (OSError, ProcessLookupError):
        pass
    print(f"{tree:.2f}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])

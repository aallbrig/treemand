#!/usr/bin/env python3
"""tti.py <treemand-binary> <cli>: seconds until `treemand -i <cli>` shows its tree.

Drives the TUI on a pseudo-terminal (answering the terminal queries Bubble Tea
sends), with an empty cache from TREEMAND_CACHE_DIR, and prints the seconds
until the tree's "Subcommands" section is on screen, or -1 if it never was.
"""
import os, pty, re, select, signal, sys, time, fcntl, termios, struct

binp, cli = sys.argv[1], sys.argv[2]
pid, fd = pty.fork()
if pid == 0:
    os.execv(binp, [binp, "-i", "--timeout=120", cli])
fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 140, 0, 0))
t0, buf, tree = time.time(), b"", -1.0
ansi = re.compile(rb"\x1b\[[0-9;?]*[ -/]*[@-~]")
while time.time() - t0 < 60:
    ready, _, _ = select.select([fd], [], [], 0.05)
    if not ready:
        continue
    try:
        chunk = os.read(fd, 65536)
    except OSError:
        break
    buf += chunk
    if b"\x1b[6n" in chunk: os.write(fd, b"\x1b[1;1R")
    if b"\x1b]11;?" in chunk: os.write(fd, b"\x1b]11;rgb:0000/0000/0000\x07")
    if b"\x1b]10;?" in chunk: os.write(fd, b"\x1b]10;rgb:ffff/ffff/ffff\x07")
    if b"Subcommands" in ansi.sub(b"", buf[-20000:]):
        tree = time.time() - t0
        break
try:
    os.write(fd, b"q")
    time.sleep(0.2)
    os.kill(pid, signal.SIGKILL)
except (OSError, ProcessLookupError):
    pass
print(f"{tree:.2f}")

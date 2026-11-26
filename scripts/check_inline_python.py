"""Fail when a shell script under scripts/ embeds Python that ruff and mypy cannot see.

    python3 -I scripts/check_inline_python.py [FILE...]

With no FILE, checks every *.sh under scripts/ that git tracks. Python in a
shell script belongs in a typed module run as `python3 -I <file>`; what stays
inline is a `python3 -I -c '...'` one-liner whose quote closes on the same
line. Reported, one per line as FILE:LINE: why:
  - a heredoc fed to Python (`<<'PY'`, or any heredoc on a python3 line)
  - Python read from stdin (`python3 -`, or `python3 -I` with no program)
  - python3 run without -I (the script's directory and cwd would be importable)
  - a `-c` program that spans lines
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

HEREDOC = re.compile(r"<<-?\s*['\"]?PY\b")
CALL = re.compile(r"\bpython3\s+(?P<rest>.*)")
# What may follow -I before the program: other single-letter interpreter flags
# (-u, -B), but not -c or -m, which start it.
FLAG = re.compile(r"-[A-Zabd-ln-z]\s")
# The start of an argument (a flag, path, variable or quoted word), as opposed
# to a redirection, pipe or `;` after a bare mention such as `command -v python3`.
ARGUMENT = re.compile(r"""[-A-Za-z0-9"'$./~]""")


def call_problem(rest: str) -> str | None:
    """Say what is wrong with one python3 invocation, given the text after `python3 `."""
    if not rest.startswith("-I"):
        return "python3 without -I" if ARGUMENT.match(rest) else None
    after = rest[2:].lstrip()
    while FLAG.match(after):
        after = after[2:].lstrip()
    if not after or after == "-" or after.startswith("- ") or after[0] in "|&;<>)":
        return "Python read from stdin"
    if after.startswith("-c "):
        program = after[3:].lstrip()
        quote = program[:1]
        if quote in {"'", '"'} and quote not in program[1:]:
            return "a -c program that spans lines"
    return None


def problems(path: Path) -> list[str]:
    found = []
    for n, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
        if line.lstrip().startswith("#"):
            continue
        if HEREDOC.search(line) or any("<<" in m.group("rest") for m in CALL.finditer(line)):
            found.append(f"{path}:{n}: a heredoc fed to Python")
            continue
        for m in CALL.finditer(line):
            why = call_problem(m.group("rest"))
            if why:
                found.append(f"{path}:{n}: {why}")
    return found


def tracked_scripts() -> list[Path]:
    out = subprocess.run(["git", "ls-files", "scripts/*.sh"], capture_output=True, text=True, check=True).stdout
    return [Path(p) for p in out.splitlines()]


def main(argv: list[str]) -> int:
    paths = [Path(a) for a in argv] or tracked_scripts()
    found = [p for path in paths for p in problems(path)]
    for line in found:
        print(line)
    if found:
        print("check_inline_python: move each into a typed module under scripts/, run as `python3 -I <file>`")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

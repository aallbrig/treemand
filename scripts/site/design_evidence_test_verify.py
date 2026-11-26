"""Check design_evidence.sh's output on design_evidence_test.sh's fixture.

Usage: python3 -I scripts/site/design_evidence_test_verify.py OUT_DIR

OUT_DIR is the --out directory design_evidence.sh wrote for the two-page
fixture site and the one-page peer. Checks the manifest, the first-screen
tiles of every page in all four views (PNG widths), the hover records, the
tiling of the tall page, that dark differs from light, and index.md. Prints
one ok/FAIL line per check and exits 1 when any fails.
"""

import json
import struct
import sys
from pathlib import Path

PNG_MAGIC = b"\x89PNG\r\n\x1a\n"
DESKTOP_WIDTH = 1280
MOBILE_WIDTH = 390
TILE_CAP = 4


class Checks:
    def __init__(self) -> None:
        self.failed = False

    def check(self, ok: bool, what: str) -> None:  # noqa: FBT001 - reads as check(condition, label)
        self.failed = self.failed or not ok
        print(f"  {'ok  ' if ok else 'FAIL'} {what}")


def width(p: Path) -> int:
    head = p.read_bytes()[:24]
    if head[:8] != PNG_MAGIC:
        return -1
    w: int = struct.unpack(">I", head[16:20])[0]
    return w


def main(out: Path) -> int:
    c = Checks()
    manifest = json.loads((out / "manifest.json").read_text())
    pages = {(r["site"], r["page"]): r for r in manifest["pages"]}
    c.check(
        set(pages) == {("treemand", "home"), ("treemand", "docs"), ("peerco", "home")},
        f"pages captured: {sorted(pages)}",
    )
    for (site, page), rec in pages.items():
        c.check(rec["status"] == 200, f"{site}/{page} status {rec['status']}")  # noqa: PLR2004
        for view in ("desktop-light", "desktop-dark", "mobile-light", "mobile-dark"):
            first = out / site / f"{page}--{view}-1.png"
            want = DESKTOP_WIDTH if view.startswith("desktop") else MOBILE_WIDTH
            c.check(first.is_file() and width(first) == want, f"{first.relative_to(out)} is a {want}px-wide PNG")
    home = pages[("treemand", "home")]["views"]["desktop-light"]["tiles"]
    # Mobile views are touch everywhere. Desktop hover follows the host: Chrome is
    # asked for a mouse, but some headless Linux hosts still report (hover: none),
    # so the manifest records it per view and only its presence is checked here.
    for view, rec in pages[("treemand", "home")]["views"].items():
        if view.startswith("mobile"):
            c.check(rec.get("hover_none") is True, f"{view} renders as touch (hover: none)")
        else:
            c.check(isinstance(rec.get("hover_none"), bool), f"{view} records hover: none = {rec.get('hover_none')}")
    c.check(len(home) == TILE_CAP, f"tall page tiled to the cap: {len(home)} tiles")
    c.check(len(pages[("treemand", "docs")]["views"]["desktop-light"]["tiles"]) == 1, "short page is one tile")
    light = (out / "treemand" / "home--desktop-light-1.png").read_bytes()
    dark = (out / "treemand" / "home--desktop-dark-1.png").read_bytes()
    c.check(light != dark, "dark view differs from light")
    c.check("treemand/home--desktop-light-1.png" in (out / "index.md").read_text(), "index.md links first screens")
    return 1 if c.failed else 0


if __name__ == "__main__":
    sys.exit(main(Path(sys.argv[1])))

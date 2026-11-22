#!/bin/sh
# Behaviour test for scripts/site/design_evidence.sh: a two-page fixture site
# (one tall, both with a dark theme) and a one-page "peer" served locally must
# produce first-screen tiles for every page in all four views, at the right
# widths, with dark differing from light, plus the manifest and index. Needs
# node 22+ and Chrome, like the site drivers.
set -eu
root="$(cd "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
peer_pid=""
cleanup() { [ -z "$peer_pid" ] || kill "$peer_pid" 2>/dev/null || true; rm -rf "$work"; }
trap cleanup EXIT INT TERM

style='<style>:root{color-scheme:light dark}body{margin:0;background:#fff;color:#111}@media (prefers-color-scheme: dark){body{background:#0d1117;color:#eee}}</style>'
site="$work/site"
mkdir -p "$site/docs"
cat >"$site/index.html" <<EOF
<!doctype html><html lang="en"><meta name="viewport" content="width=device-width"><link rel="icon" href="data:,">$style<title>home</title>
<h1>fixture</h1><div style="height:2600px"></div><p>bottom</p>
EOF
cat >"$site/docs/index.html" <<EOF
<!doctype html><html lang="en"><meta name="viewport" content="width=device-width"><link rel="icon" href="data:,">$style<title>docs</title><h1>docs</h1>
EOF
cat >"$site/sitemap.xml" <<'EOF'
<?xml version="1.0" encoding="utf-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<url><loc>https://treemand.dev/</loc></url>
<url><loc>https://treemand.dev/docs/</loc></url>
</urlset>
EOF

mkdir -p "$work/peer"
echo "<!doctype html><html lang=\"en\">$style<title>peer</title><h1>peer</h1>" >"$work/peer/index.html"
peer_port="$(python3 -I -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
python3 -I -m http.server "$peer_port" --bind 127.0.0.1 --directory "$work/peer" >/dev/null 2>&1 &
peer_pid=$!
printf '# comment\npeerco\thome\thttp://127.0.0.1:%s/\n' "$peer_port" >"$work/peers.tsv"

out="$work/out"
PEERS_FILE="$work/peers.tsv" DESIGN_PEER_GAP_MS=0 sh "$root/scripts/site/design_evidence.sh" --dir "$site" --out "$out" 2>"$work/err" \
  || { cat "$work/err" >&2; exit 1; }

python3 -I - "$out" <<'PY'
import json
import struct
import sys
from pathlib import Path

out = Path(sys.argv[1])
fail = False


def check(ok: bool, what: str) -> None:
    global fail
    fail = fail or not ok
    print(f"  {'ok  ' if ok else 'FAIL'} {what}")


def width(p: Path) -> int:
    head = p.read_bytes()[:24]
    return struct.unpack(">I", head[16:20])[0] if head[:8] == b"\x89PNG\r\n\x1a\n" else -1


manifest = json.loads((out / "manifest.json").read_text())
pages = {(r["site"], r["page"]): r for r in manifest["pages"]}
check(set(pages) == {("treemand", "home"), ("treemand", "docs"), ("peerco", "home")}, f"pages captured: {sorted(pages)}")
for (site, page), rec in pages.items():
    check(rec["status"] == 200, f"{site}/{page} status {rec['status']}")
    for view in ("desktop-light", "desktop-dark", "mobile-light", "mobile-dark"):
        first = out / site / f"{page}--{view}-1.png"
        want = 1280 if view.startswith("desktop") else 390
        check(first.is_file() and width(first) == want, f"{first.relative_to(out)} is a {want}px-wide PNG")
home = pages[("treemand", "home")]["views"]["desktop-light"]["tiles"]
# Mobile views are touch everywhere. Desktop hover follows the host: Chrome is
# asked for a mouse, but some headless Linux hosts still report (hover: none),
# so the manifest records it per view and only its presence is checked here.
for view, rec in pages[("treemand", "home")]["views"].items():
    if view.startswith("mobile"):
        check(rec.get("hover_none") is True, f"{view} renders as touch (hover: none)")
    else:
        check(isinstance(rec.get("hover_none"), bool), f"{view} records hover: none = {rec.get('hover_none')}")
check(len(home) == 4, f"tall page tiled to the cap: {len(home)} tiles")
check(len(pages[("treemand", "docs")]["views"]["desktop-light"]["tiles"]) == 1, "short page is one tile")
light = (out / "treemand" / "home--desktop-light-1.png").read_bytes()
dark = (out / "treemand" / "home--desktop-dark-1.png").read_bytes()
check(light != dark, "dark view differs from light")
check("treemand/home--desktop-light-1.png" in (out / "index.md").read_text(), "index.md links first screens")
sys.exit(1 if fail else 0)
PY

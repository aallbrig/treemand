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

python3 -I "$root/scripts/site/design_evidence_test_verify.py" "$out"

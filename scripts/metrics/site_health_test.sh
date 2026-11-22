#!/bin/sh
# Behaviour test for the site-health probe: a fixture site with one of each
# problem the probe counts, plus a second local server standing in for the
# outside world, must produce exactly the expected metrics. Needs node 22+ and
# Chrome, like the probe.
set -eu
root="$(cd "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
ext_pid=""
cleanup() { [ -z "$ext_pid" ] || kill "$ext_pid" 2>/dev/null || true; rm -rf "$work"; }
trap cleanup EXIT INT TERM

# The "external" site: a page and an image exist; anything else is a 404.
mkdir -p "$work/ext/ok"
echo '<!doctype html><title>ok</title>' >"$work/ext/ok/index.html"
printf 'GIF89a\001\000\001\000\200\000\000\000\000\000\377\377\377!\371\004\001\000\000\000\000,\000\000\000\000\001\000\001\000\000\002\002D\001\000;' >"$work/ext/pic.gif"
ext_port="$(python3 -I -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
python3 -I -m http.server "$ext_port" --bind 127.0.0.1 --directory "$work/ext" >/dev/null 2>&1 &
ext_pid=$!
ext="http://127.0.0.1:$ext_port"

site="$work/site"
mkdir -p "$site/good" "$site/orphan"
# Home: a 404 link, a missing anchor, a good anchor, a good and a dead external
# link, a non-http link, a console error and a missing image (a failed request).
cat >"$site/index.html" <<EOF
<!doctype html><html lang="en"><link rel="icon" href="data:,"><title>home</title>
<a href="/good/">good</a> <a href="/missing/">missing</a>
<a href="/good/#here">anchor</a> <a href="/good/#nope">bad anchor</a>
<a href="$ext/ok/">ext ok</a> <a href="$ext/gone/">ext gone</a> <a href="mailto:someone@example.com">mail</a>
<img src="/nope.png" alt="" width="1" height="1">
<script>console.error("boom")</script>
EOF
# A page with an uncaught exception and a cross-origin http: image (mixed
# content once the site is served over https).
cat >"$site/good/index.html" <<EOF
<!doctype html><html lang="en"><link rel="icon" href="data:,"><title>good</title>
<h2 id="here">here</h2><a href="/">home</a>
<img src="$ext/pic.gif" alt="" width="1" height="1">
<script>throw new Error("kaboom")</script>
EOF
# Reachable only from the sitemap.
echo '<!doctype html><html lang="en"><link rel="icon" href="data:,"><title>orphan</title><a href="/">home</a>' >"$site/orphan/index.html"
cat >"$site/sitemap.xml" <<'EOF'
<?xml version="1.0" encoding="utf-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<url><loc>https://treemand.dev/</loc></url>
<url><loc>https://treemand.dev/orphan/</loc></url>
</urlset>
EOF

json="$work/out.json"
SITE_DIR="$site" sh "$root/scripts/metrics/probes/site-health.sh" >"$json" 2>"$work/err" || { cat "$work/err" >&2; exit 1; }

python3 -I - "$json" <<'PY'
import json
import sys

with open(sys.argv[1]) as f:
    got = json.load(f)
want = {
    "pages": 3,
    "http_errors": 1,
    "console_errors": 1,
    "js_exceptions": 1,
    "broken_internal_links": 2,
    "broken_external_links": 1,
    "mixed_content": 1,
}
fail = False
for k, v in want.items():
    ok = got["metrics"].get(k + ".local") == v
    fail = fail or not ok
    print(f"  {'ok  ' if ok else 'FAIL'} {k}.local = {got['metrics'].get(k + '.local')} (want {v})")
assert got["probe"] == "site-health"
home = next(d for d in got["details"] if d.get("page") == "/")
hrefs = sorted(b["href"] for b in home["broken_links"])
ok = hrefs[:2] == ["/good/#nope", "/missing/"]
fail = fail or not ok
print(f"  {'ok  ' if ok else 'FAIL'} broken links named on the linking page: {hrefs}")
sys.exit(1 if fail else 0)
PY

#!/bin/sh
# Behaviour test for the site-quality probe: a fixture site with one page full
# of known problems and one clean page, plus a stand-in Lighthouse that writes
# canned reports, must produce exactly the expected metrics. Needs node 22+
# and Chrome, like the probe; axe-core is the pinned download (cached).
set -eu
root="$(cd "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT INT TERM

site="$work/site"
mkdir -p "$site/clean"
# 40,000 bytes of script, so the page weight has a known floor.
python3 -I -c 'print("/*" + "x" * 39994 + "*/")' >"$site/pad.js"
# Known axe violations: an image with no alt (critical), a button with no
# name (critical), low-contrast text (serious). A block inserted above the
# content after load is an unprovoked layout shift.
cat >"$site/index.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><link rel="icon" href="data:,"><title>bad</title>
<meta name="viewport" content="width=device-width, initial-scale=1"></head>
<body><main><h1>Bad page</h1>
<img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==" width="10" height="10">
<button></button>
<p style="color:#bbb;background:#fff">faint text</p>
<p id="anchor">Paragraph that moves when the banner lands.</p>
<script src="/pad.js"></script>
<script>setTimeout(() => { const d = document.createElement("div"); d.style.height = "300px"; d.textContent = "banner"; document.querySelector("main").prepend(d); }, 300);</script>
</main></body></html>
EOF
cat >"$site/clean/index.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><link rel="icon" href="data:,"><title>clean</title>
<meta name="viewport" content="width=device-width, initial-scale=1"></head>
<body><main><h1>Clean page</h1><p>Nothing to see.</p></main></body></html>
EOF

# The stand-in Lighthouse: performance cycles 30, 90, 60 across a page's three
# runs (median 60) on mobile and is 99 on desktop; the other categories are
# fixed, accessibility 80 on the bad page.
lh="$work/lighthouse"
cat >"$lh" <<'EOF'
#!/bin/sh
set -eu
url="" out="" preset=mobile
for a in "$@"; do
  case "$a" in
    --output-path=*) out="${a#--output-path=}" ;;
    --preset=desktop) preset=desktop ;;
    http*) url="$a" ;;
  esac
done
count="$(dirname "$out")/count-$(printf '%s' "$url$preset" | cksum | cut -d' ' -f1)"
n="$(cat "$count" 2>/dev/null || echo 0)"
echo $((n + 1)) >"$count"
if [ "$preset" = desktop ]; then perf=0.99; else
  case $((n % 3)) in 0) perf=0.3 ;; 1) perf=0.9 ;; *) perf=0.6 ;; esac
fi
case "$url" in */clean/) a11y=1 ;; *) a11y=0.8 ;; esac
cat >"$out" <<JSON
{"lighthouseVersion": "fake", "environment": {"benchmarkIndex": 1000},
 "categories": {
  "performance": {"score": $perf, "auditRefs": []},
  "accessibility": {"score": $a11y, "auditRefs": [{"id": "image-alt", "weight": 10}]},
  "best-practices": {"score": 1, "auditRefs": []},
  "seo": {"score": 0.92, "auditRefs": []}},
 "audits": {"image-alt": {"score": 0, "title": "Image elements have alt"}}}
JSON
EOF
chmod +x "$lh"

json="$work/out.json"
SITE_DIR="$site" SITE_QUALITY_PAGES="bad=/ clean=/clean/" SITE_QUALITY_RUNS=3 SITE_QUALITY_LIGHTHOUSE="$lh" \
  sh "$root/scripts/metrics/probes/site-quality.sh" >"$json" 2>"$work/err" || { cat "$work/err" >&2; exit 1; }

python3 -I "$root/scripts/metrics/site_quality_test_verify.py" "$json"

#!/usr/bin/env bash
# Regenerate the site's icons and social card from their sources in
# www/treemand/assets-src/ using headless Chrome or Chromium. The outputs are
# committed under www/treemand/static/; run this after editing a source.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../www/treemand"
chrome="${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || true)}"
[ -n "$chrome" ] || { echo "make_site_assets: set CHROME to a Chrome or Chromium binary" >&2; exit 1; }
src="$PWD/assets-src"
out="$PWD/static"
shot() { # shot <html-file> <png> <width> <height>
  "$chrome" --headless=new --no-sandbox --disable-gpu --hide-scrollbars \
    --force-device-scale-factor=1 --window-size="$3,$4" --screenshot="$2" "file://$1" >/dev/null 2>&1
}
cp "$src/icon.svg" "$out/icon.svg"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
for spec in "favicon-32.png 32" "apple-touch-icon.png 180"; do
  set -- $spec
  printf '<!doctype html><meta charset=utf-8><style>html,body{margin:0;background:transparent}img{display:block;width:%spx;height:%spx}</style><img src="file://%s/icon.svg">' "$2" "$2" "$src" >"$tmp/$1.html"
  shot "$tmp/$1.html" "$out/$1" "$2" "$2"
done
shot "$src/social-card.html" "$out/social-card.png" 1200 630
ls -l "$out/icon.svg" "$out/favicon-32.png" "$out/apple-touch-icon.png" "$out/social-card.png"

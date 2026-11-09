#!/usr/bin/env bash
# Verify production for a release tag: the published binary runs and reports the
# version, and the site answers.   usage: smoke.sh <vX.Y.Z>
set -euo pipefail
tag="${1:?usage: smoke.sh <tag>}"
repo="aallbrig/treemand"
site="https://aallbrig.github.io/treemand/"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in x86_64) arch=amd64 ;; aarch64 | arm64) arch=arm64 ;; esac
asset="treemand_${os}_${arch}.tar.gz"

echo "smoke: release $tag asset $asset"
gh release download "$tag" --repo "$repo" --pattern "$asset" --pattern checksums.txt --dir "$work"
(cd "$work" && grep " $asset\$" checksums.txt | sha256sum -c -)
tar -xzf "$work/$asset" -C "$work"
out="$("$work/treemand_${os}_${arch}" --version)"
echo "smoke: binary says: $out"
case "$out" in *"${tag#v}"*) ;; *) echo "smoke: version mismatch (wanted ${tag#v})" >&2; exit 1 ;; esac
"$work/treemand_${os}_${arch}" --depth=1 --output=json ls >/dev/null

echo "smoke: site $site"
ok=""
for _ in $(seq 1 24); do
  code="$(curl -s -o "$work/index.html" -w '%{http_code}' "$site" || true)"
  if [ "$code" = 200 ]; then ok=1; break; fi
  sleep 10
done
[ -n "$ok" ] || { echo "smoke: site not healthy" >&2; exit 1; }
for path in try/ installation/; do
  code="$(curl -s -o /dev/null -w '%{http_code}' "$site$path")"
  [ "$code" = 200 ] || { echo "smoke: $path returned $code" >&2; exit 1; }
done
echo "smoke: OK"

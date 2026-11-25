#!/bin/sh
# Verify production for a release tag: the published binary runs and reports the
# version, `go install` of the tag reports it too, and the site answers.
# usage: smoke.sh <vX.Y.Z>
set -eu
tag="${1:?usage: smoke.sh <tag>}"
repo="aallbrig/treemand"
site="https://treemand.dev/"
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
"$work/treemand_${os}_${arch}" --depth=1 --output=json ls >/dev/null 2>"$work/stderr.txt"
if grep -q "could not open cache" "$work/stderr.txt"; then
  echo "smoke: binary cannot open its cache: $(cat "$work/stderr.txt")" >&2
  exit 1
fi

# The Installation page's Go channel: the proxy fetches the module tag
# cli/treemand/<tag> on demand, so a missing tag or module path fails here.
if command -v go >/dev/null 2>&1; then
  module="github.com/aallbrig/treemand/cli/treemand"
  echo "smoke: go install $module@$tag"
  gobin="$work/gobin"
  ok=""
  for _ in 1 2 3 4 5 6; do
    if GOBIN="$gobin" GOPATH="$work/gopath" GOMODCACHE="$work/gopath/pkg/mod" GOFLAGS=-modcacherw \
      go install "$module@$tag" >"$work/go.log" 2>&1; then ok=1; break; fi
    sleep 20
  done
  [ -n "$ok" ] || { echo "smoke: go install failed: $(tail -3 "$work/go.log")" >&2; exit 1; }
  out="$("$gobin/treemand" --version)"
  echo "smoke: go-installed binary says: $out"
  case "$out" in *"$tag"*) ;; *) echo "smoke: go install version mismatch (wanted $tag)" >&2; exit 1 ;; esac
fi

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

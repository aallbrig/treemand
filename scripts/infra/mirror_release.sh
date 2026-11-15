#!/usr/bin/env bash
# mirror_release.sh <tag> <dir>
# Upload a release's archives, checksums.txt and its Sigstore bundle to
# https://treemand.dev/releases/<tag>/, then point /releases/latest.txt at it.
# Versioned objects are written with If-None-Match: * so a published version
# can never be replaced (re-runs skip what is already there). URL contract: #118.
set -euo pipefail
cd "$(dirname "$0")/../.."
export AWS_REGION=us-east-1 AWS_DEFAULT_REGION=us-east-1

tag="${1:?usage: mirror_release.sh <tag> <dir>}"
dir="${2:?usage: mirror_release.sh <tag> <dir>}"
case "$tag" in v[0-9]*) ;; *) echo "not a release tag: $tag" >&2; exit 1 ;; esac
[ -f "$dir/checksums.txt" ] || { echo "$dir/checksums.txt missing" >&2; exit 1; }
[ -f "$dir/checksums.txt.sigstore.json" ] || { echo "$dir/checksums.txt.sigstore.json missing" >&2; exit 1; }

stack_output() {
  aws cloudformation describe-stacks --stack-name treemand-site \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text
}
bucket="$(stack_output SiteBucket)"
distribution="$(stack_output DistributionId)"
if [ -n "${GITHUB_ACTIONS:-}" ]; then echo "::add-mask::$distribution"; fi

immutable='public, max-age=31536000, immutable'
put_once() { # put_once <file> <content-type>
  local key err
  key="releases/$tag/$(basename "$1")"
  if err="$(aws s3api put-object --bucket "$bucket" --key "$key" --body "$1" \
      --content-type "$2" --cache-control "$immutable" --if-none-match '*' 2>&1 >/dev/null)"; then
    echo "  uploaded $key"
  elif grep -q PreconditionFailed <<<"$err"; then
    echo "  kept     $key (already published; versions are immutable)"
  else
    echo "$err" >&2; return 1
  fi
}

echo "[mirror] $tag -> https://treemand.dev/releases/$tag/"
shopt -s nullglob
for f in "$dir"/treemand_*.tar.gz; do put_once "$f" application/gzip; done
for f in "$dir"/treemand_*.zip; do put_once "$f" application/zip; done
put_once "$dir/checksums.txt" 'text/plain; charset=utf-8'
put_once "$dir/checksums.txt.sigstore.json" application/json

printf '%s\n' "$tag" >"$dir/latest.txt"
aws s3api put-object --bucket "$bucket" --key releases/latest.txt --body "$dir/latest.txt" \
  --content-type 'text/plain; charset=utf-8' --cache-control 'public, max-age=60' >/dev/null
aws cloudfront create-invalidation --distribution-id "$distribution" --paths /releases/latest.txt >/dev/null
echo "[mirror] releases/latest.txt -> $tag"

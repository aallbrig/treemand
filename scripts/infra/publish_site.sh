#!/usr/bin/env bash
# Build the Hugo site for https://treemand.dev and sync it to the site bucket,
# leaving /releases/ alone, then invalidate the CDN. Used by CI and by hand.
set -euo pipefail
cd "$(dirname "$0")/../.."
export AWS_REGION=us-east-1 AWS_DEFAULT_REGION=us-east-1
origin="https://${DOMAIN:-treemand.dev}"

stack_output() {
  aws cloudformation describe-stacks --stack-name treemand-site \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text
}
bucket="$(stack_output SiteBucket)"
distribution="$(stack_output DistributionId)"
if [ -n "${GITHUB_ACTIONS:-}" ]; then echo "::add-mask::$distribution"; fi

out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT
cli/treemand/web/wasm/build.sh www/treemand/static/try >/dev/null
(cd www/treemand && HUGO_PARAMS_ORIGIN="$origin" hugo --quiet --minify -b "$origin/" -d "$out")
bash scripts/site_search_index.sh "$out"

echo "[site:publish] syncing $(find "$out" -type f | wc -l) files to $origin"
aws s3 sync "$out" "s3://$bucket/" --delete --exclude 'releases/*' \
  --cache-control 'public, max-age=300' --only-show-errors
aws cloudfront create-invalidation --distribution-id "$distribution" --paths '/*' >/dev/null
echo "[site:publish] done; CDN invalidation started"

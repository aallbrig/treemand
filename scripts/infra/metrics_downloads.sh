#!/usr/bin/env bash
# metrics_downloads.sh [days]  (default 30)
# Report treemand.dev usage from CloudFront access logs through Athena (the
# treemand-metrics workgroup, so query cost carries the Project tag): binary
# downloads per version and platform, then install-script, latest.txt and page
# fetches. Needs credentials that can query Athena and read the log bucket.
set -euo pipefail
export AWS_REGION=us-east-1 AWS_DEFAULT_REGION=us-east-1
days="${1:-30}"
case "$days" in *[!0-9]* | '') echo "days must be a number" >&2; exit 2 ;; esac

query() { # query <sql> -> tab-separated rows, header first
  local id state
  id="$(aws athena start-query-execution --work-group treemand-metrics \
    --query-execution-context Database=treemand_metrics \
    --query-string "$1" --query QueryExecutionId --output text)"
  while :; do
    state="$(aws athena get-query-execution --query-execution-id "$id" \
      --query QueryExecution.Status.State --output text)"
    case "$state" in
      SUCCEEDED) break ;;
      FAILED | CANCELLED)
        aws athena get-query-execution --query-execution-id "$id" \
          --query QueryExecution.Status.StateChangeReason --output text >&2
        return 1 ;;
    esac
    sleep 1
  done
  aws athena get-query-results --query-execution-id "$id" \
    --query 'ResultSet.Rows[].Data[].VarCharValue' --output json |
    python3 -c 'import json,sys
cols=int(sys.argv[1]); v=json.load(sys.stdin)
for i in range(0,len(v),cols): print("\t".join(x or "" for x in v[i:i+cols]))' "$2"
}

window="log_date >= current_date - interval '$days' day AND sc_status IN (200, 206, 304)"

echo "Binary downloads, last $days days"
query "SELECT regexp_extract(cs_uri_stem, '^/releases/(v[^/]+)/', 1) AS version,
              regexp_extract(cs_uri_stem, 'treemand_([a-z]+_[a-z0-9]+)\.', 1) AS platform,
              count(*) AS downloads
       FROM treemand_cf_logs
       WHERE $window AND cs_uri_stem LIKE '/releases/v%/treemand_%'
       GROUP BY 1, 2 ORDER BY 3 DESC" 3 | column -t -s $'\t'
echo
echo "Other fetches, last $days days"
query "SELECT what, count(*) AS requests FROM (
         SELECT CASE WHEN cs_uri_stem = '/install.sh' THEN 'install.sh'
                     WHEN cs_uri_stem = '/releases/latest.txt' THEN 'latest.txt'
                     WHEN cs_uri_stem LIKE '%/' OR cs_uri_stem LIKE '%.html' THEN 'pages'
                     ELSE 'other' END AS what
         FROM treemand_cf_logs WHERE $window)
       WHERE what <> 'other' GROUP BY 1 ORDER BY 2 DESC" 2 | column -t -s $'\t'

#!/bin/sh
# Probe: shell portability. Every shell script is POSIX sh unless
# scripts/shell-exempt.txt lists it ("path — reason"); the exemptions are the
# migration backlog, and the ratchet only lets it shrink.
#   metrics: non_posix_scripts  bash (zsh, ksh) scripts that are not exempt
#            exempt_scripts     exempt scripts still not POSIX
#            stale_exemptions   exemptions for a script that is POSIX or gone
#            posix_scripts      scripts that are POSIX sh (not judged)
#   details: each list, with the reason for every exemption
set -eu
cd "$(dirname "$0")/../../.."
sh scripts/shell_inventory.sh | awk '
  function q(s) { gsub(/\\/, "\\\\", s); gsub(/"/, "\\\"", s); return "\"" s "\"" }
  function list(a, n,   i, s) { s = ""; for (i = 1; i <= n; i++) s = s (i > 1 ? ", " : "") a[i]; return "[" s "]" }
  FNR == NR {
    if ($0 ~ /^[[:space:]]*(#|$)/) next
    path = $1; reason = $0
    sub(/^[^[:space:]]+[[:space:]]+(—|--?)[[:space:]]*/, "", reason)
    exempt[path] = reason; order[++ne] = path
    next
  }
  { kind[$2] = $1 }
  $1 == "posix" { posix++; next }
  !($2 in exempt) { bad[++nb] = q($2) }
  END {
    for (i = 1; i <= ne; i++) {
      p = order[i]
      if (kind[p] == "" || kind[p] == "posix") stale[++ns] = q(p)
      else ex[++nx] = "{\"path\": " q(p) ", \"shell\": " q(kind[p]) ", \"reason\": " q(exempt[p]) "}"
    }
    printf "{\"probe\": \"shell-portability\", \"metrics\": {\"non_posix_scripts\": %d, \"exempt_scripts\": %d, \"stale_exemptions\": %d, \"posix_scripts\": %d}, ", nb, nx, ns, posix
    printf "\"details\": [{\"non_posix_not_exempt\": %s}, {\"stale_exemptions\": %s}, {\"exempt\": %s}]}\n", list(bad, nb), list(stale, ns), list(ex, nx)
  }' scripts/shell-exempt.txt -

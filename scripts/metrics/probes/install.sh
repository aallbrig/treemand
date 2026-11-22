#!/bin/sh
# Probe (network, minutes): the documented install paths against the real
# latest release (scripts/metrics/install_paths.sh runs each one).
#   host:       oneliner, upgrade (previous -> latest), reinstall, uninstall,
#               go-source (the docs' clone + go install), go-latest (remote
#               go install, #135); brew only with PROBE_BREW=1 on macOS, since
#               it uses the real Homebrew prefix.
#   containers: oneliner, upgrade, reinstall in ubuntu:24.04, debian:stable-slim,
#               alpine:3 and fedora, when docker is available (on macOS Docker
#               Desktop is started if it is not running).
# Metrics: paths_ok, paths_failed, ok.<env>.<path> (1 or 0); details hold each
# path's row. Everything installs under temp dirs, except the opted-in brew path.
#
# Overrides: PROBE_SITE (default https://treemand.dev), PROBE_LATEST,
# PROBE_PREVIOUS, PROBE_PATHS (host paths), PROBE_BREW=1, PROBE_DOCKER=0 (no
# containers), PROBE_IMAGES (space-separated image names).
set -eu
cd "$(dirname "$0")/../../.."

site="${PROBE_SITE:-https://treemand.dev}"
tmpdir="${TMPDIR:-/tmp}"
work="$(mktemp -d "${tmpdir%/}/treemand-install-probe.XXXXXX")"
trap 'rm -rf "$work"' EXIT INT TERM
rows="$work/rows.tsv"
skips="$work/skipped.tsv"
: >"$rows"
: >"$skips"

latest="${PROBE_LATEST:-$(curl -fsSL "$site/releases/latest.txt" | tr -d ' \r\n')}"
case "$latest" in v[0-9]*) ;; *) echo "install probe: $site/releases/latest.txt does not name a release" >&2; exit 1 ;; esac
# The release before the latest, newest first from the GitHub API.
previous="${PROBE_PREVIOUS:-$(curl -fsSL 'https://api.github.com/repos/aallbrig/treemand/releases?per_page=10' |
  python3 -I -c 'import json, sys; r = [x["tag_name"] for x in json.load(sys.stdin) if not x["draft"] and not x["prerelease"]]; print(r[1] if len(r) > 1 else r[0])')}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in x86_64) arch=amd64 ;; aarch64) arch=arm64 ;; esac
host_env="$os-$arch"

host_paths="${PROBE_PATHS:-oneliner upgrade reinstall uninstall go-source go-latest}"
if [ "${PROBE_BREW:-0}" = 1 ] && [ "$os" = darwin ]; then
  host_paths="$host_paths brew"
else
  printf '%s.brew\tset PROBE_BREW=1 to run it (macOS; uses the real Homebrew prefix)\n' "$host_env" >>"$skips"
fi
sh scripts/metrics/install_paths.sh --site "$site" --env "$host_env" \
  --latest "$latest" --previous "$previous" --paths "$host_paths" >>"$rows"

images="${PROBE_IMAGES:-ubuntu:24.04 debian:stable-slim alpine:3 fedora:latest}"
container_paths="oneliner upgrade reinstall"
skip_images() { # skip_images REASON
  for img in $images; do
    slug="$(printf '%s' "$img" | sed 's/:latest$//; s/[^A-Za-z0-9]/-/g')"
    printf '%s\t%s\n' "$slug" "$1" >>"$skips"
  done
}

docker_up() {
  command -v docker >/dev/null 2>&1 || return 1
  docker info >/dev/null 2>&1 && return 0
  if [ "$os" != darwin ] || ! command -v open >/dev/null 2>&1; then return 1; fi
  open -a Docker >/dev/null 2>&1 || return 1
  i=0
  while [ $i -lt 60 ]; do
    docker info >/dev/null 2>&1 && return 0
    i=$((i + 1)); sleep 2
  done
  return 1
}

if [ "${PROBE_DOCKER:-1}" = 0 ]; then
  skip_images "containers off (PROBE_DOCKER=0)"
elif ! docker_up; then
  skip_images "docker is not available on this host"
else
  for img in $images; do
    slug="$(printf '%s' "$img" | sed 's/:latest$//; s/[^A-Za-z0-9]/-/g')"
    # curl is the one-liner's own prerequisite; nothing else is preinstalled.
    case "$img" in
      alpine*) prep="apk add --no-cache curl ca-certificates" ;;
      fedora*) prep="command -v curl || dnf install -y -q curl" ;;
      *) prep="apt-get update -qq && apt-get install -y -qq curl ca-certificates" ;;
    esac
    out="$work/$slug.tsv"
    if docker run --rm -v "$PWD/scripts/metrics/install_paths.sh:/install_paths.sh:ro" "$img" \
      sh -c "$prep >/dev/null 2>&1 && sh /install_paths.sh --site '$site' --env '$slug' --latest '$latest' --previous '$previous' --paths '$container_paths'" \
      >"$out" 2>"$work/$slug.err" && [ -s "$out" ]; then
      cat "$out" >>"$rows"
    else
      printf '%s\tcontainer did not run: %s\n' "$slug" "$(tail -1 "$work/$slug.err" | tr '\t' ' ')" >>"$skips"
    fi
  done
fi

python3 -I - "$rows" "$skips" "$latest" "$previous" <<'PY'
import json
import sys

rows_file, skips_file, latest, previous = sys.argv[1:5]
cols = ["env", "path", "status", "exit", "version", "want", "renders",
        "quarantine", "quarantine_runs", "path_hint", "signature", "note"]
details: list[dict[str, str]] = []
metrics: dict[str, int] = {"paths_ok": 0, "paths_failed": 0}
skipped: dict[str, str] = {}
with open(rows_file) as f:
    for line in f:
        parts = line.rstrip("\n").split("\t")
        if len(parts) != len(cols):
            continue
        row = dict(zip(cols, parts, strict=True))
        name = f"{row.pop('env')}.{row.pop('path')}"
        if row["status"] == "skip":
            skipped[name] = row["note"]
            continue
        ok = row["status"] == "ok"
        metrics["paths_ok" if ok else "paths_failed"] += 1
        metrics[f"ok.{name}"] = int(ok)
        details.append({"path": name, **row})
with open(skips_file) as f:
    for line in f:
        name, _, why = line.rstrip("\n").partition("\t")
        skipped[name] = why
print(json.dumps({
    "probe": "install",
    "metrics": metrics,
    "skipped": skipped,
    "details": [{"latest": latest, "previous": previous}, *details],
}))
PY

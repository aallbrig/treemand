#!/bin/sh
# Behaviour tests for the release-health probe (scripts/metrics/probes/
# release-health.sh) with the network replaced by local files
# (RELEASE_HEALTH_ROUTES), a fixed clock and a stand-in cosign. Two releases:
# a healthy world, a broken one past the 1 hour grace, the same broken one
# inside the grace, and a host without cosign.
here="$(cd "$(dirname "$0")" && pwd)"
probe="$here/probes/release-health.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0

api="https://api.github.com/repos/aallbrig/treemand"
dl="https://github.com/aallbrig/treemand/releases/download"
site="https://treemand.dev"
tap="https://raw.githubusercontent.com/aallbrig/homebrew-tap/main/Formula/treemand.rb"
archives="treemand_darwin_amd64.tar.gz treemand_darwin_arm64.tar.gz treemand_linux_amd64.tar.gz treemand_linux_arm64.tar.gz treemand_windows_amd64.zip"

sha() { python3 -I -c 'import hashlib,sys; print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "$1"; }
metric() { python3 -I -c 'import json,sys; r=json.load(open(sys.argv[1])); print(r["metrics"].get(sys.argv[2], "absent"))' "$1" "$2"; }
skip() { python3 -I -c 'import json,sys; r=json.load(open(sys.argv[1])); print(r.get("skipped", {}).get(sys.argv[2], "absent"))' "$1" "$2"; }
expect() { # expect <json-file> <metric> <value> <what>
  got="$(metric "$1" "$2")"
  if [ "$got" = "$3" ]; then echo "ok    $4"; else echo "FAIL  $4: $2 = $got, want $3"; fail=1; fi
}

# world <dir> <healthy|broken>: writes the files and <dir>/routes.tsv
world() {
  w="$1" kind="$2"
  mkdir -p "$w"
  : >"$w/routes.tsv"
  route() { printf '%s\t%s\n' "$1" "$2" >>"$w/routes.tsv"; }
  assets_json=""
  for tag in v1.1.0 v1.0.0; do
    mkdir -p "$w/$tag"
    : >"$w/$tag/checksums.txt"
    list=""
    for a in $archives; do
      if [ "$kind" = broken ] && [ "$tag" = v1.1.0 ] && [ "$a" = treemand_windows_amd64.zip ]; then continue; fi
      printf '%s %s\n' "$tag" "$a" >"$w/$tag/$a"
      s="$(sha "$w/$tag/$a")"
      printf '%s  %s\n' "$s" "$a" >>"$w/$tag/checksums.txt"
      digest="\"sha256:$s\""
      # broken v1.0.0: GitHub reports a different digest for one archive, and
      # none for another, whose download then differs from checksums.txt.
      if [ "$kind" = broken ] && [ "$tag" = v1.0.0 ]; then
        case "$a" in
          treemand_darwin_amd64.tar.gz) digest="\"sha256:$(printf '%064d' 0)\"" ;;
          treemand_linux_amd64.tar.gz) digest=null; echo tampered >"$w/$tag/$a" ;;
        esac
      fi
      route "$dl/$tag/$a" "$w/$tag/$a"
      list="$list{\"name\":\"$a\",\"digest\":$digest},"
    done
    echo bundle >"$w/$tag/checksums.txt.sigstore.json"
    route "$dl/$tag/checksums.txt" "$w/$tag/checksums.txt"
    route "$dl/$tag/checksums.txt.sigstore.json" "$w/$tag/checksums.txt.sigstore.json"
    list="$list{\"name\":\"checksums.txt\",\"digest\":null},{\"name\":\"checksums.txt.sigstore.json\",\"digest\":null}"
    case "$tag" in v1.1.0) pub=2026-11-20T10:00:00Z merged=2026-11-20T09:50:00Z ;; *) pub=2026-11-19T10:00:00Z merged=2026-11-19T09:40:00Z ;; esac
    assets_json="$assets_json{\"tag_name\":\"$tag\",\"draft\":false,\"prerelease\":false,\"published_at\":\"$pub\",\"assets\":[$list]},"
    printf '{"commit":{"committer":{"date":"%s"}}}' "$merged" >"$w/commit-$tag.json"
    route "$api/commits/$tag" "$w/commit-$tag.json"
  done
  printf '[{"tag_name":"v1.2.0-rc1","draft":false,"prerelease":true},%s{"tag_name":"v2.0.0","draft":true,"prerelease":false}]' "$assets_json" >"$w/releases.json"
  route "$api/releases?per_page=15" "$w/releases.json"

  if [ "$kind" = healthy ]; then
    tapv=1.1.0 latest=v1.1.0 pkg=success
  else
    tapv=1.0.0 latest=v1.0.0 pkg=failure
  fi
  printf 'class Treemand < Formula\n  version "%s"\nend\n' "$tapv" >"$w/formula.rb"
  route "$tap" "$w/formula.rb"
  echo "$latest" >"$w/latest.txt"
  route "$site/releases/latest.txt" "$w/latest.txt"
  if [ "$kind" = healthy ]; then
    # shellcheck disable=SC2016 # a literal install.sh line
    printf 'BASE_URL="${TREEMAND_BASE_URL:-%s/releases}"\nfetch "$BASE_URL/latest.txt"\n' "$site" >"$w/install.sh"
    route "$site/install.sh" "$w/install.sh"
    for f in $archives checksums.txt checksums.txt.sigstore.json; do route "$site/releases/v1.1.0/$f" "$w/latest.txt"; done
  fi
  cat >"$w/packages.json" <<JSON
{"workflow_runs":[
 {"created_at":"2026-11-20T10:05:00Z","status":"completed","conclusion":"$pkg","head_branch":"main","html_url":"run/pkg-new"},
 {"created_at":"2026-11-19T10:05:00Z","status":"completed","conclusion":"success","head_branch":"main","html_url":"run/pkg-old"}]}
JSON
  [ "$kind" = broken ] || route "$api/actions/workflows/release-packages.yml/runs?per_page=30" "$w/packages.json"
  [ "$kind" = healthy ] || {
    printf '{"workflow_runs":[]}' >"$w/packages-none.json"
    route "$api/actions/workflows/release-packages.yml/runs?per_page=30" "$w/packages-none.json"
  }
  cat >"$w/auto.json" <<'JSON'
{"workflow_runs":[
 {"conclusion":"success","html_url":"run/a1"},{"conclusion":"cancelled","html_url":"run/a2"},
 {"conclusion":"failure","html_url":"run/a3"},{"conclusion":"success","html_url":"run/a4"}]}
JSON
  route "$api/actions/workflows/auto-release.yml/runs?status=completed&per_page=20" "$w/auto.json"
}

# A stand-in cosign: verifies unless the bundle path names the tag in COSIGN_REJECT.
mkdir -p "$tmp/bin"
cat >"$tmp/bin/cosign" <<'SH'
#!/bin/sh
case "$*" in *"/${COSIGN_REJECT:-none}/"*) echo "error: none of the expected identities matched" >&2; exit 1 ;; esac
exit 0
SH
chmod +x "$tmp/bin/cosign"

run() { # run <world> <now> <out> [env...]
  w="$1" at="$2" out="$3"
  shift 3
  if ! env RELEASE_HEALTH_ROUTES="$w/routes.tsv" RELEASE_HEALTH_NOW="$at" RELEASE_HEALTH_COSIGN="$tmp/bin/cosign" "$@" \
    sh "$probe" >"$out" 2>"$out.err"; then
    echo "FAIL  probe exited non-zero: $(tail -3 "$out.err")"
    fail=1
  fi
}

# ── healthy ───────────────────────────────────────────────────────────────
world "$tmp/healthy" healthy
run "$tmp/healthy" 2026-11-20T12:00:00Z "$tmp/h.json"
expect "$tmp/h.json" releases_checked 2 "drafts and prereleases are not checked"
expect "$tmp/h.json" lead_time_min 15.0 "lead time is the median of merge to publish (10 and 20 min)"
expect "$tmp/h.json" assets_missing 0 "healthy: every expected asset is there"
expect "$tmp/h.json" checksum_mismatches 0 "healthy: checksums match the published digests"
expect "$tmp/h.json" signature_failures 0 "healthy: both bundles verify"
expect "$tmp/h.json" tap_lag 0 "healthy: the tap is the latest tag"
expect "$tmp/h.json" mirror_lag 0 "healthy: the mirror's latest.txt is the latest tag"
expect "$tmp/h.json" mirror_assets_missing 0 "healthy: the mirror has the latest tag's files"
expect "$tmp/h.json" install_script_missing 0 "healthy: install.sh is served and uses the mirror"
expect "$tmp/h.json" package_publish_failed 0 "healthy: the packages run for the latest tag succeeded"
expect "$tmp/h.json" auto_release_failure_pct 33.3 "one failure in three non-cancelled Auto Release runs"

# ── broken, two hours after the latest publish ────────────────────────────
world "$tmp/broken" broken
run "$tmp/broken" 2026-11-20T12:00:00Z "$tmp/b.json" COSIGN_REJECT=v1.0.0
expect "$tmp/b.json" assets_missing 1 "a missing archive is counted"
expect "$tmp/b.json" checksum_mismatches 2 "a wrong digest and a tampered download (no digest) are counted"
expect "$tmp/b.json" signature_failures 1 "a bundle cosign rejects is counted"
expect "$tmp/b.json" tap_lag 1 "an old tap formula past the grace is a lag"
expect "$tmp/b.json" mirror_lag 1 "an old latest.txt past the grace is a lag"
expect "$tmp/b.json" mirror_assets_missing 7 "the mirror's missing files for the latest tag are counted"
expect "$tmp/b.json" install_script_missing 1 "an unserved install.sh is counted"
expect "$tmp/b.json" package_publish_failed 1 "no packages run within the grace counts as failed"

# ── the same broken world, 30 minutes after publishing ────────────────────
run "$tmp/broken" 2026-11-20T10:30:00Z "$tmp/g.json"
expect "$tmp/g.json" tap_lag 0 "inside the grace a tap lag is pending, not counted"
expect "$tmp/g.json" mirror_lag 0 "inside the grace a mirror lag is pending, not counted"
expect "$tmp/g.json" mirror_assets_missing 0 "inside the grace missing mirror files are pending"
expect "$tmp/g.json" package_publish_failed 0 "inside the grace a packages run not started yet is pending"
expect "$tmp/g.json" assets_missing 1 "release assets have no grace"

# ── a failed packages run for the latest tag ──────────────────────────────
sed -i.bak 's#packages-none.json#packages.json#' "$tmp/broken/routes.tsv"
run "$tmp/broken" 2026-11-20T10:30:00Z "$tmp/p.json"
expect "$tmp/p.json" package_publish_failed 1 "a failed packages run counts at once"

# ── no cosign on this host ────────────────────────────────────────────────
run "$tmp/healthy" 2026-11-20T12:00:00Z "$tmp/n.json" RELEASE_HEALTH_COSIGN=none
expect "$tmp/n.json" signature_failures absent "without cosign there is no signature metric"
got="$(skip "$tmp/n.json" signature_failures)"
if [ "$got" = "cosign is not installed" ]; then echo "ok    without cosign the signature check is skipped"; else
  echo "FAIL  skipped.signature_failures = $got"
  fail=1
fi

exit "$fail"

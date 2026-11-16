#!/usr/bin/env bash
# Checks every install channel the site documents (www/treemand/data/install.yaml)
# against what is really published:
#   - a channel that needs a maintainer-created account or repo (Scoop, WinGet,
#     Chocolatey, AUR, Snap) is not documented until it publishes (#45);
#   - the Homebrew formula is the latest release or the one before it;
#   - the install script is served.
# Exit 0 when every documented channel is current, 1 otherwise.
#
# Offline overrides (tests): CHANNELS_DATA (install.yaml path),
# CHANNELS_RELEASES ("0.8.1 0.8.0": newest first), CHANNELS_BREW_VERSION,
# CHANNELS_SKIP_SITE=1.
set -euo pipefail
cd "$(dirname "$0")/.."

data="${CHANNELS_DATA:-www/treemand/data/install.yaml}"
fail=0
ok() { echo "ok    $*"; }
bad() { echo "FAIL  $*"; fail=1; }

for ch in Scoop WinGet Chocolatey AUR Snap; do
  if grep -qiE "^[[:space:]]*- title:[[:space:]]*${ch}([[:space:]]|$)" "$data"; then
    bad "$ch is documented but not published (see the Maintainer setup checklist issue)"
  fi
done

releases="${CHANNELS_RELEASES:-}"
if [[ -z "$releases" ]]; then
  releases="$(curl -fsSL 'https://api.github.com/repos/aallbrig/treemand/releases?per_page=10' |
    python3 -I -c 'import json,sys; r=[x["tag_name"].lstrip("v") for x in json.load(sys.stdin) if not x["draft"] and not x["prerelease"]]; print(" ".join(r[:2]))')"
fi
read -r latest previous <<<"$releases"
previous="${previous:-$latest}"

if grep -qiE '^[[:space:]]*- title:[[:space:]]*Homebrew' "$data"; then
  brew="${CHANNELS_BREW_VERSION:-}"
  if [[ -z "$brew" ]]; then
    brew="$(curl -fsSL https://raw.githubusercontent.com/aallbrig/homebrew-tap/main/Formula/treemand.rb |
      sed -n 's/^[[:space:]]*version "\(.*\)"/\1/p' | head -1)"
  fi
  if [[ "$brew" == "$latest" || "$brew" == "$previous" ]]; then
    ok "Homebrew formula $brew (latest release $latest)"
  else
    bad "Homebrew formula is ${brew:-missing}; latest release is $latest (allowed: $latest or $previous)"
  fi
fi

if [[ "${CHANNELS_SKIP_SITE:-}" != 1 ]]; then
  if curl -fsSL -o /dev/null https://treemand.dev/install.sh; then
    ok "install script served"
  else
    bad "https://treemand.dev/install.sh is not served"
  fi
fi
exit "$fail"

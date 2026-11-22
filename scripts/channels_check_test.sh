#!/bin/sh
# Hermetic tests for channels_check.sh: no network, fixture data only.
set -u
cd "$(dirname "$0")/.." || exit 1
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0
check() { # check <want-exit> <label> ; env set by caller
  want="$1" label="$2" got=0
  CHANNELS_SKIP_SITE=1 sh scripts/channels_check.sh >"$work/out" 2>&1 || got=$?
  if [ "$got" = "$want" ]; then echo "ok    $label"; else echo "FAIL  $label (exit $got, want $want)"; cat "$work/out"; fail=1; fi
}

cat >"$work/good.yaml" <<'EOF'
linux:
    channels:
      - title: Homebrew
        commands:
          - brew install treemand
      - title: Install script
EOF
cp "$work/good.yaml" "$work/scoop.yaml"
printf '      - title: Scoop\n        commands:\n          - scoop install treemand\n' >>"$work/scoop.yaml"

export CHANNELS_RELEASES="0.8.1 0.8.0"
CHANNELS_DATA="$work/good.yaml" CHANNELS_BREW_VERSION=0.8.1 check 0 "current formula passes"
CHANNELS_DATA="$work/good.yaml" CHANNELS_BREW_VERSION=0.8.0 check 0 "one release behind is allowed"
CHANNELS_DATA="$work/good.yaml" CHANNELS_BREW_VERSION=0.7.9 check 1 "two releases behind fails"
CHANNELS_DATA="$work/scoop.yaml" CHANNELS_BREW_VERSION=0.8.1 check 1 "an unpublished channel documented fails"
CHANNELS_DATA=www/treemand/data/install.yaml CHANNELS_BREW_VERSION=0.8.1 check 0 "the real install.yaml documents only published channels"
exit "$fail"

#!/bin/sh
# Lint the repo's hand-written JavaScript with Biome (task js:lint).
#
#   sh scripts/dev/js_lint.sh            every file biome.jsonc includes
#   sh scripts/dev/js_lint.sh FILE...    only these (pre-commit's staged files);
#                                        files biome.jsonc excludes are skipped
#
# Biome runs through npx at a pinned version, so the only prerequisite is Node.
# Warnings fail too: the baseline is clean and stays that way.
set -u
BIOME_VERSION=2.6.7

root="$(git rev-parse --show-toplevel)" || exit 1
cd "$root" || exit 1
if ! command -v npx >/dev/null 2>&1; then
  echo "js_lint: npx not found: install Node (sh scripts/dev/setup.sh --install)" >&2
  exit 1
fi
exec npx --yes "@biomejs/biome@$BIOME_VERSION" lint --error-on-warnings --no-errors-on-unmatched "$@"

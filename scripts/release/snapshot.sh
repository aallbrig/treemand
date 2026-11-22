#!/bin/sh
# Dry-run a release with GoReleaser (no signing, nothing published) into
# cli/treemand/dist, then check it against the release asset contract.
# Needs goreleaser and syft on PATH.
#
#   sh scripts/release/snapshot.sh
set -eu
root="$(cd "$(dirname "$0")/../.." && pwd)"
(cd "$root/cli/treemand" && goreleaser release --snapshot --clean --skip=sign)
sh "$root/scripts/release/snapshot_test.sh" "$root/cli/treemand/dist"

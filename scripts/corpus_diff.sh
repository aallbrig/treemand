#!/bin/sh
# corpus_diff.sh [ref]  (default origin/main)
# What the working tree's help parser extracts from each corpus fixture,
# compared with <ref>'s parser on the same fixtures: every subcommand, flag
# and description gained or lost, with the fixture line it comes from.
# Read it after any parser change; a change you did not intend is a bug.
set -eu
cd "$(dirname "$0")/.."
ref="${1:-origin/main}"
fixtures="$PWD/cli/treemand/discovery/testdata/corpus"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

(cd cli/treemand && go build -o "$work/new" ./tools/corpusdump)
git archive "$ref" cli/treemand | tar -x -C "$work"
mkdir -p "$work/cli/treemand/tools/corpusdump"
cp cli/treemand/tools/corpusdump/main.go "$work/cli/treemand/tools/corpusdump/"
(cd "$work/cli/treemand" && GOWORK=off go build -o "$work/old" ./tools/corpusdump)

"$work/old" "$fixtures" | LC_ALL=C sort >"$work/old.tsv"
"$work/new" "$fixtures" | LC_ALL=C sort >"$work/new.tsv"

tab="$(printf '\t')"
show() { # show <sign> <file>
  while IFS="$tab" read -r cli kind value; do
    src=""
    if [ "$kind" != desc ]; then
      word="${value%% *}"
      src="$(grep -n -m1 -wF -- "$word" "$fixtures/$cli.txt" | cut -c1-80 || true)"
    fi
    printf '%s %-12s %-5s %-40s | %s\n' "$1" "$cli" "$kind" "$value" "$src"
  done <"$2"
}
LC_ALL=C comm -23 "$work/old.tsv" "$work/new.tsv" >"$work/lost"
LC_ALL=C comm -13 "$work/old.tsv" "$work/new.tsv" >"$work/gained"
show - "$work/lost"
show + "$work/gained"
echo "corpus diff vs $ref: $(wc -l <"$work/lost") lost, $(wc -l <"$work/gained") gained"

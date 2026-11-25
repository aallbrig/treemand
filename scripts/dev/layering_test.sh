#!/bin/sh
# Tests the package import layering that depguard enforces (#476): the tree as
# it is passes, and each planted import that reaches up a layer fails lint.
# Runs on a copy of the module so the working tree is never touched.
set -u
cd "$(dirname "$0")/../.." || exit 1
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0
ok() { echo "  ok   $1"; }
bad() { echo "  FAIL $1"; fail=1; }

mkdir -p "$work/cli"
cp go.work .golangci.yml "$work/"
cp -R cli/treemand "$work/cli/treemand"
mod=github.com/aallbrig/treemand

lint() { (cd "$work" && golangci-lint run --enable-only depguard "./cli/treemand/$1/...") >"$work/out" 2>&1; }

if lint .; then ok "the tree as it is passes"; else bad "the tree as it is fails:"; cat "$work/out"; fi

# Each plant avoids an import cycle, which would fail typecheck before depguard.
# plant <package dir> <imported package> <case name>: the import must fail lint.
plant() {
	f="$work/cli/treemand/$1/zz_layering_plant.go"
	pkg="$(sed -n 's/^package \([a-z0-9_]*\).*/\1/p' "$work"/cli/treemand/"$1"/*.go | grep -v _test | head -n 1)"
	printf 'package %s\n\nimport _ "%s/%s"\n' "$pkg" "$mod" "$2" >"$f"
	if lint "$1"; then
		bad "$3: $1 importing $2 passed lint"
	elif grep -q depguard "$work/out"; then
		ok "$3"
	else
		bad "$3: lint failed for another reason:"; cat "$work/out"
	fi
	rm -f "$f"
}

# models cannot be planted: every package imports it, so any upward import is a cycle Go rejects.
plant history discovery "a leaf package imports nothing above it"
plant savedpresets render "a leaf package stays below render"
plant discovery overrides "discovery stays below overrides"
plant fuzzy presets "only discovery is exempt for presets"
plant presets mcp "presets stays below the front ends"
plant e2e cmd "only main imports cmd"
plant mcp tui "mcp does not import tui"
plant tui mcp "tui does not import mcp"
plant e2e tui "only cmd and the tools import tui"

# The single exemption: discovery may still import presets.
if grep -rq "\"$mod/presets\"" "$work/cli/treemand/discovery" && lint discovery; then
	ok "discovery's presets import is the one exemption"
else
	bad "discovery's presets import no longer passes (or no longer exists: drop the exemption)"
fi

exit "$fail"

#!/bin/sh
# Tests for the complexity probe (#475) against a fixture tree: test files do
# not count, clones and deeply nested ifs do, and each file over 800 lines
# reports its length.
# shellcheck disable=SC2015
set -u
cd "$(dirname "$0")/../.." || exit 1
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0
ok() { echo "  ok   $1"; }
bad() { echo "  FAIL $1"; fail=1; }

src="$work/src"
mkdir -p "$src"
# A complex function in a test file: it must not count.
cat >"$src/a_test.go" <<'EOF'
package fx

func complexInTest(a, b, c int) int {
	if a > 0 {
		if b > 0 {
			if c > 0 {
				for i := 0; i < a; i++ {
					if i%2 == 0 && b > 1 || c > 2 {
						return i
					}
				}
			}
		}
	}
	return 0
}
EOF
# Two copies of one function body: one clone group.
clone() {
	printf 'func %s(xs []int) (int, int) {\n' "$1"
	printf '\tlo, hi := xs[0], xs[0]\n\tfor _, x := range xs {\n\t\tif x < lo {\n\t\t\tlo = x\n\t\t}\n\t\tif x > hi {\n\t\t\thi = x\n\t\t}\n\t}\n'
	printf '\tif lo < 0 {\n\t\tlo = 0\n\t}\n\tif hi > 100 {\n\t\thi = 100\n\t}\n\treturn lo, hi\n}\n\n'
}
{
	printf 'package fx\n\n'
	clone rangeA
	clone rangeB
	# One deeply nested if (nesting complexity 4+).
	printf 'func nested(a, b, c, d int) int {\n\tif a > 0 {\n\t\tif b > 0 {\n\t\t\tif c > 0 {\n\t\t\t\tif d > 0 {\n\t\t\t\t\treturn 1\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n\treturn 0\n}\n'
} >"$src/b.go"
# A long file of 900 lines.
{
	printf 'package fx\n\n'
	i=0
	while [ "$i" -lt 898 ]; do
		printf '// line %s\n' "$i"
		i=$((i + 1))
	done
} >"$src/long.go"

COMPLEXITY_ROOT="$src" bash scripts/metrics/probes/complexity.sh >"$work/out.json" || bad "the probe failed"
m() { python3 -I -c 'import json,sys;print(json.load(open(sys.argv[1]))["metrics"].get(sys.argv[2]))' "$work/out.json" "$1"; }

[ "$(m cognitive_max)" = 10 ] && ok "a test file's complexity (17) does not count" || bad "cognitive_max = $(m cognitive_max), want 10 (nested; the test file is 17)"
[ "$(m dupl_clones)" = 1 ] && ok "two copies of a body are one clone group" || bad "dupl_clones = $(m dupl_clones), want 1"
[ "$(m nestif_over_3)" = 1 ] && ok "a deeply nested if counts" || bad "nestif_over_3 = $(m nestif_over_3), want 1"
[ "$(m "file_lines.$src/long.go")" = 900 ] && ok "a file over 800 lines reports its length" || bad "file_lines.$src/long.go = $(m "file_lines.$src/long.go"), want 900"
[ "$(m "file_lines.$src/b.go")" = None ] && ok "a short file reports no length" || bad "b.go reported a length"
[ "$(m files_over_800_lines)" = 1 ] && ok "files_over_800_lines counts the long file" || bad "files_over_800_lines = $(m files_over_800_lines)"

[ "$fail" = 0 ] && echo "complexity_probe_test: ok"
exit "$fail"

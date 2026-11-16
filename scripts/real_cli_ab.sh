#!/usr/bin/env bash
# real_cli_ab.sh [--base REF] <cli>...   (task ab -- <cli>...)
# Full discovery of real installed CLIs with <REF>'s treemand (default
# origin/main) and the working tree's, compared tree against tree: nodes,
# flags and descriptions gained or lost. Runs from an empty scratch directory
# and fails if discovery changed anything on this machine: git branches or
# status, Docker context, builders, volumes or networks, or files in the
# scratch directory. treemand runs these CLIs with arguments (#149, #153, #176).
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
base=origin/main
if [ "${1:-}" = --base ]; then base="$2"; shift 2; fi
[ $# -gt 0 ] || { echo "usage: real_cli_ab.sh [--base REF] <cli>..." >&2; exit 2; }

work="$(mktemp -d)"
cwd="$work/cwd"
mkdir -p "$cwd"
trap 'rm -rf "$work"' EXIT

(cd "$repo/cli/treemand" && go build -o "$work/new" .)
git -C "$repo" archive "$base" cli/treemand | tar -x -C "$work"
(cd "$work/cli/treemand" && GOWORK=off go build -o "$work/old" .)

snapshot() {
  git -C "$repo" branch --list
  git -C "$repo" status --porcelain
  if command -v docker >/dev/null 2>&1; then
    docker context show 2>/dev/null
    docker buildx ls --format '{{.Name}}' 2>/dev/null
    docker volume ls -q 2>/dev/null
    docker network ls --format '{{.Name}}' 2>/dev/null
  fi
}
snapshot >"$work/before"

cat >"$work/compare.py" <<'PY'
import json, sys
def load(p):
    out = {}
    def walk(n, path):
        q = path + [n["name"]]
        out[" ".join(q)] = (n.get("description", ""), {f["name"] for f in n.get("flags") or []})
        for c in n.get("children") or []:
            walk(c, q)
    walk(json.load(open(p)), [])
    return out
a, b = load(sys.argv[1]), load(sys.argv[2])
lost_nodes = sorted(set(a) - set(b))
new_nodes = sorted(set(b) - set(a))
lost_flags = [(k, f) for k in a for f in a[k][1] - b.get(k, ("", set()))[1]]
new_flags = [(k, f) for k in b for f in b[k][1] - a.get(k, ("", set()))[1]]
desc = [(k, a[k][0], b[k][0]) for k in a if k in b and a[k][0] != b[k][0]]
print(f"nodes {len(a)}->{len(b)}  lost nodes {len(lost_nodes)}  lost flags {len(lost_flags)}  "
      f"gained flags {len(new_flags)}  changed descriptions {len(desc)}")
for k in lost_nodes[:5]: print(f"    - node {k}")
for k, f in lost_flags[:5]: print(f"    - flag {k}: {f}")
for k, f in new_flags[:5]: print(f"    + flag {k}: {f}")
for k, o, n in desc[:5]: print(f"    ~ desc {k}: {o[:50]!r} -> {n[:50]!r}")
PY

for cli in "$@"; do
  (cd "$cwd" && TREEMAND_CACHE_DIR="$work/cache-old" "$work/old" --no-cache --output=json --timeout=120 "$cli" >"$work/old.json" 2>/dev/null) || true
  (cd "$cwd" && TREEMAND_CACHE_DIR="$work/cache-new" "$work/new" --no-cache --output=json --timeout=120 "$cli" >"$work/new.json" 2>/dev/null) || true
  printf '%-12s ' "$cli"
  python3 -I "$work/compare.py" "$work/old.json" "$work/new.json" || echo "could not compare (discovery failed?)"
done

snapshot >"$work/after"
changed=0
if ! diff -u "$work/before" "$work/after" >"$work/state.diff"; then
  echo "SIDE EFFECT: discovery changed this machine's state:" >&2
  grep -E '^[-+][^-+]' "$work/state.diff" >&2
  changed=1
fi
if [ -n "$(ls -A "$cwd")" ]; then
  echo "SIDE EFFECT: discovery wrote into its working directory:" >&2
  ls -A "$cwd" >&2
  changed=1
fi
[ "$changed" = 0 ] && echo "side-effect audit: clean"
exit "$changed"

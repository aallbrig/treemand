#!/usr/bin/env bash
# Build the browser (wasm) TUI: build.sh <out-dir>
# Bubble Tea and atotto/clipboard have no js/wasm support, so the build uses
# temporary patched copies of them (stub files added) via an alternate
# go.mod (-modfile). The repo's go.mod and the module cache are untouched.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
MOD="$(cd "$HERE/../.." && pwd)"
OUT="$(mkdir -p "${1:?usage: build.sh <out-dir>}" && cd "$1" && pwd)"

WORK="$(mktemp -d)"
trap 'chmod -R u+w "$WORK"; rm -rf "$WORK"' EXIT

cd "$MOD"
# Fresh CI runners have an empty module cache; `go list -m` does not download.
go mod download github.com/charmbracelet/bubbletea github.com/atotto/clipboard
BT="$(go list -m -f '{{.Dir}}' github.com/charmbracelet/bubbletea)"
CB="$(go list -m -f '{{.Dir}}' github.com/atotto/clipboard)"
cp -r "$BT" "$WORK/bubbletea"
cp -r "$CB" "$WORK/clipboard"
chmod -R u+w "$WORK"
cp "$HERE/overlay/bubbletea_js.go" "$WORK/bubbletea/zz_treemand_js.go"
cp "$HERE/overlay/clipboard_js.go" "$WORK/clipboard/zz_treemand_js.go"

cp go.mod "$WORK/go.mod"
cp go.sum "$WORK/go.sum"
GOWORK=off go mod edit -modfile="$WORK/go.mod" \
  -replace "github.com/charmbracelet/bubbletea=$WORK/bubbletea" \
  -replace "github.com/atotto/clipboard=$WORK/clipboard"

GOWORK=off GOOS=js GOARCH=wasm go build -modfile="$WORK/go.mod" -trimpath -ldflags="-s -w" \
  -o "$OUT/treemand.wasm" ./web/wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$OUT/wasm_exec.js"
ls -lh "$OUT/treemand.wasm" | awk '{print "treemand.wasm " $5}'

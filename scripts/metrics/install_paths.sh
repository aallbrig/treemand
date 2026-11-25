#!/bin/sh
# Runs treemand's documented install paths in this environment (the host, or a
# container the install probe starts) and prints one tab-separated row per path:
#
#   env path status exit version want renders quarantine quarantine_runs path_hint signature note
#
# status is ok, fail or skip; "-" marks a field that does not apply. Every path
# installs under a fresh temp dir, with HOME pointed there too, so nothing is
# written elsewhere. The one exception is the brew path, which uses the real
# Homebrew prefix and runs only when --paths names it (the probe passes it only
# with PROBE_BREW=1).
#
# Paths: oneliner upgrade reinstall uninstall go-source go-latest brew
#
#   install_paths.sh --env NAME --latest vX.Y.Z --previous vX.Y.Z \
#     [--site https://treemand.dev] [--paths "oneliner upgrade ..."]
set -eu

site=https://treemand.dev
env_name=host
paths="oneliner upgrade reinstall uninstall go-source go-latest"
latest=""
previous=""
go_repo="${PROBE_GO_REPO:-https://github.com/aallbrig/treemand.git}"
go_module="${PROBE_GO_MODULE:-github.com/aallbrig/treemand/cli/treemand}"

while [ $# -gt 0 ]; do
  case "$1" in
    --site) site="$2"; shift 2 ;;
    --env) env_name="$2"; shift 2 ;;
    --paths) paths="$2"; shift 2 ;;
    --latest) latest="$2"; shift 2 ;;
    --previous) previous="$2"; shift 2 ;;
    *) printf 'install_paths.sh: unknown option %s\n' "$1" >&2; exit 2 ;;
  esac
done
[ -n "$latest" ] || { echo "install_paths.sh: --latest is required" >&2; exit 2; }
[ -n "$previous" ] || previous="$latest"

tmpdir="${TMPDIR:-/tmp}"
work="$(mktemp -d "${tmpdir%/}/treemand-install-probe.XXXXXX")"
# The Go module cache is read-only by default; make it removable first.
trap 'chmod -R u+w "$work" 2>/dev/null; rm -rf "$work"' EXIT INT TERM

# emit PATH STATUS EXIT NOTE: one row, using the fields the last inspect set.
emit() {
  note="$(printf '%s' "$4" | tr '\t\n' '  ' | cut -c1-300)"
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$env_name" "$1" "$2" "$3" \
    "${ver:--}" "${want:--}" "${renders:--}" "${quar:--}" "${qruns:--}" "${hint:--}" "${sig:--}" "$note"
}
reset() { ver=""; want=""; renders=""; quar=""; qruns=""; hint=""; sig=""; }
skip() { reset; emit "$1" skip - "$2"; }

# oneliner HOME LOG ARGS...: the documented one-liner, exactly as a user pipes
# it, with HOME (and so the default dir and man page) inside the temp dir.
# Sets rc and hint (the PATH hint the installer printed; empty when none).
oneliner() {
  h="$1"; log="$2"; shift 2
  mkdir -p "$h"
  set +e
  curl -fsSL "$site/install.sh" |
    HOME="$h" XDG_DATA_HOME="" TREEMAND_VERSION="" TREEMAND_INSTALL_DIR="" \
      TREEMAND_BASE_URL="$site/releases" sh -s -- "$@" >"$log" 2>&1
  rc=$?
  set -e
  # The temp dir is redacted: rows are recorded in a public history.
  hint="$(grep 'is not on your PATH' "$log" | sed "s/^install\.sh: //; s|$work|<tmp>|g" | head -1)"
}

# inspect BIN: sets ver (what `treemand version` reports), renders (1 when
# `--depth=1 --output=json ls` returns a node), and on macOS quar (quarantine
# xattr present), qruns (it still runs when quarantined) and sig (code signature).
inspect() {
  ver=""; renders=0; quar="-"; qruns="-"; sig="-"
  [ -x "$1" ] || { renders="-"; return 0; }
  run="$work/run"
  mkdir -p "$run/cwd" "$run/home" "$run/cache"
  ver="$(HOME="$run/home" "$1" version 2>/dev/null | awk 'NR == 1 { print $2 }')" || ver=""
  json="$(cd "$run/cwd" && HOME="$run/home" TREEMAND_CACHE_DIR="$run/cache" "$1" --depth=1 --output=json ls 2>/dev/null)" || json=""
  case "$json" in *'"name"'*) renders=1 ;; esac
  if [ "$(uname -s)" = Darwin ]; then
    if xattr -p com.apple.quarantine "$1" >/dev/null 2>&1; then
      quar=1
      if "$1" version >/dev/null 2>&1; then qruns=1; else qruns=0; fi
    else
      quar=0
    fi
    sig="$(codesign -dv "$1" 2>&1 | sed -n 's/^Signature=//p' | head -1)"
    [ -n "$sig" ] || sig=unsigned
  fi
}

# judge PATH EXIT WANT PREFIX: emit ok, or fail with the first unmet expectation.
judge() {
  want="$3"
  if [ "$2" != 0 ]; then emit "$1" fail "$2" "$4exit $2: $(tail -1 "$log" 2>/dev/null)"
  elif [ -z "$ver" ]; then emit "$1" fail "$2" "${4}no binary, or it does not report a version"
  elif [ "$want" != source ] && [ "$ver" != "$want" ]; then emit "$1" fail "$2" "${4}installed $ver, latest is $want"
  elif [ "$renders" != 1 ]; then emit "$1" fail "$2" "${4}no tree from --depth=1 --output=json ls"
  elif [ "$quar" = 1 ] && [ "$qruns" != 1 ]; then emit "$1" fail "$2" "${4}quarantined and does not run"
  elif [ "$hint" = "" ]; then emit "$1" fail "$2" "${4}no PATH hint, though the dir is not on PATH"
  else emit "$1" ok "$2" "${4}${5:-}"
  fi
}

path_oneliner() {
  reset; d="$work/oneliner"
  oneliner "$d/home" "$d/log" --dir "$d/bin"
  log="$d/log"; inspect "$d/bin/treemand"
  judge oneliner "$rc" "$latest" ""
}

path_upgrade() {
  reset; d="$work/upgrade"
  oneliner "$d/home" "$d/log1" --version "$previous" --dir "$d/bin"
  log="$d/log1"; inspect "$d/bin/treemand"
  if [ "$rc" != 0 ] || [ "$ver" != "$previous" ]; then
    want="$previous"; emit upgrade fail "$rc" "--version $previous installed ${ver:-nothing}: $(tail -1 "$log")"
    return 0
  fi
  oneliner "$d/home" "$d/log2" --dir "$d/bin"
  log="$d/log2"; inspect "$d/bin/treemand"
  judge upgrade "$rc" "$latest" "" "$previous -> $ver"
}

path_reinstall() {
  reset; d="$work/reinstall"
  oneliner "$d/home" "$d/log1" --dir "$d/bin"
  oneliner "$d/home" "$d/log" --dir "$d/bin"
  log="$d/log"; inspect "$d/bin/treemand"
  stray="$(find "$d/bin" -name '.treemand.*' | head -1)"
  if [ -n "$stray" ]; then want="$latest"; emit reinstall fail "$rc" "a second install left $(basename "$stray")"; return 0; fi
  judge reinstall "$rc" "$latest" "" "same version twice"
}

# uninstall: install to the documented default location, follow the Installation
# page's uninstall steps, and check nothing treemand installed is left behind.
path_uninstall() {
  reset; d="$work/uninstall"; h="$d/home"
  mkdir -p "$d"
  page="$(curl -fsSL "$site/installation/" 2>/dev/null || true)"
  case "$page" in
    *[Uu]ninstall*'.local/bin/treemand'*'treemand.1'*) ;;
    *) want="$latest"; emit uninstall fail - "no uninstall documented at $site/installation/ (wanted the files to delete)"; return 0 ;;
  esac
  oneliner "$h" "$d/log"
  log="$d/log"; inspect "$h/.local/bin/treemand"
  if [ "$rc" != 0 ] || [ "$ver" != "$latest" ]; then judge uninstall "$rc" "$latest" "install before uninstall: "; return 0; fi
  rm -f "$h/.local/bin/treemand" "$h/.local/share/man/man1/treemand.1"
  left="$(find "$h" -name '*treemand*' | sed "s|^$h|~|" | tr '\n' ' ')"
  if [ -n "$left" ]; then want="$latest"; emit uninstall fail 0 "the documented uninstall left: $left"; return 0; fi
  judge uninstall 0 "$latest" "" "documented steps remove everything"
}

# go_run: a temp GOPATH, GOBIN and build cache; nothing lands in the user's.
go_run() { # go_run DIR LOG CWD ARGS...
  gd="$1"; glog="$2"; gcwd="$3"; shift 3
  mkdir -p "$gd/gopath" "$gd/bin" "$gd/cache" "$gd/home"
  set +e
  (cd "$gcwd" && HOME="$gd/home" GOPATH="$gd/gopath" GOBIN="$gd/bin" GOCACHE="$gd/cache" GOFLAGS=-modcacherw "$@") >"$glog" 2>&1
  rc=$?
  set -e
}

# go-source: the Installation page's "Build from source" section.
path_go_source() {
  command -v go >/dev/null 2>&1 || { skip go-source "go not installed"; return 0; }
  command -v git >/dev/null 2>&1 || { skip go-source "git not installed"; return 0; }
  reset; d="$work/go-source"; log="$d/log"
  mkdir -p "$d"
  if ! git clone -q --depth 1 "$go_repo" "$d/src" >"$log" 2>&1; then
    want=source; emit go-source fail 1 "git clone failed: $(tail -1 "$log")"; return 0
  fi
  go_run "$d" "$log" "$d/src/cli/treemand" go install .
  inspect "$d/bin/treemand"; hint="-"
  judge go-source "$rc" source "" "reports $ver"
}

# go-latest: the Installation page's "Go" channel (issue #135).
path_go_latest() {
  command -v go >/dev/null 2>&1 || { skip go-latest "go not installed"; return 0; }
  reset; d="$work/go-latest"; log="$d/log"
  mkdir -p "$d/cwd"
  go_run "$d" "$log" "$d/cwd" go install "$go_module@latest"
  inspect "$d/bin/treemand"; hint="-"
  if [ "$rc" != 0 ]; then
    want="$latest"; emit go-latest fail "$rc" "go install $go_module@latest: $(grep -m1 'declares its path' "$log" || grep -v 'downloading' "$log" | tail -1)"
    return 0
  fi
  judge go-latest "$rc" "$latest" ""
}

# brew: the real Homebrew prefix. Uninstalls again only if it was not installed before.
path_brew() {
  command -v brew >/dev/null 2>&1 || { skip brew "brew not installed"; return 0; }
  reset; d="$work/brew"; log="$d/log"; mkdir -p "$d"
  had=0; brew list --formula treemand >/dev/null 2>&1 && had=1
  set +e; brew install aallbrig/tap/treemand >"$log" 2>&1; rc=$?; set -e
  inspect "$(brew --prefix)/bin/treemand"; hint="-"
  note="was installed before: kept"
  if [ "$had" = 0 ]; then
    if brew uninstall treemand >>"$log" 2>&1; then note="installed, then brew uninstall"; else note="brew uninstall failed"; fi
  fi
  judge brew "$rc" "$latest" "" "$note"
}

for p in $paths; do
  case "$p" in
    oneliner) path_oneliner ;;
    upgrade) path_upgrade ;;
    reinstall) path_reinstall ;;
    uninstall) path_uninstall ;;
    go-source) path_go_source ;;
    go-latest) path_go_latest ;;
    brew) path_brew ;;
    *) skip "$p" "unknown path" ;;
  esac
done

#!/bin/sh
# Provision a treemand dev host (macOS or Linux): the tools `task ci` and the
# dev session need (list: scripts/dev/tools.txt) and the git hooks.
# Dry run by default: it reports and changes nothing. --install installs what
# is missing with the host's package manager (brew, apt or dnf), falling back
# to go install, pipx or nvm, then installs and verifies the git hooks.
# Exits 1 while a required tool is missing or the hooks are not active.

usage() {
  printf '%s\n' \
    "usage: setup.sh [--install]" \
    "  (no flag)  report present and missing tools and the hook state; change nothing" \
    "  --install  install what is missing, then the git hooks" \
    "env: TREEMAND_REPO_ROOT (repo to set up), CHROME (Chrome or Chromium binary), NVM_DIR"
}

case "$0" in */*) tools_dir="${0%/*}" ;; *) tools_dir=. ;; esac
tools_dir="$(cd "$tools_dir" && pwd)"
repo_root="${TREEMAND_REPO_ROOT:-$(cd "$tools_dir/../.." && pwd)}"
hooks_script="${TREEMAND_SETUP_HOOKS:-$tools_dir/hooks.sh}"
nvm_version=v0.40.3
# shellcheck source=scripts/dev/tools.sh
. "$tools_dir/tools.sh"

install=0
for arg in "$@"; do
  case "$arg" in
    --install) install=1 ;;
    -h | --help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
done

os="$(uname -s 2>/dev/null)"
has() { command -v "$1" >/dev/null 2>&1; }
pm=none
case "$os" in
  Darwin) if has brew; then pm=brew; fi ;;
  *) if has apt-get; then pm=apt; elif has dnf; then pm=dnf; elif has brew; then pm=brew; fi ;;
esac
sudo=""
case "$pm" in apt | dnf) if [ "$(id -u 2>/dev/null)" != 0 ]; then sudo=sudo; fi ;; esac

# go install and pipx put tools in these directories; look there too, and
# say so at the end when they are not on the caller's PATH.
extra_path=""
add_path() {
  case ":$PATH:" in *":$1:"*) ;; *) PATH="$PATH:$1"; extra_path="$extra_path $1" ;; esac
}
gobin() {
  b="$(go env GOBIN 2>/dev/null)"
  if [ -z "$b" ]; then p="$(go env GOPATH 2>/dev/null)"; [ -n "$p" ] && b="$p/bin"; fi
  echo "${b:-$HOME/go/bin}"
}
extend_path() { add_path "$(gobin)"; add_path "$HOME/.local/bin"; }
extend_path

# row_method name need brew apt dnf fallback: how this host installs the tool.
# Sets m_kind (pkg, fallback or manual), m_arg and m_cmd (what a person would type).
row_method() {
  pkg="$(tool_pkg "$pm" "$3" "$4" "$5")"
  if [ "$pm" != none ] && [ "$pkg" != - ]; then
    m_kind=pkg
    m_arg="$pkg"
    m_cmd="$(pkg_cmd "$pm" "$pkg" "$sudo")"
  else
    m_arg="$6"
    m_cmd="$(fallback_cmd "$6")"
    case "$6" in url:*) m_kind=manual ;; *) m_kind=fallback ;; esac
  fi
}

apt_updated=0
apt_update() {
  [ "$apt_updated" = 1 ] && return 0
  apt_updated=1
  $sudo apt-get update
}

install_node() {
  if [ ! -s "$(nvm_sh)" ]; then
    # PROFILE=/dev/null: nvm's installer leaves shell profiles alone.
    curl -fsSL "https://raw.githubusercontent.com/nvm-sh/nvm/$nvm_version/install.sh" | PROFILE=/dev/null bash || return 1
  fi
  # shellcheck disable=SC1090
  (cd "$repo_root" && { . "$(nvm_sh)" || true; } && nvm install)
}

run_install() {
  case "$m_kind" in
    pkg)
      case "$pm" in
        brew)
          case "$m_arg" in
            cask:*) brew install --cask "${m_arg#cask:}" ;;
            *) brew install "$m_arg" ;;
          esac
          ;;
        apt) apt_update && $sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y "$m_arg" ;;
        dnf) $sudo dnf install -y "$m_arg" ;;
      esac
      ;;
    fallback)
      case "$m_arg" in
        go:*) has go && go install "${m_arg#go:}" ;;
        pipx:*) has pipx && pipx install "${m_arg#pipx:}" ;;
        gem:*) has gem && $sudo gem install "${m_arg#gem:}" ;;
        nvm) install_node ;;
      esac
      ;;
  esac
}

missing0=" "
tried=" "
note_initial() { tool_present "$1" || missing0="$missing0$1 "; }

# install_one phase name ...: install a tool missing at the start whose method
# belongs to this phase (system packages first, so go and pipx exist after).
install_one() {
  phase="$1"
  shift
  case "$missing0" in *" $1 "*) ;; *) return 0 ;; esac
  tool_present "$1" && return 0
  row_method "$@"
  [ "$m_kind" = "$phase" ] || return 0
  echo "==> $1: $m_cmd"
  run_install || echo "==> $1: install failed"
  tried="$tried$1 "
}
phase_pkg() { install_one pkg "$@"; }
phase_fallback() { install_one fallback "$@"; }

req_missing=0
opt_missing=0
report_row() {
  if tool_present "$1"; then
    case "$missing0" in *" $1 "*) status=installed ;; *) status=ok ;; esac
    how=""
  else
    row_method "$@"
    case "$tried" in *" $1 "*) status=failed ;; *) status=missing ;; esac
    how="$m_cmd"
    if [ "$2" = required ]; then req_missing=$((req_missing + 1)); else opt_missing=$((opt_missing + 1)); fi
  fi
  printf '  %-10s %-14s %-9s %s\n' "$status" "$1" "$2" "$how"
}

if [ "$install" = 1 ]; then mode=install; else mode="dry run"; fi
echo "treemand dev host setup ($mode)"
echo "os: ${os:-unknown}; package manager: $pm${sudo:+ (via sudo)}"
echo

tools_each note_initial
if [ "$install" = 1 ]; then
  tools_each phase_pkg
  extend_path
  tools_each phase_fallback
  echo
fi

printf '  %-10s %-14s %-9s %s\n' STATUS TOOL NEED "HOW TO INSTALL"
tools_each report_row
echo

echo "git hooks"
hooks_ok=1
if [ "$install" = 1 ]; then sh "$hooks_script" install || hooks_ok=0; fi
sh "$hooks_script" verify || hooks_ok=0
echo

echo "$req_missing required tools missing, $opt_missing optional tools missing"
if [ "$install" = 0 ] && { [ $((req_missing + opt_missing)) -gt 0 ] || [ "$hooks_ok" = 0 ]; }; then
  echo "dry run: nothing was changed. Install with: sh scripts/dev/setup.sh --install (or task setup -- --install)"
fi
for d in $extra_path; do
  [ -d "$d" ] && echo "note: add $d to PATH (go install and pipx put tools there)"
done

[ "$req_missing" = 0 ] && [ "$hooks_ok" = 1 ]

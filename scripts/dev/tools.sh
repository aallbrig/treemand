# shellcheck shell=sh disable=SC2154
# Shared by scripts/dev/setup.sh and scripts/dev/bootstrap.sh: reads the tool
# list (tools.txt) and answers "is it present?" and "how is it installed?".
# Source it after setting tools_dir (this directory) and repo_root (SC2154).
# Builtins only, so the callers' tests can run with a stub-only PATH.

tools_file="$tools_dir/tools.txt"

# tools_each FN: calls FN name need brew apt dnf fallback for every row.
# The list is read on fd 3, so FN may run installers that read stdin.
tools_each() {
  while IFS='|' read -r t_name t_need t_brew t_apt t_dnf t_fallback <&3; do
    case "$t_name" in '#'* | '') continue ;; esac
    "$1" "$t_name" "$t_need" "$t_brew" "$t_apt" "$t_dnf" "$t_fallback"
  done 3<"$tools_file"
}

# The Chrome or Chromium binary site:audit would use (same lookup as scripts/site_audit.sh).
chrome_path() {
  if [ -n "${CHROME:-}" ] && [ -x "$CHROME" ]; then echo "$CHROME"; return 0; fi
  for c in google-chrome chromium chromium-browser; do
    if command -v "$c" >/dev/null 2>&1; then command -v "$c"; return 0; fi
  done
  mac="${TREEMAND_MAC_CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
  [ -x "$mac" ] && echo "$mac"
}

# Node at the major version .nvmrc pins: installed through nvm, or already on PATH.
nvm_sh() { echo "${NVM_DIR:-$HOME/.nvm}/nvm.sh"; }
node_present() {
  want=24
  if [ -r "$repo_root/.nvmrc" ]; then read -r want <"$repo_root/.nvmrc" || true; fi
  if [ -s "$(nvm_sh)" ]; then
    # shellcheck disable=SC1090
    (. "$(nvm_sh)" >/dev/null 2>&1; nvm which "$want" >/dev/null 2>&1) && return 0
  fi
  v="$(node --version 2>/dev/null)" || return 1
  major="${v#v}"
  major="${major%%.*}"
  case "$want$major" in *[!0-9]*) return 1 ;; esac
  [ "$major" -ge "$want" ]
}

tool_present() {
  case "$1" in
    node) node_present ;;
    chrome) [ -n "$(chrome_path)" ] ;;
    # Extended edition: "+extended", or "+withdeploy" in newer releases (it includes extended).
    hugo) case "$(hugo version 2>/dev/null)" in *extended* | *withdeploy*) return 0 ;; *) return 1 ;; esac ;;
    *) command -v "$1" >/dev/null 2>&1 ;;
  esac
}

# The package for manager $1 (brew|apt|dnf|none) from a row's columns, or -.
tool_pkg() {
  case "$1" in brew) echo "$2" ;; apt) echo "$3" ;; dnf) echo "$4" ;; *) echo - ;; esac
}

# A human-readable install command for a fallback column.
fallback_cmd() {
  case "$1" in
    go:*) echo "go install ${1#go:}" ;;
    pipx:*) echo "pipx install ${1#pipx:}" ;;
    gem:*) echo "gem install ${1#gem:}" ;;
    nvm) echo "install nvm, then nvm install (version from .nvmrc)" ;;
    url:*) echo "see ${1#url:}" ;;
    *) echo "$1" ;;
  esac
}

# The command a package manager would run for package $2 ($3: sudo prefix or empty).
pkg_cmd() {
  case "$1" in
    brew) case "$2" in cask:*) echo "brew install --cask ${2#cask:}" ;; *) echo "brew install $2" ;; esac ;;
    apt) echo "${3:+$3 }apt-get install -y $2" ;;
    dnf) echo "${3:+$3 }dnf install -y $2" ;;
  esac
}

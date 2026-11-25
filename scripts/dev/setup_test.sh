#!/bin/sh
# shellcheck disable=SC2016,SC2329
# Behaviour tests for scripts/dev/setup.sh. Runs it with a PATH that holds only
# stubs (package managers, go, pipx, the tools themselves), so nothing is
# installed for real and the table is deterministic. The hooks step is a stub
# here; hooks_test.sh covers it against a real git repo.
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0
check() { if ! eval "$2"; then echo "setup_test FAIL: $1" >&2; fail=1; fi; }

bin="$tmp/bin"
calls="$tmp/calls"
sh_bin="$(command -v dash || command -v sh)"
chmod_bin="$(command -v chmod)"

stub() { printf '#!%s\n%s\n' "$sh_bin" "$2" >"$bin/$1"; chmod +x "$bin/$1"; }
# A tool stub that only has to exist on PATH.
tool() { for t; do stub "$t" 'exit 0'; done; }

fresh() {
  rm -rf "$bin" "$calls" "$tmp/gopath"
  mkdir -p "$bin" "$tmp/gopath/bin"
  : >"$calls"
  ln -s "$sh_bin" "$bin/sh"
  stub uname "echo $1"
  stub id "echo 1000"
  stub sudo "\"\$@\""
  # The hooks step: records the call, and is active once installed.
  stub hooks.sh "echo \"hooks \$*\" >>\"$calls\"; [ \"\$1\" = install ] && : >\"$tmp/hooked\"; [ -e \"$tmp/hooked\" ]"
  rm -f "$tmp/hooked"
  # go: `go env` answers GOPATH/GOBIN; `go install mod/cmd/name@v` creates name.
  stub go "echo \"go \$*\" >>\"$calls\"
case \$1 in
  env) [ \"\$2\" = GOPATH ] && echo \"$tmp/gopath\"; [ \"\$2\" = GOVERSION ] && echo go1.26.10; exit 0 ;;
  version) echo 'go version go1.26.10 test/arch' ;;
  install) n=\"\${2##*/}\"; n=\"\${n%%@*}\"; [ \"\$n\" = v4 ] && n=yq; [ \"\$n\" = v8 ] && n=gitleaks
           printf '#!/bin/sh\nexit 0\n' >\"$tmp/gopath/bin/\$n\"; $chmod_bin +x \"$tmp/gopath/bin/\$n\" ;;
esac"
  stub pipx "echo \"pipx \$*\" >>\"$calls\"; printf '#!/bin/sh\nexit 0\n' >\"$bin/\$2\"; $chmod_bin +x \"$bin/\$2\""
}

# brew that "installs" a formula by creating the command it provides.
brew_installs() {
  stub brew "echo \"brew \$*\" >>\"$calls\"
[ \"\$1\" = install ] || exit 0
shift; [ \"\$1\" = --cask ] && shift
case \$1 in
  go-task) n=task ;; coreutils) n=timeout ;; python@3.12) n=python3 ;; *) n=\$1 ;;
esac
if [ \"\$n\" = hugo ]; then printf '#!/bin/sh\necho hugo v0.157.0+extended\n' >\"$bin/hugo\"
else printf '#!/bin/sh\nexit 0\n' >\"$bin/\$n\"; fi
$chmod_bin +x \"$bin/\$n\""
}

run() { env -i HOME="$tmp" PATH="$bin" NVM_DIR="$tmp/nvm" TREEMAND_MAC_CHROME="$tmp/no-chrome" TREEMAND_SETUP_HOOKS="$bin/hooks.sh" "$@" "$sh_bin" "$here/setup.sh" $args >"$tmp/out" 2>&1; echo $? >"$tmp/rc"; }
rc() { cat "$tmp/rc"; }
row() { grep -Eq "^ +$1 +$2 " "$tmp/out"; }

# --- macOS, dry run (the default): reports, installs nothing, fails on missing.
fresh Darwin
brew_installs
tool git curl
args=''
run
check "dry run exits 1 while required tools are missing" '[ "$(rc)" = 1 ]'
check "present tool reported ok" 'row ok git'
check "missing tool reported with its brew command" 'grep -Eq "^ +missing +hugo +required +brew install hugo" "$tmp/out"'
check "cask hint" 'grep -q "brew install --cask google-chrome" "$tmp/out"'
check "go fallback hint" 'grep -q "go install golang.org/x/vuln/cmd/govulncheck@v1.10.0" "$tmp/out"'
check "dry run says how to install" 'grep -q -- "--install" "$tmp/out"'
check "dry run installs nothing" '! grep -Eq "^(brew|pipx) " "$calls" && ! grep -q "^go install" "$calls"'
check "dry run verifies hooks" 'grep -q "^hooks verify" "$calls"'
check "dry run never installs hooks" '! grep -q "^hooks install" "$calls"'
check "names the package manager" 'grep -qi "package manager: brew" "$tmp/out"'

# --- macOS, --install: installs what is missing, then the hooks.
stub node 'echo v24.11.0'
stub claude "echo \"claude \$*\" >>\"$calls\"
case \"\$2\" in
  list) [ -e \"$tmp/plugin\" ] && echo '  gopls-lsp@claude-plugins-official' ;;
  install) : >\"$tmp/plugin\" ;;
esac
exit 0"
stub chrome-stub 'exit 0'
args='--install'
run CHROME="$bin/chrome-stub"
check "install exits 0 once everything required is present" '[ "$(rc)" = 0 ]'
check "installs formulae with brew" 'grep -q "^brew install go-task" "$calls"'
check "installs go-only tools with go install" 'grep -q "^go install golang.org/x/vuln/cmd/govulncheck@v1.10.0" "$calls"'
check "reports installed" 'row installed task'
check "already present stays ok" 'row ok git'
check "installs hooks" 'grep -q "^hooks install" "$calls"'
check "installs nothing already present" '! grep -qx "brew install git" "$calls"'
check "summary" 'grep -Eq "0 required tools missing" "$tmp/out"'
check "installs gopls and dlv" 'row installed gopls && row installed dlv'
check "installs the Go LSP plugin at user scope" 'grep -q "^claude plugin install gopls-lsp@claude-plugins-official --scope user" "$calls" && row installed gopls-lsp'

# --- dlv older than the Go toolchain is unsuitable; --install upgrades it.
stub dlv 'echo "Version: 1.20.2"'
args=''
run CHROME="$bin/chrome-stub"
check "old dlv is unsuitable" 'row unsuitable dlv'
check "old dlv hint" 'grep -q "go install github.com/go-delve/delve/cmd/dlv@latest" "$tmp/out"'
stub dlv 'echo "Version: 1.26.0"'
run CHROME="$bin/chrome-stub"
check "dlv at the toolchain version is ok" 'row ok dlv'

# --- Only an optional tool missing: still exits 0, still reported.
rm -f "$bin/watchexec"
: >"$calls"
args=''
run CHROME="$bin/chrome-stub"
check "optional missing exits 0" '[ "$(rc)" = 0 ]'
check "optional missing reported" 'row missing watchexec'

# --- hugo without the extended edition counts as missing.
stub hugo 'echo hugo v0.157.0'
run CHROME="$bin/chrome-stub"
check "non-extended hugo is unsuitable" 'row unsuitable hugo'
check "non-extended hugo fails" '[ "$(rc)" = 1 ]'

# --- Linux with apt, dry run, not root: sudo apt-get hints and go fallbacks.
fresh Linux
stub apt-get "echo \"apt-get \$*\" >>\"$calls\"; exit 0"
args=''
run
check "linux dry run exits 1" '[ "$(rc)" = 1 ]'
check "names apt" 'grep -qi "package manager: apt" "$tmp/out"'
check "apt hint uses sudo" 'grep -Eq "^ +missing +jq +required +sudo apt-get install -y jq" "$tmp/out"'
check "apt has no go-task: go fallback" 'grep -Eq "^ +missing +task +required +go install github.com/go-task/task/v3/cmd/task@latest" "$tmp/out"'
check "pipx fallback" 'grep -Eq "^ +missing +ruff +required +pipx install ruff" "$tmp/out"'
check "nvm for node" 'grep -Eq "^ +missing +node +required +.*nvm" "$tmp/out"'
check "linux dry run installs nothing" '! grep -q "^apt-get" "$calls"'

# --- Linux with apt, --install, apt failing: failed rows, exit 1, one update.
stub apt-get "echo \"apt-get \$*\" >>\"$calls\"; exit 100"
args='--install'
run
check "failed install exits 1" '[ "$(rc)" = 1 ]'
check "failed install reported" 'row failed jq'
check "apt-get update runs once" '[ "$(grep -c "^apt-get update" "$calls")" = 1 ]'
check "go fallback still tried" 'grep -q "^go install github.com/go-task/task/v3/cmd/task@latest" "$calls"'
check "go-installed tool found in GOPATH/bin" 'row installed task'
check "points at GOPATH/bin when it is not on PATH" 'grep -q "$tmp/gopath/bin" "$tmp/out"'

# --- Linux with dnf, root: no sudo, dnf package names.
fresh Linux
stub id 'echo 0'
stub dnf "echo \"dnf \$*\" >>\"$calls\"; exit 0"
args=''
run
check "names dnf" 'grep -qi "package manager: dnf" "$tmp/out"'
check "dnf package name" 'grep -Eq "^ +missing +shellcheck +required +dnf install -y ShellCheck" "$tmp/out"'

# --- No package manager at all: manual hints, still a clean report.
fresh Linux
args=''
run
check "no manager exits 1" '[ "$(rc)" = 1 ]'
check "no manager says so" 'grep -qi "package manager: none" "$tmp/out"'
check "no manager gives the docs page" 'grep -q "https://jqlang.org/download/" "$tmp/out"'

# --- Arguments.
args='--help'
run
check "--help exits 0" '[ "$(rc)" = 0 ]'
check "--help shows usage" 'grep -q "usage" "$tmp/out"'
args='--bogus'
run
check "unknown flag exits 2" '[ "$(rc)" = 2 ]'

[ "$fail" = 0 ] && echo "setup_test: ok"
exit "$fail"

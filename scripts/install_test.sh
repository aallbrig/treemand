#!/usr/bin/env bash
# Hermetic tests for www/treemand/static/install.sh: a fake release is served
# from a local HTTP server (no network, no real release, no real elevated
# privileges). Run through `task install:test`; part of `task ci`.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 1
script="$PWD/www/treemand/static/install.sh"

work="$(mktemp -d)"
server_pid=""
cleanup() { [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true; rm -rf "$work"; }
trap cleanup EXIT

pass=0
fail=0
ok() { pass=$((pass + 1)); printf '  ok   %s\n' "$1"; }
bad() { fail=$((fail + 1)); printf '  FAIL %s\n' "$1"; }
check() { # check NAME COMMAND...  (passes when the command succeeds)
  local name="$1"; shift
  if "$@" >/dev/null 2>&1; then ok "$name"; else bad "$name"; fi
}

# ── A fake release: latest and v9.9.9, four platforms, real checksums ───────
site="$work/site"
make_release() { # make_release DIR LABEL
  local dir="$1" label="$2" os arch stage
  mkdir -p "$dir"
  : >"$dir/checksums.txt"
  for os in linux darwin; do
    for arch in amd64 arm64; do
      stage="$(mktemp -d -p "$work")"
      printf '#!/bin/sh\necho "treemand %s (fake %s/%s)"\n' "$label" "$os" "$arch" >"$stage/treemand_${os}_${arch}"
      chmod +x "$stage/treemand_${os}_${arch}"
      printf '.TH TREEMAND 1\n' >"$stage/treemand.1"
      tar -czf "$dir/treemand_${os}_${arch}.tar.gz" -C "$stage" "treemand_${os}_${arch}" treemand.1
      printf '%s  treemand_%s_%s.tar.gz\n' "$(sha256sum "$dir/treemand_${os}_${arch}.tar.gz" | cut -d' ' -f1)" "$os" "$arch" >>"$dir/checksums.txt"
    done
  done
}
make_release "$site/latest/download" v1.0.0
make_release "$site/download/v9.9.9" v9.9.9

port="${INSTALL_TEST_PORT:-18282}"
python3 -m http.server "$port" --bind 127.0.0.1 --directory "$site" >"$work/server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 50); do curl -sf "http://127.0.0.1:$port/latest/download/checksums.txt" >/dev/null && break; sleep 0.1; done
base="http://127.0.0.1:$port"

# A stand-in for the privileged helper: it only records how it was called. The
# real thing is never run by these tests.
shim="$work/shim"
mkdir -p "$shim"
printf '#!/bin/sh\necho "$*" >>"%s/sudo.log"\nexit 0\n' "$work" >"$shim/sudo"
chmod +x "$shim/sudo"

# run_install HOME_DIR ARGS...: runs the installer against the fake release.
run_install() {
  local home="$1"; shift
  mkdir -p "$home"
  HOME="$home" TREEMAND_BASE_URL="$base" PATH="$shim:$PATH" sh "$script" "$@"
}

echo "install.sh"

# 1. Default install: user directory, executable, man page, no privileges.
h="$work/h1"; : >"$work/sudo.log"
out="$(run_install "$h" 2>&1)"; rc=$?
[ $rc -eq 0 ] && ok "default install exits 0" || bad "default install exits 0 (rc=$rc: $out)"
check "binary installed to ~/.local/bin and executable" test -x "$h/.local/bin/treemand"
case "$("$h/.local/bin/treemand" 2>&1)" in *"v1.0.0"*) ok "installed the latest release" ;; *) bad "installed the latest release" ;; esac
check "man page installed under ~/.local/share/man/man1" test -f "$h/.local/share/man/man1/treemand.1"
check "no privileged helper was used" test ! -s "$work/sudo.log"
case "$out" in *"checksum OK"*) ok "reports the verified checksum" ;; *) bad "reports the verified checksum" ;; esac
case "$out" in *"not on your PATH"*) ok "warns when the directory is not on PATH" ;; *) bad "warns when the directory is not on PATH" ;; esac
check "no temporary files left in the install directory" test -z "$(find "$h/.local/bin" -name '.treemand.*')"

# 2. Pinned versions, with and without the leading v.
for v in v9.9.9 9.9.9; do
  h="$work/h2-$v"
  run_install "$h" --version "$v" >/dev/null 2>&1
  case "$("$h/.local/bin/treemand" 2>&1)" in *"v9.9.9"*) ok "--version $v installs that release" ;; *) bad "--version $v installs that release" ;; esac
done

# 3. A tampered archive must be refused and nothing installed.
rm -rf "$work/tamper" && cp -r "$site" "$work/tamper"
printf 'tampered' >>"$work/tamper/latest/download/treemand_linux_amd64.tar.gz"
printf 'tampered' >>"$work/tamper/latest/download/treemand_darwin_amd64.tar.gz"
printf 'tampered' >>"$work/tamper/latest/download/treemand_linux_arm64.tar.gz"
printf 'tampered' >>"$work/tamper/latest/download/treemand_darwin_arm64.tar.gz"
tport=$((port + 1))
python3 -m http.server "$tport" --bind 127.0.0.1 --directory "$work/tamper" >/dev/null 2>&1 &
tpid=$!
for _ in $(seq 1 50); do curl -sf "http://127.0.0.1:$tport/latest/download/checksums.txt" >/dev/null && break; sleep 0.1; done
h="$work/h3"; mkdir -p "$h"
out="$(HOME="$h" TREEMAND_BASE_URL="http://127.0.0.1:$tport" sh "$script" 2>&1)"; rc=$?
kill "$tpid" 2>/dev/null || true
[ $rc -ne 0 ] && ok "a tampered archive is refused (exit $rc)" || bad "a tampered archive is refused"
case "$out" in *"checksum mismatch"*) ok "the refusal says checksum mismatch" ;; *) bad "the refusal says checksum mismatch" ;; esac
check "nothing was installed after a mismatch" test ! -e "$h/.local/bin/treemand"

# 4. checksums.txt without the asset's line.
rm -rf "$work/nosum" && cp -r "$site" "$work/nosum"
: >"$work/nosum/latest/download/checksums.txt"
nport=$((port + 2))
python3 -m http.server "$nport" --bind 127.0.0.1 --directory "$work/nosum" >/dev/null 2>&1 &
npid=$!
for _ in $(seq 1 50); do curl -sf "http://127.0.0.1:$nport/latest/download/checksums.txt" >/dev/null && break; sleep 0.1; done
h="$work/h4"; mkdir -p "$h"
out="$(HOME="$h" TREEMAND_BASE_URL="http://127.0.0.1:$nport" sh "$script" 2>&1)"; rc=$?
kill "$npid" 2>/dev/null || true
if [ $rc -ne 0 ] && [[ "$out" == *"no entry"* ]]; then ok "a missing checksum entry is refused"; else bad "a missing checksum entry is refused"; fi

# 5. Unsupported platforms and an unreachable release.
out="$(TREEMAND_ARCH=mips run_install "$work/h5" 2>&1)"; rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"unsupported CPU architecture"* ]]; then ok "an unsupported CPU is refused clearly"; else bad "an unsupported CPU is refused clearly"; fi
out="$(TREEMAND_OS=windows run_install "$work/h5" 2>&1)"; rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"treemand_windows_amd64.zip"* ]]; then ok "Windows is pointed at the zip"; else bad "Windows is pointed at the zip"; fi
out="$(TREEMAND_OS=plan9 run_install "$work/h5" 2>&1)"; rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"unsupported operating system"* ]]; then ok "an unsupported OS is refused clearly"; else bad "an unsupported OS is refused clearly"; fi
out="$(HOME="$work/h6" TREEMAND_BASE_URL="http://127.0.0.1:1" sh "$script" 2>&1)"; rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"could not download"* ]]; then ok "an unreachable release fails with a clear message"; else bad "an unreachable release fails with a clear message"; fi

# 6. Detection overrides pick the right asset.
out="$(TREEMAND_OS=darwin TREEMAND_ARCH=arm64 run_install "$work/h7" --dry-run 2>&1)"
[[ "$out" == *"treemand_darwin_arm64.tar.gz"* ]] && ok "TREEMAND_OS and TREEMAND_ARCH choose the asset" || bad "TREEMAND_OS and TREEMAND_ARCH choose the asset"
out="$(TREEMAND_ARCH=aarch64 run_install "$work/h7" --dry-run 2>&1)"
[[ "$out" == *"_arm64.tar.gz"* ]] && ok "aarch64 maps to arm64" || bad "aarch64 maps to arm64"

# 7. --dry-run touches nothing and needs no network.
h="$work/h8"
out="$(HOME="$h" TREEMAND_BASE_URL="http://127.0.0.1:1" sh "$script" --dry-run 2>&1)"; rc=$?
[ $rc -eq 0 ] && [[ "$out" == *"would download"* ]] && ok "--dry-run explains and exits 0 without network" || bad "--dry-run explains and exits 0 without network"
check "--dry-run installs nothing" test ! -e "$h/.local/bin"

# 8. --dir, --no-man, and upgrading over an existing install.
h="$work/h9"
run_install "$h" --dir "$work/custom/bin" --no-man >/dev/null 2>&1
check "--dir installs there" test -x "$work/custom/bin/treemand"
check "--no-man skips the man page" test ! -e "$h/.local/share/man/man1/treemand.1"
run_install "$h" --dir "$work/custom/bin" --version v9.9.9 --no-man >/dev/null 2>&1
case "$("$work/custom/bin/treemand" 2>&1)" in *"v9.9.9"*) ok "re-running upgrades in place" ;; *) bad "re-running upgrades in place" ;; esac

# 9. Privileges: never without --system; with it, only for an unwritable directory.
ro="$work/readonly"; mkdir -p "$ro"; chmod 555 "$ro"
if [ "$(id -u)" != 0 ]; then
  : >"$work/sudo.log"
  out="$(run_install "$work/h10" --dir "$ro" 2>&1)"; rc=$?
  if [ $rc -ne 0 ] && [[ "$out" == *"not writable"* ]]; then ok "an unwritable --dir fails without using privileges"; else bad "an unwritable --dir fails without using privileges"; fi
  check "no privileged helper without --system" test ! -s "$work/sudo.log"
  : >"$work/sudo.log"
  run_install "$work/h10" --system --dir "$ro" >/dev/null 2>&1
  if grep -q 'cp ' "$work/sudo.log" && grep -q "$ro" "$work/sudo.log"; then ok "--system uses the privileged helper for an unwritable directory"; else bad "--system uses the privileged helper for an unwritable directory"; fi
else
  echo "  skip privileged-helper tests (running as root)"
fi
chmod 755 "$ro"

# 10. Option handling.
out="$(sh "$script" --help 2>&1)"; rc=$?
[ $rc -eq 0 ] && [[ "$out" == *"Usage: install.sh"* ]] && ok "--help prints usage and exits 0" || bad "--help prints usage and exits 0"
out="$(sh "$script" --bogus 2>&1)"; rc=$?
[ $rc -ne 0 ] && [[ "$out" == *"unknown option"* ]] && ok "an unknown option is refused" || bad "an unknown option is refused"
out="$(sh "$script" --version 2>&1)"; rc=$?
[ $rc -ne 0 ] && [[ "$out" == *"needs a value"* ]] && ok "an option missing its value is refused" || bad "an option missing its value is refused"

# 11. The published script and the Installation page agree with the repository.
echo "documentation"
gomod="$(awk '$1 == "go" { split($2, v, "."); print v[1] "." v[2]; exit }' cli/treemand/go.mod)"
page="www/treemand/content/installation.md"
if grep -q "Requires Go $gomod+" "$page"; then ok "the Installation page states Go $gomod (go.mod)"; else bad "the Installation page states Go $gomod (go.mod)"; fi
if grep -rq "aallbrig.github.io/treemand/install.sh" "$page" www/treemand/data 2>/dev/null; then ok "the Installation page documents the published install.sh URL"; else bad "the Installation page documents the published install.sh URL"; fi
if grep -q "install.sh" README.md; then ok "the README documents install.sh"; else bad "the README documents install.sh"; fi

echo
echo "install tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]

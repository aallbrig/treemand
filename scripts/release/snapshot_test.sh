#!/bin/sh
# Check a GoReleaser dist/ against the release asset contract that install.sh,
# the treemand.dev mirror, the Homebrew tap and release-packages.yml rely on
# (see cli/treemand/.goreleaser.yaml).
#
#   sh scripts/release/snapshot_test.sh [dist-dir]   (default: cli/treemand/dist)
#
# task release:snapshot (scripts/release/snapshot.sh) builds the dist and runs this.
set -eu
dist="${1:-cli/treemand/dist}"
fails=0
fail() { echo "FAIL: $*"; fails=$((fails + 1)); }
pass() { echo "ok:   $*"; }

[ -d "$dist" ] || { echo "no dist directory at $dist" >&2; exit 2; }

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

sums="$dist/checksums.txt"
[ -f "$sums" ] && pass "checksums.txt exists" || fail "checksums.txt is missing"

# checked <file>: checksums.txt has a "<sha256>  <name>" line matching the bytes.
checked() {
  _name="$(basename "$1")"
  _want="$(awk -v f="$_name" '$2 == f { print $1 }' "$sums" 2>/dev/null)"
  if [ -z "$_want" ]; then fail "checksums.txt has no line for $_name"
  elif [ "$_want" != "$(sha256 "$1")" ]; then fail "checksums.txt line for $_name does not match"
  else pass "$_name is in checksums.txt"; fi
}

# Archives: the exact names and members install.sh and the tap unpack.
for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  a="$dist/treemand_$target.tar.gz"
  if [ ! -f "$a" ]; then fail "treemand_$target.tar.gz is missing"; continue; fi
  members="$(tar tzf "$a" | sed 's|^\./||' | LC_ALL=C sort | tr '\n' ' ')"
  want="treemand.1 treemand_$target "
  [ "$members" = "$want" ] && pass "treemand_$target.tar.gz holds: $want" ||
    fail "treemand_$target.tar.gz holds '$members', want '$want'"
  checked "$a"
  # The tap and release-packages.yml read this line with a substring grep.
  n="$(grep -c "treemand_$target.tar.gz" "$sums" || true)"
  [ "$n" = 1 ] && pass "one checksums.txt line mentions treemand_$target.tar.gz" ||
    fail "$n checksums.txt lines mention treemand_$target.tar.gz, want 1 (consumers grep for it)"
  [ -f "$a.sbom.json" ] && pass "treemand_$target.tar.gz.sbom.json exists" ||
    fail "treemand_$target.tar.gz.sbom.json is missing"
done

z="$dist/treemand_windows_amd64.zip"
if [ -f "$z" ]; then
  members="$(unzip -Z1 "$z" | tr '\n' ' ')"
  [ "$members" = "treemand_windows_amd64.exe " ] && pass "treemand_windows_amd64.zip holds the .exe only" ||
    fail "treemand_windows_amd64.zip holds '$members', want 'treemand_windows_amd64.exe'"
  checked "$z"
  n="$(grep -c treemand_windows_amd64.zip "$sums" || true)"
  [ "$n" = 1 ] && pass "one checksums.txt line mentions treemand_windows_amd64.zip" ||
    fail "$n checksums.txt lines mention treemand_windows_amd64.zip, want 1 (consumers grep for it)"
else
  fail "treemand_windows_amd64.zip is missing"
fi

# Linux packages: one of each format per architecture, all checksummed.
for ext in deb rpm apk; do
  n=0
  for p in "$dist"/treemand_*."$ext" "$dist"/treemand-*."$ext"; do
    [ -f "$p" ] || continue
    n=$((n + 1))
    checked "$p"
  done
  [ "$n" -eq 2 ] && pass "two .$ext packages" || fail "found $n .$ext packages, want 2"
done

# A package installs the binary as plain treemand plus the man page.
deb="$(ls "$dist"/treemand_*_amd64.deb 2>/dev/null | head -n 1)"
if [ -n "$deb" ] && command -v dpkg-deb >/dev/null 2>&1; then
  files="$(dpkg-deb -c "$deb")"
  for f in ./usr/bin/treemand ./usr/share/man/man1/treemand.1; do
    echo "$files" | grep -q " $f\$" && pass "$(basename "$deb") installs $f" ||
      fail "$(basename "$deb") does not install $f"
  done
fi

# The binary for this host runs and reports a version.
host="$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
if [ -f "$dist/treemand_$host.tar.gz" ]; then
  tmp="$(mktemp -d)"
  tar xzf "$dist/treemand_$host.tar.gz" -C "$tmp"
  if out="$("$tmp/treemand_$host" version 2>&1)" && echo "$out" | grep -q 'v[0-9]'; then
    pass "treemand_$host version: $(echo "$out" | head -n 1)"
  else
    fail "treemand_$host version printed '$out'"
  fi
  rm -rf "$tmp"
fi

[ "$fails" -eq 0 ] || { echo "$fails contract check(s) failed"; exit 1; }
echo "release asset contract holds"

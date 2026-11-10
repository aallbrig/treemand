#!/bin/sh
# treemand installer: downloads the release archive for your OS and CPU,
# verifies its SHA-256 checksum against the release's checksums.txt, and
# installs the binary (and man page). It never uses sudo unless you pass --system.
#
#   curl -fsSL https://aallbrig.github.io/treemand/install.sh | sh
#   curl -fsSL https://aallbrig.github.io/treemand/install.sh | sh -s -- --version v0.6.1 --dir ~/bin
#
# The checksum protects against a corrupted or truncated download. It comes from
# the same GitHub release as the archive, so it does not defend against a
# compromised release: read this script, or build from source, if that matters.
set -eu

BASE_URL="${TREEMAND_BASE_URL:-https://github.com/aallbrig/treemand/releases}"
version="${TREEMAND_VERSION:-latest}"
dir="${TREEMAND_INSTALL_DIR:-}"
system=0
dry_run=0
want_man=1

say() { printf '%s\n' "$*"; }
warn() { printf 'install.sh: %s\n' "$*" >&2; }
die() { warn "$*"; exit 1; }

usage() {
  cat <<'USAGE'
Usage: install.sh [options]

Installs treemand from its GitHub release, verifying the SHA-256 checksum.

Options:
  --version VERSION  install this release (for example v0.6.1); default: latest
  --dir DIR          install the binary into DIR (default: ~/.local/bin)
  --system           install into /usr/local/bin, using sudo if it is not writable
  --no-man           do not install the man page
  --dry-run          show what would be downloaded and installed, then stop
  -h, --help         show this help

Environment: TREEMAND_VERSION, TREEMAND_INSTALL_DIR, TREEMAND_OS, TREEMAND_ARCH
(override detection), TREEMAND_BASE_URL (alternative release location).
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version) [ $# -ge 2 ] || die "--version needs a value"; version="$2"; shift 2 ;;
    --dir) [ $# -ge 2 ] || die "--dir needs a value"; dir="$2"; shift 2 ;;
    --system) system=1; shift ;;
    --no-man) want_man=0; shift ;;
    --dry-run) dry_run=1; shift ;;
    -h | --help) usage; exit 0 ;;
    *) die "unknown option: $1 (try --help)" ;;
  esac
done

case "$version" in latest | v*) ;; *) version="v$version" ;; esac

os="${TREEMAND_OS:-$(uname -s)}"
case "$os" in
  Linux | linux) os=linux ;;
  Darwin | darwin) os=darwin ;;
  MINGW* | MSYS* | CYGWIN* | windows | Windows*)
    die "on Windows, download treemand_windows_amd64.zip from $BASE_URL/latest and unzip it onto your PATH (see https://aallbrig.github.io/treemand/installation/)" ;;
  *) die "unsupported operating system: $os" ;;
esac
arch="${TREEMAND_ARCH:-$(uname -m)}"
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) die "unsupported CPU architecture: $arch (release builds exist for amd64 and arm64)" ;;
esac

asset="treemand_${os}_${arch}.tar.gz"
if [ "$version" = latest ]; then
  release_url="$BASE_URL/latest/download"
else
  release_url="$BASE_URL/download/$version"
fi

if [ "$system" = 1 ]; then
  [ -n "$dir" ] || dir=/usr/local/bin
  mandir=/usr/local/share/man/man1
else
  [ -n "$dir" ] || dir="${HOME:?HOME is not set}/.local/bin"
  mandir="${XDG_DATA_HOME:-${HOME:-}/.local/share}/man/man1"
fi

if [ "$dry_run" = 1 ]; then
  say "would download  $release_url/$asset"
  say "would verify    against $release_url/checksums.txt (SHA-256)"
  say "would install   treemand into $dir"
  if [ "$want_man" = 1 ]; then say "would install   man page into $mandir"; fi
  exit 0
fi

fetch() { # fetch URL DEST
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 2 -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$2" "$1"
  else
    die "curl or wget is required"
  fi
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$1" | sed 's/^.*= //'
  else
    die "sha256sum, shasum or openssl is required to verify the download"
  fi
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

say "downloading $asset ($version)"
fetch "$release_url/$asset" "$tmp/$asset" || die "could not download $release_url/$asset"
fetch "$release_url/checksums.txt" "$tmp/checksums.txt" || die "could not download $release_url/checksums.txt"

want="$(awk -v f="$asset" '$2 == f { print $1 }' "$tmp/checksums.txt")"
[ -n "$want" ] || die "checksums.txt has no entry for $asset"
have="$(sha256_of "$tmp/$asset")"
if [ "$want" != "$have" ]; then
  die "checksum mismatch for $asset (expected $want, got $have); nothing was installed"
fi
say "checksum OK ($have)"

tar -xzf "$tmp/$asset" -C "$tmp" || die "could not unpack $asset"
bin="$tmp/treemand_${os}_${arch}"
[ -f "$bin" ] || die "$asset does not contain treemand_${os}_${arch}"

# put SRC DIR NAME MODE: copy under a temporary name, then rename, so a running
# or half-written file is never replaced in place. With --system and a directory
# that is not writable, the same steps run through sudo; that is the only place
# sudo is ever used.
put() {
  if [ "$system" = 1 ] && ! mkdir -p "$2" 2>/dev/null; then
    command -v sudo >/dev/null 2>&1 || die "$2 is not writable and sudo is not available"
    say "using sudo to write to $2"
    sudo mkdir -p "$2" && sudo cp "$1" "$2/.$3.$$" && sudo chmod "$4" "$2/.$3.$$" && sudo mv "$2/.$3.$$" "$2/$3"
  elif [ "$system" = 1 ] && [ ! -w "$2" ]; then
    command -v sudo >/dev/null 2>&1 || die "$2 is not writable and sudo is not available"
    say "using sudo to write to $2"
    sudo cp "$1" "$2/.$3.$$" && sudo chmod "$4" "$2/.$3.$$" && sudo mv "$2/.$3.$$" "$2/$3"
  else
    mkdir -p "$2" || die "cannot create $2 (try --dir, or --system)"
    [ -w "$2" ] || die "$2 is not writable (try --dir, or --system to use sudo)"
    cp "$1" "$2/.$3.$$" && chmod "$4" "$2/.$3.$$" && mv "$2/.$3.$$" "$2/$3"
  fi
}

put "$bin" "$dir" treemand 0755
say "installed $dir/treemand"
if [ "$want_man" = 1 ] && [ -f "$tmp/treemand.1" ]; then
  put "$tmp/treemand.1" "$mandir" treemand.1 0644 && say "installed man page $mandir/treemand.1"
fi

case ":${PATH:-}:" in
  *":$dir:"*) ;;
  *) warn "$dir is not on your PATH; add it, for example:  export PATH=\"$dir:\$PATH\"" ;;
esac
"$dir/treemand" version 2>/dev/null || true

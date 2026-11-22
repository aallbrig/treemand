package e2e

// fakeCLIs are the stand-in tools, keyed by executable name. Each prints the
// help style of a real framework, so the whole path (exec, parse, render) is
// exercised without any real CLI installed.
var fakeCLIs = map[string]string{
	// The root answers at once; every subcommand takes $FAKE_DELAY seconds,
	// like a big CLI whose full crawl is slow. Logs to $FAKE_LOG.
	"fakeslow": `[ -n "$FAKE_LOG" ] && echo "$*" >>"$FAKE_LOG"
case "$1" in
alpha|beta)
  sleep "${FAKE_DELAY:-0}"
  if [ "$2" = deep ]; then printf 'deep things\n\nUsage:\n  fakeslow %s deep\n\nFlags:\n  -h, --help   help\n      --deep-flag  very deep\n' "$1"
  else printf '%s things\n\nUsage:\n  fakeslow %s [command]\n\nAvailable Commands:\n  deep        go deeper\n\nFlags:\n  -h, --help   help\n      --%s-flag  only for %s\n' "$1" "$1" "$1" "$1"; fi;;
*) printf 'fakeslow is slow below the root\n\nUsage:\n  fakeslow [command]\n\nAvailable Commands:\n  alpha       first\n  beta        second\n\nFlags:\n  -h, --help   help for fakeslow\n';;
esac
`,

	// Logs every invocation to $FAKE_LOG, so cache tests can prove a run never
	// touched the CLI. Three cobra-style commands, each with one subcommand.
	"fakecache": `[ -n "$FAKE_LOG" ] && echo "$*" >>"$FAKE_LOG"
case "$1" in
alpha|beta|gamma) printf '%s things\n\nUsage:\n  fakecache %s [command]\n\nAvailable Commands:\n  deep        go deeper\n\nFlags:\n  -h, --help   help for %s\n      --%s-only  only for %s\n' "$1" "$1" "$1" "$1" "$1";;
*) printf 'fakecache caches things\n\nUsage:\n  fakecache [command]\n\nAvailable Commands:\n  alpha       first\n  beta        second\n  gamma       third\n\nFlags:\n  -h, --help   help for fakecache\n';;
esac
`,

	// cobra (Go): "Available Commands:" and "Flags:" sections.
	"fakecobra": `case "$1" in
build) cat <<'H'
Build things

Usage:
  fakecobra build [flags]

Flags:
  -h, --help            help for build
  -o, --output string   output file
H
;;
deploy) cat <<'H'
Deploy things

Usage:
  fakecobra deploy [flags]

Flags:
      --dry-run   do not apply
  -h, --help      help for deploy
H
;;
*) cat <<'H'
fakecobra does cobra-style things

Usage:
  fakecobra [command]

Available Commands:
  build       Build things
  deploy      Deploy things

Flags:
  -h, --help      help for fakecobra
  -v, --verbose   verbose output
  -q, --quiet     suppress output
      --no-cache  disable cache
H
;;
esac
`,

	// argparse (Python): subcommands in a positional-arguments block.
	"fakeargp": `case "$1" in
init) printf 'usage: fakeargp init [-h] [--force]\n\nInitialise a project\n\noptions:\n  -h, --help  show this help message and exit\n  --force     overwrite\n';;
run) printf 'usage: fakeargp run [-h] [--jobs N]\n\nRun it\n\noptions:\n  -h, --help  show this help message and exit\n  --jobs N    parallel jobs\n';;
*) printf 'usage: fakeargp [-h] [--config FILE] {init,run} ...\n\nFake argparse tool\n\npositional arguments:\n  {init,run}\n    init      initialise a project\n    run       run it\n\noptions:\n  -h, --help     show this help message and exit\n  --config FILE  config file\n';;
esac
`,

	// clap (Rust): name+version first line, SUBCOMMANDS section.
	"fakeclap": `case "$1" in
add) printf 'fakeclap-add 1.0\nAdd a thing\n\nUSAGE:\n    fakeclap add [OPTIONS] <NAME>\n\nOPTIONS:\n    -h, --help    Print help information\n    -f, --force   Overwrite\n';;
list) printf 'fakeclap-list 1.0\nList things\n\nUSAGE:\n    fakeclap list\n\nOPTIONS:\n    -h, --help    Print help information\n';;
*) printf 'fakeclap 1.0\nA fake clap tool\n\nUSAGE:\n    fakeclap [OPTIONS] <SUBCOMMAND>\n\nOPTIONS:\n    -h, --help       Print help information\n    -V, --version    Print version information\n\nSUBCOMMANDS:\n    add     Add a thing\n    list    List things\n';;
esac
`,

	// A CLI whose --version is slow (like aws or pip, which start an interpreter):
	// a cache hit must never pay for it.
	"fakeslowver": `if [ "$1" = "--version" ]; then sleep 3; echo "fakeslowver 1.0"; exit 0; fi
case "$1" in
alpha) printf 'Alpha thing\n\nFlags:\n  -h, --help   help\n';;
*) printf 'fakeslowver does things\n\nUsage:\n  fakeslowver [command]\n\nAvailable Commands:\n  alpha   Alpha thing\n\nFlags:\n  -h, --help   help\n';;
esac
`,

	// A CLI that never answers: --timeout must bound discovery.
	"fakehang": "exec sleep 600\n",

	// Hangs on its first two calls (the racing --help and -h of the root, as
	// kubectl on a loaded CI runner) and answers after that. Logs to $FAKE_LOG.
	"fakeslowstart": `echo "$*" >>"$FAKE_LOG"
[ "$(wc -l <"$FAKE_LOG")" -le 2 ] && exec sleep 600
case "$1" in
alpha) printf 'alpha things\n\nUsage:\n  fakeslowstart alpha\n\nFlags:\n  -h, --help   help for alpha\n';;
*) printf 'fakeslowstart starts slowly\n\nUsage:\n  fakeslowstart [command]\n\nAvailable Commands:\n  alpha       first\n\nFlags:\n  -h, --help   help for fakeslowstart\n';;
esac
`,

	// Prints errors and exits non-zero for any help request.
	"fakenohelp": "echo 'unknown flag' >&2\nexit 2\n",

	// apt (#65): no help lists an option. Its commands print the general
	// help of the tool they forward to, or fail when the Linux Mint wrapper
	// needs sudo; only the man page names the options (fakeAptMan).
	"fakeapt": `case "$1" in
install) echo 'sudo: a terminal is required to read the password' >&2; exit 1;;
list|show) cat <<'H'
apt 2.6.2 (arm64)
Usage: apt [options] command

Most used commands:
  list - list packages based on package names
  show - show package details
  install - install packages

See apt(8) for more information about the available commands.
H
;;
*) cat <<'H'
Usage: fakeapt command [options]

Commands:
  install          - Install and/or upgrade packages
  list             - List packages handled by the system
  show             - Display detailed information about a package
H
;;
esac
`,
}

// fakeAptMan is man as it answers for fakeapt. TestApt_* put it first on
// PATH; every other test keeps the real man, for which no fake has a page.
const fakeAptMan = `#!/bin/sh
[ "$1" = fakeapt ] || exit 16
cat <<'M'
FAKEAPT(8)                            APT                           FAKEAPT(8)

NAME
       fakeapt - command-line interface

SYNOPSIS
       fakeapt [-h] [-o=config_string] [-c=config_file] [-t=target_release]
           [-a=architecture] {list | show | install pkg}

DESCRIPTION
       fakeapt provides a high-level commandline interface.
M
`

package e2e

// fakeCLIs are the stand-in tools, keyed by executable name. Each prints the
// help style of a real framework, so the whole path (exec, parse, render) is
// exercised without any real CLI installed.
var fakeCLIs = map[string]string{
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

	// A CLI that never answers: --timeout must bound discovery.
	"fakehang": "exec sleep 600\n",

	// Prints errors and exits non-zero for any help request.
	"fakenohelp": "echo 'unknown flag' >&2\nexit 2\n",
}

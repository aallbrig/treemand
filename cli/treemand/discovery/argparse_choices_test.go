package discovery_test

import (
	"slices"
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

// argparse subparsers (issue #81), excerpts from the corpus fixtures.
func TestParseHelp_argparseChoiceBlocks(t *testing.T) {
	cases := map[string]struct {
		help string
		want []string
	}{
		"pre-commit (positional arguments, complete braces)": {
			"usage: pre-commit [-h] [-V]\n\npositional arguments:\n" +
				"  {autoupdate,clean,gc,run,help}\n" +
				"    autoupdate          Auto-update pre-commit config to the latest repos'\n" +
				"                        versions.\n" +
				"    clean               Clean out pre-commit files.\n" +
				"    gc                  Clean unused cached repos.\n" +
				"    run                 Run hooks.\n" +
				"    help                Show help for a specific command.\n\n" +
				"options:\n  -h, --help            show this help message and exit\n",
			[]string{"autoupdate", "clean", "gc", "run"},
		},
		"pipx (subcommands:, wrapped braces)": {
			"usage: pipx [-h]\n\nsubcommands:\n  Get help for commands with pipx COMMAND --help\n\n" +
				"  {install,uninject,inject,upgrade,list,\n" +
				"    install             Install a package\n" +
				"    uninject            Uninstall injected packages from an existing Virtual\n" +
				"                        Environment\n" +
				"    inject              Install packages into an existing Virtual Environment\n" +
				"    upgrade             Upgrade a package\n" +
				"    list                List installed packages\n" +
				"    run                 Download the latest version of a package to a\n" +
				"                        temporary virtual environment, then run an app from\n" +
				"                        it.\n",
			[]string{"install", "uninject", "inject", "upgrade", "list", "run"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := discovery.ParseHelpOutput(c.help)
			for _, w := range c.want {
				if !slices.Contains(p.Subcommands, w) {
					t.Errorf("missing %q; got %v", w, p.Subcommands)
				}
			}
			for _, bad := range []string{"versions", "Environment", "temporary", "it"} {
				if slices.Contains(p.Subcommands, bad) {
					t.Errorf("continuation word %q became a subcommand", bad)
				}
			}
		})
	}
}

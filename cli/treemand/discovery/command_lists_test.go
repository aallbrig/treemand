package discovery_test

import (
	"slices"
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

// Command-list formats, excerpts from the corpus fixtures.
func TestParseHelp_commandListFormats(t *testing.T) {
	cases := map[string]struct {
		cli, help string
		want      []string
		notWant   []string
	}{
		"kingpin flush-left rows (#165)": {"promtool",
			"usage: promtool [<flags>] <command> [<args> ...]\n\nTooling for the Prometheus monitoring system.\n\nFlags:\n  -h, --[no-]help  Show context-sensitive help.\n\nCommands:\n" +
				"help [<command>...]\n    Show help.\n\n" +
				"check service-discovery [<flags>] <config-file> <job>\n    Perform service discovery for the given job name.\n\n" +
				"check config [<flags>] <config-files>...\n    Check if the config files are valid or not.\n\n" +
				"query instant [<flags>] <server> <expr>\n    Run instant query.\n\n" +
				"query series --match=MATCH [<flags>] <server>\n    Run series query.\n\n" +
				"check metrics [<flags>]\n    Lint metrics.\n\n    examples:\n\n    $ cat metrics.prom | promtool check metrics\n\n" +
				"test rules <test-rule-file>...\n    Unit tests for rules.\n\n" +
				"tsdb bench write [<flags>] [<file>]\n    Run a write performance benchmark.\n",
			[]string{"check", "query", "test", "tsdb"}, []string{"service-discovery", "config", "instant"}},
		"thor rows name the program (#167)": {"thor",
			"Commands:\n  thor help [COMMAND]  # Describe available commands or one specific command\n" +
				"  thor install NAME    # Install an optionally named Thor file into your syst...\n" +
				"  thor list [SEARCH]   # List the available thor commands (--substring means ...\n" +
				"  thor version         # Show Thor version\n",
			[]string{"install", "list", "version"}, []string{"thor"}},
		"yarn bullets (#157)": {"yarn",
			"Usage: yarn [command] [flags]\n\n  Displays help information.\n\n  Commands:\n    - access\n    - add\n    - audit\n    - install\n    - remove\n\n  Run `yarn help COMMAND` for more information on specific commands.\n",
			[]string{"add", "install", "remove"}, nil},
		"busybox function list (#158)": {"busybox",
			"Usage: busybox [function [arguments]...]\n   or: busybox --list[-full]\n\nCurrently defined functions:\n" +
				"\t[, [[, acpid, adjtimex, ar, arch, arp, arping, ascii, ash, awk, base64,\n" +
				"\tbasename, bc, blkdiscard, ls, sh, tar, zcat\n",
			[]string{"acpid", "ls", "sh", "tar"}, []string{"[", "[["}},
		"openssl last grid row (#162)": {"openssl",
			"help:\n\nStandard commands\n" +
				"asn1parse         ca                ciphers           cmp               \n" +
				"storeutl          ts                verify            version           \n" +
				"x509              \n\nMessage Digest commands (see the `dgst' command for more details)\n" +
				"blake2b512        blake2s256        md4               md5               \n",
			[]string{"asn1parse", "version", "x509", "md5"}, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := discovery.ParseHelpOutputFor(c.help, c.cli)
			for _, w := range c.want {
				if !slices.Contains(p.Subcommands, w) {
					t.Errorf("missing %q; got %v", w, p.Subcommands)
				}
			}
			for _, w := range c.notWant {
				if slices.Contains(p.Subcommands, w) {
					t.Errorf("%q is not a subcommand; got %v", w, p.Subcommands)
				}
			}
		})
	}
}

// Man-page command references (issue #159): "bundle install(1) bundle-install.1.html".
func TestParseHelp_manPageCommandReferences(t *testing.T) {
	help := "BUNDLE(1)                   General Commands Manual                  BUNDLE(1)\n\n" +
		"NAME\n       bundle - Ruby Dependency Management\n\n" +
		"PRIMARY COMMANDS\n" +
		"       bundle install(1) bundle-install.1.html\n              Install the gems specified by the Gemfile or Gemfile.lock\n\n" +
		"       bundle update(1) bundle-update.1.html\n              Update dependencies to their latest versions\n\n" +
		"       bundle exec(1) bundle-exec.1.html\n              Execute a script in the current bundle\n"
	p := discovery.ParseHelpOutputFor(help, "bundle")
	for _, w := range []string{"install", "update", "exec"} {
		if !slices.Contains(p.Subcommands, w) {
			t.Errorf("missing %q; got %v", w, p.Subcommands)
		}
	}
	if slices.Contains(p.Subcommands, "bundle") || slices.Contains(p.Subcommands, "bundler-") {
		t.Errorf("program name or fragment as subcommand: %v", p.Subcommands)
	}
}

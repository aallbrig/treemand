package discovery_test

import (
	"os"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

// The description is what the tool is, not its banner (issue #155). Each case
// is the head of a corpus fixture.
func TestParseHelp_descriptionSkipsBanners(t *testing.T) {
	cases := []struct{ cli, want, notWant string }{
		{"ffmpeg", "Universal media converter", ""},
		{"rsync", "rsync is a file transfer program", ""},
		{"ansible", "Define and run", ""},
		{"pre-commit", "", "{"},
		{"ip", "", "|"},
	}
	for _, c := range cases {
		t.Run(c.cli, func(t *testing.T) {
			b, err := os.ReadFile("testdata/corpus/" + c.cli + ".txt")
			if err != nil {
				t.Fatal(err)
			}
			p := discovery.ParseHelpOutputFor(discovery.SanitizeHelp(string(b)), c.cli)
			if c.want != "" && !strings.Contains(p.Description, c.want) {
				t.Errorf("description = %q, want it to contain %q", p.Description, c.want)
			}
			if c.notWant != "" && strings.Contains(p.Description, c.notWant) {
				t.Errorf("description = %q, should not contain %q", p.Description, c.notWant)
			}
		})
	}
}

// An alias under ALIASES is not a description, however far down it sits.
func TestParseHelp_aliasIsNotADescription(t *testing.T) {
	help := "Delete a variable on one of the following levels:\n" +
		"- repository (default): available to GitHub Actions runs or Dependabot in a repository\n" +
		"- environment: available to GitHub Actions runs for a deployment environment in a repository\n" +
		"- organization: available to GitHub Actions runs or Dependabot within an organization\n\n\n" +
		"USAGE\n  gh variable delete <variable-name> [flags]\n\nALIASES\n  gh variable remove\n"
	if d := discovery.ParseHelpOutputFor(help, "gh").Description; d == "gh variable remove" {
		t.Errorf("the alias became the description")
	}
}

// clap prints "name version" before the description (issue #82); the version
// line is not the description.
func TestParseHelp_clapVersionLineIsNotADescription(t *testing.T) {
	cases := []struct{ help, want string }{
		{"fakeclap-add 1.0\nAdd a thing\n\nUSAGE:\n    fakeclap add [OPTIONS] <NAME>\n", "Add a thing"},
		{"ripgrep 14.1.0\nAndrew Gallant <jamslam@gmail.com>\n\nripgrep (rg) recursively searches the current directory\n\nUSAGE:\n    rg [OPTIONS] PATTERN\n", "ripgrep (rg) recursively searches the current directory"},
		{"tool v0.3.2-beta\nDo tool things\n", "Do tool things"},
		{"fakeclap-list 1.0\n\nUSAGE:\n    fakeclap list\n", ""},
	}
	for _, c := range cases {
		if d := discovery.ParseHelpOutputFor(c.help, "fakeclap").Description; d != c.want {
			t.Errorf("description = %q, want %q", d, c.want)
		}
	}
}

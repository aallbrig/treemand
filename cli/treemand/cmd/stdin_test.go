package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/cmd"
)

// runCmdStdin runs a fresh root command with stdin, returning stdout and stderr.
func runCmdStdin(stdin string, args ...string) (string, string, error) {
	c := cmd.NewRootCmd()
	var out, errb bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errb)
	c.SetIn(strings.NewReader(stdin))
	c.SetArgs(args)
	err := c.Execute()
	return out.String(), errb.String(), err
}

const stdinToolHelp = "Usage: nosuchtool-92 COMMAND\n\nCommands:\n  alpha  First\n  beta   Second\n\nOptions:\n  -q, --quiet  Say less\n"

func TestStdin_treeFromPipedHelp(t *testing.T) {
	out, errOut, err := runCmdStdin(stdinToolHelp, "--stdin", "--no-color", "nosuchtool-92")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	for _, want := range []string{"nosuchtool-92", "alpha", "beta", "--quiet"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(errOut, "2 subcommands are marked") || !strings.Contains(errOut, "--stdin") {
		t.Errorf("stub note = %q", errOut)
	}
}

func TestStdin_errors(t *testing.T) {
	cases := []struct {
		name, stdin string
		args        []string
		code        int
		want        string
	}{
		{"empty", "\n\t ", []string{"--stdin", "x"}, cmd.ExitFailure, "no help text on stdin"},
		{"interactive", stdinToolHelp, []string{"--stdin", "-i", "x"}, cmd.ExitUsage, "cannot be combined with -i"},
		{"expand", stdinToolHelp, []string{"--stdin", "--expand-stubs=2", "x"}, cmd.ExitUsage, "cannot be combined with --expand-stubs"},
		{"bad output", stdinToolHelp, []string{"--stdin", "--output=dot", "x"}, cmd.ExitUsage, "unknown output format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := runCmdStdin(tc.stdin, tc.args...)
			if err == nil || cmd.ExitCodeFor(err) != tc.code || !strings.Contains(cmd.FormatError(err), tc.want) {
				t.Errorf("err = %v (exit %d), want exit %d containing %q", err, cmd.ExitCodeFor(err), tc.code, tc.want)
			}
			if out != "" {
				t.Errorf("stdout = %q, want empty", out)
			}
		})
	}
}

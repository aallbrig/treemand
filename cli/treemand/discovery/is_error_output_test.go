package discovery

import "testing"

func TestIsErrorOutput(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		// Previously failing: Go-style "<name>: unknown command"
		{"go mod prefix", "go mod: unknown command\nRun 'go help mod' for usage.", true},
		{"go work prefix", "go work: unknown command\nRun 'go help work' for usage.", true},
		// Standard "unknown command" at line start (existing behavior preserved)
		{"bare unknown command", "unknown command foo\nUsage: ...", true},
		// error: prefix
		{"error prefix", "error: no such file", true},
		// [error] tag
		{"error bracket", "[ERROR] something went wrong", true},
		// Normal help should NOT be flagged
		{"normal help", "Go mod provides access to operations on modules.", false},
		{"usage header", "Usage: git [options] <command>", false},
		// A usage error that starts with "Usage:" is still an error (click, flask -h).
		{"click no such option", "Usage: flask [OPTIONS] COMMAND [ARGS]...\nTry 'flask --help' for help.\n\nError: No such option '-h'.", true},
		{"cobra unknown shorthand", "Error: unknown shorthand flag: 'h' in -h\nUsage:\n  tool [flags]", true},
		{"getopt invalid option", "tool: invalid option -- 'h'\nTry 'tool --help' for more information.", true},
		{"getopt unrecognized", "tool: unrecognized option '--help'\nUsage: tool [-x]", true},
		{"argparse unrecognized", "usage: tool [-x]\ntool: error: unrecognized arguments: -h", true},
		// Full help after a one-line warning is help (flask --help with no app).
		{"warning then help", "Error: Could not locate a Flask application. Use the '--app' option.\n\nUsage: flask [OPTIONS] COMMAND [ARGS]...\n\n  A general utility script for Flask applications.\n\nOptions:\n  --version  Show the flask version\n  --help     Show this message and exit.\n\nCommands:\n  routes  Show the routes for the app.\n  run     Run a development server.\n  shell   Run a shell in the app context.\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isErrorOutput(tc.input); got != tc.want {
				t.Errorf("isErrorOutput(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

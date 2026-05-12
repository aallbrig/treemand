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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isErrorOutput(tc.input); got != tc.want {
				t.Errorf("isErrorOutput(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

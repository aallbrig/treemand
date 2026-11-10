package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Exit codes. They are documented in the README; scripts may rely on them.
const (
	// ExitOK: the tree was produced.
	ExitOK = 0
	// ExitFailure: a runtime failure, for example the CLI was not found or
	// discovery could not run.
	ExitFailure = 1
	// ExitUsage: treemand was invoked wrongly (missing or extra argument,
	// unknown flag, bad flag value, unknown output format or shell).
	ExitUsage = 2
)

// usageError marks a mistake in how treemand was invoked. It carries a hint
// telling the user what to do instead.
type usageError struct {
	msg  string
	hint string
}

func (e *usageError) Error() string { return e.msg }

func newUsageError(hint, format string, args ...any) error {
	return &usageError{msg: fmt.Sprintf(format, args...), hint: hint}
}

// ExitCodeFor maps an error returned by a command to the process exit code.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	var ue *usageError
	if errors.As(err, &ue) {
		return ExitUsage
	}
	return ExitFailure
}

// FormatError renders an error for stderr: the message, then any hint.
func FormatError(err error) string {
	var ue *usageError
	if errors.As(err, &ue) && ue.hint != "" {
		return "Error: " + ue.msg + "\n" + ue.hint
	}
	return "Error: " + err.Error()
}

const rootUsageHint = "Usage: treemand [flags] <cli>\n" +
	"  e.g. treemand git        print git's command tree\n" +
	"       treemand -i kubectl explore kubectl interactively\n" +
	"Run 'treemand --help' for all flags."

// rootArgs requires exactly one CLI name and explains what to do otherwise.
func rootArgs(_ *cobra.Command, args []string) error {
	switch len(args) {
	case 1:
		return nil
	case 0:
		return newUsageError(rootUsageHint, "missing CLI name")
	default:
		return newUsageError(
			"treemand takes one CLI name. To look inside a subcommand use -i, or narrow the tree with --filter.",
			"expected one CLI name, got %d (%s)", len(args), strings.Join(args, " "))
	}
}

// flagError turns cobra's flag parsing errors into usage errors.
func flagError(_ *cobra.Command, err error) error {
	return newUsageError("Run 'treemand --help' for all flags.", "%s", err.Error())
}

var shells = []string{"bash", "zsh", "fish", "powershell"}

// completionArgs requires exactly one known shell.
func completionArgs(_ *cobra.Command, args []string) error {
	hint := "Usage: treemand completion <" + strings.Join(shells, "|") + ">\n" +
		"Run 'treemand completion --help' for install steps per shell."
	if len(args) == 0 {
		return newUsageError(hint, "missing shell (one of: %s)", strings.Join(shells, ", "))
	}
	if len(args) > 1 {
		return newUsageError(hint, "expected one shell, got %d", len(args))
	}
	for _, s := range shells {
		if args[0] == s {
			return nil
		}
	}
	return newUsageError(hint, "unknown shell %q (one of: %s)", args[0], strings.Join(shells, ", "))
}

// checkOutputFormat rejects an unknown --output before any discovery runs.
func checkOutputFormat(format string) error {
	switch format {
	case "", "text", "json", "yaml":
		return nil
	}
	return newUsageError("Choose one of: text, json or yaml.", "unknown output format: %s", format)
}

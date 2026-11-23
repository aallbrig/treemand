package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aallbrig/treemand/discovery"
)

// cfgStdin is --stdin: read the root's help text from stdin instead of
// running the CLI.
var cfgStdin bool

const stdinHint = "Pipe the CLI's help in, e.g. docker --help | treemand --stdin docker"

// errNoStdinHelp is returned when --stdin reads nothing but whitespace.
var errNoStdinHelp = errors.New("no help text on stdin\n" + stdinHint)

// checkStdinFlags rejects flags that would make a --stdin run execute the
// CLI: the TUI crawls it in the background and --expand-stubs probes stubs.
func checkStdinFlags() error {
	if cfgInteractive {
		return newUsageError(stdinHint, "--stdin cannot be combined with -i: the TUI runs the CLI to expand commands")
	}
	if cfgExpandStubs > 0 {
		return newUsageError(stdinHint, "--stdin cannot be combined with --expand-stubs: expanding runs the CLI")
	}
	return nil
}

// runFromStdin builds the tree from a help text on stdin. Nothing is
// executed and nothing is cached: subcommands stay undiscovered stubs.
func runFromStdin(cmd *cobra.Command, cliName string) error {
	if err := checkStdinFlags(); err != nil {
		return err
	}
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok && isTerminal(f) {
		return newUsageError(stdinHint, "--stdin needs a help text piped in, but stdin is a terminal")
	}
	// Bounded like a help probe: a flood is cut at the same cap.
	data, err := io.ReadAll(io.LimitReader(in, int64(discovery.MaxHelpBytes)))
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	text := string(data)
	if strings.TrimSpace(text) == "" {
		return errNoStdinHelp
	}
	ov, err := loadOverride(cliName)
	if err != nil {
		return err
	}
	node := discovery.FromHelpText(cliName, text)
	applyOverride(ov, node)
	return output(cmd, node, configFromFlags(), nil)
}

// isTerminal reports whether f is a character device such as a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

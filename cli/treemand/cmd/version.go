package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Build-time variables injected via -ldflags:
//
//	-X github.com/aallbrig/treemand/cmd.Version=v1.2.3
//	-X github.com/aallbrig/treemand/cmd.Commit=abc1234
//	-X github.com/aallbrig/treemand/cmd.BuildDate=2025-01-01
var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)

// versionString returns the canonical version string shown by both
// the `version` subcommand and the `--version` flag.
func versionString() string {
	commit := Commit
	// Fall back to VCS info embedded by `go build` when no ldflags were used.
	if commit == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" && len(s.Value) >= 7 {
					commit = s.Value[:7]
				}
			}
		}
	}

	s := "treemand " + Version
	if commit != "" {
		s += " (" + commit + ")"
	}
	if BuildDate != "" {
		s += " built " + BuildDate
	}
	return s
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Print the version, git commit, and build date of this treemand binary.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), versionString())
	},
}

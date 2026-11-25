package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Build-time variables injected via -ldflags:
//
//	-X github.com/aallbrig/treemand/cli/treemand/cmd.Version=v1.2.3
//	-X github.com/aallbrig/treemand/cli/treemand/cmd.Commit=abc1234
//	-X github.com/aallbrig/treemand/cli/treemand/cmd.BuildDate=2025-01-01
//
// Execute replaces a "dev" Version with the module version when there is one.
var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)

// resolveVersion returns the ldflags version, or, for a binary built by
// `go install github.com/aallbrig/treemand/cli/treemand@vX.Y.Z` (no ldflags),
// the module version the Go toolchain records in the build info.
func resolveVersion(ldflag string, read func() (*debug.BuildInfo, bool)) string {
	if ldflag != "dev" {
		return ldflag
	}
	if info, ok := read(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return ldflag
}

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
	Args:  noArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), versionString())
	},
}

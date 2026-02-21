package cmd

import (
"fmt"

"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var versionCmd = &cobra.Command{
Use:   "version",
Short: "Print the version of treemand",
Run: func(cmd *cobra.Command, args []string) {
fmt.Fprintln(cmd.OutOrStdout(), "treemand", Version)
},
}

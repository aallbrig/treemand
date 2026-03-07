package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var genDocsOutputDir string

// genDocsCmd is a hidden command used during release builds to generate
// man pages and markdown reference docs from the cobra command tree.
var genDocsCmd = &cobra.Command{
	Use:    "gendocs",
	Short:  "Generate man page and markdown docs (used during release builds)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := os.MkdirAll(genDocsOutputDir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}

		manDir := genDocsOutputDir + "/man"
		mdDir := genDocsOutputDir + "/md"
		for _, d := range []string{manDir, mdDir} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", d, err)
			}
		}

		// Man page
		now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		if Version != "dev" {
			now = time.Now()
		}
		header := &doc.GenManHeader{
			Title:   "TREEMAND",
			Section: "1",
			Date:    &now,
			Source:  "treemand " + Version,
			Manual:  "Treemand Manual",
		}
		// GenManTree needs the root command; rebuild it clean without gendocs
		root := buildDocRoot()
		if err := doc.GenManTree(root, header, manDir); err != nil {
			return fmt.Errorf("gen man: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Man pages written to %s\n", manDir)

		// Markdown
		if err := doc.GenMarkdownTree(root, mdDir); err != nil {
			return fmt.Errorf("gen markdown: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Markdown docs written to %s\n", mdDir)

		return nil
	},
}

// buildDocRoot returns a clean cobra command tree for documentation generation.
// It mirrors Execute() but omits gendocs itself to avoid circular reference.
func buildDocRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   rootCmd.Use,
		Short: rootCmd.Short,
		Long:  rootCmd.Long,
		// DisableAutoGenTag avoids embedding a generation timestamp that
		// would make docs differ between builds on the same commit.
		DisableAutoGenTag: true,
	}
	root.Version = versionString()

	// Mirror all persistent flags
	root.PersistentFlags().BoolP("interactive", "i", false, "Launch interactive TUI")
	root.PersistentFlags().StringP("strategy", "s", "help", "Discovery strategies (comma-separated: help,completions)")
	root.PersistentFlags().Int("depth", -1, "Max tree depth (-1 = unlimited)")
	root.PersistentFlags().String("filter", "", "Only show nodes matching pattern")
	root.PersistentFlags().String("exclude", "", "Exclude nodes matching pattern")
	root.PersistentFlags().Bool("commands-only", false, "Hide flags and positionals")
	root.PersistentFlags().Bool("full-path", false, "Show full command paths")
	root.PersistentFlags().String("output", "text", "Output format: text, json")
	root.PersistentFlags().Bool("no-color", false, "Disable color output")
	root.PersistentFlags().Bool("no-cache", false, "Disable caching")
	root.PersistentFlags().Int("timeout", 30, "Discovery timeout in seconds")
	root.PersistentFlags().Bool("debug", false, "Enable debug logging")

	root.AddCommand(&cobra.Command{
		Use:               versionCmd.Use,
		Short:             versionCmd.Short,
		Long:              versionCmd.Long,
		DisableAutoGenTag: true,
	})
	root.AddCommand(&cobra.Command{
		Use:               cacheCmd.Use,
		Short:             cacheCmd.Short,
		Long:              cacheCmd.Long,
		DisableAutoGenTag: true,
	})

	return root
}

func init() {
	genDocsCmd.Flags().StringVarP(&genDocsOutputDir, "output-dir", "o", "dist/docs", "Directory to write generated docs")
}

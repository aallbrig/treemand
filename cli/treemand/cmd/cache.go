package cmd

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/aallbrig/treemand/cache"
	"github.com/aallbrig/treemand/config"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the treemand discovery cache",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear [cli]",
	Short: "Clear cached discovery results",
	Long: `Clear removes discovered CLI trees from the local cache.

Without arguments, clears the entire cache.
With a CLI name, clears only entries for that CLI.

Examples:
  treemand cache clear          # clear all cached entries
  treemand cache clear git      # clear only git's cached entries`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.DefaultConfig()
		c, err := cache.Open(cfg.CacheDir)
		if err != nil {
			return fmt.Errorf("open cache: %w", err)
		}
		defer c.Close()

		if len(args) == 0 {
			if err := c.Clear(); err != nil {
				return fmt.Errorf("clear cache: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Cache cleared.")
			return nil
		}

		cli := args[0]
		if err := c.ClearCLI(cli); err != nil {
			return fmt.Errorf("clear cache for %q: %w", cli, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Cache cleared for %q.\n", cli)
		return nil
	},
}

var cacheListCmd = &cobra.Command{
	Use:   "list",
	Short: "List CLIs with cached discovery results",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.DefaultConfig()
		c, err := cache.Open(cfg.CacheDir)
		if err != nil {
			return fmt.Errorf("open cache: %w", err)
		}
		defer c.Close()

		entries, err := c.ListEntries()
		if err != nil {
			return fmt.Errorf("list cache: %w", err)
		}
		if len(entries) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "(cache is empty)")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CLI\tVERSION\tSTRATEGY\tCACHED AT\tSIZE")
		fmt.Fprintln(w, "---\t-------\t--------\t---------\t----")
		for _, e := range entries {
			age := formatAge(time.Since(e.CachedAt))
			size := formatBytes(e.SizeBytes)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				e.CLI, e.Version, e.Strategy, age, size)
		}
		return w.Flush()
	},
}

func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func formatBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
	}
}

func init() {
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheListCmd)
}

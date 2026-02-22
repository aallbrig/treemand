package cmd

import (
	"fmt"

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

		names, err := c.ListCLIs()
		if err != nil {
			return fmt.Errorf("list cache: %w", err)
		}
		if len(names) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "(cache is empty)")
			return nil
		}
		for _, name := range names {
			fmt.Fprintln(cmd.OutOrStdout(), name)
		}
		return nil
	},
}

func init() {
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheListCmd)
}

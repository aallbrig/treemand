package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/history"
)

var historyCmd = &cobra.Command{
	Use:   "history <cli>",
	Short: "List commands you copied or ran from the TUI for a CLI",
	Long: `History lists the commands you copied or ran from the interactive TUI
(Ctrl+E) for one CLI, most recent first, with how often each was used.

History is kept on this machine only, under the cache directory, and holds
at most ` + fmt.Sprint(history.MaxEntries) + ` commands per CLI. Turn it off with no_history: true in the
config file or TREEMAND_NO_HISTORY=1.

Examples:
  treemand history git
  treemand history kubectl --output json`,
	Args: oneCLIArg,
	RunE: runHistory,
}

// historyStore is the history for cliName, or nil when history is off.
func historyStore(cfg *config.Config, cliName string) *history.Store {
	if cfg.NoHistory {
		return nil
	}
	return history.Open(history.Dir(cfg.CacheDir), filepath.Base(cliName))
}

func runHistory(cmd *cobra.Command, args []string) error {
	if err := checkOutputFormat(cfgOutput); err != nil {
		return err
	}
	cfg := configFromFlags()
	store := historyStore(cfg, args[0])
	if store == nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "treemand: history is off (no_history is set)")
		return nil
	}
	entries, err := store.Recent()
	if err != nil {
		return err
	}
	if len(entries) == 0 && (cfgOutput == "" || cfgOutput == "text") {
		fmt.Fprintf(cmd.ErrOrStderr(), "No history for %s yet. Copy or run a command from 'treemand -i %s' (Ctrl+E).\n", args[0], args[0])
		return nil
	}
	return writeHistory(cmd.OutOrStdout(), entries, cfgOutput)
}

func writeHistory(w io.Writer, entries []history.Entry, format string) error {
	if entries == nil {
		entries = []history.Entry{}
	}
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	case "yaml":
		return yaml.NewEncoder(w).Encode(entries)
	}
	for _, e := range entries {
		if _, err := fmt.Fprintln(w, e.Command); err != nil {
			return err
		}
	}
	return nil
}

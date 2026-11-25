package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	"github.com/aallbrig/treemand/cli/treemand/config"
	"github.com/aallbrig/treemand/cli/treemand/savedpresets"
)

// cfgPreset is --preset: open the TUI with this saved preset loaded.
var cfgPreset string

// startPreset is the saved preset --preset resolved to, for this run.
var startPreset *savedpresets.Preset

var presetsCmd = &cobra.Command{
	Use:   "presets <cli>",
	Short: "List the commands you saved under a name in the TUI for a CLI",
	Long: `Presets lists the commands you saved under a name in the interactive TUI
for one CLI. In the TUI, Ctrl+O opens them: a saves the assembled command
under a name, Enter loads one, x deletes one. Open the TUI with a preset
already loaded with treemand --preset <name> <cli>.

Presets are a plain-text file per CLI in the config directory
(<config dir>/treemand/presets/<cli>.txt), one "name  command" per line.
Lines starting with # are comments; the file is safe to edit by hand.

Examples:
  treemand presets git
  treemand presets kubectl --output json
  treemand --preset amend git`,
	Args: oneCLIArg,
	RunE: runPresets,
}

// savedPresetsStore is the saved presets of cliName.
func savedPresetsStore(cliName string) (*savedpresets.Store, error) {
	dir, err := config.PresetsDir()
	if err != nil {
		return nil, err
	}
	return savedpresets.Open(dir, filepath.Base(cliName)), nil
}

// tuiPresets is the store the TUI saves to, or nil without a config
// directory (the TUI then says presets are unavailable).
func tuiPresets(cliName string) *savedpresets.Store {
	s, err := savedPresetsStore(cliName)
	if err != nil {
		return nil
	}
	return s
}

// resolvePreset looks up --preset for cliName before anything is probed:
// an unknown name is a usage error that lists the saved names. --preset
// opens the TUI.
func resolvePreset(cliName string) error {
	startPreset = nil
	if cfgPreset == "" {
		return nil
	}
	store, err := savedPresetsStore(cliName)
	if err != nil {
		return err
	}
	p, ok, err := store.Get(cfgPreset)
	if err != nil {
		return err
	}
	if !ok {
		list, _ := store.List()
		return newUsageError(presetHint(cliName, list), "no saved preset %q for %s", cfgPreset, filepath.Base(cliName))
	}
	startPreset = &p
	cfgInteractive = true
	return nil
}

func presetHint(cliName string, list []savedpresets.Preset) string {
	if len(list) == 0 {
		return fmt.Sprintf("Save one in 'treemand -i %s' with Ctrl+O, then a.", cliName)
	}
	names := make([]string, len(list))
	for i, p := range list {
		names[i] = p.Name
	}
	return "Saved presets: " + strings.Join(names, ", ")
}

func runPresets(cmd *cobra.Command, args []string) error {
	if err := checkOutputFormat(cfgOutput); err != nil {
		return err
	}
	store, err := savedPresetsStore(args[0])
	if err != nil {
		return err
	}
	list, err := store.List()
	if err != nil {
		return err
	}
	if len(list) == 0 && (cfgOutput == "" || cfgOutput == "text") {
		fmt.Fprintf(cmd.ErrOrStderr(), "No presets for %s yet. Save one in 'treemand -i %s' with Ctrl+O, then a.\n", args[0], args[0])
		return nil
	}
	return writePresets(cmd.OutOrStdout(), list, cfgOutput)
}

func writePresets(w io.Writer, list []savedpresets.Preset, format string) error {
	if list == nil {
		list = []savedpresets.Preset{}
	}
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(list)
	case "yaml":
		return yaml.NewEncoder(w).Encode(list)
	}
	width := 0
	for _, p := range list {
		width = max(width, len(p.Name))
	}
	for _, p := range list {
		if _, err := fmt.Fprintf(w, "%-*s  %s\n", width, p.Name, p.Command); err != nil {
			return err
		}
	}
	return nil
}

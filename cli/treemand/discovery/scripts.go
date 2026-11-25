package discovery

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// MaxScriptBytes caps how much of a completion script is read. Generated
// scripts for big CLIs reach about 700 KB; a larger file is skipped.
var MaxScriptBytes int64 = 4 << 20

// ScriptDirs lists where each shell keeps installed completion scripts.
type ScriptDirs struct {
	Fish, Zsh, Bash []string
}

// ScriptsDiscoverer reads the CLI's installed shell completion script (fish,
// then zsh, then bash) and parses the subcommands and flags it lists. The
// script is only read, never sourced or run, and the CLI is never executed,
// so it is safe for any CLI. Scripts that ask the CLI at completion time
// (cobra, argcomplete) hold no static data and yield nothing.
type ScriptsDiscoverer struct {
	// Dirs returns the directories to search; nil means DefaultScriptDirs.
	Dirs func() ScriptDirs
}

// NewScriptsDiscoverer creates a ScriptsDiscoverer over the standard
// directories.
func NewScriptsDiscoverer() *ScriptsDiscoverer { return &ScriptsDiscoverer{} }

func (s *ScriptsDiscoverer) Name() string { return "scripts" }

// Discover returns the tree the CLI's completion script describes, or nil
// when it has none. A script describes the whole CLI, so only the root is
// answered; args is for subcommand discovery and yields nil.
func (s *ScriptsDiscoverer) Discover(_ context.Context, cliName string, args []string) (*models.Node, error) {
	name := filepath.Base(cliName)
	if len(args) > 0 || !scriptCommandNameRe.MatchString(name) {
		return nil, nil //nolint:nilnil // nothing to read is a normal case
	}
	dirs := DefaultScriptDirs()
	if s.Dirs != nil {
		dirs = s.Dirs()
	}
	for _, src := range []struct {
		dirs  []string
		files []string
		parse func(string, string) *scriptTree
	}{
		{dirs.Fish, []string{name + ".fish"}, parseFishScript},
		{dirs.Zsh, []string{"_" + name}, parseZshScript},
		{dirs.Bash, []string{name, name + ".bash", "_" + name}, parseBashScript},
	} {
		if text, ok := readFirstScript(src.dirs, src.files); ok {
			if node := src.parse(text, name).node(); node != nil {
				return node, nil
			}
		}
	}
	return nil, nil //nolint:nilnil // no completion script installed is a normal case
}

// readFirstScript reads the first of files found in dirs.
func readFirstScript(dirs, files []string) (string, bool) {
	for _, dir := range dirs {
		for _, f := range files {
			if text, ok := readScript(filepath.Join(dir, f)); ok {
				return text, true
			}
		}
	}
	return "", false
}

// readScript reads a regular file no larger than MaxScriptBytes.
func readScript(path string) (string, bool) {
	fh, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer fh.Close()
	info, err := fh.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxScriptBytes {
		return "", false
	}
	data, err := io.ReadAll(io.LimitReader(fh, MaxScriptBytes))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// DefaultScriptDirs returns the standard completion directories: under the
// user's data directory, $XDG_DATA_DIRS and the usual install prefixes
// (Homebrew on macOS and Linux included), plus fish's per-user directory
// and the bash-completion.d directories.
func DefaultScriptDirs() ScriptDirs {
	var d ScriptDirs
	home, _ := os.UserHomeDir()
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" && home != "" {
		configHome = filepath.Join(home, ".config")
	}
	if configHome != "" {
		d.Fish = append(d.Fish, filepath.Join(configHome, "fish", "completions"))
	}
	for _, base := range dataDirs(home) {
		d.Fish = append(d.Fish, filepath.Join(base, "fish", "vendor_completions.d"), filepath.Join(base, "fish", "completions"))
		d.Zsh = append(d.Zsh, filepath.Join(base, "zsh", "site-functions"), filepath.Join(base, "zsh", "vendor-completions"))
		d.Bash = append(d.Bash, filepath.Join(base, "bash-completion", "completions"))
	}
	for _, etc := range []string{"/etc", "/usr/local/etc", "/opt/homebrew/etc", "/home/linuxbrew/.linuxbrew/etc"} {
		d.Bash = append(d.Bash, filepath.Join(etc, "bash_completion.d"))
	}
	return d
}

// dataDirs returns the share directories to search, user's first, without
// duplicates.
func dataDirs(home string) []string {
	var bases []string
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		bases = append(bases, dataHome)
	} else if home != "" {
		bases = append(bases, filepath.Join(home, ".local", "share"))
	}
	if env := os.Getenv("XDG_DATA_DIRS"); env != "" {
		bases = append(bases, filepath.SplitList(env)...)
	}
	bases = append(bases, "/opt/homebrew/share", "/home/linuxbrew/.linuxbrew/share", "/usr/local/share", "/usr/share")
	seen := map[string]bool{}
	var out []string
	for _, b := range bases {
		b = strings.TrimSpace(b)
		if b != "" && filepath.IsAbs(b) && !seen[b] {
			seen[b] = true
			out = append(out, b)
		}
	}
	return out
}

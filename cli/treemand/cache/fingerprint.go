package cache

import (
	"fmt"
	"os"
	"path/filepath"
)

// BinaryFingerprint identifies the executable at path by what is on disk: its
// resolved location (symlinks followed), size and modification time. It costs
// one stat call, where asking the CLI for its version starts the CLI (seconds
// for aws and pip), and it changes whenever a package manager upgrades the
// tool, either by replacing the file or by repointing a "current" symlink.
// It returns "" when the file cannot be inspected.
func BinaryFingerprint(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}
	info, err := os.Stat(resolved)
	if err != nil || info.IsDir() {
		return ""
	}
	return fmt.Sprintf("bin:%s:%d:%d", resolved, info.Size(), info.ModTime().UnixNano())
}

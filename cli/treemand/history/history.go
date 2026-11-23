// Package history remembers the commands a user copied or ran from the TUI,
// one small JSON file per CLI. It is local only: nothing here leaves the
// machine.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaxEntries caps how many distinct commands a CLI's history keeps.
const MaxEntries = 100

// Entry is one remembered command.
type Entry struct {
	Command string    `json:"command"`
	Count   int       `json:"count"`
	Last    time.Time `json:"last"`
}

// Store is the history of one CLI.
type Store struct {
	path string
}

// Dir is where histories live under a treemand cache directory.
func Dir(cacheDir string) string {
	return filepath.Join(cacheDir, "history")
}

// Open returns the store for cli inside dir. The file is created on the
// first Add.
func Open(dir, cli string) *Store {
	return &Store{path: filepath.Join(dir, fileName(cli))}
}

// Path is the store's file.
func (s *Store) Path() string { return s.path }

// fileName turns a CLI name into a safe file name: path separators and
// anything unusual become underscores, so a name cannot leave dir.
func fileName(cli string) string {
	var b strings.Builder
	for _, r := range filepath.Base(cli) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		b.WriteString("_")
	}
	return b.String() + ".json"
}

// Recent returns the remembered commands, most recently used first. A
// missing file is an empty history.
func (s *Store) Recent() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("read history %s: %w", s.path, err)
	}
	return entries, nil
}

// Add records cmd as used at now: it moves to the front, its count grows,
// and the oldest entries beyond MaxEntries are dropped. Whitespace is
// normalised and an empty command is ignored. An unreadable file is
// replaced.
func (s *Store) Add(cmd string, now time.Time) error {
	cmd = strings.Join(strings.Fields(cmd), " ")
	if cmd == "" {
		return nil
	}
	old, _ := s.Recent()
	entries := []Entry{{Command: cmd, Count: 1, Last: now}}
	for _, e := range old {
		if e.Command == cmd {
			entries[0].Count += e.Count
			continue
		}
		entries = append(entries, e)
	}
	if len(entries) > MaxEntries {
		entries = entries[:MaxEntries]
	}
	return s.write(entries)
}

func (s *Store) write(entries []Entry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".history-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(append(data, '\n'))
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), s.path)
	}
	if werr != nil {
		_ = os.Remove(tmp.Name())
	}
	return werr
}

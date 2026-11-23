// Package savedpresets keeps the assembled commands a user saved under a
// name for one target CLI, so the TUI can load them again later.
//
// Each CLI has one plain-text file, <config dir>/treemand/presets/<cli>.txt,
// meant to be read and edited by hand:
//
//	# name    command
//	amend     git commit --amend --no-edit
//	graph     git log --oneline --graph --all
//
// The name is the first word of a line and the command is the rest. Blank
// lines and lines starting with # are kept as they are. A line that is not a
// valid preset is ignored when reading and kept when writing.
//
// These are not the built-in per-CLI tuning in package presets.
package savedpresets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// MaxNameLen caps how long a preset name may be.
const MaxNameLen = 64

// Preset is one saved command.
type Preset struct {
	Name    string `json:"name" yaml:"name"`
	Command string `json:"command" yaml:"command"`
}

// Store is the saved presets of one CLI.
type Store struct {
	path string
}

// Open returns the store for cli inside dir. The file is created on the
// first Save.
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
		if isNameRune(r) && r != '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 || cli == "" {
		return "_.txt"
	}
	return b.String() + ".txt"
}

func isNameRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
		r == '-' || r == '_' || r == '.'
}

// ValidName reports why name cannot name a preset, or nil when it can:
// letters, digits, '.', '_' and '-', at most MaxNameLen of them.
func ValidName(name string) error {
	if name == "" {
		return errors.New("a preset needs a name")
	}
	if len(name) > MaxNameLen {
		return fmt.Errorf("preset name is longer than %d characters", MaxNameLen)
	}
	for _, r := range name {
		if !isNameRune(r) {
			return fmt.Errorf("preset name %q may only use letters, digits, '.', '_' and '-'", name)
		}
	}
	return nil
}

// parseLine returns the preset a line holds, if it holds one.
func parseLine(line string) (Preset, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || ValidName(fields[0]) != nil {
		return Preset{}, false
	}
	return Preset{Name: fields[0], Command: strings.Join(fields[1:], " ")}, true
}

// lines reads the file's lines; a missing file has none.
func (s *Store) lines() ([]string, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	text := strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// List returns the saved presets in file order. When a name appears twice,
// the later command wins.
func (s *Store) List() ([]Preset, error) {
	lines, err := s.lines()
	if err != nil {
		return nil, err
	}
	var out []Preset
	at := map[string]int{}
	for _, line := range lines {
		p, ok := parseLine(line)
		if !ok {
			continue
		}
		if i, seen := at[p.Name]; seen {
			out[i] = p
			continue
		}
		at[p.Name] = len(out)
		out = append(out, p)
	}
	return out, nil
}

// Get returns the preset called name.
func (s *Store) Get(name string) (Preset, bool, error) {
	list, err := s.List()
	for _, p := range list {
		if p.Name == name {
			return p, true, err
		}
	}
	return Preset{}, false, err
}

// Save stores command under name, replacing a preset of the same name in
// place. Whitespace in the command, newlines included, becomes single spaces.
func (s *Store) Save(name, command string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	command = strings.Join(strings.Fields(command), " ")
	if command == "" {
		return errors.New("there is no command to save")
	}
	lines, err := s.lines()
	if err != nil {
		return err
	}
	line := name + "  " + command
	kept, found := withoutPreset(lines, name, line)
	if !found {
		kept = append(kept, line)
	}
	return s.write(kept)
}

// Delete removes the preset called name and reports whether there was one.
func (s *Store) Delete(name string) (bool, error) {
	lines, err := s.lines()
	if err != nil {
		return false, err
	}
	kept, found := withoutPreset(lines, name, "")
	if !found {
		return false, nil
	}
	return true, s.write(kept)
}

// withoutPreset drops every line of the preset called name; the first is
// replaced by replacement unless it is empty.
func withoutPreset(lines []string, name, replacement string) ([]string, bool) {
	var kept []string
	found := false
	for _, l := range lines {
		if p, ok := parseLine(l); ok && p.Name == name {
			if !found && replacement != "" {
				kept = append(kept, replacement)
			}
			found = true
			continue
		}
		kept = append(kept, l)
	}
	return kept, found
}

func (s *Store) write(lines []string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".presets-*")
	if err != nil {
		return err
	}
	var data string
	if len(lines) > 0 {
		data = strings.Join(lines, "\n") + "\n"
	}
	_, werr := tmp.WriteString(data)
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

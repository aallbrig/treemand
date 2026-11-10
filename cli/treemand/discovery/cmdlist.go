package discovery

import (
	"context"
	"regexp"
	"strings"

	"github.com/aallbrig/treemand/models"
)

// Some CLIs (tmux) have no per-command help and expose their commands through
// a "list-commands" command instead. When --help yields no subcommands at the
// root, treemand asks for that list, parses the synopsis lines into flags and
// takes descriptions from the man page's command section.

// CommandListEntry is one line of a "name (alias) synopsis" command list.
type CommandListEntry struct {
	Name     string
	Alias    string
	Synopsis string
}

var commandListLineRe = regexp.MustCompile(`^([a-z][a-z0-9-]*)(?: \(([a-z][a-z0-9-]*)\))?(?: (.*))?$`)

// ParseCommandList parses `<cli> list-commands` output. It returns nil unless
// the output is clearly a command list: at least four entries, and at least
// 80% of the non-empty lines are entries (so prose and errors never match).
func ParseCommandList(out string) []CommandListEntry {
	var entries []CommandListEntry
	nonEmpty := 0
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			continue
		}
		nonEmpty++
		if line[0] == ' ' || line[0] == '\t' {
			continue
		}
		if m := commandListLineRe.FindStringSubmatch(line); m != nil {
			entries = append(entries, CommandListEntry{Name: m[1], Alias: m[2], Synopsis: strings.TrimSpace(m[3])})
		}
	}
	if len(entries) < 4 || len(entries)*5 < nonEmpty*4 {
		return nil
	}
	return entries
}

var synopsisFlagRe = regexp.MustCompile(`\[-([A-Za-z]+)(?:\s+([A-Za-z][A-Za-z-]*))?\]`)

// SynopsisFlags extracts flags from a synopsis such as
// "[-dE] [-c working-directory] key": a bracketed letter cluster is a set of
// boolean flags, a single letter followed by a word is a flag taking a value.
func SynopsisFlags(synopsis string) []models.Flag {
	var flags []models.Flag
	seen := map[string]bool{}
	add := func(f models.Flag) {
		if !seen[f.Name] {
			seen[f.Name] = true
			flags = append(flags, f)
		}
	}
	for _, m := range synopsisFlagRe.FindAllStringSubmatch(synopsis, -1) {
		letters, value := m[1], m[2]
		if value != "" && len(letters) == 1 {
			add(models.Flag{Name: "-" + letters, ValueType: value})
			continue
		}
		for _, r := range letters {
			add(models.Flag{Name: "-" + string(r), ValueType: "bool"})
		}
	}
	return flags
}

// ManCommandDescriptions finds, for each name, the first sentence of the
// paragraph that follows its heading in a man page's command section:
//
//	attach-session [-dErx] [-t
//	        target-session]
//	      (alias: attach)
//	        If run from outside tmux, create a new client...
func ManCommandDescriptions(plain string, names []string) map[string]string {
	lines := strings.Split(plain, "\n")
	start := 0
	for i, l := range lines {
		if strings.TrimSpace(l) == "COMMANDS" && !strings.HasPrefix(l, " ") {
			start = i + 1
			break
		}
	}
	out := map[string]string{}
	for _, name := range names {
		for i := start; i < len(lines); i++ {
			if !isCommandHeading(lines[i], name) {
				continue
			}
			if d := paragraphAfterHeading(lines, i); d != "" {
				out[name] = d
				break
			}
		}
	}
	return out
}

func indentOf(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

func isCommandHeading(line, name string) bool {
	if indentOf(line) == 0 || indentOf(line) > 8 {
		return false
	}
	t := strings.TrimSpace(line)
	return t == name || strings.HasPrefix(t, name+" ")
}

// paragraphAfterHeading skips the (possibly wrapped) synopsis and the alias
// line after the heading at index i, then returns the first sentence of the
// prose paragraph.
func paragraphAfterHeading(lines []string, i int) string {
	depth := strings.Count(lines[i], "[") - strings.Count(lines[i], "]")
	j := i + 1
	for ; j < len(lines) && depth > 0; j++ {
		depth += strings.Count(lines[j], "[") - strings.Count(lines[j], "]")
	}
	if j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "(alias:") {
		j++
	}
	var para []string
	for ; j < len(lines) && strings.TrimSpace(lines[j]) != ""; j++ {
		para = append(para, strings.TrimSpace(lines[j]))
	}
	text := strings.Join(strings.Fields(strings.Join(para, " ")), " ")
	if k := strings.Index(text, ". "); k >= 0 {
		return text[:k+1]
	}
	return text
}

// commandListSkip lists target runners for which "list-commands" would be
// treated as a target to build, and so could execute project code.
var commandListSkip = map[string]bool{
	"make": true, "gmake": true, "bmake": true, "ninja": true, "just": true, "task": true, "rake": true,
}

// commandListChildren builds child nodes from `<cli> list-commands`, or
// returns nil when the CLI has no such command.
func (h *HelpDiscoverer) commandListChildren(ctx context.Context, cliName string, fullPath []string) []*models.Node {
	if commandListSkip[cliName] {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()
	out, _, err := runCapped(cctx, resolveBinary(cliName), []string{"list-commands"}, nil, MaxHelpBytes)
	if err != nil {
		return nil
	}
	entries := ParseCommandList(string(out))
	if entries == nil {
		return nil
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	descs := map[string]string{}
	if plain, merr := NewManDiscoverer().fetchManPage(ctx, cliName); merr == nil && plain != "" {
		descs = ManCommandDescriptions(plain, names)
	}

	children := make([]*models.Node, 0, len(entries))
	for _, e := range entries {
		desc := descs[e.Name]
		if e.Alias != "" {
			if desc != "" {
				desc += " "
			}
			desc += "(alias: " + e.Alias + ")"
		}
		children = append(children, &models.Node{
			Name:        e.Name,
			FullPath:    append(append([]string{}, fullPath...), e.Name),
			Description: desc,
			Flags:       SynopsisFlags(e.Synopsis),
			Discovered:  true,
		})
	}
	return children
}

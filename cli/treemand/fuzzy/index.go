package fuzzy

import (
	"sort"
	"strings"

	"github.com/aallbrig/treemand/models"
)

// Kind is the type of graph element an Entry refers to.
type Kind int

// Entry kinds.
const (
	KindCommand Kind = iota
	KindFlag
	KindPositional
)

// Entry is one searchable element of the CLI graph.
type Entry struct {
	Kind  Kind
	Node  *models.Node // owning node (the command itself for KindCommand)
	Name  string       // command name, "--flag", or positional name
	Desc  string
	Extra string // lower-weight text (help text excerpt)
}

// Path returns the full command path of the owning node.
func (e Entry) Path() string { return e.Node.FullCommand() }

// Result is a scored match.
type Result struct {
	Entry     Entry
	Score     int
	NameIdx   []int  // matched rune indices in the display label
	MatchedIn string // "name", "desc" or "help"
}

const helpExcerptLen = 2000

// Build flattens the node graph into searchable entries. Inherited flags are
// indexed only on the node that declares them first (the ancestor).
func Build(root *models.Node) []Entry {
	var out []Entry
	root.Walk(func(n *models.Node) {
		if n.Virtual {
			return
		}
		help := n.HelpText
		if len(help) > helpExcerptLen {
			help = help[:helpExcerptLen]
		}
		out = append(out, Entry{Kind: KindCommand, Node: n, Name: n.Name, Desc: n.Description, Extra: help})
		for _, f := range n.Flags {
			if f.Inherited {
				continue
			}
			name := "--" + strings.TrimLeft(f.Name, "-")
			out = append(out, Entry{Kind: KindFlag, Node: n, Name: name, Desc: f.Description})
		}
		for _, p := range n.Positionals {
			out = append(out, Entry{Kind: KindPositional, Node: n, Name: p.Name, Desc: p.Description})
		}
	})
	return out
}

// Label is the text shown (and highlighted) for an entry.
func (e Entry) Label() string {
	if e.Kind == KindCommand {
		return e.Path()
	}
	return e.Path() + " " + e.Name
}

// Search ranks entries against query. Name matches outweigh description
// matches, which outweigh help-text matches. Multiple space-separated terms
// must all match (in any field of the same entry).
func Search(entries []Entry, query string, limit int) []Result {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return nil
	}
	var res []Result
	for _, e := range entries {
		total := 0
		var idx []int
		where := "name"
		all := true
		label := e.Label()
		for _, t := range terms {
			s, ix, ok := Match(t, label)
			field := "name"
			s *= 3
			if !ok {
				s, _, ok = Match(t, e.Desc)
				field = "desc"
				if ok && containsFold(e.Desc, t) {
					s += 4
				}
			}
			if !ok && e.Extra != "" && containsFold(e.Extra, t) {
				s, ok, field = 1, true, "help"
			}
			if !ok {
				all = false
				break
			}
			if field == "name" {
				idx = append(idx, ix...)
			} else if where == "name" {
				where = field
			}
			total += s
		}
		if !all {
			continue
		}
		if e.Kind == KindCommand {
			total += 2
		}
		res = append(res, Result{Entry: e, Score: total, NameIdx: idx, MatchedIn: where})
	}
	sort.SliceStable(res, func(i, j int) bool { return res[i].Score > res[j].Score })
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aallbrig/treemand/cli/treemand/fuzzy"
	"github.com/aallbrig/treemand/cli/treemand/models"
)

// Depth limits for the discover tool. Search uses searchDepth.
const (
	defaultDepth = 2
	maxDepth     = 5
	searchDepth  = 3
	defaultLimit = 20
	maxLimit     = 100
)

type toolFunc func(s *Server, ctx context.Context, args json.RawMessage) *toolResult

var tools = map[string]toolFunc{
	"discover": (*Server).discoverTool,
	"search":   (*Server).searchTool,
	"assemble": (*Server).assembleTool,
}

var cliProp = map[string]any{
	"type":        "string",
	"description": "Name of a command on PATH, such as git or kubectl (a bare name, not a path).",
}

// toolList is what tools/list returns, in a fixed order.
var toolList = []map[string]any{
	{
		"name": "discover",
		"description": "Return the command tree of a CLI: subcommands, flags (name, short name, value type, " +
			"description) and positional arguments, as JSON following the schema `treemand schema` prints. " +
			"Nodes with \"stub\": true were not discovered yet; ask for them with assemble or a deeper depth.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cli":          cliProp,
				"depth":        map[string]any{"type": "integer", "minimum": 1, "maximum": maxDepth, "description": fmt.Sprintf("Levels of subcommands to discover (default %d).", defaultDepth)},
				"include_help": map[string]any{"type": "boolean", "description": "Keep each command's raw --help text (large; default false)."},
				"refresh":      map[string]any{"type": "boolean", "description": "Discover the CLI again even if its tree is cached, and update the cache (default false), like treemand --refresh."},
			},
			"required": []string{"cli"},
		},
	},
	{
		"name": "search",
		"description": "Fuzzy-search a CLI's commands, flags and positional arguments by keyword, best match first. " +
			"Use it to answer \"which command does X\" without reading the whole tree.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cli":   cliProp,
				"query": map[string]any{"type": "string", "description": "Words to match against names and descriptions, such as \"remote add\" or \"dry run\"."},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": maxLimit, "description": fmt.Sprintf("Most results to return (default %d).", defaultLimit)},
			},
			"required": []string{"cli", "query"},
		},
	},
	{
		"name": "assemble",
		"description": "Build a shell-quoted command line from a subcommand path, flags and arguments, checking the path " +
			"and every flag against the CLI's discovered help (flags the help does not list are kept, with a warning). " +
			"The command is returned, never run.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cli":  cliProp,
				"path": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Subcommand names below the CLI, such as [\"remote\", \"add\"]."},
				"flags": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":  map[string]any{"type": "string", "description": "Flag name, long or short, with or without dashes."},
							"value": map[string]any{"type": "string", "description": "Value for a flag that takes one; omit for a switch."},
						},
						"required": []string{"name"},
					},
				},
				"args": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Positional arguments, appended after the flags."},
			},
			"required": []string{"cli"},
		},
	},
}

// cliNameRe is a bare command name: no path separators, spaces or a leading
// dash or dot, so a tool call can only name a program found on PATH.
var cliNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

func checkCLI(cli string) error {
	if !cliNameRe.MatchString(cli) {
		return fmt.Errorf("cli must be a bare command name on PATH (letters, digits, '.', '_', '+', '-'), got %q", cli)
	}
	return nil
}

// tree validates the request's CLI and discovers its tree.
func (s *Server) tree(ctx context.Context, req TreeRequest) (*models.Node, *toolResult) {
	cli := req.CLI
	if err := checkCLI(cli); err != nil {
		return nil, toolError("%v", err)
	}
	root, err := s.Discover(ctx, req)
	if err != nil {
		return nil, toolError("discovering %s failed: %v", cli, err)
	}
	if root == nil {
		return nil, toolError("discovering %s returned nothing", cli)
	}
	return root, nil
}

func decode(args json.RawMessage, v any) *toolResult {
	if err := json.Unmarshal(args, v); err != nil {
		return toolError("invalid arguments: %v", err)
	}
	return nil
}

func (s *Server) discoverTool(ctx context.Context, args json.RawMessage) *toolResult {
	var a struct {
		CLI         string `json:"cli"`
		Depth       int    `json:"depth"`
		IncludeHelp bool   `json:"include_help"`
		Refresh     bool   `json:"refresh"`
	}
	if bad := decode(args, &a); bad != nil {
		return bad
	}
	depth := a.Depth
	if depth <= 0 {
		depth = defaultDepth
	}
	depth = min(depth, maxDepth)
	root, bad := s.tree(ctx, TreeRequest{CLI: a.CLI, Depth: depth, Refresh: a.Refresh})
	if bad != nil {
		return bad
	}
	out := root.Clone()
	if !a.IncludeHelp {
		out.Walk(func(n *models.Node) { n.HelpText = "" })
	}
	out.SchemaVersion = models.SchemaVersion
	return jsonResult(out, true)
}

// Hit is one search result.
type Hit struct {
	Kind        string `json:"kind"`    // command, flag or positional
	Command     string `json:"command"` // full command path, such as "git remote add"
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

var kindNames = map[fuzzy.Kind]string{
	fuzzy.KindCommand: "command", fuzzy.KindFlag: "flag", fuzzy.KindPositional: "positional",
}

func (s *Server) searchTool(ctx context.Context, args json.RawMessage) *toolResult {
	var a struct {
		CLI   string `json:"cli"`
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if bad := decode(args, &a); bad != nil {
		return bad
	}
	if strings.TrimSpace(a.Query) == "" {
		return toolError("query is required")
	}
	limit := a.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	root, bad := s.tree(ctx, TreeRequest{CLI: a.CLI, Depth: searchDepth})
	if bad != nil {
		return bad
	}
	hits := []Hit{}
	for _, r := range fuzzy.Search(fuzzy.Build(root), a.Query, limit) {
		h := Hit{Kind: kindNames[r.Entry.Kind], Command: r.Entry.Path(), Description: r.Entry.Desc}
		if r.Entry.Kind != fuzzy.KindCommand {
			h.Name = r.Entry.Name
		}
		hits = append(hits, h)
	}
	return jsonResult(hits, false)
}

// Assembled is the result of assemble.
type Assembled struct {
	Command string   `json:"command"` // shell-quoted, ready to paste
	Argv    []string `json:"argv"`    // the same command, one element per argument
	// Warnings name flags that the CLI's help does not list.
	Warnings []string `json:"warnings,omitempty"`
}

type flagArg struct {
	Name  string  `json:"name"`
	Value *string `json:"value"`
}

func (s *Server) assembleTool(ctx context.Context, args json.RawMessage) *toolResult {
	var a struct {
		CLI   string    `json:"cli"`
		Path  []string  `json:"path"`
		Flags []flagArg `json:"flags"`
		Args  []string  `json:"args"`
	}
	if bad := decode(args, &a); bad != nil {
		return bad
	}
	root, bad := s.tree(ctx, TreeRequest{CLI: a.CLI, Depth: max(len(a.Path), 1)})
	if bad != nil {
		return bad
	}
	chain, bad := s.walkPath(ctx, a.CLI, root, a.Path)
	if bad != nil {
		return bad
	}
	argv := append([]string{a.CLI}, a.Path...)
	var warnings []string
	for _, f := range a.Flags {
		if strings.TrimLeft(f.Name, "-") == "" {
			return toolError("flag name %q is empty", f.Name)
		}
		flag, ok := findFlag(chain, f.Name)
		if !ok {
			// Help text can miss flags, so an unknown flag is kept and flagged.
			flag = spell(models.Flag{Name: f.Name})
			warnings = append(warnings, fmt.Sprintf("%s is not in the help of %q (known flags: %s)", flag, chain[len(chain)-1].FullCommand(), flagNames(chain)))
		}
		argv = append(argv, flag)
		if f.Value != nil {
			argv = append(argv, *f.Value)
		}
	}
	argv = append(argv, a.Args...)
	quoted := make([]string, len(argv))
	for i, x := range argv {
		quoted[i] = ShellQuote(x)
	}
	return jsonResult(Assembled{Command: strings.Join(quoted, " "), Argv: argv, Warnings: warnings}, true)
}

// walkPath follows path from root and returns every node on the way, root
// first. A stub on the way is discovered when the server can expand.
func (s *Server) walkPath(ctx context.Context, cli string, root *models.Node, path []string) ([]*models.Node, *toolResult) {
	chain := []*models.Node{root}
	n := root
	for i := 0; ; i++ {
		if (n.Stub || !n.Discovered) && s.Expand != nil {
			if err := s.Expand(ctx, cli, root, n); err != nil {
				return nil, toolError("discovering %q failed: %v", n.FullCommand(), err)
			}
		}
		if i == len(path) {
			return chain, nil
		}
		next := n.Find(path[i])
		if next == nil || next.Virtual {
			return nil, toolError("%q has no subcommand %q; its subcommands are: %s", n.FullCommand(), path[i], childNames(n))
		}
		n = next
		chain = append(chain, n)
	}
}

// findFlag looks name up on the last node of chain, then its ancestors, and
// returns the flag as the CLI spells it (the long form when there is one).
func findFlag(chain []*models.Node, name string) (string, bool) {
	want := strings.TrimLeft(name, "-")
	if want == "" {
		return "", false
	}
	for i := len(chain) - 1; i >= 0; i-- {
		for _, f := range chain[i].Flags {
			if strings.TrimLeft(f.Name, "-") == want || (f.ShortName != "" && strings.TrimLeft(f.ShortName, "-") == want) {
				return spell(f), true
			}
		}
	}
	return "", false
}

// spell is how a flag is written on the command line.
func spell(f models.Flag) string {
	name := f.Name
	if name == "" {
		name = f.ShortName
	}
	if strings.HasPrefix(name, "-") {
		return name
	}
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

func childNames(n *models.Node) string {
	var names []string
	for _, c := range n.Children {
		if !c.Virtual {
			names = append(names, c.Name)
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

func flagNames(chain []*models.Node) string {
	seen := map[string]bool{}
	var names []string
	for i := len(chain) - 1; i >= 0; i-- {
		for _, f := range chain[i].Flags {
			if s := spell(f); !seen[s] {
				seen[s] = true
				names = append(names, s)
			}
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

// safeShellRe matches words a POSIX shell reads literally without quotes.
var safeShellRe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// ShellQuote quotes s for a POSIX shell: as is when every character is safe,
// otherwise in single quotes with each embedded quote written as '\”.
func ShellQuote(s string) string {
	if safeShellRe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

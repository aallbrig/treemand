package discovery

import (
	"context"
	"slices"
	"strings"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// CompletionsDiscoverer discovers subcommands by running the CLI's built-in
// shell completion mechanism (compatible with Cobra's __complete protocol).
// It runs: <cli> __complete "" and parses each output line as:
//
//	<name>\t<description>
//
// The result is a root node with stub children for each discovered subcommand.
// This is intentionally shallow — it discovers only the top-level subcommands.
type CompletionsDiscoverer struct {
	// Launcher launches the CLI; nil means ExecLauncher.
	Launcher Launcher
}

// NewCompletionsDiscoverer creates a CompletionsDiscoverer.
func NewCompletionsDiscoverer() *CompletionsDiscoverer { return &CompletionsDiscoverer{} }

func (c *CompletionsDiscoverer) Name() string { return "completions" }

// Discover runs <cliName> __complete "" and parses the output.
// Returns nil, nil when the CLI does not support __complete (not an error).
func (c *CompletionsDiscoverer) Discover(ctx context.Context, cliName string, args []string) (*models.Node, error) {
	// The user chose this strategy, which sends cobra's hidden "__complete".
	p := Probe{Path: args, Args: append(slices.Clone(args), "__complete", ""), Basis: Declared}
	r := Runner{Launcher: c.Launcher}.Run(ctx, resolveBinary(cliName), p)
	if r.Err != nil {
		return nil, nil //nolint:nilerr // __complete unsupported means no completions, not a failure
	}

	children := ParseCompletionOutput(strings.TrimSpace(r.Out), append([]string{cliName}, args...))
	if len(children) == 0 {
		return nil, nil
	}

	fullPath := []string{cliName}
	fullPath = append(fullPath, args...)
	root := &models.Node{
		Name:     cliName,
		FullPath: fullPath,
		Children: children,
	}
	return root, nil
}

// ParseCompletionOutput converts Cobra __complete output to stub child nodes.
// Each line is "<name>\t<description>" or just "<name>".
// Lines starting with ':' are directives (completion codes) and are skipped.
func ParseCompletionOutput(out string, parentPath []string) []*models.Node {
	var children []*models.Node
	seen := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		// Skip flag completions (lines starting with "-").
		if strings.HasPrefix(line, "-") {
			continue
		}
		name, desc, _ := strings.Cut(line, "\t")
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		childPath := make([]string, len(parentPath)+1)
		copy(childPath, parentPath)
		childPath[len(parentPath)] = name
		children = append(children, &models.Node{
			Name:        name,
			FullPath:    childPath,
			Description: desc,
			Stub:        true,
		})
	}
	return children
}

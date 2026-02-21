// Package discovery provides strategies for discovering CLI command hierarchies.
package discovery

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/aallbrig/treemand/models"
)

// Discoverer is the interface for CLI hierarchy discovery strategies.
type Discoverer interface {
	// Name returns the strategy name.
	Name() string
	// Discover builds a command tree rooted at the given args.
	Discover(ctx context.Context, cliName string, args []string) (*models.Node, error)
}

// HelpDiscoverer uses --help output to discover subcommands and flags.
type HelpDiscoverer struct {
	MaxDepth int
	Timeout  time.Duration
}

// NewHelpDiscoverer creates a HelpDiscoverer with sensible defaults.
func NewHelpDiscoverer(maxDepth int) *HelpDiscoverer {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	return &HelpDiscoverer{MaxDepth: maxDepth, Timeout: 5 * time.Second}
}

func (h *HelpDiscoverer) Name() string { return "help" }

// Discover runs the CLI with --help and recursively discovers subcommands.
func (h *HelpDiscoverer) Discover(ctx context.Context, cliName string, args []string) (*models.Node, error) {
	return h.discover(ctx, cliName, args, 0)
}

func (h *HelpDiscoverer) discover(ctx context.Context, cliName string, args []string, depth int) (*models.Node, error) {
	fullPath := append([]string{cliName}, args...)
	node := &models.Node{
		Name:       lastElement(fullPath),
		FullPath:   fullPath,
		Discovered: true,
	}

	helpText, err := h.runHelp(ctx, cliName, args)
	if err != nil {
		// Not all CLIs support --help; treat as leaf
		node.Description = fmt.Sprintf("(could not get help: %v)", err)
		return node, nil
	}
	node.HelpText = helpText

	parsed := parseHelpOutput(helpText)
	node.Description = parsed.description
	node.Flags = parsed.flags
	node.Positionals = parsed.positionals

	if depth < h.MaxDepth {
		for _, sub := range parsed.subcommands {
			subCtx, cancel := context.WithTimeout(ctx, h.Timeout)
			child, err := h.discover(subCtx, cliName, append(args, sub), depth+1)
			cancel()
			if err != nil {
				child = &models.Node{Name: sub, FullPath: append(fullPath, sub), Discovered: false}
			}
			node.Children = append(node.Children, child)
		}
	}

	return node, nil
}

func (h *HelpDiscoverer) runHelp(ctx context.Context, cliName string, args []string) (string, error) {
	cmdArgs := append(args, "--help")
	cmd := exec.CommandContext(ctx, cliName, cmdArgs...)
	// Many CLIs write help to stderr
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		return string(out), nil
	}
	// Fallback: try -h
	cmdArgs = append(args, "-h")
	cmd = exec.CommandContext(ctx, cliName, cmdArgs...)
	out, err = cmd.CombinedOutput()
	if len(out) > 0 {
		return string(out), nil
	}
	return "", err
}

// parsedHelp holds the results of parsing --help output.
type parsedHelp struct {
	description string
	flags       []models.Flag
	positionals []models.Positional
	subcommands []string
}

var (
	flagLineRe = regexp.MustCompile(
		`^\s{1,6}(-[A-Za-z](?:,\s*)?)?` +
			`(--[A-Za-z][A-Za-z0-9_-]*)` +
			`(?:\s+(?:<([^>]+)>|\[([^\]]+)\]|([A-Z_]+)))?` +
			`(?:\s{2,}(.+))?$`,
	)
	subcmdLineRe = regexp.MustCompile(`^\s{2,6}([a-z][a-z0-9_-]*)(?:\s{2,}(.+))?$`)
)

// parseHelpOutput parses --help text into structured data.
func parseHelpOutput(text string) parsedHelp {
	var result parsedHelp
	lines := strings.Split(text, "\n")

	section := ""
	seenSubs := map[string]bool{}

	for i, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))

		// Section detection
		if isSection(lower, "available command", "command", "subcommand", "management command") {
			section = "commands"
			continue
		}
		if isSection(lower, "flag", "option", "global flag", "global option") {
			section = "flags"
			continue
		}
		if isSection(lower, "usage") {
			section = "usage"
			continue
		}
		if isSection(lower, "description") {
			section = "description"
			continue
		}
		if strings.HasSuffix(lower, ":") && lower != "" {
			section = ""
		}

		// Capture first non-empty line as description (before sections)
		if result.description == "" && i < 4 && strings.TrimSpace(line) != "" &&
			!strings.HasPrefix(strings.TrimSpace(line), "-") {
			result.description = strings.TrimSpace(line)
		}

		switch section {
		case "flags":
			if f, ok := parseFlag(line); ok {
				result.flags = append(result.flags, f)
			}
		case "commands":
			if m := subcmdLineRe.FindStringSubmatch(line); m != nil {
				name := m[1]
				if !seenSubs[name] && !isKeyword(name) {
					seenSubs[name] = true
					result.subcommands = append(result.subcommands, name)
				}
			}
		}

		// Also detect flags outside sections
		if section == "" || section == "usage" {
			if f, ok := parseFlag(line); ok {
				// deduplicate
				found := false
				for _, ef := range result.flags {
					if ef.Name == f.Name {
						found = true
						break
					}
				}
				if !found {
					result.flags = append(result.flags, f)
				}
			}
		}
	}

	// Parse positionals from usage line
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "usage:") {
			result.positionals = parsePositionals(line)
			break
		}
	}

	return result
}

func parseFlag(line string) (models.Flag, bool) {
	m := flagLineRe.FindStringSubmatch(line)
	if m == nil {
		return models.Flag{}, false
	}
	f := models.Flag{
		Name:      m[2],
		ShortName: strings.Trim(m[1], "-, "),
	}
	// value type
	switch {
	case m[3] != "":
		f.ValueType = m[3]
	case m[4] != "":
		f.ValueType = m[4]
	case m[5] != "":
		f.ValueType = strings.ToLower(m[5])
	default:
		f.ValueType = "bool"
	}
	f.Description = strings.TrimSpace(m[6])
	return f, true
}

func parsePositionals(usageLine string) []models.Positional {
	var result []models.Positional
	// Match <required> and [optional] patterns
	req := regexp.MustCompile(`<([^>]+)>`)
	opt := regexp.MustCompile(`\[([^\]]+)\]`)
	for _, m := range req.FindAllStringSubmatch(usageLine, -1) {
		if !strings.HasPrefix(m[1], "-") {
			result = append(result, models.Positional{Name: m[1], Required: true})
		}
	}
	for _, m := range opt.FindAllStringSubmatch(usageLine, -1) {
		name := strings.TrimPrefix(m[1], "...")
		if !strings.HasPrefix(name, "-") && !strings.Contains(name, "=") {
			result = append(result, models.Positional{Name: name, Required: false})
		}
	}
	return result
}

func isSection(lower string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(lower, kw) && strings.HasSuffix(lower, ":") {
			return true
		}
	}
	return false
}

var keywords = map[string]bool{
	"help": true, "version": true, "true": true, "false": true,
}

func isKeyword(s string) bool { return keywords[s] }

func lastElement(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return path[len(path)-1]
}

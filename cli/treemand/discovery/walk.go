// Package discovery provides strategies for discovering CLI command hierarchies.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aallbrig/treemand/models"
)

// Discoverer is the interface for CLI hierarchy discovery strategies.
type Discoverer interface {
	Name() string
	Discover(ctx context.Context, cliName string, args []string) (*models.Node, error)
}

// HelpDiscoverer uses --help output to discover subcommands and flags.
type HelpDiscoverer struct {
	MaxDepth int
	// Timeout bounds each launch of the CLI.
	Timeout       time.Duration
	StubThreshold int // max subcommands before creating stubs instead of eager discovery
	// ChildrenAsStubs lists every subcommand as a name-only stub instead of
	// discovering it, so a caller can crawl the tree one node at a time.
	ChildrenAsStubs bool

	form atomic.Int32 // helpForm the root answered with; children try it first

	manPages sync.Map // cliName -> its man page as plain text ("" for none)

	// HelpSubcommand: the CLI documents "<cli> [path...] help" as its help
	// (its preset says so), so that form is asked first.
	HelpSubcommand bool
	// Launcher launches the CLI; nil means ExecLauncher.
	Launcher Launcher
}

// NewHelpDiscoverer creates a HelpDiscoverer with sensible defaults.
func NewHelpDiscoverer(maxDepth int) *HelpDiscoverer {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	return &HelpDiscoverer{MaxDepth: maxDepth, Timeout: 5 * time.Second, StubThreshold: 150}
}

func (h *HelpDiscoverer) Name() string { return "help" }

// Discover runs the CLI with --help and recursively discovers subcommands.
func (h *HelpDiscoverer) Discover(ctx context.Context, cliName string, args []string) (*models.Node, error) {
	node, err := h.discover(ctx, cliName, args, 0)
	if err == nil && node != nil {
		if node.Description == "" && len(args) == 0 {
			node.Description = h.manOneLiner(ctx, cliName)
		}
		models.MarkInheritedFlags(node)
	}
	return node, err
}

// manOneLiner returns the NAME one-liner from cliName's man page, or "" when
// there is no man page. Used when --help is only a usage synopsis.
func (h *HelpDiscoverer) manOneLiner(ctx context.Context, cliName string) string {
	plain := h.manPage(ctx, cliName)
	if plain == "" {
		return ""
	}
	return ParseHelpOutput(plain).Description
}

// manPage returns cliName's man page as plain text, or "" when it has none.
// It is fetched once per discoverer: the root may need it for its options,
// its description and its command list.
func (h *HelpDiscoverer) manPage(ctx context.Context, cliName string) string {
	if v, ok := h.manPages.Load(cliName); ok {
		return v.(string)
	}
	plain, err := NewManDiscoverer().fetchManPage(ctx, cliName)
	if err != nil {
		plain = ""
	}
	h.manPages.Store(cliName, plain)
	return plain
}

func (h *HelpDiscoverer) discover(ctx context.Context, cliName string, args []string, depth int) (*models.Node, error) {
	return h.discoverWithHelp(ctx, cliName, args, depth, "")
}

// discoverWithHelp is discover for a node whose help text the caller already
// has (knownHelp != ""), so the command is not run a second time.
func (h *HelpDiscoverer) discoverWithHelp(ctx context.Context, cliName string, args []string, depth int, knownHelp string) (*models.Node, error) {
	fullPath := make([]string, 0, 1+len(args))
	fullPath = append(fullPath, cliName)
	fullPath = append(fullPath, args...)

	node := &models.Node{
		Name:       fullPath[len(fullPath)-1],
		FullPath:   fullPath,
		Discovered: true,
	}

	atRoot := depth == 0 && len(args) == 0
	helpText, err := knownHelp, error(nil)
	if helpText == "" {
		helpText, err = h.fetchHelp(ctx, cliName, args, atRoot)
	}
	var noTree *noRootHelpError
	if errors.As(err, &noTree) {
		return nil, err
	}
	if err != nil || helpText == "" {
		node.DiscoveryErr = fmt.Sprintf("could not get help: %v", err)
		return node, nil
	}
	node.HelpText = helpText

	parsed := ParseHelpOutputFor(helpText, cliName)
	node.Description = parsed.Description
	node.Positionals = parsed.Positionals
	var forAnyCommand []models.Flag
	node.Flags, forAnyCommand = h.rootOptions(ctx, cliName, helpText, parsed.Flags, atRoot)

	if atRoot && len(parsed.Subcommands) == 0 {
		helpText, parsed = h.cleoListFallback(ctx, cliName, node, helpText, parsed)
	}

	// No subcommands in --help at the root: some CLIs (tmux) list their
	// commands through "list-commands" instead.
	if atRoot && len(parsed.Subcommands) == 0 {
		if kids := h.commandListChildren(ctx, cliName, helpText, fullPath); len(kids) > 0 {
			node.Children = kids
			return node, nil
		}
	}

	if depth >= h.MaxDepth || len(parsed.Subcommands) == 0 {
		return node, nil
	}
	// When a command has a very large number of subcommands (e.g. aws
	// with 200+ services), eagerly running --help on every child would
	// take minutes. Instead, create lightweight stub nodes that carry
	// only the name and path. They can be expanded on demand by the
	// caller (TUI toggle, or re-running discovery at a deeper depth).
	if h.ChildrenAsStubs || len(parsed.Subcommands) > h.stubThreshold() {
		node.Children = stubChildren(fullPath, parsed, forAnyCommand)
		return node, nil
	}
	walk := childWalk{cliName: cliName, args: args, fullPath: fullPath, depth: depth, parentHelp: helpText,
		globalFlags: forAnyCommand}
	node.Children = h.discoverChildren(ctx, walk, parsed)
	return node, nil
}

// noRootHelpError reports that the root never answered with help in time.
// Without the root's help there is no tree: a bare root would pass for a CLI
// with no commands (#229).
type noRootHelpError struct{ err error }

func (e *noRootHelpError) Error() string { return e.err.Error() }

func (e *noRootHelpError) Unwrap() error { return e.err }

// fetchHelp runs the help for one node. A root whose help timed out is asked
// once more with a longer budget, since a loaded host can push it past one
// command's (kubectl on a CI runner); if it still does not answer, the error
// is a *noRootHelpError.
func (h *HelpDiscoverer) fetchHelp(ctx context.Context, cliName string, args []string, atRoot bool) (string, error) {
	text, err := h.runHelp(ctx, cliName, args)
	if !atRoot {
		return text, err
	}
	if errors.Is(err, ErrHelpTimedOut) && ctx.Err() == nil {
		text, err = h.runHelp(withRootRetryBudget(ctx), cliName, args)
	}
	if err == nil || (!errors.Is(err, ErrHelpTimedOut) && ctx.Err() == nil) {
		return text, err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		err = ErrHelpTimedOut
	}
	return "", &noRootHelpError{fmt.Errorf("help for %s %w", cliName, err)}
}

// cleoListFallback handles cleo apps (poetry, #166): their --help is the help
// of their "list" command, which prints the real command list. Running "list"
// is safe only because the help itself documents it as the command that lists
// commands. It returns the help text and parse to use from here on, and
// updates node when the list is used.
func (h *HelpDiscoverer) cleoListFallback(ctx context.Context, cliName string, node *models.Node, helpText string, parsed ParsedHelp) (string, ParsedHelp) {
	if !cleoListHelpRe.MatchString(helpText) {
		return helpText, parsed
	}
	var timedOut atomic.Bool
	p := Probe{Args: []string{"list"}, Basis: Documented, DocumentedIn: helpText}
	out := h.runOne(ctx, resolveBinary(cliName), p, &timedOut)
	if out == "" || isErrorOutput(out) {
		return helpText, parsed
	}
	listed := ParseHelpOutputFor(out, cliName)
	if len(listed.Subcommands) == 0 {
		return helpText, parsed
	}
	node.HelpText = out
	node.Flags = listed.Flags
	if listed.Description != "" {
		node.Description = listed.Description
	}
	return out, listed
}

// stubThreshold is the subcommand count above which children become stubs.
func (h *HelpDiscoverer) stubThreshold() int {
	if h.StubThreshold <= 0 {
		return DefaultStubThreshold
	}
	return h.StubThreshold
}

// stubChildren returns one undiscovered stub per subcommand, carrying the
// description the parent's help gives it.
func stubChildren(fullPath []string, parsed ParsedHelp, forAnyCommand []models.Flag) []*models.Node {
	children := make([]*models.Node, 0, len(parsed.Subcommands))
	for _, sub := range parsed.Subcommands {
		children = append(children, &models.Node{
			Name:        sub,
			FullPath:    append(append([]string{}, fullPath...), sub),
			Discovered:  false,
			Stub:        true,
			Description: parsed.SubcommandDescs[sub],
			Flags:       inheritedCopy(forAnyCommand),
		})
	}
	return children
}

// childWalk is the parent a child is discovered under.
type childWalk struct {
	cliName    string
	args       []string
	fullPath   []string
	depth      int
	parentHelp string
	// globalFlags are the parent's options that every command accepts (see
	// rootOptions); a child with no flags of its own takes them, marked
	// inherited.
	globalFlags []models.Flag
}

// discoverChildren discovers every subcommand concurrently (at most eight
// at a time), keeping the help's order. A child with no description of its
// own takes the one the parent's help gives it.
func (h *HelpDiscoverer) discoverChildren(ctx context.Context, walk childWalk, parsed ParsedHelp) []*models.Node {
	const maxWorkers = 8
	sem := make(chan struct{}, maxWorkers)
	results := make([]*models.Node, len(parsed.Subcommands))
	var wg sync.WaitGroup
	for i, sub := range parsed.Subcommands {
		wg.Add(1)
		go func(i int, sub string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = h.discoverChild(ctx, walk, sub)
		}(i, sub)
	}
	wg.Wait()
	var children []*models.Node
	for _, child := range results {
		if child == nil {
			continue
		}
		if child.Description == "" {
			child.Description = parsed.SubcommandDescs[child.Name]
		}
		children = append(children, child)
	}
	return children
}

// discoverChild runs the help of one subcommand and discovers it.
//
// A child that ends with no flags of its own takes the options its parent's
// usage accepts with any command, marked inherited: apt prints no help for
// its commands (they echo the general help of apt, apt-get or apt-cache, or
// fail when the Linux Mint wrapper needs sudo), yet "apt install -o ..." and
// "apt install -t <release>" are valid.
func (h *HelpDiscoverer) discoverChild(ctx context.Context, walk childWalk, sub string) *models.Node {
	child := h.discoverChildNode(ctx, walk, sub)
	if len(child.Flags) == 0 {
		child.Flags = inheritedCopy(walk.globalFlags)
	}
	return child
}

// discoverChildNode runs the help of one subcommand and discovers it.
func (h *HelpDiscoverer) discoverChildNode(ctx context.Context, walk childWalk, sub string) *models.Node {
	subArgs := append(append([]string{}, walk.args...), sub)
	subFull := append(append([]string{}, walk.fullPath...), sub)
	childHelp, err := h.runHelp(ctx, walk.cliName, subArgs)
	if err != nil || childHelp == "" {
		return &models.Node{
			Name:         sub,
			FullPath:     subFull,
			Discovered:   true,
			DiscoveryErr: fmt.Sprintf("could not get help: %v", err),
		}
	}
	if repeatsParent(walk.parentHelp, childHelp) {
		// The CLI ignored the subcommand and printed its parent's help:
		// keep the node, but do not descend into the same help again.
		childParsed := ParseHelpOutputFor(childHelp, walk.cliName)
		return &models.Node{
			Name:        sub,
			FullPath:    subFull,
			Discovered:  true,
			HelpText:    childHelp,
			Description: childParsed.Description,
			Flags:       childParsed.Flags,
			Positionals: childParsed.Positionals,
		}
	}
	child, err := h.discoverWithHelp(ctx, walk.cliName, subArgs, walk.depth+1, childHelp)
	if err != nil {
		return &models.Node{Name: sub, FullPath: subFull}
	}
	return child
}

// resolveBinary finds the executable for cliName.
// Tries PATH first, then ./cliName (current dir), then the directory of the
// running executable so that "treemand treemand" works without PATH changes.
func resolveBinary(cliName string) string {
	p, _ := resolveBinaryOrError(cliName)
	return p
}

// ResolveBinary returns the path the CLI named cliName runs from, as discovery
// will execute it.
func ResolveBinary(cliName string) string { return resolveBinary(cliName) }

// resolveBinaryOrError is like resolveBinary but returns an error when the
// binary cannot be located anywhere on the system.
func resolveBinaryOrError(cliName string) (string, error) {
	if p, err := exec.LookPath(cliName); err == nil {
		return p, nil
	}
	// Try the current working directory
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, cliName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	// Try the directory of the running executable (e.g. ./treemand treemand)
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), cliName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return cliName, fmt.Errorf("command %q not found in PATH or current directory", cliName)
}

// CheckAvailable returns an error if cliName is on the never-probe list
// (a *NeverProbeError) or cannot be resolved to an executable. Call this before starting discovery to give the user a clear
// error message instead of a cryptic "no help output" stub node.
func CheckAvailable(cliName string) error {
	if err := CheckProbeAllowed(cliName); err != nil {
		return err
	}
	_, err := resolveBinaryOrError(cliName)
	return err
}

// rootOptions returns the root's options and those of them every command
// accepts. When the root's help lists no options, they come from its man page
// (apt's help never lists one, #65), and when its usage also takes options
// with any command ("Usage: apt command [options]"), each command accepts
// them: its own help, which says nothing of options, cannot list them either.
// A CLI whose help lists its options is left as it is: its commands' helps
// list theirs.
func (h *HelpDiscoverer) rootOptions(ctx context.Context, cliName, helpText string, flags []models.Flag, atRoot bool) (own, forAnyCommand []models.Flag) {
	if !atRoot || len(flags) > 0 {
		return flags, nil
	}
	man := ManFlags(h.manPage(ctx, cliName))
	if !optionsWithCommandRe.MatchString(helpText) {
		return man, nil
	}
	return man, man
}

// optionsWithCommandRe matches a usage line that takes options with any
// command: "Usage: apt command [options]", "Usage: apt [options] command".
var optionsWithCommandRe = regexp.MustCompile(`(?im)^\s*usage:.*(?:\[options?\].*\bcommand\b|\bcommand\b.*\[options?\])`)

// inheritedCopy returns a copy of flags, each marked inherited.
func inheritedCopy(flags []models.Flag) []models.Flag {
	if len(flags) == 0 {
		return nil
	}
	out := make([]models.Flag, len(flags))
	for i, f := range flags {
		f.Inherited = true
		out[i] = f
	}
	return out
}

// allInherited reports whether every flag is an inherited one (true for none).
func allInherited(flags []models.Flag) bool {
	for _, f := range flags {
		if !f.Inherited {
			return false
		}
	}
	return true
}

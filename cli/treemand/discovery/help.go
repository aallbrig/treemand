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
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/aallbrig/treemand/models"
	// Layering exemption (.golangci.yml depguard): TODO(#481) removes it.
	"github.com/aallbrig/treemand/presets"
)

// Discoverer is the interface for CLI hierarchy discovery strategies.
type Discoverer interface {
	Name() string
	Discover(ctx context.Context, cliName string, args []string) (*models.Node, error)
}

// HelpDiscoverer uses --help output to discover subcommands and flags.
type HelpDiscoverer struct {
	MaxDepth      int
	Timeout       time.Duration
	StubThreshold int // max subcommands before creating stubs instead of eager discovery
	// ChildrenAsStubs lists every subcommand as a name-only stub instead of
	// discovering it, so a caller can crawl the tree one node at a time.
	ChildrenAsStubs bool

	form atomic.Int32 // helpForm the root answered with; children try it first

	manPages sync.Map // cliName -> its man page as plain text ("" for none)
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
	out := h.runOne(ctx, resolveBinary(cliName), []string{"list"}, &timedOut)
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
		return 50
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
	if childHelp == walk.parentHelp {
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

// pagerEnv are environment variable overrides appended to every help command
// so that tools that pipe through a pager (AWS, man, etc.) emit plain text.
var pagerEnv = []string{
	"AWS_PAGER=",
	"PAGER=cat",
	"MANPAGER=cat",
	"GIT_PAGER=cat",
	// Man pages (git <cmd> --help) wrap to the terminal's width; a fixed width
	// makes what is parsed, and cached, the same in any terminal.
	"MANWIDTH=80",
}

// truncatedHelpRe matches messages that indicate --help output is abbreviated
// and a more complete form is available (e.g. curl's "use --help all").
var truncatedHelpRe = regexp.MustCompile(`(?i)--help all|--help <category>|not the full help`)

// killWaitDelay bounds how long we wait for a command's output pipes to close
// after its context ends. Without it, an orphaned child that inherited stdout
// keeps CombinedOutput blocked long after the timeout.
const killWaitDelay = time.Second

// helpSlots bounds how many help processes run at once across the whole
// discovery, however deep or wide the tree is.
var helpSlots atomic.Pointer[chan struct{}]

// DefaultMaxConcurrentHelp is the default cap: twice the CPU count, within 4..16.
func DefaultMaxConcurrentHelp() int {
	n := runtime.NumCPU() * 2
	if n < 4 {
		n = 4
	}
	if n > 16 {
		n = 16
	}
	return n
}

// SetMaxConcurrentHelp sets the cap on concurrent help processes; n <= 0
// restores the default.
func SetMaxConcurrentHelp(n int) {
	if n <= 0 {
		n = DefaultMaxConcurrentHelp()
	}
	ch := make(chan struct{}, n)
	helpSlots.Store(&ch)
}

// acquireHelpSlot waits for a free process slot (or ctx to end) and returns
// the function that gives it back.
func acquireHelpSlot(ctx context.Context) (func(), bool) {
	p := helpSlots.Load()
	if p == nil {
		SetMaxConcurrentHelp(0)
		p = helpSlots.Load()
	}
	ch := *p
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, true
	case <-ctx.Done():
		return nil, false
	}
}

// ErrHelpTimedOut reports that every help form of a command ran out of its
// own time: the CLI may be overloaded, not refusing.
var ErrHelpTimedOut = errors.New("timed out")

// rootRetryScale multiplies the per-command budget for the one retry of a
// root whose help timed out.
const rootRetryScale = 3

type budgetScaleKey struct{}

// withRootRetryBudget returns ctx with each help command's budget scaled up
// for the root's retry; the run's own deadline still bounds it.
func withRootRetryBudget(ctx context.Context) context.Context {
	return context.WithValue(ctx, budgetScaleKey{}, rootRetryScale)
}

// commandTimeout is the time allowed for one help command.
func (h *HelpDiscoverer) commandTimeout(ctx context.Context) time.Duration {
	t := h.Timeout
	if t <= 0 {
		t = 5 * time.Second
	}
	if scale, ok := ctx.Value(budgetScaleKey{}).(int); ok && scale > 1 {
		t *= time.Duration(scale)
	}
	return t
}

// helpForm is one way of asking a CLI for help. The order is the priority:
// when several forms work, the earlier one wins.
type helpForm int32

const (
	formUnknown helpForm = iota
	formFlag             // <cli> [sub...] --help (and "--help all" for truncated help)
	formShort            // <cli> [sub...] -h
	formSuffix           // <cli> [sub...] help     (aws)
	formPrefix           // <cli> help [sub...]     (go mod, go build)
)

// learnedForm returns the flag form of the first successful flag answer, or
// formUnknown before any.
func (h *HelpDiscoverer) learnedForm() helpForm { return helpForm(h.form.Load()) }

// HelpForm names the flag form the CLI answered help with ("--help" or
// "-h"), or "" when no flag form has answered.
func (h *HelpDiscoverer) HelpForm() string {
	switch h.learnedForm() {
	case formFlag:
		return "--help"
	case formShort:
		return "-h"
	}
	return ""
}

// runHelp gets help text for args under cliName.
//
// Flag forms (--help, -h) only ever print help, so they may race: the
// preferred one (the form the CLI answered with before, else --help) gets a
// short head start and the other joins it if it is slow, and the
// higher-priority answer wins. A CLI answering --help costs one process per
// node.
//
// Positional forms ("<cmd> <sub> help", "help <cmd> <sub>") pass "help" as an
// argument, which a subcommand may act on: "git branch help" creates a branch,
// "docker context use help" switches context. They therefore never race and
// never run ahead of the flags: only after both flag forms have failed, one at
// a time, exactly as before form learning.
func (h *HelpDiscoverer) runHelp(ctx context.Context, cliName string, args []string) (string, error) {
	resolved := resolveBinary(cliName)

	flags := []helpForm{formFlag, formShort}
	if h.learnedForm() == formShort {
		flags = []helpForm{formShort, formFlag}
	}
	// at.timedOut is set when a form ran out of its own time. That is no
	// answer: the CLI may be overloaded, not refusing. Positional forms pass
	// "help" as an argument a subcommand may act on ("npm prune help" prunes,
	// #176), so they only ever follow flag forms that answered and refused.
	var at formAttempts
	// A preset may document "<cli> [path...] help" as the CLI's help (aws):
	// ask that first, one call instead of two failing flag forms before it.
	if p, ok := presets.Lookup(cliName); ok && p.HelpSubcommand {
		if s := h.tryForm(ctx, resolved, args, formSuffix, &at); s != "" {
			return s, nil
		}
	}
	// Until the CLI's flag form is known, race the two (hedged). Once a node
	// has answered one, ask that alone and the other only if it refuses:
	// hedging every slow child doubled the processes (gh: 457 for 236 nodes,
	// #216).
	race := h.raceForms
	if h.learnedForm() != formUnknown {
		race = h.formsInTurn
	}
	if s, f := race(ctx, resolved, args, flags, &at); s != "" {
		h.form.CompareAndSwap(int32(formUnknown), int32(f))
		return s, nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if at.timedOut.Load() {
		return "", ErrHelpTimedOut
	}
	// A flag form that was refused but still printed a usage synopsis is the
	// CLI's help: BSD tools answer --help with "mkdir: illegal option -- -"
	// and then "usage: mkdir [-pv] ...". Such a CLI takes operands, not
	// subcommands, so a positional "help" would be acted on ("mkdir help"
	// creates ./help, #273): never send one, use the usage instead.
	if s := at.refusedUsage(); s != "" {
		return s, nil
	}

	if s := h.tryForm(ctx, resolved, args, formSuffix, &at); s != "" {
		return s, nil
	}
	if len(args) > 0 {
		if s := h.tryForm(ctx, resolved, args, formPrefix, &at); s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("no help output from %s", cliName)
}

// raceForms runs flag forms concurrently and returns the output of the
// highest-priority one that produced help. It returns as soon as that is
// decided, cancelling (killing) the forms still running. Never pass it a
// positional form: see runHelp.
func (h *HelpDiscoverer) raceForms(ctx context.Context, resolved string, args []string, forms []helpForm, at *formAttempts) (string, helpForm) {
	for _, f := range forms {
		if f != formFlag && f != formShort {
			panic("discovery: only flag help forms may be raced")
		}
	}
	if len(forms) == 0 {
		return "", formUnknown
	}
	if len(forms) == 1 {
		return h.tryForm(ctx, resolved, args, forms[0], at), forms[0]
	}
	rctx, cancel := context.WithCancel(ctx)
	// Cancel the losers and wait for them: a form still running after
	// raceForms returns would outlive the discovery that started it (and could
	// report into hooks such as OnOutputTruncated after the caller moved on).
	var wg sync.WaitGroup
	defer wg.Wait()
	defer cancel()
	results := make([]chan string, len(forms))
	start := func(i int) {
		results[i] = make(chan string, 1)
		wg.Add(1)
		go func(ch chan<- string, f helpForm) {
			defer wg.Done()
			ch <- h.tryForm(rctx, resolved, args, f, at)
		}(results[i], forms[i])
	}
	// Hedge: the preferred form gets a head start, so a CLI that answers it
	// quickly never pays for the other. A slow or failing one holds the other
	// back by at most hedgeDelay.
	start(0)
	firstDone := false
	select {
	case s := <-results[0]:
		if s != "" {
			return s, forms[0]
		}
		firstDone = true
	case <-time.After(hedgeDelay):
	case <-ctx.Done():
		return "", formUnknown
	}
	for i := 1; i < len(forms); i++ {
		start(i)
	}
	for i, ch := range results {
		if i == 0 && firstDone {
			continue
		}
		if s := <-ch; s != "" {
			return s, forms[i]
		}
	}
	return "", formUnknown
}

// formsInTurn tries flag forms one after another, stopping at the first that
// answers, or when one times out (a timeout is no refusal).
func (h *HelpDiscoverer) formsInTurn(ctx context.Context, resolved string, args []string, forms []helpForm, at *formAttempts) (string, helpForm) {
	for _, f := range forms {
		if s := h.tryForm(ctx, resolved, args, f, at); s != "" {
			return s, f
		}
		if at.timedOut.Load() || ctx.Err() != nil {
			break
		}
	}
	return "", formUnknown
}

// hedgeDelay is how long the preferred flag form runs alone before the other
// is started alongside it. A var so a test can widen it: a forked shell script
// can take longer than 250ms when the whole suite runs under -cover.
var hedgeDelay = 250 * time.Millisecond

// formAttempts is what the help forms tried for one node learned besides
// their answers. Racing forms share it.
type formAttempts struct {
	timedOut atomic.Bool
	mu       sync.Mutex
	// usage holds refused flag-form output that carries a usage synopsis,
	// by flag form.
	usage map[helpForm]string
}

func (a *formAttempts) noteRefusedUsage(f helpForm, s string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.usage == nil {
		a.usage = map[helpForm]string{}
	}
	a.usage[f] = s
}

// refusedUsage returns the refused flag-form output with a usage synopsis,
// --help's before -h's, or "".
func (a *formAttempts) refusedUsage() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, f := range []helpForm{formFlag, formShort} {
		if s := a.usage[f]; s != "" {
			return s
		}
	}
	return ""
}

// usageLineRe matches a usage synopsis line: "usage: mkdir [-pv] ...".
var usageLineRe = regexp.MustCompile(`(?im)^[ \t]*usage:[ \t]+\S`)

// tryForm runs one help form and returns its sanitized output, or "" when the
// command printed nothing or an error instead of help.
func (h *HelpDiscoverer) tryForm(ctx context.Context, resolved string, args []string, f helpForm, at *formAttempts) string {
	var cmdArgs []string
	switch f {
	case formFlag:
		cmdArgs = append(append(cmdArgs, args...), "--help")
	case formShort:
		cmdArgs = append(append(cmdArgs, args...), "-h")
	case formSuffix:
		cmdArgs = append(append(cmdArgs, args...), "help")
	case formPrefix:
		cmdArgs = append(append(cmdArgs, "help"), args...)
	default:
		return ""
	}
	s := h.runOne(ctx, resolved, cmdArgs, &at.timedOut)
	if s == "" {
		return ""
	}
	if isErrorOutput(s) {
		if (f == formFlag || f == formShort) && usageLineRe.MatchString(s) {
			at.noteRefusedUsage(f, s)
		}
		return ""
	}
	return h.fullerHelp(ctx, resolved, args, f, s, at)
}

// fullerHelp returns the complete help a flag-form answer s points at, or s.
// Truncated help (curl) points at "--help all". A short answer that names
// "<cli> help <path>" as the details (the go command, #66) points at that
// form, which is the CLI's own, so it is safe to ask.
func (h *HelpDiscoverer) fullerHelp(ctx context.Context, resolved string, args []string, f helpForm, s string, at *formAttempts) string {
	if f == formFlag && truncatedHelpRe.MatchString(s) {
		if all := h.runOne(ctx, resolved, append(append([]string{}, args...), "--help", "all"), &at.timedOut); all != "" {
			return all
		}
	}
	if (f == formFlag || f == formShort) && pointsToHelpPrefix(s, resolved, args) {
		if full := h.runOne(ctx, resolved, append([]string{"help"}, args...), &at.timedOut); full != "" && !isErrorOutput(full) {
			return full
		}
	}
	return s
}

// helpPointerRes match a pointer to fuller help; the first group is the
// command: "Run 'go help build' for details." (with "and 'go help testflag'"
// variants) and "For more information run\n\tgo help doc".
var helpPointerRes = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^Run '([^']+)'(?: and '[^']+')* for (?:details|more information)\.?\s*$`),
	regexp.MustCompile(`(?m)^For more information,? run:?\s*\n[ \t]+(\S[^\n]*?)\s*$`),
}

// pointsToHelpPrefix reports whether s is a short help (a usage line and a
// pointer) that names exactly "<cli> help <args...>". Only that command is
// followed: a pointer to anything else is never run.
func pointsToHelpPrefix(s, resolved string, args []string) bool {
	if len(args) == 0 || strings.Count(strings.TrimSpace(s), "\n") > 12 {
		return false
	}
	want := strings.Join(append([]string{strings.TrimSuffix(filepath.Base(resolved), ".exe"), "help"}, args...), " ")
	for _, re := range helpPointerRes {
		if m := re.FindStringSubmatch(s); m != nil {
			return strings.Join(strings.Fields(m[1]), " ") == want
		}
	}
	return false
}

// runOne runs a single help command with the pager variables set and returns
// its sanitized output. h.Timeout bounds this one command only: a deadline
// shared by a group of children would starve the later ones.
func (h *HelpDiscoverer) runOne(ctx context.Context, resolved string, cmdArgs []string, timedOut *atomic.Bool) string {
	// Take a process slot first: time spent waiting for one must not count
	// against the command's own timeout.
	release, ok := acquireHelpSlot(ctx)
	if !ok {
		return ""
	}
	defer release()
	cctx, cancel := context.WithTimeout(ctx, h.commandTimeout(ctx))
	defer cancel()
	out, _, _ := runCapped(cctx, resolved, cmdArgs, pagerEnv, MaxHelpBytes)
	if cctx.Err() != nil && ctx.Err() != nil {
		return "" // cancelled by a race winner or the caller, not a real answer
	}
	if cctx.Err() == context.DeadlineExceeded {
		timedOut.Store(true)
		return ""
	}
	return SanitizeHelp(string(out))
}

// ParsedHelp holds structured results of parsing --help output.
// Exported so tests and other packages can use it directly.
type ParsedHelp struct {
	Description string
	Flags       []models.Flag
	Positionals []models.Positional
	Subcommands []string
	// SubcommandDescs holds the one-line description each subcommand has in
	// this command's own command list; used when the child's help has none.
	SubcommandDescs map[string]string
	DocsURL         string
	// Sections holds named flag groups (e.g. Godot's "General options:",
	// "Debug options:"). Only populated when multiple distinct sections exist.
	Sections []ParsedSection
}

// ParsedSection is a named group of flags found under a section header.
type ParsedSection struct {
	Name  string
	Flags []models.Flag
}

// section labels we recognize
const (
	secNone     = ""
	secFlags    = "flags"
	secCommands = "commands"
	secUsage    = "usage"
	secDesc     = "description"
	secName     = "name" // man-page NAME section
	secExamples = "examples"
	secAliases  = "aliases"
	secValues   = "values" // a list under a header the parser does not know (#154)
)

// sectionHeaders maps lower-cased keywords that appear in section header lines.
var sectionHeaders = map[string]string{
	"available commands":      secCommands,
	"available command":       secCommands,
	"available services":      secCommands, // AWS man-page style
	"available service":       secCommands,
	"management commands":     secCommands,
	"management command":      secCommands,
	"commands":                secCommands,
	"subcommands":             secCommands,
	"the commands are":        secCommands, // Go toolchain style: "The commands are:"
	"all commands":            secCommands, // npm style: "All commands:"
	"standard commands":       secCommands, // openssl style: "Standard commands"
	"message digest commands": secCommands, // openssl: "Message Digest commands"
	"cipher commands":         secCommands, // openssl: "Cipher commands"
	"utilities":               secCommands, // man pages: bundle "UTILITIES"
	"flags":                   secFlags,
	"options":                 secFlags,
	"global flags":            secFlags,
	"global options":          secFlags,
	"optional arguments":      secFlags,
	"arguments":               secFlags,
	"usage":                   secUsage,
	"use":                     secUsage,
	"name":                    secName,
	"description":             secDesc,
	"examples":                secExamples,
	"example":                 secExamples,
	"aliases":                 secAliases,
	// free-text sections that contain non-command content
	"supported formats":        secDesc,
	"supported engines":        secDesc,
	"supported types":          secDesc,
	"supported protocols":      secDesc,
	"supported backends":       secDesc,
	"supported drivers":        secDesc,
	"output formats":           secDesc,
	"output modes":             secDesc,
	"discovery strategies":     secDesc,
	"interactive tui controls": secDesc,
	"tui controls":             secDesc,
	"interactive":              secDesc,
	"output":                   secDesc,
	"notes":                    secDesc,
	"note":                     secDesc,
	"see also":                 secDesc,
	"environment variables":    secDesc,
	"environment":              secDesc,
	"configuration":            secDesc,
	"config":                   secDesc,
	"caching":                  secDesc,
	"cache":                    secDesc,
}

// awsBulletRe matches AWS man-page bullet list items.
// After stripping man-page bold formatting, "+o word" bullets appear as either
// "+o word" (plain text) or "o word" (when the "o" was rendered bold via backspace).
var awsBulletRe = regexp.MustCompile(`^\s{2,}(?:\+o|o)\s+([a-z][a-z0-9_-]*)$`)

// awsFlagRe matches AWS man-page flag lines: "       --flag (type)"
var awsFlagRe = regexp.MustCompile(`^\s{2,}(--[A-Za-z][A-Za-z0-9_-]*)\s+\(([^)]+)\)\s*$`)

// regexes compiled once
var (
	// long flag: --flag or --flag=type or --flag <type> or --flag type
	longFlagRe = regexp.MustCompile(
		`^\s{0,8}` +
			`(?:(-[A-Za-z0-9])(?:,\s*|\s+))?` + // optional short: -x, or -x (space)
			`(--[A-Za-z][A-Za-z0-9_-]*)` + // long: --flag
			`(?:` +
			`(?:\[=[A-Za-z][A-Za-z0-9_-]*\])` + // [=WHEN] GNU optional-value style
			`|(?:[= ](?:<([^>]+)>|\[([^\]]+)\]|([A-Za-z][A-Za-z0-9_-]*|'[^']*'|\[\]|\d+(?:\.\d+)?)))` + // =<type>, =type, kubectl =false / ='' / =[] / =500
			`)?` +
			`:?` + // kubectl: "--flag=false:" with the description on the next line
			`(?:\s+(.*))?$`, // description (1+ spaces gap)
	)
	// short-only flag: -v or -v <value>
	shortOnlyFlagRe = regexp.MustCompile(
		`^\s{2,8}(-[A-Za-z0-9])(?:\s+(?:<([^>]+)>|([A-Za-z][A-Za-z0-9_-]*)))?(?:\s{2,}(.*))?$`,
	)
	// manpageHeaderRe matches lines like "GIT-CLONE(1)   Git Manual   GIT-CLONE(1)".
	// These appear at the top of man-formatted --help output and should never be
	// used as the command description.
	manpageHeaderRe = regexp.MustCompile(`^[A-Z][A-Z0-9_.-]+\(\d*\)\s`)
	// nameSectionDescRe matches the NAME section body: "   git-clone - Short desc"
	nameSectionDescRe = regexp.MustCompile(`^[\s\t]+\S.*?\s+[-\x{2013}\x{2014}]\s+(.+)$`)
	// subcommand line: 2–8 leading spaces, lowercase word; args like [PATTERN...] may appear
	// between name and description (e.g. systemctl's "  list-units [PAT...]   description")
	subcmdRe = regexp.MustCompile(`^\s{2,8}([a-z][a-z0-9_-]*)(?:.*?\s{2,}(.+))?$`)
	// positional in usage: <required> or [optional]
	reqArgRe = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9_.-]*)>`)
	optArgRe = regexp.MustCompile(`\[([A-Za-z][A-Za-z0-9_.-]*)(?:\.{3})?\]`)
	// URL in help text
	urlRe = regexp.MustCompile(`https?://[^\s]+`)
)

// ansiRe matches ANSI terminal escape codes (colors, etc.)
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHF]`)

// manpageBoldRe matches man-page backspace-based bold/underline sequences.
// Man pages encode bold as "char\bchar" and underline as "_\bchar".
// Replacing "char\b" with "" leaves only the visible character.
var manpageBoldRe = regexp.MustCompile(`.\x08`)

// buildMarkerRe matches Godot-style build-availability markers at the start of
// a flag description: a single uppercase letter (R/D/X/E) followed by 2+ spaces.
// e.g. "R  Display this help message." → "Display this help message."
var buildMarkerRe = regexp.MustCompile(`^[A-Z]\s{2,}`)

// goTabSubcmdRe matches Go toolchain style tab-indented subcommand lines:
//
//	\tbug         start a bug report
var goTabSubcmdRe = regexp.MustCompile(`^\t([a-z][a-z0-9_-]*)(?:\s{2,}(.+))?$`)

// validCmdNameRe matches a bare command name (npm comma-separated list entries).
var validCmdNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// gridCmdRe matches a multi-column grid entry (openssl style): lowercase word, min 2 chars.
var gridCmdRe = regexp.MustCompile(`^[a-z][a-z0-9_-]+$`)

// stripBuildMarker removes a leading build-availability marker if present.
func stripBuildMarker(s string) string {
	return strings.TrimSpace(buildMarkerRe.ReplaceAllString(s, ""))
}

// allGridEntries reports whether every token in parts looks like a command name.
// Used to detect openssl-style multi-column grids of subcommands.
func allGridEntries(parts []string) bool {
	for _, p := range parts {
		if !gridCmdRe.MatchString(p) {
			return false
		}
	}
	return true
}

// isErrorOutput reports whether s looks like a CLI error message rather than
// help text. Used to skip error output from CLIs (e.g. aws, go) that print an
// error instead of help when --help is passed without a required argument.
// Checks for Contains("unknown command") rather than HasPrefix because Go-style
// CLIs emit "<name>: unknown command" (e.g. "go mod: unknown command").
func isErrorOutput(s string) bool {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return false
	}
	// A complete help body is help, even after a warning line: flask --help
	// with no app prints "Error: Could not locate a Flask application" and
	// then its full help.
	if hasHelpBody(lines) {
		return false
	}
	first := strings.ToLower(lines[0])
	if strings.Contains(first, "[error]") ||
		strings.HasPrefix(first, "error:") ||
		strings.Contains(first, "unknown command") ||
		strings.Contains(first, "invalid command") {
		return true
	}
	// A short answer that names a rejected option is a usage error, even when
	// it starts with a usage line (flask -h: "Usage: … Error: No such option").
	if len(lines) <= 6 {
		low := strings.ToLower(strings.Join(lines, "\n"))
		for _, m := range usageErrorMarkers {
			if strings.Contains(low, m) {
				return true
			}
		}
	}
	return false
}

// usageErrorMarkers are how argument parsers say an option was rejected.
var usageErrorMarkers = []string{
	"no such option",         // click
	"unknown shorthand flag", // cobra
	"unknown flag",           // cobra, pflag
	"unknown option",         // many
	"invalid option",         // getopt: "invalid option -- 'h'"
	"unrecognized option",    // getopt_long
	"unrecognized arguments", // argparse
	"illegal option",         // BSD getopt
}

// helpSectionHeaders are section titles that only real help has.
var helpSectionHeaders = map[string]bool{
	"options:": true, "commands:": true, "flags:": true, "global flags:": true,
	"available commands:": true, "subcommands:": true,
	"positional arguments:": true, "optional arguments:": true,
}

// hasHelpBody reports whether the output is a full help page: long enough and
// with at least one section a usage error never has.
func hasHelpBody(lines []string) bool {
	if len(lines) < 8 {
		return false
	}
	for _, l := range lines[1:] {
		if helpSectionHeaders[strings.ToLower(strings.TrimSpace(l))] {
			return true
		}
	}
	return false
}

// skipSubcmdWords are words that look like subcommands but aren't.
var skipSubcmdWords = map[string]bool{
	"true": true, "false": true,
	"none": true, "all": true, "on": true, "off": true,
	"yes": true, "no": true, "default": true,
	// "or" starts a usage continuation ("   or  java [options] -jar …").
	"or": true,
	// Single-letter "commands" are almost always hotkey hints in help text, not subcommands.
	"a": true, "b": true, "c": true, "d": true, "e": true, "f": true,
	"g": true, "h": true, "i": true, "j": true, "k": true, "l": true,
	"m": true, "n": true, "o": true, "p": true, "q": true, "r": true,
	"s": true, "t": true, "u": true, "v": true, "w": true, "x": true,
	"y": true, "z": true,
}

// stripANSI removes ANSI terminal escape codes from a string.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// stripManpageFormatting removes backspace-based bold/underline sequences used
// by groff/man output. Example: "+\bo" (bold o rendered on top of +) → "o".
func stripManpageFormatting(s string) string {
	return manpageBoldRe.ReplaceAllString(s, "")
}

// ParseHelpOutput parses --help output into structured ParsedHelp.
// Exported so it can be tested directly.
func ParseHelpOutput(text string) ParsedHelp {
	return ParseHelpOutputFor(text, "")
}

// ParseHelpOutputFor parses --help output with knowledge of the CLI name being
// introspected, so self-referential example lines don't create bogus subcommands.
func ParseHelpOutputFor(text, selfName string) ParsedHelp {
	text = stripANSI(text)
	text = stripManpageFormatting(text)
	p := &helpParser{
		lines:           strings.Split(text, "\n"),
		selfName:        selfName,
		seenSubs:        map[string]bool{},
		seenFlags:       map[string]bool{},
		usageEnd:        -1,
		choiceIndent:    -1,
		choiceRowIndent: -1,
	}
	for i, raw := range p.lines {
		trimmed := strings.TrimSpace(raw)
		p.line(helpLine{i: i, raw: raw, trimmed: trimmed, lower: strings.ToLower(trimmed)})
	}
	return p.finish()
}

// helpLine is one line of help text in the forms the rules test.
type helpLine struct {
	i                   int
	raw, trimmed, lower string
}

// helpParser is one pass over a help text. Each rule is a method; line runs
// them in order, and a rule that consumes the line ends it.
type helpParser struct {
	lines    []string
	selfName string
	result   ParsedHelp

	section    string
	bulletSubs []string // AWS man-page "o service" bullets: authoritative when 3+
	seenSubs   map[string]bool
	seenFlags  map[string]bool
	usageLines []string

	// inNameSection: inside the man-page NAME section, whose
	// "   git-clone - Short desc" line is the cleanest description.
	inNameSection bool
	// currentSectionName names the flag group being read ("General options"),
	// so flags are grouped under it (Godot).
	currentSectionName string
	// pendingFlag is an AWS-style "--flag (type)" (or kubectl "--flag=false:")
	// whose description is on the next non-empty line.
	pendingFlag *models.Flag

	// inUsageBlock is true from a "usage:" line through its indented
	// continuation lines, which are synopsis text and never a description.
	inUsageBlock bool
	// descSkipped counts lines that cannot be the description and should not
	// use up the window it is looked for in: banners, and the value lists
	// under unknown headers (rsync's "Capabilities:"). usageEnd is where a
	// usage block ended: the paragraph after a long wrapped usage (argparse)
	// is the description wherever it falls.
	descSkipped, usageEnd int
	// argparse subparser block (#81): a "{a,b,c…}" line, then one row per
	// subcommand at the first row's indent; deeper rows continue a description.
	choiceIndent, choiceRowIndent int
	lastGridRow                   bool
	inGrammar                     bool // inside "OBJECT := { a | b … }" (ip, #160)
}

func (p *helpParser) line(l helpLine) {
	p.trackUsageBlock(l)
	p.flushPendingFlag(l)
	if p.sectionHeader(l) || p.choiceBlock(l) || p.commandGrid(l) || p.unknownHeader(l) {
		return
	}
	p.maybeResetSection(l)
	p.captureDescription(l)
	p.collectUsageAndURL(l)

	switch p.section {
	case secFlags:
		p.flagLine(l)
	case secCommands:
		// No command name starts with "-": a described option row here is a
		// flag. "go help build" wraps "...shared by the build, clean, get,
		// install, list, run,\nand test commands:" over its flag list (#66).
		// An undescribed one is a wrapped example ("    --lint=none").
		if f, ok := parseFlag(l.raw); ok && strings.HasPrefix(l.trimmed, "-") {
			if f = p.withNextLineDescription(l, f); f.Description != "" {
				p.addFlag(f)
			}
			return
		}
		p.commandLine(l)
	case secExamples, secAliases, secDesc, secValues:
		// Narrative text, examples or aliases: example usage may reference
		// flags worth surfacing, but these never list subcommands.
		p.addParsedFlag(l)
	case secNone, secUsage:
		// Outside named sections: flags, and subcommands with stricter checks.
		p.addParsedFlag(l)
		p.proseFlags(l)
		p.freeFormSubcommand(l.raw)
	}
}

func (p *helpParser) trackUsageBlock(l helpLine) {
	switch {
	case strings.HasPrefix(l.lower, "usage:"):
		p.inUsageBlock = true
	case l.trimmed == "" || (l.raw[0] != ' ' && l.raw[0] != '\t'):
		if p.inUsageBlock {
			p.usageEnd = l.i
		}
		p.inUsageBlock = false
	}
	if p.result.Description == "" && l.trimmed != "" &&
		(bannerRe.MatchString(l.trimmed) || (p.section == secValues && l.raw[0] == ' ') || versionRowRe.MatchString(l.raw)) {
		p.descSkipped++
	}
}

// flushPendingFlag completes a pending flag at the next non-empty line, which
// is its description unless it is another flag.
func (p *helpParser) flushPendingFlag(l helpLine) {
	if p.pendingFlag == nil || l.trimmed == "" {
		return
	}
	if !strings.HasPrefix(l.trimmed, "--") && awsFlagRe.FindString(l.raw) == "" {
		p.pendingFlag.Description = l.trimmed
	}
	if !p.seenFlags[p.pendingFlag.Name] {
		p.addFlag(*p.pendingFlag)
	}
	p.pendingFlag = nil
}

// sectionHeader detects "Flags:", "Available Commands:", "GLOBAL OPTIONS"…
// Bullet items ("o appconfig") are never headers, even when the name ends in
// a header keyword such as "config" or "cache". Inside a command list, a line
// indented 4+ spaces belongs to the previous row's description (kingpin's
// "    examples:"), and so does an indented bare word (a cleo namespace label).
func (p *helpParser) sectionHeader(l helpLine) bool {
	describesRow := p.section == secCommands &&
		(indentOf(l.raw) >= 4 || (indentOf(l.raw) >= 1 && !strings.ContainsAny(l.trimmed, " :")))
	sec := detectSection(l.lower)
	if sec == secNone || awsBulletRe.MatchString(l.raw) || describesRow {
		return false
	}
	// A named flag group ("General options:") groups the flags under it.
	p.currentSectionName = ""
	if sec == secFlags {
		p.currentSectionName = strings.TrimSuffix(l.trimmed, ":")
	}
	p.section = sec
	p.inNameSection = sec == secName
	return true
}

// choiceBlock reads subcommand lists that are not rows: a grammar of the
// command word ("OBJECT := { address | link … }", possibly wrapped; only OBJECT
// and COMMAND, as "OPTIONS := {…}" lists options) and argparse's "{a,b,c}"
// choices with their rows.
func (p *helpParser) choiceBlock(l helpLine) bool {
	if p.inGrammar {
		p.addGrammar(l.trimmed)
		return true
	}
	if m := grammarRe.FindStringSubmatch(l.trimmed); m != nil && (m[1] == "OBJECT" || m[1] == "COMMAND") {
		p.addGrammar(m[2])
		return true
	}
	if strings.HasPrefix(l.trimmed, "{") && !p.inUsageBlock && !strings.Contains(l.trimmed, " ") {
		p.choiceIndent, p.choiceRowIndent = indentOf(l.raw), -1
		if strings.HasSuffix(l.trimmed, "}") {
			for _, n := range strings.Split(strings.Trim(l.trimmed, "{}"), ",") {
				p.addChoice(strings.TrimSpace(n), "")
			}
		}
		return true
	}
	if p.choiceIndent < 0 || l.trimmed == "" {
		return false
	}
	ind := indentOf(l.raw)
	if ind <= p.choiceIndent {
		p.choiceIndent = -1
		return false
	}
	if p.choiceRowIndent < 0 {
		p.choiceRowIndent = ind
	}
	if m := argparseRowRe.FindStringSubmatch(l.raw); m != nil && ind == p.choiceRowIndent {
		p.addChoice(m[1], strings.TrimSpace(m[2]))
	}
	return true
}

// commandGrid reads flush-left rows inside a commands section before they can
// reset it: openssl's multi-column grid of names, and kingpin's (#165)
// "check config [<flags>] <files>..." rows, each followed by an indented
// description, whose first word is the subcommand.
func (p *helpParser) commandGrid(l helpLine) bool {
	if p.section != secCommands || l.trimmed == "" || strings.HasPrefix(l.raw, " ") || strings.HasPrefix(l.raw, "\t") {
		p.lastGridRow = false
		return false
	}
	parts := strings.Fields(l.raw)
	// A grid's last row may hold a single entry (openssl "x509", #162).
	if (len(parts) >= 2 || (len(parts) == 1 && p.lastGridRow)) && allGridEntries(parts) {
		for _, name := range parts {
			if !p.seenSubs[name] && !skipSubcmdWords[name] {
				p.seenSubs[name] = true
				p.result.Subcommands = append(p.result.Subcommands, name)
			}
		}
		p.lastGridRow = true
		return true
	}
	p.lastGridRow = false
	if m := kingpinRowRe.FindStringSubmatch(l.raw); m != nil && nextLineIndented(p.lines, l.i) {
		p.addChoice(m[1], "")
		return true
	}
	return false
}

// unknownHeader handles a header the parser does not know ("Features:",
// "Dump List:", " FORMAT is one of the following:"): it introduces a list of
// values, not of commands (#154), so flags are read there but no subcommands.
// One that names commands ("These are common Git commands …:", stripe's
// "Available Operations:") stays free-form, and a multi-call binary's function
// list (busybox "Currently defined functions:") is a command list (#158).
func (p *helpParser) unknownHeader(l helpLine) bool {
	if l.trimmed == "" || !strings.HasSuffix(l.trimmed, ":") || len(l.raw)-len(strings.TrimLeft(l.raw, " ")) > 2 ||
		strings.HasPrefix(l.trimmed, "-") || p.inUsageBlock {
		return false
	}
	if strings.Contains(l.lower, "functions") {
		p.section = secCommands
		return true
	}
	if namesCommands(l.lower) {
		return false
	}
	p.section = secValues
	p.currentSectionName = ""
	p.inNameSection = false
	return true
}

// maybeResetSection: a flush-left line that is not a header ends the section,
// except a man-page footer like "TOOLNAME()".
func (p *helpParser) maybeResetSection(l helpLine) {
	if l.i > 4 && l.trimmed != "" && !strings.HasPrefix(l.raw, " ") &&
		!strings.HasPrefix(l.raw, "\t") && p.section != secNone &&
		!strings.HasSuffix(l.lower, ":") && detectSection(l.lower) == secNone &&
		!strings.HasSuffix(l.trimmed, "()") {
		p.section = secNone
		p.currentSectionName = ""
		p.inNameSection = false
	}
}

// captureDescription: first the man-page NAME line ("   git-clone - Short
// description"), else the first prose line near the top.
func (p *helpParser) captureDescription(l helpLine) {
	if p.result.Description != "" || l.trimmed == "" {
		return
	}
	if p.inNameSection {
		if m := nameSectionDescRe.FindStringSubmatch(l.raw); m != nil {
			p.result.Description = strings.TrimSpace(m[1])
		}
	}
	inWindow := l.i < 10+p.descSkipped || (p.usageEnd >= 0 && l.i <= p.usageEnd+2)
	if p.result.Description == "" && inWindow && p.isProse(l) {
		p.result.Description = l.trimmed
	}
}

// isProse: the line can be the description: not a header, flag, usage,
// banner, list row, argparse choices or grammar.
func (p *helpParser) isProse(l helpLine) bool {
	return !p.inUsageBlock &&
		!bannerRe.MatchString(l.trimmed) && !versionRowRe.MatchString(l.raw) &&
		!nameVersionRe.MatchString(l.trimmed) && !authorLineRe.MatchString(l.trimmed) &&
		!inNonProseList(p.section, l.raw) &&
		!strings.HasPrefix(l.trimmed, "{") && // argparse choices: "{init,run,…}"
		!strings.Contains(l.trimmed, ":= ") && strings.Count(l.trimmed, "|") < 3 && // grammar: "OBJECT := { a | b }"
		!introducesIndentedList(p.lines, l.i) && !listRowRe.MatchString(l.raw) &&
		!manpageHeaderRe.MatchString(l.trimmed) && // "GIT-CLONE(1)   Git Manual"
		!strings.HasPrefix(l.trimmed, "-") &&
		!strings.HasSuffix(l.trimmed, " -") && // empty man NAME body: "aws -"
		!isUsageLine(l.trimmed, p.selfName) &&
		!strings.HasSuffix(l.trimmed, ":") && // section header, not prose
		!strings.HasPrefix(l.lower, "usage") &&
		!strings.HasPrefix(l.lower, "use ") &&
		!strings.HasPrefix(l.lower, "name") &&
		!strings.HasPrefix(l.lower, "synopsis") &&
		l.lower != "description"
}

// collectUsageAndURL keeps usage / synopsis lines for positional parsing and
// the first docs URL anywhere in the text.
func (p *helpParser) collectUsageAndURL(l helpLine) {
	if strings.HasPrefix(l.lower, "usage:") || strings.HasPrefix(l.lower, "use:") ||
		l.lower == "synopsis" || (p.section == secUsage && l.trimmed != "") {
		p.usageLines = append(p.usageLines, l.raw)
	}
	if p.result.DocsURL == "" {
		if m := urlRe.FindString(l.raw); m != "" {
			p.result.DocsURL = m
		}
	}
}

func (p *helpParser) flagLine(l helpLine) {
	// AWS man-page flag style: "       --flag (type)"
	if m := awsFlagRe.FindStringSubmatch(l.raw); m != nil {
		f := models.Flag{Name: m[1], ValueType: m[2]}
		if m[2] == "boolean" {
			f.ValueType = "bool"
		}
		p.pendingFlag = &f
		return
	}
	f, ok := parseFlag(l.raw)
	if !ok {
		return
	}
	// kubectl style: "--flag=false:" then the description on the next line.
	if f.Description == "" && strings.HasSuffix(l.trimmed, ":") {
		p.pendingFlag = &f
		return
	}
	p.addFlag(p.withNextLineDescription(l, f))
}

// commandLine reads one row of a commands section; the first shape that
// matches wins.
func (p *helpParser) commandLine(l helpLine) {
	// Thor (#167): "  thor install NAME    # Install …": the row starts with
	// the program's own name, the command is the next word.
	if m := thorRowRe.FindStringSubmatch(l.raw); m != nil && m[1] == p.selfName {
		p.addChoice(m[2], strings.TrimSpace(m[3]))
		return
	}
	// Man-page command reference (#159): "bundle install(1) …", "git-clone(1)".
	if m := manRefRowRe.FindStringSubmatch(l.raw); m != nil && m[1] == p.selfName {
		p.addChoice(m[2], "")
		return
	}
	// Namespaced command (cleo, #166): "  cache clear   Clears …": the
	// namespace is the subcommand, its words below it.
	if m := namespaceRowRe.FindStringSubmatch(l.raw); m != nil {
		p.addChoice(m[1], "")
		return
	}
	// Bullet list (yarn 1, #157): "    - access".
	if m := bulletCmdRe.FindStringSubmatch(l.raw); m != nil {
		p.addChoice(m[1], "")
		return
	}
	// Tab-indented subcommand (Go toolchain style): "\tbug  start a bug report"
	if m := goTabSubcmdRe.FindStringSubmatch(l.raw); m != nil {
		p.addSub(m[1])
		return
	}
	if p.commaList(l) {
		return
	}
	// AWS man-page bullet: "       +o subcmd"
	if m := awsBulletRe.FindStringSubmatch(l.raw); m != nil {
		if !slices.Contains(p.bulletSubs, m[1]) {
			p.bulletSubs = append(p.bulletSubs, m[1])
		}
		p.addSub(m[1])
		return
	}
	if m := subcmdRe.FindStringSubmatch(l.raw); m != nil && p.addSub(m[1]) && m[2] != "" {
		p.setSubDesc(m[1], m[2])
	}
}

// commaList reads an npm-style "  access, adduser, audit, ..." row, only when
// every comma-delimited token is a bare command name, so description text like
// "by file names, stdin, resources" is not read as commands.
func (p *helpParser) commaList(l helpLine) bool {
	if !strings.Contains(l.raw, ",") || strings.HasPrefix(l.trimmed, "-") {
		return false
	}
	names, ok := commaCommandNames(l.raw)
	if !ok || len(names) == 0 {
		return false
	}
	for _, name := range names {
		p.addSub(name)
	}
	return true
}

func commaCommandNames(raw string) ([]string, bool) {
	var names []string
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		switch {
		case name == "":
		case validCmdNameRe.MatchString(name):
			names = append(names, name)
		case !strings.ContainsFunc(name, unicode.IsLetter):
			// symbol-named entries ("[", "[[" in busybox) are skipped
		default:
			return nil, false
		}
	}
	return names, true
}

// freeFormSubcommand reads git-style free-form lists: an indented word and a
// required description that reads as words (a version row "libavutil  59.  8.100"
// is not a command). A name ending in "-" is a fragment of "bundler-<command>".
func (p *helpParser) freeFormSubcommand(raw string) {
	m := subcmdRe.FindStringSubmatch(raw)
	if m == nil || m[2] == "" || startsWithDigit(m[2]) || strings.HasSuffix(m[1], "-") {
		return
	}
	p.addSub(m[1])
}

func (p *helpParser) addParsedFlag(l helpLine) {
	if f, ok := parseFlag(l.raw); ok {
		p.addFlag(p.withNextLineDescription(l, f))
	}
}

// proseFlagRe matches a paragraph that opens by naming flags, as "go help
// <cmd>" documents them (#66): "The -fmt flag reformats ...", "The
// -require=path@version and -droprequire=path flags ...".
var (
	proseFlagRe      = regexp.MustCompile(`^The (-{1,2}[A-Za-z][A-Za-z0-9_-]*(?:=\S+)?(?:(?:,? (?:and|or) |, )-{1,2}[A-Za-z][A-Za-z0-9_-]*(?:=\S+)?)*) flags?\b`)
	proseFlagTokenRe = regexp.MustCompile(`(-{1,2}[A-Za-z][A-Za-z0-9_-]*)(?:=(\S+))?`)
)

// proseFlags adds the flags a "The -x flag ..." paragraph opens with. The
// sentence must start the line, so a mention mid-paragraph ("Note that
// -require overrides ...") is never a flag.
func (p *helpParser) proseFlags(l helpLine) {
	m := proseFlagRe.FindStringSubmatch(l.raw)
	if m == nil {
		return
	}
	for _, t := range proseFlagTokenRe.FindAllStringSubmatch(m[1], -1) {
		f := models.Flag{Name: t[1], ValueType: strings.TrimRight(t[2], ",")}
		if f.ValueType == "" {
			f.ValueType = "bool"
		}
		if len(f.Name) == 2 {
			f.ShortName = f.Name[1:]
		}
		p.addFlag(f)
	}
}

// withNextLineDescription gives a flag row with no description of its own the
// next line's text when that line is indented deeper and is not another flag:
// Go's flag package ("  -cpuprofile string" over "    \twrite cpu profile")
// and "go help <cmd>" ("\t-race" over "\t\tenable data race detection.").
func (p *helpParser) withNextLineDescription(l helpLine, f models.Flag) models.Flag {
	if f.Description != "" || l.i+1 >= len(p.lines) {
		return f
	}
	next := p.lines[l.i+1]
	t := strings.TrimSpace(next)
	if t == "" || strings.HasPrefix(t, "-") || leadingWidth(next) <= leadingWidth(l.raw) {
		return f
	}
	f.Description = t
	return f
}

// leadingWidth is a line's indentation with a tab counted as eight columns.
func leadingWidth(s string) int {
	w := 0
	for _, r := range s {
		switch r {
		case ' ':
			w++
		case '\t':
			w += 8 - w%8
		default:
			return w
		}
	}
	return w
}

// addFlag appends a flag once, and to the named section being read, if any.
func (p *helpParser) addFlag(f models.Flag) {
	if p.seenFlags[f.Name] {
		return
	}
	p.seenFlags[f.Name] = true
	p.result.Flags = append(p.result.Flags, f)
	if p.currentSectionName == "" {
		return
	}
	n := len(p.result.Sections)
	if n == 0 || p.result.Sections[n-1].Name != p.currentSectionName {
		p.result.Sections = append(p.result.Sections, ParsedSection{Name: p.currentSectionName})
		n++
	}
	p.result.Sections[n-1].Flags = append(p.result.Sections[n-1].Flags, f)
}

// addSub records a subcommand once; it reports whether the name was new.
func (p *helpParser) addSub(name string) bool {
	if p.seenSubs[name] || skipSubcmdWords[name] || name == p.selfName {
		return false
	}
	p.seenSubs[name] = true
	p.result.Subcommands = append(p.result.Subcommands, name)
	return true
}

func (p *helpParser) setSubDesc(name, desc string) {
	if p.result.SubcommandDescs == nil {
		p.result.SubcommandDescs = map[string]string{}
	}
	p.result.SubcommandDescs[name] = strings.TrimSpace(desc)
}

// addChoice records a subcommand from a structured list, when it is a valid
// command name.
func (p *helpParser) addChoice(name, desc string) {
	if name == "" || !validCmdNameRe.MatchString(name) || !p.addSub(name) {
		return
	}
	if desc != "" {
		p.setSubDesc(name, desc)
	}
}

func (p *helpParser) addGrammar(text string) {
	body, closed := text, false
	if j := strings.Index(body, "}"); j >= 0 {
		body, closed = body[:j], true
	}
	for _, n := range strings.Split(body, "|") {
		p.addChoice(strings.TrimSpace(n), "")
	}
	p.inGrammar = !closed
}

func (p *helpParser) finish() ParsedHelp {
	r := &p.result
	if p.pendingFlag != nil {
		p.addFlag(*p.pendingFlag)
	}
	// Sections with fewer than 2 flags are noise.
	filtered := r.Sections[:0]
	for _, s := range r.Sections {
		if len(s.Flags) >= 2 {
			filtered = append(filtered, s)
		}
	}
	r.Sections = filtered
	for _, ul := range p.usageLines {
		r.Positionals = append(r.Positionals, parsePositionals(ul)...)
	}
	// Flags the synopsis names but no row lists ("go mod tidy" documents them
	// in prose) come after the listed ones, which carry descriptions, and
	// belong to no flag section.
	p.currentSectionName = ""
	for _, ul := range p.usageLines {
		for _, f := range usageLineFlags(ul) {
			p.addFlag(f)
		}
	}
	r.Description = strings.Join(strings.Fields(r.Description), " ")
	// A man-page bullet list ("o service") is authoritative: prose elsewhere in
	// the page ("... help each ...") must not add phantom subcommands.
	if len(p.bulletSubs) >= 3 {
		r.Subcommands = p.bulletSubs
	}
	r.Positionals = dedupePositionals(r.Positionals)
	return *r
}

// sectionRank orders sections when a header matches more than one keyword;
// lower wins.
var sectionRank = map[string]int{
	secCommands: 0, secFlags: 1, secUsage: 2, secName: 3, secAliases: 4, secExamples: 5, secDesc: 6,
}

// betterSection reports whether (sec, kw) should replace (bestSec, bestKw).
func betterSection(sec, kw, bestSec, bestKw string) bool {
	if sectionRank[sec] != sectionRank[bestSec] {
		return sectionRank[sec] < sectionRank[bestSec]
	}
	if len(kw) != len(bestKw) {
		return len(kw) > len(bestKw)
	}
	return kw < bestKw
}

// detectSection returns a section constant if the line is a recognized header.
// Handles "Title Case:" (cobra/click style) and "UPPER CASE" (man/AWS style).
func detectSection(lower string) string {
	// Strip optional trailing colon present in cobra/click style.
	candidate := strings.TrimSuffix(strings.TrimSpace(lower), ":")

	// Exact match handles most cases ("flags", "commands", "global options", etc.)
	if s, ok := sectionHeaders[candidate]; ok {
		return s
	}

	// Handle headers with parenthetical suffixes, e.g. openssl's
	// "Cipher commands (see the 'enc' command for more details)".
	// Strip everything from " (" onward and re-check.
	// The header ends at its closing parenthesis; a wrapped prose line such as
	// "name (like 'C++' or 'LaTeX') … Use" runs on past it and is not a header.
	if idx := strings.Index(candidate, " ("); idx > 0 && strings.HasSuffix(candidate, ")") {
		prefix := strings.TrimSpace(candidate[:idx])
		if s, ok := sectionHeaders[prefix]; ok {
			return s
		}
	}

	// Prefix/suffix match for compound headers like "general options:",
	// "required arguments:", "available services:", etc.
	// Only apply to short candidates (section headers are rarely > 30 chars);
	// this prevents false-positives on lines like "Usage: ls [OPTION]... [FILE]..."
	// A real header is plain words: it never starts with "-" (a flag line) or
	// uses column spacing (a "name   description" row). Without this guard
	// "-v, --verbose   verbose output" is taken as an "output" header and the
	// flag, and the rest of its section, is lost.
	if strings.HasPrefix(candidate, "-") || strings.Contains(candidate, "  ") {
		return secNone
	}
	if len(candidate) <= 30 {
		// A header can match several keywords ("environment commands" matches
		// both "commands" and "environment"). Choose deterministically: sections
		// that hold lists (commands, flags) beat free-text ones, then the longer
		// keyword, then the keyword itself, so the result never depends on map order.
		bestSec, bestKw := secNone, ""
		for kw, sec := range sectionHeaders {
			if !strings.HasPrefix(candidate, kw) && !strings.HasSuffix(candidate, kw) {
				continue
			}
			if bestSec == secNone || betterSection(sec, kw, bestSec, bestKw) {
				bestSec, bestKw = sec, kw
			}
		}
		return bestSec
	}
	return secNone
}

// parseFlag tries to parse a flag definition line.
func parseFlag(line string) (models.Flag, bool) {
	// Try long flag regex first
	if m := longFlagRe.FindStringSubmatch(line); m != nil {
		f := models.Flag{
			Name:      m[2],
			ShortName: strings.TrimLeft(m[1], "-"),
		}
		switch {
		case m[3] != "":
			f.ValueType = m[3]
		case m[4] != "":
			f.ValueType = m[4]
		case m[5] != "":
			f.ValueType = normalizeFlagDefault(m[5])
		default:
			f.ValueType = "bool"
		}
		f.Description = stripBuildMarker(m[6])
		return f, true
	}
	// Try short-only flag
	if m := shortOnlyFlagRe.FindStringSubmatch(line); m != nil {
		f := models.Flag{
			Name:      m[1],
			ShortName: strings.TrimLeft(m[1], "-"),
		}
		switch {
		case m[2] != "":
			f.ValueType = m[2]
		case m[3] != "":
			f.ValueType = m[3]
		default:
			f.ValueType = "bool"
		}
		f.Description = stripBuildMarker(m[4])
		return f, true
	}
	if f, ok := parseOptionSpec(line); ok {
		return f, true
	}
	return parseSingleDashOption(line)
}

// singleDashRe matches single-dash options (#156), as Go's flag package
// ("  -cpuprofile string" with the description on the next line, "  -d<TAB>…"),
// "go help <cmd>" ("<TAB>-C dir", #66) and mitchellh/cli ("  -chdir=DIR    Switch …")
// print them. After the name and an optional " type" or "=VALUE" comes a tab,
// a 2+ space gap or the end of the line, so "-v verbose output" (prose after
// one space) never matches.
var singleDashRe = regexp.MustCompile(`^(?: {2,4}|\t)(-[A-Za-z][A-Za-z0-9_.-]*)(?:[ =]([A-Za-z<][^\s]*))?(?:\t+| {2,}|$)(.*)$`)

func parseSingleDashOption(line string) (models.Flag, bool) {
	m := singleDashRe.FindStringSubmatch(line)
	if m == nil {
		return models.Flag{}, false
	}
	value := m[2]
	if strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") && strings.Count(value, "<") == 1 {
		value = value[1 : len(value)-1] // "<file>" -> "file"; "<var>[:<type>]=<value>" stays whole
	}
	f := models.Flag{Name: m[1], ValueType: value, Description: strings.TrimSpace(m[3])}
	if f.ValueType == "" {
		f.ValueType = "bool"
	}
	if len(m[1]) == 2 {
		f.ShortName = m[1][1:]
	}
	return f, true
}

// parseOptionSpec reads option lines the single-short-then-long regex does
// not cover (#161). The spec is everything before the first gap of two or more
// spaces; it is a comma-separated list of option names, each optionally
// followed by a value, in any order:
//
//	-C DIRECTORY, --directory=DIRECTORY   (value after the short name)
//	-n, --quiet, --silent                 (several aliases)
//	-i[SUFFIX], --in-place[=SUFFIX]       (attached and optional values)
//	--json, -j                            (long name first)
//	-pl,--projects <arg>                  (multi-letter short, no space)
//	-p, --plain...                        (repeatable)
//
// Every part must be an option and one must be a --long name, so prose never
// matches. Single-dash long options without one (gofmt, terraform) are #156.
func parseOptionSpec(line string) (models.Flag, bool) {
	t := strings.TrimLeft(line, " \t")
	if len(line)-len(t) > 12 || !strings.HasPrefix(t, "-") {
		return models.Flag{}, false
	}
	// kingpin negatable booleans: "--[no-]help" is the option --help.
	t = strings.ReplaceAll(t, "--[no-]", "--")
	spec, desc := t, ""
	if loc := specGapRe.FindStringIndex(t); loc != nil {
		spec, desc = t[:loc[0]], strings.TrimSpace(t[loc[1]:])
	}
	var long, short, value string
	for _, part := range splitTopLevelCommas(spec) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m := optionTokenRe.FindStringSubmatch(part)
		if m == nil {
			return models.Flag{}, false
		}
		v, ok := optionValue(m[3])
		if !ok {
			return models.Flag{}, false
		}
		if value == "" {
			value = v
		}
		switch {
		case m[1] == "--":
			if long == "" {
				long = "--" + m[2]
			}
		case short == "":
			short = m[2]
		}
	}
	if long == "" {
		return models.Flag{}, false
	}
	if value == "" {
		value = "bool"
	}
	return models.Flag{Name: long, ShortName: short, ValueType: value, Description: stripBuildMarker(desc)}, true
}

var (
	specGapRe     = regexp.MustCompile(`\s{2,}|\t`)
	optionTokenRe = regexp.MustCompile(`^(--?)([A-Za-z0-9][A-Za-z0-9_-]*)(.*)$`)
	valueWordRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_:.-]*$`)
)

// optionValue normalizes what follows an option name: " FILE", "=FILE",
// "[=N]", "[N]", " <arg>", "=..." or nothing. A trailing "..." alone marks a
// repeatable option, not a value. ok is false when the text is not a value.
func optionValue(rest string) (string, bool) {
	// A repeatable list suffix ("={a|b}[,...]") says nothing about the value.
	r := strings.TrimSuffix(strings.TrimSpace(rest), "[,...]")
	switch {
	case r == "" || r == "...":
		return "", true
	case strings.HasPrefix(r, "[=") && strings.HasSuffix(r, "]"):
		r = r[2 : len(r)-1]
	case strings.HasPrefix(r, "[") && strings.HasSuffix(r, "]"):
		r = r[1 : len(r)-1]
	case strings.HasPrefix(r, "="):
		r = r[1:]
	}
	r = strings.TrimSuffix(strings.TrimPrefix(r, "<"), ">")
	if r == "" || strings.ContainsAny(r, " \t") {
		return "", false
	}
	if r == "..." || strings.HasPrefix(r, "[") || valueWordRe.MatchString(r) ||
		(strings.HasPrefix(r, "{") && strings.HasSuffix(r, "}")) { // choices: {jit|rubyopt|...}
		return r, true
	}
	return "", false
}

// normalizeFlagDefault turns a kubectl-style default shown in place of a type
// (=false, =500, =”, =[]) into a value type; real type names pass through.
func normalizeFlagDefault(v string) string {
	switch {
	case v == "true" || v == "false":
		return "bool"
	case v == "[]":
		return "list"
	case strings.HasPrefix(v, "'"):
		return "string"
	case v != "" && v[0] >= '0' && v[0] <= '9':
		return "int"
	}
	return v
}

// positionalPlaceholders are all-caps words in usage lines that represent
// option/flag slots, not real positional arguments.
var positionalPlaceholders = map[string]bool{
	"OPTION": true, "OPTIONS": true, "OPTS": true, "OPT": true,
	"FLAG": true, "FLAGS": true, "ARG": true, "ARGS": true,
	"ARGUMENTS": true, "PARAMS": true, "PARAMETERS": true,
	"SHORT-OPTION": true, "LONG-OPTION": true,
	// cobra/click/common framework placeholders (lowercased inputs are uppercased before lookup)
	"COMMAND": true, "COMMANDS": true,
	"SUBCOMMAND": true, "SUBCOMMANDS": true,
	"CMD": true, "CMDS": true,
	"RESOURCE": true, "NAME": true, "TYPE": true, "OBJECT": true,
}

// bracketOptionRe matches bracket-enclosed option patterns that should not be
// treated as positional arguments: [--long-opt...] and [-X <value>] forms.
var bracketOptionRe = regexp.MustCompile(`\[-[^\]]+\]`)

// usageOptionRe matches one option alone in a usage-line bracket, with an
// optional value: "[-v]", "[--verbose]", "[-o output]", "[-go=version]",
// "[--config <file>]". Brackets holding alternatives ("[-v | --version]")
// or nested optional values ("[--exec-path[=<path>]]") do not match.
var usageOptionRe = regexp.MustCompile(`\[(--?[A-Za-z][A-Za-z0-9_-]*)(?:[ =](<[A-Za-z][^<>\[\]|]*>|[A-Za-z][A-Za-z0-9_.,-]*))?\]`)

// wordOptionRe matches a single-dash word option, which is lowercase
// ("-json", "-testcache"); "[-Olevel]" (GNU find) is -O with a value.
var wordOptionRe = regexp.MustCompile(`^-[a-z][a-z0-9-]+$`)

// usageLineFlags reads the options a usage line names one per bracket (#66):
// "usage: go mod tidy [-e] [-v] [-diff] [-go=version]". BSD tools cluster
// single-letter options in one bracket ("[-pv]" is -p and -v), so a
// multi-letter single-dash option without a value counts only when the same
// line also lists a single letter alone ("[-e]"), which a clustering tool
// would have merged.
func usageLineFlags(line string) []models.Flag {
	ms := usageOptionRe.FindAllStringSubmatch(line, -1)
	lonePerLetter := false
	for _, m := range ms {
		lonePerLetter = lonePerLetter || (len(m[1]) == 2 && m[2] == "")
	}
	var flags []models.Flag
	for _, m := range ms {
		if f, ok := usageOptionFlag(m[1], strings.Trim(m[2], "<>"), lonePerLetter); ok {
			flags = append(flags, f)
		}
	}
	return flags
}

// usageOptionFlag turns one bracketed usage option into a flag, or reports
// false for a possible BSD letter cluster (see usageLineFlags).
func usageOptionFlag(name, value string, lonePerLetter bool) (models.Flag, bool) {
	long := strings.HasPrefix(name, "--")
	word := len(name) > 2 && !long
	if word && value == "" && (!lonePerLetter || !wordOptionRe.MatchString(name)) {
		return models.Flag{}, false
	}
	f := models.Flag{Name: name, ValueType: value}
	if f.ValueType == "" {
		f.ValueType = "bool"
	}
	if len(name) == 2 {
		f.ShortName = name[1:]
	}
	return f, true
}

// parsePositionals extracts positional args from a usage line.
func parsePositionals(line string) []models.Positional {
	var result []models.Positional
	seen := map[string]bool{}
	// Skip the "usage:" prefix for matching
	searchLine := line
	if idx := strings.Index(strings.ToLower(line), "usage:"); idx >= 0 {
		searchLine = line[idx+6:]
	}
	// Strip [--option] bracket patterns to avoid false positives from
	// lines like "git [--exec-path[=<path>]]".
	searchLine = bracketOptionRe.ReplaceAllString(searchLine, "")
	for _, m := range reqArgRe.FindAllStringSubmatch(searchLine, -1) {
		name := m[1]
		canonical := strings.TrimRight(strings.ToUpper(name), ".+")
		if !seen[name] && !strings.ContainsAny(name, "= ") && !positionalPlaceholders[canonical] {
			seen[name] = true
			result = append(result, models.Positional{Name: name, Required: true})
		}
	}
	for _, m := range optArgRe.FindAllStringSubmatch(searchLine, -1) {
		name := m[1]
		canonical := strings.TrimRight(strings.ToUpper(name), ".+")
		if !seen[name] && !strings.ContainsAny(name, "= ") && !positionalPlaceholders[canonical] {
			seen[name] = true
			result = append(result, models.Positional{Name: name, Required: false})
		}
	}
	return result
}

func dedupePositionals(ps []models.Positional) []models.Positional {
	seen := map[string]bool{}
	var out []models.Positional
	for _, p := range ps {
		if !seen[p.Name] {
			seen[p.Name] = true
			out = append(out, p)
		}
	}
	return out
}

// agentHintRe matches machine hints some CLIs print when they detect an AI
// agent (e.g. <claude-code-hint v="1" ... />); they are not help text.
var agentHintRe = regexp.MustCompile(`(?m)[ \t]*<[a-z-]+-hint\b[^>]*/>[ \t]*\r?\n?`)

// SanitizeHelp strips terminal escapes, man-page overstrike sequences and
// agent hint tags from raw help output, then trims surrounding whitespace.
func SanitizeHelp(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = manpageBoldRe.ReplaceAllString(s, "")
	s = agentHintRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// isUsageLine reports whether line is a bare usage synopsis such as
// "stripe charges <operation> [parameters...]" rather than prose.
// listRowRe matches an indented "name   text" row of a command list. Needs 3+
// spaces so double-spaced justified man-page prose is not mistaken for a row.
var listRowRe = regexp.MustCompile(`^[ \t]+[a-z][a-z0-9_-]*[ \t]{3,}\S`)

// introducesIndentedList reports whether line i is a heading for the indented
// block right below it (git's "start a working area (...)" followed by its
// commands), which makes it a section title rather than a description.
func introducesIndentedList(lines []string, i int) bool {
	if i+1 >= len(lines) || lines[i+1] == "" {
		return false
	}
	next := lines[i+1]
	return (next[0] == ' ' || next[0] == '\t') && strings.TrimSpace(next) != "" &&
		(lines[i][0] != ' ' && lines[i][0] != '\t')
}

func isUsageLine(line, cliName string) bool {
	return strings.HasPrefix(line, cliName+" ") && strings.ContainsAny(line, "<[")
}

// startsWithDigit reports whether s begins with a digit, as version numbers do.
func startsWithDigit(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && s[0] >= '0' && s[0] <= '9'
}

// commandWords are the words CLIs use for their subcommands in list headers.
var commandWords = []string{"command", "operation", "action", "topic", "verb", "service", "function"}

// namesCommands reports whether a header line introduces a list of commands.
func namesCommands(lower string) bool {
	for _, w := range commandWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// splitTopLevelCommas splits an option spec on commas outside [] and {}:
// "--disable={jit|rubyopt}[,...]" is one option, "-n, --quiet" two.
func splitTopLevelCommas(spec string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range spec {
		switch r {
		case '[', '{':
			depth++
		case ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, spec[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, spec[start:])
}

// bannerRe matches the banner lines some CLIs print before their help: name
// and version, copyright, licence, build details. They are not a description.
var bannerRe = regexp.MustCompile(`(?i)\bversion\s+v?\d|\bv\d+\.\d+|copyright|\(c\)\s|licen[cs]ed under|built with|^configuration:|^web site:|no warranty|free software|redistribute it|general public licen|source distribution`)

// nameVersionRe matches clap's first help line, a bare "name version"
// ("fakeclap-add 1.0", "ripgrep 14.1.0").
var nameVersionRe = regexp.MustCompile(`^[A-Za-z][\w.+-]*\s+v?\d+(\.\d+)+([-+][\w.+-]*)?$`)

// authorLineRe matches clap's author line ("Andrew Gallant <jam@example.com>").
var authorLineRe = regexp.MustCompile(`^[^<>]*<[^<>\s]+@[^<>\s]+>$`)

// versionRowRe matches library version rows ("  libavutil      59.  8.100 / …").
var versionRowRe = regexp.MustCompile(`^\s+[A-Za-z][\w.-]*\s{2,}\d+\.`)

// inNonProseList reports whether rawLine is an entry of a list that is never
// a description: a value list under an unknown header (rsync "Capabilities:"),
// or an alias or example ("ALIASES\n  gh variable remove").
func inNonProseList(section, rawLine string) bool {
	if rawLine == "" || rawLine[0] != ' ' {
		return false
	}
	return section == secValues || section == secAliases || section == secExamples
}

// argparseRowRe is a subparser row: "    install             Install a package".
var argparseRowRe = regexp.MustCompile(`^\s+([a-z][a-z0-9_-]*)(?:\s{2,}(\S.*))?$`)

var (
	// kingpinRowRe is a kingpin command row: lowercase words, then arguments.
	kingpinRowRe = regexp.MustCompile(`^([a-z][a-z0-9_-]*)(?: [a-z][a-z0-9_-]*)*(?: [\[<-].*)?$`)
	// thorRowRe is a thor row: "  <program> <command> [ARGS]  # description".
	thorRowRe = regexp.MustCompile(`^\s+(\S+)\s+([a-z][a-z0-9_:-]*)\b[^#]*#\s*(.*)$`)
	// manRefRowRe is a man-page command reference: "bundle install(1) …".
	manRefRowRe = regexp.MustCompile(`^\s+(\S+?)[ -]([a-z][a-z0-9_-]*)\(\d+[a-z]*\)`)
	// cleoListHelpRe recognizes the help of a cleo app's "list" command:
	// "Usage:" followed by "  list [options] …" (what "poetry --help" prints).
	cleoListHelpRe = regexp.MustCompile(`(?m)^Usage:\s*\n\s+list(?:\s|$)`)
	// grammarRe is a grammar rule: "where  OBJECT := { address | …".
	grammarRe = regexp.MustCompile(`^(?:where\s+)?([A-Z][A-Z_]*)\s*:=\s*\{(.*)$`)
	// namespaceRowRe is a namespaced command row: "  cache clear        Clears …".
	namespaceRowRe = regexp.MustCompile(`^\s+([a-z][a-z0-9_-]*) [a-z][a-z0-9_-]*\s{2,}\S`)
	// bulletCmdRe is a bulleted command: "    - access".
	bulletCmdRe = regexp.MustCompile(`^\s+- ([a-z][a-z0-9_-]*)\s*$`)
)

// nextLineIndented reports whether the next non-empty line after i is indented.
func nextLineIndented(lines []string, i int) bool {
	for _, l := range lines[i+1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		return l[0] == ' ' || l[0] == '\t'
	}
	return false
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

// Package discovery provides strategies for discovering CLI command hierarchies.
package discovery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
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
	MaxDepth      int
	Timeout       time.Duration
	StubThreshold int // max subcommands before creating stubs instead of eager discovery
	// ChildrenAsStubs lists every subcommand as a name-only stub instead of
	// discovering it, so a caller can crawl the tree one node at a time.
	ChildrenAsStubs bool

	form atomic.Int32 // helpForm the root answered with; children try it first
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
			node.Description = manOneLiner(ctx, cliName)
		}
		models.MarkInheritedFlags(node)
	}
	return node, err
}

// manOneLiner returns the NAME one-liner from cliName's man page, or "" when
// there is no man page. Used when --help is only a usage synopsis.
func manOneLiner(ctx context.Context, cliName string) string {
	plain, err := NewManDiscoverer().fetchManPage(ctx, cliName)
	if err != nil || plain == "" {
		return ""
	}
	return ParseHelpOutput(plain).Description
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

	helpText, err := knownHelp, error(nil)
	if helpText == "" {
		helpText, err = h.runHelp(ctx, cliName, args)
	}
	if err != nil || helpText == "" {
		node.DiscoveryErr = fmt.Sprintf("could not get help: %v", err)
		return node, nil
	}
	node.HelpText = helpText

	parsed := ParseHelpOutputFor(helpText, cliName)
	node.Description = parsed.Description
	node.Flags = parsed.Flags
	node.Positionals = parsed.Positionals

	// No subcommands in --help at the root: some CLIs (tmux) list their
	// commands through "list-commands" instead.
	if depth == 0 && len(args) == 0 && len(parsed.Subcommands) == 0 {
		if kids := h.commandListChildren(ctx, cliName, helpText, fullPath); len(kids) > 0 {
			node.Children = kids
			return node, nil
		}
	}

	if depth < h.MaxDepth && len(parsed.Subcommands) > 0 {
		// When a command has a very large number of subcommands (e.g. aws
		// with 200+ services), eagerly running --help on every child would
		// take minutes. Instead, create lightweight stub nodes that carry
		// only the name and path. They can be expanded on demand by the
		// caller (TUI toggle, or re-running discovery at a deeper depth).
		threshold := h.StubThreshold
		if threshold <= 0 {
			threshold = 50
		}
		if h.ChildrenAsStubs || len(parsed.Subcommands) > threshold {
			for _, sub := range parsed.Subcommands {
				subFull := append(append([]string{}, fullPath...), sub)
				node.Children = append(node.Children, &models.Node{
					Name:        sub,
					FullPath:    subFull,
					Discovered:  false,
					Stub:        true,
					Description: parsed.SubcommandDescs[sub],
				})
			}
			return node, nil
		}

		const maxWorkers = 8
		sem := make(chan struct{}, maxWorkers)
		type result struct {
			idx   int
			child *models.Node
		}
		results := make([]result, len(parsed.Subcommands))
		var wg sync.WaitGroup
		for i, sub := range parsed.Subcommands {
			wg.Add(1)
			go func(i int, sub string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				subArgs := append(append([]string{}, args...), sub)
				subFull := append(append([]string{}, fullPath...), sub)
				childHelp, err := h.runHelp(ctx, cliName, subArgs)
				if err != nil || childHelp == "" {
					results[i] = result{i, &models.Node{
						Name:         sub,
						FullPath:     subFull,
						Discovered:   true,
						DiscoveryErr: fmt.Sprintf("could not get help: %v", err),
					}}
					return
				}
				var child *models.Node
				if childHelp == helpText {
					childParsed := ParseHelpOutputFor(childHelp, cliName)
					child = &models.Node{
						Name:        sub,
						FullPath:    subFull,
						Discovered:  true,
						HelpText:    childHelp,
						Description: childParsed.Description,
						Flags:       childParsed.Flags,
						Positionals: childParsed.Positionals,
					}
				} else {
					var cerr error
					child, cerr = h.discoverWithHelp(ctx, cliName, subArgs, depth+1, childHelp)
					if cerr != nil {
						child = &models.Node{Name: sub, FullPath: subFull}
					}
				}
				results[i] = result{i, child}
			}(i, sub)
		}
		wg.Wait()
		for _, r := range results {
			if r.child != nil && r.child.Description == "" {
				r.child.Description = parsed.SubcommandDescs[r.child.Name]
			}
			if r.child != nil {
				node.Children = append(node.Children, r.child)
			}
		}
	}
	return node, nil
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

// CheckAvailable returns an error if cliName cannot be resolved to an
// executable. Call this before starting discovery to give the user a clear
// error message instead of a cryptic "no help output" stub node.
func CheckAvailable(cliName string) error {
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

// commandTimeout is the time allowed for one help command.
func (h *HelpDiscoverer) commandTimeout() time.Duration {
	if h.Timeout <= 0 {
		return 5 * time.Second
	}
	return h.Timeout
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
	// timedOut is set when a form ran out of its own time. That is no answer:
	// the CLI may be overloaded, not refusing. Positional forms pass "help" as
	// an argument a subcommand may act on ("npm prune help" prunes, #176), so
	// they only ever follow flag forms that answered and refused.
	var timedOut atomic.Bool
	if s, f := h.raceForms(ctx, resolved, args, flags, &timedOut); s != "" {
		h.form.CompareAndSwap(int32(formUnknown), int32(f))
		return s, nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if timedOut.Load() {
		return "", fmt.Errorf("help for %s timed out", cliName)
	}

	if s := h.tryForm(ctx, resolved, args, formSuffix, &timedOut); s != "" {
		return s, nil
	}
	if len(args) > 0 {
		if s := h.tryForm(ctx, resolved, args, formPrefix, &timedOut); s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("no help output from %s", cliName)
}

// raceForms runs flag forms concurrently and returns the output of the
// highest-priority one that produced help. It returns as soon as that is
// decided, cancelling (killing) the forms still running. Never pass it a
// positional form: see runHelp.
func (h *HelpDiscoverer) raceForms(ctx context.Context, resolved string, args []string, forms []helpForm, timedOut *atomic.Bool) (string, helpForm) {
	for _, f := range forms {
		if f != formFlag && f != formShort {
			panic("discovery: only flag help forms may be raced")
		}
	}
	if len(forms) == 0 {
		return "", formUnknown
	}
	if len(forms) == 1 {
		return h.tryForm(ctx, resolved, args, forms[0], timedOut), forms[0]
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
			ch <- h.tryForm(rctx, resolved, args, f, timedOut)
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

// hedgeDelay is how long the preferred flag form runs alone before the other
// is started alongside it.
const hedgeDelay = 250 * time.Millisecond

// tryForm runs one help form and returns its sanitized output, or "" when the
// command printed nothing or an error instead of help.
func (h *HelpDiscoverer) tryForm(ctx context.Context, resolved string, args []string, f helpForm, timedOut *atomic.Bool) string {
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
	s := h.runOne(ctx, resolved, cmdArgs, timedOut)
	if s == "" || isErrorOutput(s) {
		return ""
	}
	// Truncated help (curl) points at a complete form; prefer it.
	if f == formFlag && truncatedHelpRe.MatchString(s) {
		if all := h.runOne(ctx, resolved, append(append([]string{}, args...), "--help", "all"), timedOut); all != "" {
			return all
		}
	}
	return s
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
	cctx, cancel := context.WithTimeout(ctx, h.commandTimeout())
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
	var result ParsedHelp
	lines := strings.Split(text, "\n")
	section := secNone
	var bulletSubs []string
	seenSubs := map[string]bool{}
	seenFlags := map[string]bool{}
	usageLines := []string{}

	// inNameSection is set when we detect the man-page NAME section; used to
	// extract a clean short description from "   git-clone - Short desc" lines.
	inNameSection := false

	// Section-grouping state: track the current flag-section header name so we
	// can build ParsedSection children for tools like Godot.
	currentSectionName := ""
	sectionFlagCount := map[string]int{} // section name → flag count added so far

	// pendingFlag holds a partially-parsed AWS-style flag whose description is
	// on the next non-empty line ("--flag (type)" followed by "   description").
	var pendingFlag *models.Flag

	// addFlag appends a flag to result.Flags and (if we are in a named section)
	// also to the corresponding ParsedSection entry.
	addFlag := func(f models.Flag) {
		if seenFlags[f.Name] {
			return
		}
		seenFlags[f.Name] = true
		result.Flags = append(result.Flags, f)
		if currentSectionName != "" {
			n := len(result.Sections)
			if n == 0 || result.Sections[n-1].Name != currentSectionName {
				result.Sections = append(result.Sections, ParsedSection{Name: currentSectionName})
				n++
			}
			result.Sections[n-1].Flags = append(result.Sections[n-1].Flags, f)
			sectionFlagCount[currentSectionName]++
		}
	}

	// inUsageBlock is true from a "usage:" line through its indented
	// continuation lines, which are synopsis text and never a description.
	inUsageBlock := false
	// descSkipped counts lines that cannot be the description and should not
	// use up the window it is looked for in: banners, and the value lists
	// under unknown headers (rsync's "Capabilities:"). usageEnd is where a
	// usage block ended: the paragraph after a long wrapped usage (argparse)
	// is the description wherever it falls.
	descSkipped, usageEnd := 0, -1
	// argparse subparser block (#81): a "{a,b,c…}" line, then one row per
	// subcommand at the first row's indent; deeper rows continue a description.
	choiceIndent, choiceRowIndent := -1, -1
	addChoice := func(name, desc string) {
		if name == "" || seenSubs[name] || skipSubcmdWords[name] || name == selfName || !validCmdNameRe.MatchString(name) {
			return
		}
		seenSubs[name] = true
		result.Subcommands = append(result.Subcommands, name)
		if desc != "" {
			if result.SubcommandDescs == nil {
				result.SubcommandDescs = map[string]string{}
			}
			result.SubcommandDescs[name] = desc
		}
	}
	for i, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "usage:"):
			inUsageBlock = true
		case trimmed == "" || (rawLine[0] != ' ' && rawLine[0] != '\t'):
			if inUsageBlock {
				usageEnd = i
			}
			inUsageBlock = false
		}
		if result.Description == "" && trimmed != "" &&
			(bannerRe.MatchString(trimmed) || (section == secValues && rawLine[0] == ' ') || versionRowRe.MatchString(rawLine)) {
			descSkipped++
		}

		// Flush a pending AWS-style flag when we encounter a non-empty line.
		if pendingFlag != nil && trimmed != "" {
			// If the next line is another flag, don't use it as a description.
			if !strings.HasPrefix(trimmed, "--") && awsFlagRe.FindString(rawLine) == "" {
				pendingFlag.Description = trimmed
			}
			if !seenFlags[pendingFlag.Name] {
				addFlag(*pendingFlag)
			}
			pendingFlag = nil
		}

		// Detect section header: "Flags:", "Available Commands:", "GLOBAL OPTIONS", etc.
		// Bullet list items ("o appconfig") are never headers, even when the
		// name ends in a header keyword such as "config" or "cache".
		if sec := detectSection(lower); sec != secNone && !awsBulletRe.MatchString(rawLine) {
			// For named flag-group sections (e.g. "General options:", "Debug options:"),
			// remember the human-readable name so flags get grouped under it.
			if sec == secFlags {
				// Capture the raw header text (without trailing colon) as the section name.
				currentSectionName = strings.TrimSuffix(trimmed, ":")
			} else {
				currentSectionName = ""
			}
			section = sec
			inNameSection = (sec == secName)
			continue
		}
		if strings.HasPrefix(trimmed, "{") && !inUsageBlock && !strings.Contains(trimmed, " ") {
			choiceIndent, choiceRowIndent = indentOf(rawLine), -1
			if strings.HasSuffix(trimmed, "}") {
				for _, n := range strings.Split(strings.Trim(trimmed, "{}"), ",") {
					addChoice(strings.TrimSpace(n), "")
				}
			}
			continue
		}
		if choiceIndent >= 0 && trimmed != "" {
			if ind := indentOf(rawLine); ind > choiceIndent {
				if choiceRowIndent < 0 {
					choiceRowIndent = ind
				}
				if m := argparseRowRe.FindStringSubmatch(rawLine); m != nil && ind == choiceRowIndent {
					addChoice(m[1], strings.TrimSpace(m[2]))
				}
				continue
			}
			choiceIndent = -1
		}
		// Multi-column command grid (openssl style): when already inside a commands
		// section, non-indented lines may be whitespace-separated columns of command
		// names. Handle these BEFORE the section-reset check so they don't incorrectly
		// reset the section.
		if section == secCommands && trimmed != "" &&
			!strings.HasPrefix(rawLine, " ") && !strings.HasPrefix(rawLine, "\t") {
			parts := strings.Fields(rawLine)
			if len(parts) >= 2 && allGridEntries(parts) {
				for _, p := range parts {
					if !seenSubs[p] && !skipSubcmdWords[p] {
						seenSubs[p] = true
						result.Subcommands = append(result.Subcommands, p)
					}
				}
				continue
			}
		}
		// A header the parser does not know ("Features:", "Dump List:",
		// " FORMAT is one of the following:") introduces a list of values, not
		// of commands (#154): read flags there, but no subcommands. One that
		// names commands ("These are common Git commands …:", stripe's
		// "Available Operations:") stays free-form.
		if trimmed != "" && strings.HasSuffix(trimmed, ":") && len(rawLine)-len(strings.TrimLeft(rawLine, " ")) <= 2 &&
			!strings.HasPrefix(trimmed, "-") && !namesCommands(lower) && !inUsageBlock {
			section = secValues
			currentSectionName = ""
			inNameSection = false
			continue
		}
		// A non-indented non-empty line resets the section — EXCEPT for
		// man-page headers that are themselves section detectors (handled above).
		// We only reset when it's clearly not a section header.
		if i > 4 && trimmed != "" && !strings.HasPrefix(rawLine, " ") &&
			!strings.HasPrefix(rawLine, "\t") && section != secNone &&
			!strings.HasSuffix(lower, ":") &&
			detectSection(lower) == secNone {
			// Man-page footers like "TOOLNAME()" at end of page shouldn't reset.
			if !strings.HasSuffix(trimmed, "()") {
				section = secNone
				currentSectionName = ""
				inNameSection = false
			}
		}

		// Capture description:
		// Priority 1 — man-page NAME section: "   git-clone - Short description"
		if inNameSection && result.Description == "" && trimmed != "" {
			if m := nameSectionDescRe.FindStringSubmatch(rawLine); m != nil {
				result.Description = strings.TrimSpace(m[1])
			}
		}
		// Priority 2 — first non-empty, non-header, non-flag line in first 10 lines.
		// Skip man-page header lines like "GIT-CLONE(1)   Git Manual".
		if result.Description == "" && (i < 10+descSkipped || (usageEnd >= 0 && i <= usageEnd+2)) && trimmed != "" && !inUsageBlock &&
			!bannerRe.MatchString(trimmed) && !versionRowRe.MatchString(rawLine) &&
			!inNonProseList(section, rawLine) &&
			!strings.HasPrefix(trimmed, "{") && // argparse choices: "{init,run,…}"
			!strings.Contains(trimmed, ":= ") && strings.Count(trimmed, "|") < 3 && // grammar: "OBJECT := { a | b }"
			!introducesIndentedList(lines, i) && !listRowRe.MatchString(rawLine) &&
			!manpageHeaderRe.MatchString(trimmed) &&
			!strings.HasPrefix(trimmed, "-") &&
			!strings.HasSuffix(trimmed, " -") && // empty man NAME body: "aws -"
			!isUsageLine(trimmed, selfName) &&
			!strings.HasSuffix(trimmed, ":") && // section header, not prose
			!strings.HasPrefix(lower, "usage") &&
			!strings.HasPrefix(lower, "use ") &&
			!strings.HasPrefix(lower, "name") &&
			!strings.HasPrefix(lower, "synopsis") &&
			lower != "name" && lower != "synopsis" && lower != "description" {
			result.Description = trimmed
		}

		// Collect all usage / synopsis lines for positional parsing.
		if strings.HasPrefix(lower, "usage:") || strings.HasPrefix(lower, "use:") ||
			lower == "synopsis" ||
			(section == secUsage && trimmed != "") {
			usageLines = append(usageLines, rawLine)
		}

		// Detect docs URL anywhere in text.
		if result.DocsURL == "" {
			if m := urlRe.FindString(rawLine); m != "" {
				result.DocsURL = m
			}
		}

		switch section {
		case secFlags:
			// AWS man-page flag style: "       --flag (type)"
			if m := awsFlagRe.FindStringSubmatch(rawLine); m != nil {
				f := models.Flag{Name: m[1], ValueType: m[2]}
				if m[2] == "boolean" {
					f.ValueType = "bool"
				}
				pendingFlag = &f
				continue
			}
			if f, ok := parseFlag(rawLine); ok {
				// kubectl style: "--flag=false:" then the description on the next line.
				if f.Description == "" && strings.HasSuffix(trimmed, ":") {
					pendingFlag = &f
					continue
				}
				addFlag(f)
			}
		case secCommands:
			// Tab-indented subcommand (Go toolchain style): "\tbug  start a bug report"
			if m := goTabSubcmdRe.FindStringSubmatch(rawLine); m != nil {
				name := m[1]
				if !seenSubs[name] && !skipSubcmdWords[name] && name != selfName {
					seenSubs[name] = true
					result.Subcommands = append(result.Subcommands, name)
				}
				continue
			}
			// Comma-separated command list (npm style): "  access, adduser, audit, ..."
			// Only fire when every comma-delimited token is a valid bare command name.
			// This prevents matching description text like "by file names, stdin, resources".
			if strings.Contains(rawLine, ",") && !strings.HasPrefix(trimmed, "-") {
				parts := strings.Split(rawLine, ",")
				var names []string
				allValid := true
				for _, part := range parts {
					name := strings.TrimSpace(part)
					if name == "" {
						continue
					}
					if validCmdNameRe.MatchString(name) {
						names = append(names, name)
					} else {
						allValid = false
						break
					}
				}
				if allValid && len(names) > 0 {
					for _, name := range names {
						if !seenSubs[name] && !skipSubcmdWords[name] && name != selfName {
							seenSubs[name] = true
							result.Subcommands = append(result.Subcommands, name)
						}
					}
					continue
				}
			}
			// AWS man-page bullet: "       +o subcmd"
			if m := awsBulletRe.FindStringSubmatch(rawLine); m != nil {
				name := m[1]
				bulletSubs = append(bulletSubs, name)
				if !seenSubs[name] && !skipSubcmdWords[name] && name != selfName {
					seenSubs[name] = true
					result.Subcommands = append(result.Subcommands, name)
				}
				continue
			}
			if m := subcmdRe.FindStringSubmatch(rawLine); m != nil {
				name := m[1]
				if !seenSubs[name] && !skipSubcmdWords[name] && name != selfName {
					seenSubs[name] = true
					result.Subcommands = append(result.Subcommands, name)
					if m[2] != "" {
						if result.SubcommandDescs == nil {
							result.SubcommandDescs = map[string]string{}
						}
						result.SubcommandDescs[name] = strings.TrimSpace(m[2])
					}
				}
			}
		case secExamples, secAliases, secDesc, secValues:
			// These sections contain narrative text, examples, or aliases —
			// not subcommand lists. Parse flags only (e.g. example usage may
			// reference flags we want to surface), but never infer subcommands.
			if f, ok := parseFlag(rawLine); ok {
				addFlag(f)
			}
		case secNone, secUsage:
			// Outside named sections: pick up flags and subcommands with stricter checks.
			if f, ok := parseFlag(rawLine); ok {
				addFlag(f)
			}
			// Git-style free-form subcommand lists: indented word + required description.
			// The description reads as words: a version row ("libavutil  59.  8.100")
			// is not a command.
			if m2 := subcmdRe.FindStringSubmatch(rawLine); m2 != nil && m2[2] != "" && !startsWithDigit(m2[2]) {
				name := m2[1]
				if !seenSubs[name] && !skipSubcmdWords[name] && name != selfName {
					seenSubs[name] = true
					result.Subcommands = append(result.Subcommands, name)
				}
			}
		}
	}

	// Flush any trailing pending flag.
	if pendingFlag != nil {
		addFlag(*pendingFlag)
	}

	// Drop sections that have fewer than 2 flags — they are noise.
	{
		filtered := result.Sections[:0]
		for _, s := range result.Sections {
			if len(s.Flags) >= 2 {
				filtered = append(filtered, s)
			}
		}
		result.Sections = filtered
		_ = sectionFlagCount // used implicitly via addFlag
	}

	// Parse positionals from all collected usage lines
	for _, ul := range usageLines {
		result.Positionals = append(result.Positionals, parsePositionals(ul)...)
	}
	result.Description = strings.Join(strings.Fields(result.Description), " ")
	// A man-page bullet list ("o service") is authoritative: prose elsewhere in
	// the page ("... help each ...") must not add phantom subcommands.
	if len(bulletSubs) >= 3 {
		result.Subcommands = bulletSubs
	}
	// Deduplicate positionals
	result.Positionals = dedupePositionals(result.Positionals)

	return result
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
	return parseOptionSpec(line)
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
var commandWords = []string{"command", "operation", "action", "topic", "verb", "service"}

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

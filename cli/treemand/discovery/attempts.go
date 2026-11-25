package discovery

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// truncatedHelpRe matches messages that indicate --help output is abbreviated
// and a more complete form is available (e.g. curl's "use --help all").
var truncatedHelpRe = regexp.MustCompile(`(?i)--help all|--help <category>|not the full help`)

// ErrHelpTimedOut reports that every help form of a command ran out of its
// own time: the CLI may be overloaded, not refusing.
var ErrHelpTimedOut = errors.New("timed out")

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
	if h.HelpSubcommand {
		if s := h.tryForm(ctx, resolved, args, formSuffix, Declared, &at); s != "" {
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

	if s := h.tryForm(ctx, resolved, args, formSuffix, HelpFallback, &at); s != "" {
		return s, nil
	}
	if len(args) > 0 {
		if s := h.tryForm(ctx, resolved, args, formPrefix, HelpFallback, &at); s != "" {
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
		return h.tryForm(ctx, resolved, args, forms[0], NoWords, at), forms[0]
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
			ch <- h.tryForm(rctx, resolved, args, f, NoWords, at)
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
		if s := h.tryForm(ctx, resolved, args, f, NoWords, at); s != "" {
			return s, f
		}
		if at.timedOut.Load() || ctx.Err() != nil {
			break
		}
	}
	return "", formUnknown
}

// hedgeDelay is how long the preferred flag form runs alone before the other
// is started alongside it.
const hedgeDelay = 250 * time.Millisecond

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
// command printed nothing or an error instead of help. basis is why a
// positional form may send "help" (flag forms send no word).
func (h *HelpDiscoverer) tryForm(ctx context.Context, resolved string, args []string, f helpForm, basis Basis, at *formAttempts) string {
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
	s := h.runOne(ctx, resolved, Probe{Path: args, Args: cmdArgs, Basis: basis}, &at.timedOut)
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
		p := Probe{Path: args, Args: append(slices.Clone(args), "--help", "all"), Basis: Documented, DocumentedIn: s}
		if all := h.runOne(ctx, resolved, p, &at.timedOut); all != "" {
			return all
		}
	}
	if (f == formFlag || f == formShort) && pointsToHelpPrefix(s, resolved, args) {
		p := Probe{Path: args, Args: append([]string{"help"}, args...), Basis: Documented, DocumentedIn: s}
		if full := h.runOne(ctx, resolved, p, &at.timedOut); full != "" && !isErrorOutput(full) {
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

// runner is the probe runner this discoverer launches the CLI through.
func (h *HelpDiscoverer) runner() Runner {
	return Runner{Launcher: h.Launcher, Timeout: h.Timeout}
}

// runOne launches one help probe and returns its sanitized output, or ""
// when it was refused, cancelled (by a race winner or the caller) or timed
// out, which it notes in timedOut.
func (h *HelpDiscoverer) runOne(ctx context.Context, resolved string, p Probe, timedOut *atomic.Bool) string {
	r := h.runner().Run(ctx, resolved, p)
	switch {
	case r.Cancelled:
		return ""
	case r.TimedOut:
		timedOut.Store(true)
		return ""
	case errors.Is(r.Err, ErrUndocumentedWord):
		return ""
	}
	return SanitizeHelp(r.Out)
}

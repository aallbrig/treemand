package discovery

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

// A Launcher starts the target CLI once and returns what it printed (stdout
// and stderr together). err is the process's own failure (a non-zero exit, a
// missing binary) or the context's. The runner decides whether a launch may
// happen; a Launcher only performs it.
type Launcher interface {
	Launch(ctx context.Context, bin string, args, env []string) (out []byte, err error)
}

// ExecLauncher runs the CLI as a process: in an isolated home, with output
// capped at MaxHelpBytes and recorded in the context's ProbeLog.
type ExecLauncher struct{}

// Launch runs bin with args and env appended to its environment.
func (ExecLauncher) Launch(ctx context.Context, bin string, args, env []string) ([]byte, error) {
	out, _, err := runCapped(ctx, bin, args, env, MaxHelpBytes)
	return out, err
}

// Basis is why a probe may send a word that is neither a flag nor on the
// command path. Such a word is an operand to a CLI that does not know it:
// "mkdir help" creates ./help, "npm prune help" prunes.
type Basis int

const (
	// NoWords: the probe sends only the command path and flags.
	NoWords Basis = iota
	// Documented: the CLI's own text (DocumentedIn: its help, its man page,
	// a pointer it printed) names every word.
	Documented
	// HelpFallback: the word is "help", sent after both flag forms were
	// refused with no usage synopsis (ADR 0002).
	HelpFallback
	// Declared: a preset or a strategy the user chose says the CLI takes the
	// words (aws "<path> help", cobra's "__complete").
	Declared
)

// A Probe is one launch of the target CLI.
type Probe struct {
	// Path is the command path below the CLI, each word listed by its
	// parent's help.
	Path []string
	// Args is the whole argument list sent.
	Args []string
	// Basis is why the words of Args outside Path and the flags are safe.
	Basis Basis
	// DocumentedIn is the CLI's own text that names those words (Documented).
	DocumentedIn string
}

// ErrUndocumentedWord is returned, without launching anything, for a probe
// that would send a word nothing vouches for.
var ErrUndocumentedWord = errors.New("refused to send an undocumented argument")

// words returns the arguments of p outside its path and the flags.
func (p Probe) words() []string {
	rest := slices.Clone(p.Path)
	var ws []string
	for _, a := range p.Args {
		if i := slices.Index(rest, a); i >= 0 {
			rest = slices.Delete(rest, i, i+1)
			continue
		}
		if !strings.HasPrefix(a, "-") {
			ws = append(ws, a)
		}
	}
	return ws
}

// check enforces the positional gate: only send a word the CLI documents.
func (p Probe) check() error {
	ws := p.words()
	if len(ws) == 0 {
		return nil
	}
	ok := false
	switch p.Basis {
	case Documented:
		ok = true
		for _, w := range ws {
			ok = ok && (w == "" || containsWord(p.DocumentedIn, w))
		}
	case HelpFallback:
		ok = len(ws) == 1 && ws[0] == "help"
	case Declared:
		ok = true
	}
	if !ok {
		return fmt.Errorf("%w: %q", ErrUndocumentedWord, ws)
	}
	return nil
}

// containsWord reports whether text has w delimited by non-word characters.
func containsWord(text, w string) bool {
	isWord := func(r rune) bool {
		return r == '-' || r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9')
	}
	for i := 0; ; {
		j := strings.Index(text[i:], w)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(w)
		before := start == 0 || !isWord(rune(text[start-1]))
		after := end == len(text) || !isWord(rune(text[end]))
		if before && after {
			return true
		}
		i = start + 1
	}
}

// pagerEnv are environment variable overrides appended to every launch so
// that tools that pipe through a pager (AWS, man, etc.) emit plain text.
var pagerEnv = []string{
	"AWS_PAGER=",
	"PAGER=cat",
	"MANPAGER=cat",
	"GIT_PAGER=cat",
	// Man pages (git <cmd> --help) wrap to the terminal's width; a fixed width
	// makes what is parsed, and cached, the same in any terminal.
	"MANWIDTH=80",
}

// killWaitDelay bounds how long we wait for a command's output pipes to close
// after its context ends. Without it, an orphaned child that inherited stdout
// keeps CombinedOutput blocked long after the timeout.
const killWaitDelay = time.Second

// helpSlots bounds how many target-CLI processes run at once across the
// whole discovery, however deep or wide the tree is.
var helpSlots atomic.Pointer[chan struct{}]

// DefaultMaxConcurrentHelp is the default cap: twice the CPU count, within 4..16.
func DefaultMaxConcurrentHelp() int {
	return min(max(runtime.NumCPU()*2, 4), 16)
}

// SetMaxConcurrentHelp sets the cap on concurrent target-CLI processes;
// n <= 0 restores the default.
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

// defaultCommandTimeout bounds one launch when the runner sets no timeout.
const defaultCommandTimeout = 5 * time.Second

// rootRetryScale multiplies the per-command budget for the one retry of a
// root whose help timed out.
const rootRetryScale = 3

type budgetScaleKey struct{}

// withRootRetryBudget returns ctx with each launch's budget scaled up for
// the root's retry; the run's own deadline still bounds it.
func withRootRetryBudget(ctx context.Context) context.Context {
	return context.WithValue(ctx, budgetScaleKey{}, rootRetryScale)
}

// Runner is the one way discovery launches the target CLI. Every launch
// passes the positional gate, takes a process slot, runs within its own
// timeout and gets the pager environment.
type Runner struct {
	// Launcher performs the launch; nil means ExecLauncher.
	Launcher Launcher
	// Timeout bounds one launch (not a group of them: a deadline shared by
	// siblings would starve the later ones); 0 means 5s.
	Timeout time.Duration
}

// Result is what one launch produced.
type Result struct {
	// Out is the CLI's combined output.
	Out string
	// TimedOut is set when the launch ran out of its own time: the CLI may be
	// overloaded, not refusing.
	TimedOut bool
	// Cancelled is set when the caller's context ended the launch (a race
	// winner, the run's deadline): Out is no answer.
	Cancelled bool
	// Err is the launch's error: ErrUndocumentedWord, a non-zero exit, or a
	// context error.
	Err error
}

// timeout is the time allowed for one launch under ctx.
func (r Runner) timeout(ctx context.Context) time.Duration {
	t := r.Timeout
	if t <= 0 {
		t = defaultCommandTimeout
	}
	if scale, ok := ctx.Value(budgetScaleKey{}).(int); ok && scale > 1 {
		t *= time.Duration(scale)
	}
	return t
}

// Run launches bin as p describes.
func (r Runner) Run(ctx context.Context, bin string, p Probe) Result {
	if err := p.check(); err != nil {
		return Result{Err: err}
	}
	// Take a process slot first: time spent waiting for one must not count
	// against the launch's own timeout.
	release, ok := acquireHelpSlot(ctx)
	if !ok {
		return Result{Cancelled: true, Err: ctx.Err()}
	}
	defer release()
	cctx, cancel := context.WithTimeout(ctx, r.timeout(ctx))
	defer cancel()
	l := r.Launcher
	if l == nil {
		l = ExecLauncher{}
	}
	out, err := l.Launch(cctx, bin, p.Args, pagerEnv)
	switch {
	case ctx.Err() != nil:
		return Result{Out: string(out), Cancelled: true, Err: ctx.Err()}
	case errors.Is(cctx.Err(), context.DeadlineExceeded):
		return Result{Out: string(out), TimedOut: true, Err: cctx.Err()}
	}
	return Result{Out: string(out), Err: err}
}

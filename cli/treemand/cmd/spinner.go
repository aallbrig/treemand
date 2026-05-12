package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
)

// Spinner writes animated braille frames to w while a long operation runs.
// When w is not a terminal the spinner is a no-op.
type Spinner struct {
	w     io.Writer
	isTTY bool
	stop  chan struct{}
	done  chan struct{}
}

// NewSpinner creates a Spinner that writes to w.
func NewSpinner(w io.Writer) *Spinner {
	tty := false
	if f, ok := w.(*os.File); ok {
		tty = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	return &Spinner{w: w, isTTY: tty}
}

// Start begins rendering the spinner with the given label in a background
// goroutine. It is a no-op when the output is not a terminal.
func (s *Spinner) Start(label string) {
	if !s.isTTY {
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				fmt.Fprintf(s.w, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(s.w, "\r%s %s", frames[i%len(frames)], label)
				i++
			}
		}
	}()
}

// Stop halts the spinner and clears the line. Safe to call when not started.
func (s *Spinner) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	<-s.done
	s.stop = nil
	s.done = nil
}

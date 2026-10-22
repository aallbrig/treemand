//go:build js && wasm

// Command wasm runs the real treemand TUI inside a browser terminal
// (xterm.js). The page feeds it keystrokes and renders the ANSI it writes.
package main

import (
	"encoding/json"
	"io"
	"syscall/js"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/tui"
)

// jsWriter forwards terminal output to the page's treemandOutput callback.
type jsWriter struct{}

func (jsWriter) Write(p []byte) (int, error) {
	js.Global().Call("treemandOutput", string(p))
	return len(p), nil
}

var (
	program *tea.Program
	input   chan []byte
)

// start(treeJSON string, cols, rows int) launches the TUI.
func start(_ js.Value, args []js.Value) any {
	var root models.Node
	if err := json.Unmarshal([]byte(args[0].String()), &root); err != nil {
		return err.Error()
	}
	cols, rows := args[1].Int(), args[2].Int()

	tui.Offline = true
	lipgloss.SetColorProfile(termenv.TrueColor)

	input = make(chan []byte, 1024)
	pr, pw := io.Pipe()
	go func() {
		for b := range input {
			if _, err := pw.Write(b); err != nil {
				return
			}
		}
	}()

	cfg := config.DefaultConfig()
	m := tui.NewModel(&root, cfg)
	program = tea.NewProgram(m,
		tea.WithInput(pr),
		tea.WithOutput(jsWriter{}),
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
		tea.WithoutSignalHandler(),
	)
	go func() {
		_, _ = program.Run()
		js.Global().Call("treemandExited")
	}()
	program.Send(tea.WindowSizeMsg{Width: cols, Height: rows})
	return nil
}

// write(data string) delivers keyboard/mouse bytes from xterm.js.
func write(_ js.Value, args []js.Value) any {
	if input != nil {
		input <- []byte(args[0].String())
	}
	return nil
}

// resize(cols, rows int).
func resize(_ js.Value, args []js.Value) any {
	if program != nil {
		program.Send(tea.WindowSizeMsg{Width: args[0].Int(), Height: args[1].Int()})
	}
	return nil
}

// stop() ends the running program.
func stop(_ js.Value, _ []js.Value) any {
	if program != nil {
		program.Quit()
		program = nil
	}
	return nil
}

func main() {
	g := js.Global()
	g.Set("treemandStart", js.FuncOf(start))
	g.Set("treemandWrite", js.FuncOf(write))
	g.Set("treemandResize", js.FuncOf(resize))
	g.Set("treemandStop", js.FuncOf(stop))
	if cb := g.Get("treemandReady"); cb.Type() == js.TypeFunction {
		cb.Invoke()
	}
	select {}
}

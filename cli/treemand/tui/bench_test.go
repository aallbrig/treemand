package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/tui"
)

// Benchmarks behind the tui-responsiveness probe (scripts/metrics/probes/tui.sh).
// Each measures what one key press costs on an aws-sized tree: Update plus the
// View the terminal redraws.

func benchModel(b *testing.B, cmds, flags int, expanded bool) *tui.Model {
	b.Helper()
	m := tui.NewModel(scaleTree(422, cmds, flags), config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if expanded {
		m.Update(key("e"))
	}
	m.View()
	return m
}

func BenchmarkKeyDown_stubTree(b *testing.B) {
	m := benchModel(b, 0, 0, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(key("down"))
		m.View()
	}
}

func BenchmarkKeyDown_expandedTree(b *testing.B) {
	m := benchModel(b, 20, 5, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(key("down"))
		m.View()
	}
}

func BenchmarkView_expandedTree(b *testing.B) {
	m := benchModel(b, 20, 5, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.View()
	}
}

func BenchmarkExpandAll_expandedTree(b *testing.B) {
	m := benchModel(b, 20, 5, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		m.Update(key("E"))
		b.StartTimer()
		m.Update(key("e"))
		m.View()
	}
}

func BenchmarkFilterKeystroke_stubTree(b *testing.B) {
	m := benchModel(b, 0, 0, false)
	m.Update(key("/"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(key("s"))
		m.View()
		m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m.View()
	}
}

package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/aallbrig/treemand/history"
)

func TestRecents_runIsRecordedAndCtrlRLoadsIt(t *testing.T) {
	m, root := newModalModel(t)
	store := history.Open(t.TempDir(), "git")
	m.SetHistory(store)

	selectNode(t, m, root.Children[1])
	press(m, "enter", "ctrl+e", "enter")
	entries, err := store.Recent()
	if err != nil || len(entries) != 1 || entries[0].Command != "git remote" {
		t.Fatalf("history after run = %+v, %v", entries, err)
	}

	// A fresh session sees the command and loads it into the preview.
	m2, _ := newModalModel(t)
	m2.SetHistory(store)
	press(m2, "ctrl+r")
	if !m2.rm.active {
		t.Fatal("ctrl+r should open the recent commands")
	}
	if view := m2.View(); !strings.Contains(view, "Recent Commands") || !strings.Contains(view, "git remote") {
		t.Fatalf("recents view missing the command:\n%s", view)
	}
	press(m2, "enter")
	if m2.rm.active || previewText(m2) != "git remote" {
		t.Fatalf("after enter: active=%v preview=%q", m2.rm.active, previewText(m2))
	}
}

func TestRecents_navigateAndClose(t *testing.T) {
	m, _ := newModalModel(t)
	store := history.Open(t.TempDir(), "git")
	now := time.Now()
	_ = store.Add("git status", now)
	_ = store.Add("git status", now)
	_ = store.Add("git log", now.Add(time.Second))
	m.SetHistory(store)

	before := previewText(m)
	press(m, "ctrl+r", "up", "down", "down", "down")
	if m.rm.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 (clamped to the last entry)", m.rm.cursor)
	}
	if view := m.View(); !strings.Contains(view, "git status ×2") {
		t.Errorf("view should show the use count:\n%s", view)
	}
	press(m, "k", "j")
	press(m, "esc")
	if m.rm.active || previewText(m) != before {
		t.Fatalf("esc: active=%v preview=%q", m.rm.active, previewText(m))
	}
	press(m, "ctrl+r", "ctrl+r")
	if m.rm.active {
		t.Error("a second ctrl+r should close the list")
	}
}

func TestRecents_emptyAndOff(t *testing.T) {
	m, _ := newModalModel(t)
	press(m, "ctrl+r")
	if m.rm.active || !strings.Contains(m.statusMsg, "history is off") {
		t.Fatalf("history off: active=%v status=%q", m.rm.active, m.statusMsg)
	}

	m.SetHistory(history.Open(t.TempDir(), "git"))
	before := previewText(m)
	press(m, "ctrl+r")
	if view := m.View(); !strings.Contains(view, "No commands yet") {
		t.Fatalf("empty history view:\n%s", view)
	}
	press(m, "enter")
	if m.rm.active || previewText(m) != before {
		t.Errorf("enter on an empty list: active=%v preview=%q", m.rm.active, previewText(m))
	}
}

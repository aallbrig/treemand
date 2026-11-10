package discovery_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

const tmuxListCommands = `attach-session (attach) [-dErx] [-c working-directory] [-f flags] [-t target-session]
bind-key (bind) [-nr] [-T key-table] [-N note] key [command [arguments]]
break-pane (breakp) [-abdP] [-F format] [-n window-name] [-s src-pane] [-t dst-window]
kill-server
list-commands [-F format] [command]
new-session (new) [-AdDEPX] [-c start-directory] [-n window-name] [-s session-name] [shell-command]
`

func TestParseCommandList_tmuxStyle(t *testing.T) {
	entries := discovery.ParseCommandList(tmuxListCommands)
	if len(entries) != 6 {
		t.Fatalf("got %d entries, want 6", len(entries))
	}
	first := entries[0]
	if first.Name != "attach-session" || first.Alias != "attach" {
		t.Errorf("first = %+v, want attach-session (attach)", first)
	}
	if entries[3].Name != "kill-server" || entries[3].Synopsis != "" {
		t.Errorf("kill-server = %+v, want no alias or synopsis", entries[3])
	}
}

func TestParseCommandList_rejectsProse(t *testing.T) {
	prose := "Error: unknown command\nSee the manual for details.\nThis is not a command list at all.\n"
	if got := discovery.ParseCommandList(prose); len(got) != 0 {
		t.Errorf("prose parsed as %d entries, want 0", len(got))
	}
}

func TestSynopsisFlags(t *testing.T) {
	flags := discovery.SynopsisFlags("[-dE] [-c working-directory] [-t target-session] key")
	got := map[string]string{}
	for _, f := range flags {
		got[f.Name] = f.ValueType
	}
	want := map[string]string{"-d": "bool", "-E": "bool", "-c": "working-directory", "-t": "target-session"}
	for name, vt := range want {
		if got[name] != vt {
			t.Errorf("flag %s value type = %q, want %q (all: %v)", name, got[name], vt, got)
		}
	}
}

const tmuxManCommands = `COMMANDS
       attach-session   [-dErx]   [-c   working-directory]   [-f   flags]  [-t
               target-session]
                     (alias: attach)
               If run from outside tmux, create a new client  in  the  current
               terminal and attach it to target-session.  If used from inside,
               switch  the  current  client.

       kill-server
               Kill the tmux server and clients and destroy all sessions.
`

func TestManCommandDescriptions(t *testing.T) {
	got := discovery.ManCommandDescriptions(tmuxManCommands, []string{"attach-session", "kill-server", "missing"})
	if want := "If run from outside tmux, create a new client in the current terminal and attach it to target-session."; got["attach-session"] != want {
		t.Errorf("attach-session = %q, want %q", got["attach-session"], want)
	}
	if want := "Kill the tmux server and clients and destroy all sessions."; got["kill-server"] != want {
		t.Errorf("kill-server = %q, want %q", got["kill-server"], want)
	}
	if _, ok := got["missing"]; ok {
		t.Error("unknown command should have no description")
	}
}

func TestHelpDiscoverer_listCommandsFallback(t *testing.T) {
	dir := t.TempDir()
	cli := "#!/bin/sh\n" +
		"if [ \"$1\" = list-commands ]; then\n" +
		"  printf 'attach-session (attach) [-dE] [-t target-session]\\nkill-server\\nlist-commands [-F format]\\nnew-session (new) [-s session-name]\\nrename-window (renamew) [-t target-window] new-name\\n'\n" +
		"else printf 'usage: lc [-2CDlNuVv] [command [flags]]\\n'; fi\n"
	man := "#!/bin/sh\nprintf 'NAME\\n       lc - a multiplexer\\n\\nCOMMANDS\\n       kill-server\\n               Kill the server and all sessions.\\n'\n"
	for name, body := range map[string]string{"lc": cli, "man": man} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	node, err := discovery.NewHelpDiscoverer(2).Discover(context.Background(), "lc", nil)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(node.Children) != 5 {
		t.Fatalf("got %d children, want 5", len(node.Children))
	}
	byName := map[string]int{}
	for i, c := range node.Children {
		byName[c.Name] = i
	}
	attach := node.Children[byName["attach-session"]]
	if !strings.Contains(attach.Description, "attach") {
		t.Errorf("attach-session description %q should mention its alias", attach.Description)
	}
	if len(attach.Flags) == 0 {
		t.Error("attach-session should carry flags parsed from its synopsis")
	}
	if got := node.Children[byName["kill-server"]].Description; !strings.Contains(got, "Kill the server") {
		t.Errorf("kill-server description = %q, want the man page sentence", got)
	}
	if got := strings.Join(attach.FullPath, " "); got != "lc attach-session" {
		t.Errorf("FullPath = %q, want %q", got, "lc attach-session")
	}
}

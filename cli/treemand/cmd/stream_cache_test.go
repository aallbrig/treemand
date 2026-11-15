package cmd_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// fakeTool writes a cobra-style CLI that logs its invocations and puts it
// first on PATH, with an isolated cache. Returns the log path and cache dir.
func fakeTool(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
echo "$*" >>'` + logPath + `'
case "$1" in
alpha|beta) printf '%s things\n\nUsage:\n  streamtool %s [flags]\n\nFlags:\n  -h, --help   help\n      --%s-flag  only for %s\n' "$1" "$1" "$1" "$1";;
*) printf 'streamtool\n\nUsage:\n  streamtool [command]\n\nAvailable Commands:\n  alpha       first\n  beta        second\n\nFlags:\n  -h, --help   help\n';;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "streamtool"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cacheDir := filepath.Join(dir, "cache")
	t.Setenv("TREEMAND_CACHE_DIR", cacheDir)
	return logPath, cacheDir
}

type outNode struct {
	Name     string     `json:"name"`
	Stub     bool       `json:"stub"`
	Flags    []struct{} `json:"flags"`
	Children []*outNode `json:"children"`
}

func TestRoot_aPartialCachedTreeIsCompletedBeforeOutput(t *testing.T) {
	resetViper()
	logPath, cacheDir := fakeTool(t)
	if _, err := runCmd("--output=json", "streamtool"); err != nil {
		t.Fatal(err)
	}
	// Leave the cached tree as an interrupted TUI session would: the
	// subcommands still stubs.
	db, err := sql.Open("sqlite", filepath.Join(cacheDir, "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	partial := `{"name":"streamtool","full_path":["streamtool"],"discovered":true,"children":[` +
		`{"name":"alpha","full_path":["streamtool","alpha"],"stub":true},` +
		`{"name":"beta","full_path":["streamtool","beta"],"stub":true}]}`
	if _, err := db.Exec(`UPDATE trees SET data = ?`, partial); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd("--output=json", "streamtool")
	if err != nil {
		t.Fatal(err)
	}
	var root outNode
	brace := strings.Index(out, "{")
	if brace < 0 {
		t.Fatalf("no JSON in output:\n%s", out)
	}
	if err := json.Unmarshal([]byte(out[brace:]), &root); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	for _, c := range root.Children {
		if c.Stub || len(c.Flags) == 0 {
			t.Errorf("%s was printed incomplete (stub=%v)", c.Name, c.Stub)
		}
	}
	calls, _ := os.ReadFile(logPath)
	if !strings.Contains(string(calls), "alpha --help") {
		t.Errorf("the pending subcommands were not discovered: %q", calls)
	}

	// The completed tree was written back: a third run touches nothing.
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd("--output=json", "streamtool"); err != nil {
		t.Fatal(err)
	}
	if calls, _ := os.ReadFile(logPath); len(strings.TrimSpace(string(calls))) != 0 {
		t.Errorf("the completed tree was not cached: %q", calls)
	}
}

package e2e

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// cacheEnv shares one cache directory (and invocation log) across runs.
func cacheEnv(t *testing.T) (envs []string, logPath, cacheDir string) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")
	cacheDir = filepath.Join(dir, "cache")
	return []string{"FAKE_LOG=" + logPath, "TREEMAND_CACHE_DIR=" + cacheDir}, logPath, cacheDir
}

func invocations(t *testing.T, logPath string) int {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(b)) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(string(b)), "\n"))
}

func resetLog(t *testing.T, logPath string) {
	t.Helper()
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

type jsonNode struct {
	Name     string      `json:"name"`
	Stub     bool        `json:"stub"`
	Children []*jsonNode `json:"children"`
}

func child(t *testing.T, root *jsonNode, name string) *jsonNode {
	t.Helper()
	for _, c := range root.Children {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no child %q in %+v", name, root.Children)
	return nil
}

func jsonTree(t *testing.T, out string) *jsonNode {
	t.Helper()
	var n jsonNode
	if err := json.Unmarshal([]byte(out), &n); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return &n
}

func TestCache_aSecondRunNeverTouchesTheCLI(t *testing.T) {
	envs, logPath, _ := cacheEnv(t)
	first := run(t, 20*time.Second, envs, "--output=json", "fakecache")
	if first.exit != 0 || invocations(t, logPath) == 0 {
		t.Fatalf("first run: exit %d, %d invocations\n%s", first.exit, invocations(t, logPath), first.stderr)
	}
	resetLog(t, logPath)
	second := run(t, 20*time.Second, envs, "--output=json", "fakecache")
	if n := invocations(t, logPath); n != 0 {
		t.Errorf("a cached CLI was run %d times on the second run", n)
	}
	if second.stdout != first.stdout {
		t.Errorf("the cached tree differs from the crawled one")
	}
}

func TestCache_anEntryOlderThanADayIsStillUsed(t *testing.T) {
	envs, logPath, cacheDir := cacheEnv(t)
	run(t, 20*time.Second, envs, "--output=json", "fakecache")
	// Age the entry by three days. The old 24h expiry re-crawled it; entries
	// are keyed on the binary, so only a changed CLI (or 30 days) should.
	db, err := sql.Open("sqlite", filepath.Join(cacheDir, "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE trees SET cached_at = cached_at - 3*86400`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	resetLog(t, logPath)
	run(t, 20*time.Second, envs, "--output=json", "fakecache")
	if n := invocations(t, logPath); n != 0 {
		t.Errorf("a 3-day-old entry was re-crawled (%d invocations)", n)
	}
}

func TestCache_refreshRecrawlsAndUpdatesTheEntry(t *testing.T) {
	envs, logPath, _ := cacheEnv(t)
	run(t, 20*time.Second, envs, "--output=json", "fakecache")
	resetLog(t, logPath)
	res := run(t, 20*time.Second, envs, "--output=json", "--refresh", "fakecache")
	if res.exit != 0 || invocations(t, logPath) == 0 {
		t.Fatalf("--refresh did not re-crawl: exit %d, %d invocations", res.exit, invocations(t, logPath))
	}
	resetLog(t, logPath)
	run(t, 20*time.Second, envs, "--output=json", "fakecache")
	if n := invocations(t, logPath); n != 0 {
		t.Errorf("the refreshed entry was not stored (%d invocations)", n)
	}
}

func TestCache_commandsExpandedInTheTUIAreRemembered(t *testing.T) {
	envs, logPath, _ := cacheEnv(t)
	// A stub threshold of 2 leaves fakecache's three commands as stubs.
	s := startWithEnv(t, envs, "-i", "--stub-threshold=2", "fakecache")
	s.waitFor("alpha", 10*time.Second)
	s.stepDownUntil("fakecache alpha", 8) // the status bar names the selection; landing on a stub discovers it
	s.waitFor("expanded: alpha", 10*time.Second)
	s.send("q")
	if code := s.exitCode(10 * time.Second); code != 0 {
		t.Fatalf("treemand -i exited %d", code)
	}

	resetLog(t, logPath)
	res := run(t, 20*time.Second, envs, "--output=json", "--stub-threshold=2", "fakecache")
	if n := invocations(t, logPath); n != 0 {
		t.Errorf("the second run touched the CLI %d times", n)
	}
	alpha := child(t, jsonTree(t, res.stdout), "alpha")
	if alpha.Stub || len(alpha.Children) == 0 {
		t.Errorf("alpha, expanded in the TUI, came back from the cache as a stub: %+v", alpha)
	}
	if beta := child(t, jsonTree(t, res.stdout), "beta"); !beta.Stub {
		t.Errorf("beta was never expanded but is no longer a stub")
	}
}

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

package cmd_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The test binary doubles as a fake CLI: copied under another name and run
// with fakeModeEnv set, TestMain answers like a small cobra-style tool. This
// runs a real subprocess on every OS without shell scripts or installed CLIs.
const (
	fakeModeEnv = "TREEMAND_CMD_FAKE"
	fakeLogEnv  = "TREEMAND_CMD_FAKE_LOG"
	fakeMapEnv  = "TREEMAND_CMD_FAKE_MAP" // "fail" makes --map exit 1
)

func TestMain(m *testing.M) {
	switch os.Getenv(fakeModeEnv) {
	case "":
		os.Exit(m.Run())
	case "cli":
		os.Exit(fakeCLIMain(os.Args[1:]))
	case "editor":
		logCall(os.Args[1:])
		os.Exit(0)
	}
}

func logCall(args []string) {
	if p := os.Getenv(fakeLogEnv); p != "" {
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintln(f, strings.Join(args, " "))
			_ = f.Close()
		}
	}
}

const fakeMap = `stripe
├── customers
│   ├── create
│   └── list
└── alpha
    └── rotate
`

func fakeCLIMain(args []string) int {
	logCall(args)
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	switch {
	case len(args) == 1 && args[0] == "--map":
		if os.Getenv(fakeMapEnv) == "fail" {
			fmt.Fprintln(os.Stderr, "unknown flag: --map")
			return 1
		}
		fmt.Print(fakeMap)
	case len(args) == 1 && args[0] == "--version":
		fmt.Println(name + " version 1.2.3")
	case len(args) > 1 && args[0] == "alpha" && args[1] == "deep":
		fmt.Printf("deep things\n\nUsage:\n  %s alpha deep [flags]\n\nFlags:\n  -h, --help   help\n", name)
	case len(args) > 0 && args[0] == "alpha":
		fmt.Printf("alpha things\n\nUsage:\n  %s alpha [command]\n\nAvailable Commands:\n  deep        a nested command\n\nFlags:\n  -h, --help        help\n      --alpha-flag   only for alpha\n", name)
	case len(args) > 0 && args[0] == "beta":
		fmt.Printf("%s things\n\nUsage:\n  %s %s [flags]\n\nFlags:\n  -h, --help        help\n      --%s-flag   only for %s\n",
			args[0], name, args[0], args[0], args[0])
	default:
		fmt.Printf("%s does fake things\n\nUsage:\n  %s [command]\n\nAvailable Commands:\n  alpha       first command\n  beta        second command\n\nFlags:\n  -h, --help   help\n", name, name)
	}
	return 0
}

// fakeEnv is one test's fake tools: a directory first on PATH, a call log and
// an isolated cache and config home.
type fakeEnv struct {
	dir, log, cache string
}

func newFakeEnv(t *testing.T) *fakeEnv {
	t.Helper()
	resetViper()
	dir := t.TempDir()
	e := &fakeEnv{dir: dir, log: filepath.Join(dir, "calls.log"), cache: filepath.Join(dir, "cache")}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TREEMAND_CACHE_DIR", e.cache)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv(fakeModeEnv, "cli")
	t.Setenv(fakeLogEnv, e.log)
	return e
}

// tool installs the fake CLI under name and returns its path.
func (e *fakeEnv) tool(t *testing.T, name string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dst := filepath.Join(e.dir, name)
	copyFile(t, exe, dst)
	return dst
}

// calls returns the logged invocations and clears the log.
func (e *fakeEnv) calls(t *testing.T) string {
	t.Helper()
	b, _ := os.ReadFile(e.log)
	_ = os.Remove(e.log)
	return string(b)
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

package discovery

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// neverProbe names tools treemand never starts, not even with --help: they
// stop processes or the machine, and some versions act on any argument they
// do not understand. Matched on the executable's base name, without ".exe"
// and ignoring case.
var neverProbe = map[string]bool{
	"kill": true, "pkill": true, "killall": true, "killall5": true, "skill": true, "xkill": true,
	"reboot": true, "shutdown": true, "halt": true, "poweroff": true, "init": true, "telinit": true,
	"taskkill": true, "tskill": true, "logoff": true,
}

// NeverProbeError is returned for a CLI on the never-probe list.
type NeverProbeError struct{ Name string }

func (e *NeverProbeError) Error() string {
	return fmt.Sprintf("treemand never runs %s: it stops processes or the machine and may act instead of printing help\n"+
		"Hint: to browse it, pipe a help text in instead: man %s | treemand --stdin %s", e.Name, e.Name, e.Name)
}

// probeBase is the name a never-probe check compares: the base name, lower
// case, without a Windows ".exe".
func probeBase(path string) string {
	b := strings.ToLower(filepath.Base(path))
	return strings.TrimSuffix(b, ".exe")
}

// CheckProbeAllowed returns a *NeverProbeError when cliName, or the file its
// PATH entry links to, is on the never-probe list.
func CheckProbeAllowed(cliName string) error {
	names := []string{cliName}
	if p, err := exec.LookPath(cliName); err == nil {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			names = append(names, real)
		}
	}
	for _, n := range names {
		if neverProbe[probeBase(n)] {
			return &NeverProbeError{Name: probeBase(cliName)}
		}
	}
	return nil
}

// toolchainRootVars are variables version-manager shims and toolchain proxies
// derive from HOME. Each is pinned to the real home's directory when the user
// has not set it and that directory exists, so pyenv, rustup, nvm and similar
// shims still find their tools inside an isolated probe.
var toolchainRootVars = []struct{ name, rel string }{
	{"PYENV_ROOT", ".pyenv"},
	{"RBENV_ROOT", ".rbenv"},
	{"NODENV_ROOT", ".nodenv"},
	{"GOENV_ROOT", ".goenv"},
	{"PLENV_ROOT", ".plenv"},
	{"ASDF_DATA_DIR", ".asdf"},
	{"NVM_DIR", ".nvm"},
	{"VOLTA_HOME", ".volta"},
	{"SDKMAN_DIR", ".sdkman"},
	{"CARGO_HOME", ".cargo"},
	{"RUSTUP_HOME", ".rustup"},
	{"KREW_ROOT", ".krew"},
	{"GOPATH", "go"},
	{"MISE_DATA_DIR", ".local/share/mise"},
	{"MISE_CONFIG_DIR", ".config/mise"},
}

var (
	toolchainEnvOnce sync.Once
	toolchainEnv     []string
)

// toolchainRoots returns the NAME=path entries that keep shims working, read
// once from the real environment.
func toolchainRoots() []string {
	toolchainEnvOnce.Do(func() { toolchainEnv = computeToolchainRoots() })
	return toolchainEnv
}

func computeToolchainRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var env []string
	for _, v := range toolchainRootVars {
		if os.Getenv(v.name) != "" {
			continue
		}
		p := filepath.Join(home, filepath.FromSlash(v.rel))
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			env = append(env, v.name+"="+p)
		}
	}
	// go reads GOTOOLCHAIN, GOPROXY and the like from its env file under the
	// user config directory, which an isolated probe would hide.
	if os.Getenv("GOENV") == "" {
		if cfg, err := os.UserConfigDir(); err == nil {
			if p := filepath.Join(cfg, "go", "env"); fileExists(p) {
				env = append(env, "GOENV="+p)
			}
		}
	}
	return env
}

func fileExists(p string) bool {
	info, err := os.Stat(p) //nolint:gosec // a path under the user's own config directory
	return err == nil && !info.IsDir()
}

// IsolateProbe prepares cmd to run a target CLI: it refuses a CLI on the
// never-probe list, then points HOME, the XDG base directories and the
// temporary directory (and their Windows equivalents) at a fresh directory,
// so a tool that writes config or caches on start writes there. extraEnv is
// appended last. The returned cleanup removes the directory; call it after
// the command has finished.
func IsolateProbe(cmd *exec.Cmd, extraEnv []string) (cleanup func(), err error) {
	if neverProbe[probeBase(cmd.Path)] {
		return nil, &NeverProbeError{Name: probeBase(cmd.Path)}
	}
	dir, err := os.MkdirTemp("", "treemand-probe-")
	if err != nil {
		return nil, fmt.Errorf("creating the probe directory: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	home := filepath.Join(dir, "home")
	tmp := filepath.Join(dir, "tmp")
	for _, d := range []string{home, tmp} {
		if err := os.Mkdir(d, 0o700); err != nil {
			cleanup()
			return nil, fmt.Errorf("creating the probe directory: %w", err)
		}
	}
	env := append(cmd.Environ(), toolchainRoots()...)
	env = append(env,
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_CACHE_HOME="+filepath.Join(home, ".cache"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME="+filepath.Join(home, ".local", "state"),
		"TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp,
	)
	if runtime.GOOS == "windows" {
		env = append(env,
			"USERPROFILE="+home,
			"APPDATA="+filepath.Join(home, "AppData", "Roaming"),
			"LOCALAPPDATA="+filepath.Join(home, "AppData", "Local"),
		)
	}
	env = append(env, extraEnv...)
	cmd.Env = env
	return cleanup, nil
}

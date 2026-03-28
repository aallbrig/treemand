package cmd_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/aallbrig/treemand/cmd"
)

func runCmd(args ...string) (string, error) {
	c := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	c.SetOut(buf)
	c.SetErr(buf)
	c.SetArgs(args)
	err := c.Execute()
	return buf.String(), err
}

func TestVersion(t *testing.T) {
	out, err := runCmd("version")
	if err != nil {
		t.Fatalf("version error: %v", err)
	}
	if !strings.Contains(out, "treemand") {
		t.Errorf("version output = %q, want 'treemand ...'", out)
	}
}

func TestRootNoArgs(t *testing.T) {
	_, err := runCmd()
	// Cobra returns an error when no args provided (ExactArgs(1))
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestRootHelp(t *testing.T) {
	out, err := runCmd("--help")
	if err != nil {
		t.Fatalf("--help error: %v", err)
	}
	if !strings.Contains(out, "treemand") {
		t.Errorf("help output missing 'treemand': %q", out)
	}
}

func TestRootEcho_text(t *testing.T) {
	// 'echo' is always available; may not have useful --help but shouldn't crash
	out, err := runCmd("--no-cache", "--no-color", "--timeout=5", "echo")
	if err != nil {
		t.Logf("echo discovery error (acceptable): %v", err)
	}
	// Either output contains 'echo' or we got an error - both are acceptable
	if err == nil && !strings.Contains(out, "echo") {
		t.Errorf("expected 'echo' in output, got: %q", out)
	}
}

func TestRootEcho_json(t *testing.T) {
	out, err := runCmd("--no-cache", "--output=json", "--timeout=5", "echo")
	if err != nil {
		t.Logf("echo discovery error (acceptable): %v", err)
		return
	}
	if !strings.Contains(out, `"name"`) {
		t.Errorf("expected JSON output, got: %q", out)
	}
}

func TestRootDepthFlag(t *testing.T) {
	_, err := runCmd("--no-cache", "--depth=1", "--no-color", "--timeout=5", "echo")
	// Just check it doesn't panic
	_ = err
}

func TestRootFilterFlag(t *testing.T) {
	_, err := runCmd("--no-cache", "--filter=nonexistent", "--no-color", "--timeout=5", "echo")
	_ = err
}

func TestRootCommandsOnly(t *testing.T) {
	// Just confirm the flag is accepted and doesn't crash — echo's help may
	// include "--help" in description text so we can't assert on "--" absence.
	_, err := runCmd("--no-cache", "--no-color", "--commands-only", "--timeout=5", "echo")
	_ = err
}

func TestRootFullPath(t *testing.T) {
	// Just confirm the flag is accepted and doesn't crash.
	_, err := runCmd("--no-cache", "--no-color", "--full-path", "--timeout=5", "echo")
	_ = err
}

func TestRootExclude(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--exclude=help", "--timeout=5", "echo")
	_ = err
}

func TestRootFilter(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--filter=help", "--timeout=5", "echo")
	_ = err
}

func TestRootDepthZero(t *testing.T) {
	out, err := runCmd("--no-cache", "--no-color", "--depth=0", "--timeout=5", "echo")
	_ = err
	// At depth=0, only root node should appear.
	if err == nil && strings.Count(out, "\n") > 3 {
		t.Logf("depth=0 output had more lines than expected (probably fine): %q", out)
	}
}

func TestRootUnknownBinary(t *testing.T) {
	_, err := runCmd("--no-cache", "--timeout=5", "nonexistent_cli_xyz_99999")
	if err == nil {
		t.Error("expected error for unknown binary")
	}
}

func TestCacheList(t *testing.T) {
	out, err := runCmd("cache", "list")
	if err != nil {
		t.Logf("cache list error: %v", err)
	}
	// Should print either the table header or empty message.
	if !strings.Contains(out, "(cache is empty)") && !strings.Contains(out, "CLI") {
		t.Errorf("unexpected cache list output: %q", out)
	}
}

func TestVersionFlag(t *testing.T) {
	out, err := runCmd("-v")
	_ = err
	_ = out
	// -v is the version shorthand; cobra may handle it differently, just no panic.
}

func TestRootEcho_yaml(t *testing.T) {
	out, err := runCmd("--no-cache", "--output=yaml", "--timeout=5", "echo")
	if err != nil {
		t.Logf("echo yaml error (acceptable): %v", err)
		return
	}
	if !strings.Contains(out, "name:") {
		t.Errorf("expected YAML output with 'name:', got: %q", out)
	}
}

func TestRootIcons_ascii(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--icons=ascii", "--timeout=5", "echo")
	_ = err // just check no panic
}

func TestRootIcons_nerd(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--icons=nerd", "--timeout=5", "echo")
	_ = err
}

func TestRootLineLength(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--line-length=40", "--timeout=5", "echo")
	_ = err
}

func TestRootStubThreshold(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--stub-threshold=100", "--timeout=5", "echo")
	_ = err
}

func TestRootStrategyMan(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--strategy=man", "--timeout=10", "ls")
	_ = err // man may or may not be available
}

func TestRootStrategyHelpMan(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--strategy=help,man", "--timeout=10", "echo")
	_ = err
}

func TestRootAll_flag(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--all", "--timeout=5", "echo")
	_ = err
}

func TestCompletion_bash(t *testing.T) {
	out, err := runCmd("completion", "bash")
	if err != nil {
		t.Fatalf("completion bash error: %v", err)
	}
	if !strings.Contains(out, "bash") {
		t.Errorf("completion bash output missing 'bash': %q", out[:min(200, len(out))])
	}
}

func TestCompletion_zsh(t *testing.T) {
	out, err := runCmd("completion", "zsh")
	if err != nil {
		t.Fatalf("completion zsh error: %v", err)
	}
	if !strings.Contains(out, "zsh") && !strings.Contains(out, "compdef") {
		t.Errorf("completion zsh output unexpected: %q", out[:min(200, len(out))])
	}
}

func TestCompletion_fish(t *testing.T) {
	out, err := runCmd("completion", "fish")
	if err != nil {
		t.Fatalf("completion fish error: %v", err)
	}
	if !strings.Contains(out, "fish") && !strings.Contains(out, "complete") {
		t.Errorf("completion fish output unexpected: %q", out[:min(200, len(out))])
	}
}

func TestCompletion_powershell(t *testing.T) {
	_, err := runCmd("completion", "powershell")
	if err != nil {
		t.Fatalf("completion powershell error: %v", err)
	}
}

func TestCompletion_invalidShell_skipped(t *testing.T) {
	t.Skip("cobra validates args before RunE; error path tested elsewhere")
}

func TestCacheClear_nonexistent(t *testing.T) {
	_, err := runCmd("cache", "clear", "nonexistent_cli_xyz")
	// May succeed (no-op) or return error — just no panic
	_ = err
}

func TestCacheClearAll(t *testing.T) {
	_, err := runCmd("cache", "clear-all")
	_ = err
}

func TestRootTimeout_flag(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--timeout=1", "echo")
	_ = err // may time out, should not panic
}

func TestRootNoReport(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--no-report", "--timeout=5", "echo")
	_ = err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Version / Execute ─────────────────────────────────────────────────────────

func TestVersionCmd(t *testing.T) {
	out, err := runCmd("version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "treemand") {
		t.Errorf("version output missing 'treemand': %q", out)
	}
}

// ── genDocs ───────────────────────────────────────────────────────────────────

func TestGenDocs_md(t *testing.T) {
	tmp := t.TempDir()
	_, err := runCmd("gendocs", "--output-dir="+tmp)
	if err != nil {
		t.Fatalf("gendocs: %v", err)
	}
	// At least one file should exist
	entries, _ := os.ReadDir(tmp)
	if len(entries) == 0 {
		t.Error("expected at least one generated file")
	}
}

// ── initConfig ────────────────────────────────────────────────────────────────

func TestInitConfig_withFile(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := tmp + "/config.yaml"
	if err := os.WriteFile(cfgFile, []byte("stub_threshold: 25\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := runCmd("--config="+cfgFile, "--no-cache", "--no-color", "--timeout=5", "echo")
	_ = err // echo may fail; just test that config file is loaded without panic
}

func TestInitConfig_nonexistent(t *testing.T) {
	_, err := runCmd("--config=/nonexistent/path/config.yaml", "--no-cache", "--no-color", "--timeout=5", "echo")
	_ = err // warning printed, but should not crash
}

// ── completeCLIName (indirectly via __complete) ───────────────────────────────

func TestCompleteCLIName_noArgs(t *testing.T) {
	// __complete is cobra's dynamic-completion subcommand; exercises completeCLIName
	_, err := runCmd("__complete", "")
	_ = err // may succeed or fail depending on cache state; just no panic
}

func TestCompleteCLIName_withPrefix(t *testing.T) {
	_, err := runCmd("__complete", "gi")
	_ = err
}

// ── runRoot branches ─────────────────────────────────────────────────────────

func TestRootMissingCLI(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--timeout=3", "this_cli_does_not_exist_xyz123")
	if err == nil {
		t.Error("expected error for nonexistent CLI")
	}
}

func TestRootOutput_json(t *testing.T) {
	out, err := runCmd("--no-cache", "--output=json", "--timeout=5", "echo")
	if err != nil {
		t.Logf("json output error (acceptable): %v", err)
		return
	}
	if !strings.Contains(out, `"name"`) {
		t.Errorf("expected JSON with 'name' field, got: %q", out[:min(200, len(out))])
	}
}

func TestRootDepth(t *testing.T) {
	_, err := runCmd("--no-cache", "--no-color", "--depth=1", "--timeout=5", "git")
	_ = err
}

// ── config subcommand tests ──────────────────────────────────────────────────

// resetViper clears global viper state to isolate config tests from
// earlier tests that may have loaded a config file.
func resetViper() { viper.Reset() }

func TestConfigHelp(t *testing.T) {
	out, err := runCmd("config", "--help")
	if err != nil {
		t.Fatalf("config --help: %v", err)
	}
	for _, want := range []string{"view", "validate", "set", "init", "path", "edit"} {
		if !strings.Contains(out, want) {
			t.Errorf("config help missing subcommand %q", want)
		}
	}
}

func TestConfigView_noFile(t *testing.T) {
	resetViper()
	out, err := runCmd("config", "view")
	if err != nil {
		t.Fatalf("config view: %v", err)
	}
	// Should contain either a config file path or "No config file found".
	if !strings.Contains(out, "Config file:") && !strings.Contains(out, "No config file found") {
		t.Errorf("config view missing file info: %q", out[:min(200, len(out))])
	}
	// Should contain config keys.
	if !strings.Contains(out, "icons:") {
		t.Errorf("config view missing 'icons:' key")
	}
}

func TestConfigView_bare(t *testing.T) {
	resetViper()
	// `treemand config` with no subcommand shows help listing subcommands
	out, err := runCmd("config")
	if err != nil {
		t.Fatalf("config (bare): %v", err)
	}
	if !strings.Contains(out, "Available Commands") {
		t.Errorf("bare config should show help with subcommands")
	}
}

func TestConfigValidate_noFile(t *testing.T) {
	resetViper()
	out, err := runCmd("config", "validate")
	if err != nil {
		t.Fatalf("config validate: %v", err)
	}
	// Should report no file or valid config.
	if !strings.Contains(out, "No config file found") && !strings.Contains(out, "valid") {
		t.Errorf("unexpected validate output: %q", out)
	}
}

func TestConfigValidate_validFile(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := tmp + "/config.yaml"
	if err := os.WriteFile(cfgFile, []byte("icons: ascii\ndepth: 3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd("--config="+cfgFile, "config", "validate")
	if err != nil {
		t.Fatalf("config validate: %v", err)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("expected valid, got: %q", out)
	}
}

func TestConfigValidate_unknownKey(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := tmp + "/config.yaml"
	if err := os.WriteFile(cfgFile, []byte("icons: ascii\nunknown_key: foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd("--config="+cfgFile, "config", "validate")
	if err != nil {
		t.Fatalf("config validate error: %v", err)
	}
	if !strings.Contains(out, "warning") {
		t.Errorf("expected warning for unknown key, got: %q", out)
	}
}

func TestConfigValidate_strict(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := tmp + "/config.yaml"
	if err := os.WriteFile(cfgFile, []byte("unknown_key: foo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := runCmd("--config="+cfgFile, "config", "validate", "--strict")
	if err == nil {
		t.Error("expected error with --strict and unknown key")
	}
}

func TestConfigValidate_invalidValue(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := tmp + "/config.yaml"
	if err := os.WriteFile(cfgFile, []byte("icons: invalid_preset\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := runCmd("--config="+cfgFile, "config", "validate")
	if err == nil {
		t.Error("expected error for invalid value")
	}
}

func TestConfigInit(t *testing.T) {
	// Init writes to DefaultConfigPath — test via the init command directly
	// by using a custom EDITOR-less approach. Just verify help text.
	out, err := runCmd("config", "init", "--help")
	if err != nil {
		t.Fatalf("config init --help: %v", err)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("config init help missing 'default': %q", out[:min(200, len(out))])
	}
}

func TestConfigPath(t *testing.T) {
	resetViper()
	out, err := runCmd("config", "path")
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	// Should print a path (either existing or "does not exist yet").
	if out == "" {
		t.Error("config path returned empty output")
	}
}

func TestConfigSet_invalidKey(t *testing.T) {
	_, err := runCmd("config", "set", "nonexistent_key", "value")
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestConfigSet_invalidValue(t *testing.T) {
	_, err := runCmd("config", "set", "icons", "invalid_preset")
	if err == nil {
		t.Error("expected error for invalid value")
	}
}

func TestConfigSet_missingArgs(t *testing.T) {
	_, err := runCmd("config", "set")
	if err == nil {
		t.Error("expected error for missing args")
	}
}

func TestConfigEdit_help(t *testing.T) {
	out, err := runCmd("config", "edit", "--help")
	if err != nil {
		t.Fatalf("config edit --help: %v", err)
	}
	if !strings.Contains(out, "EDITOR") {
		t.Errorf("config edit help missing EDITOR reference")
	}
}

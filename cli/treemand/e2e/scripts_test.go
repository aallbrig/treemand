package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakescriptedFish is the fish completion script fakescripted "installed":
// it knows deploy, which the CLI's --help never lists, and a command
// substitution that must never run.
const fakescriptedFish = `function __fish_fakescripted_needs_command
    set -l cmd (commandline -opc)
    test (count $cmd) -eq 1
end
complete -c fakescripted -n "__fish_fakescripted_needs_command" -s v -l verbose -d 'Print more'
complete -c fakescripted -n "__fish_fakescripted_needs_command" -f -a "build" -d 'Build things'
complete -c fakescripted -n "__fish_fakescripted_needs_command" -f -a "deploy" -d 'Ship it to an environment'
complete -c fakescripted -n "__fish_seen_subcommand_from deploy" -l target -d 'Environment to deploy to' -r -f -a "(touch $FAKE_TOUCHED)"
`

// scriptsEnv installs fakescripted's completion script in a data directory
// named by XDG_DATA_DIRS and returns the env, the invocation log and the
// file that running the script's substitution would create.
func scriptsEnv(t *testing.T) (envs []string, logPath, touched string) {
	t.Helper()
	dir := t.TempDir()
	completions := filepath.Join(dir, "share", "fish", "vendor_completions.d")
	if err := os.MkdirAll(completions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(completions, "fakescripted.fish"), []byte(fakescriptedFish), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath, touched = filepath.Join(dir, "calls.log"), filepath.Join(dir, "touched")
	return []string{"XDG_DATA_DIRS=" + filepath.Join(dir, "share"), "FAKE_LOG=" + logPath, "FAKE_TOUCHED=" + touched}, logPath, touched
}

type scriptsTree struct {
	Flags []struct {
		Name string
	}
	Children []struct {
		Name        string
		Description string
		Flags       []struct{ Name, Description string }
	}
}

func TestScripts_completionScriptFillsWhatHelpMisses(t *testing.T) {
	envs, _, touched := scriptsEnv(t)
	r := run(t, 30*time.Second, envs, "--no-cache", "--no-color", "--output=json", "--strategy=help,scripts", "fakescripted")
	if r.exit != 0 {
		t.Fatalf("exit %d\nstderr: %s", r.exit, r.stderr)
	}
	var tree scriptsTree
	if err := json.Unmarshal([]byte(r.stdout), &tree); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, r.stdout)
	}
	children := map[string]int{}
	for i, c := range tree.Children {
		children[c.Name] = i
	}
	if _, ok := children["build"]; !ok {
		t.Errorf("help's build is missing: %+v", tree.Children)
	}
	i, ok := children["deploy"]
	if !ok {
		t.Fatalf("deploy (only in the completion script) is missing: %+v", tree.Children)
	}
	deploy := tree.Children[i]
	if deploy.Description != "Ship it to an environment" || len(deploy.Flags) == 0 || deploy.Flags[0].Name != "--target" {
		t.Errorf("deploy = %+v, want its description and --target from the script", deploy)
	}
	mustContain(t, "stdout", r.stdout, `"--verbose"`)
	if _, err := os.Stat(touched); err == nil {
		t.Error("a command substitution in the completion script was run")
	}

	t.Run("text tree", func(t *testing.T) {
		r := run(t, 30*time.Second, envs, "--no-cache", "--no-color", "-s", "help,scripts", "fakescripted")
		mustContain(t, "stdout", r.stdout, "deploy", "Ship it to an environment", "--verbose")
	})
}

func TestScripts_aloneNeverRunsTheCLI(t *testing.T) {
	envs, logPath, _ := scriptsEnv(t)
	r := run(t, 30*time.Second, envs, "--no-cache", "--no-color", "--output=json", "--strategy=scripts", "fakescripted")
	if r.exit != 0 {
		t.Fatalf("exit %d\nstderr: %s", r.exit, r.stderr)
	}
	mustContain(t, "stdout", r.stdout, `"deploy"`, `"build"`, `"--target"`)
	if data, err := os.ReadFile(logPath); err == nil && len(data) > 0 {
		t.Errorf("fakescripted was run by the scripts strategy:\n%s", data)
	}
}

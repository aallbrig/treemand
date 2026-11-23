package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// stdinHelp is a docker-like help text, as `docker --help` would print it.
const stdinHelp = `Usage:  docker [OPTIONS] COMMAND

A self-sufficient runtime for containers

Common Commands:
  run         Create and run a new container from an image
  build       Build an image from a Dockerfile
  ps          List containers

Global Options:
      --config string      Location of client config files
  -D, --debug              Enable debug mode
  -H, --host list          Daemon socket to connect to
  -v, --version            Print version information and quit

Run 'docker COMMAND --help' for more information on a command.
`

// runStdin runs treemand with stdin and a PATH whose only "docker" is a trap:
// if treemand ran it, the trap leaves a marker file behind.
func runStdin(t *testing.T, stdin string, args ...string) (result, bool) {
	t.Helper()
	trapDir := t.TempDir()
	marker := filepath.Join(trapDir, "ran")
	trap := "#!/bin/sh\ntouch '" + marker + "'\necho 'Usage: docker trapped'\n"
	if err := os.WriteFile(filepath.Join(trapDir, "docker"), []byte(trap), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Env = env(t, "PATH="+trapDir+string(os.PathListSeparator)+"/usr/bin:/bin")
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.exit = exitErr.ExitCode()
	default:
		t.Fatalf("running treemand %v: %v", args, err)
	}
	_, statErr := os.Stat(marker)
	return res, statErr == nil
}

func TestStdin_parsesPipedHelpAsTheRootAndRunsNothing(t *testing.T) {
	r, ran := runStdin(t, stdinHelp, "--stdin", "--output=json", "docker")
	if r.exit != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr: %q", r.exit, r.stderr)
	}
	if ran {
		t.Error("treemand ran the docker binary; --stdin must execute nothing")
	}
	root := parseJSON(t, r.stdout)
	if root.Name != "docker" || !root.Discovered {
		t.Errorf("root = %q discovered=%v, want docker discovered", root.Name, root.Discovered)
	}
	if root.Description != "A self-sufficient runtime for containers" {
		t.Errorf("root description = %q", root.Description)
	}
	for _, f := range []string{"--config", "--debug", "--host", "--version"} {
		if !root.hasFlag(f) {
			t.Errorf("root flag %s missing: %+v", f, root.Flags)
		}
	}
	for _, name := range []string{"run", "build", "ps"} {
		c, ok := root.child(name)
		if !ok {
			t.Errorf("child %s missing: %+v", name, root.Children)
			continue
		}
		if c.Discovered {
			t.Errorf("child %s is discovered=true; its help was never read, want false", name)
		}
	}
	if run, _ := root.child("run"); run.Description != "Create and run a new container from an image" {
		t.Errorf("run description = %q, want the one the root's help gives", run.Description)
	}
}

func TestStdin_worksWithTextAndYAML(t *testing.T) {
	r, ran := runStdin(t, stdinHelp, "--stdin", "--no-color", "docker")
	if r.exit != 0 || ran {
		t.Fatalf("text: exit %d ran=%v stderr: %s", r.exit, ran, r.stderr)
	}
	mustContain(t, "text tree", r.stdout, "docker", "run", "build", "ps", "--debug")
	mustContain(t, "stub note", r.stderr, "3 subcommands", "--stdin")
	if strings.Contains(r.stderr, "Use -i") {
		t.Errorf("the stub note suggests -i, which --stdin cannot use:\n%s", r.stderr)
	}

	r, ran = runStdin(t, stdinHelp, "--stdin", "--output=yaml", "docker")
	if r.exit != 0 || ran {
		t.Fatalf("yaml: exit %d ran=%v stderr: %s", r.exit, ran, r.stderr)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(r.stdout), &doc); err != nil || doc["name"] != "docker" {
		t.Errorf("yaml output = %v (err %v):\n%s", doc["name"], err, r.stdout)
	}
}

func TestStdin_worksForACLIThatIsNotInstalled(t *testing.T) {
	help := strings.ReplaceAll(stdinHelp, "docker", "podman9")
	r, _ := runStdin(t, help, "--stdin", "--output=json", "podman9")
	if r.exit != 0 {
		t.Fatalf("exit %d, stderr: %s", r.exit, r.stderr)
	}
	if root := parseJSON(t, r.stdout); root.Name != "podman9" || len(root.Children) != 3 {
		t.Errorf("root = %q with %d children, want podman9 with 3", root.Name, len(root.Children))
	}
}

func TestStdin_emptyInputFails(t *testing.T) {
	r, ran := runStdin(t, "  \n", "--stdin", "docker")
	if r.exit != 1 || ran {
		t.Fatalf("exit %d ran=%v, want 1 and nothing run; stderr: %s", r.exit, ran, r.stderr)
	}
	mustContain(t, "stderr", r.stderr, "no help text on stdin")
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty on failure", r.stdout)
	}
}

func TestStdin_flagsThatWouldRunTheCLIAreUsageErrors(t *testing.T) {
	for _, flag := range []string{"-i", "--expand-stubs=1"} {
		r, ran := runStdin(t, stdinHelp, "--stdin", flag, "docker")
		if r.exit != 2 || ran {
			t.Errorf("%s: exit %d ran=%v, want 2 and nothing run; stderr: %s", flag, r.exit, ran, r.stderr)
		}
		mustContain(t, flag+" stderr", r.stderr, "--stdin")
	}
}

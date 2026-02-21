package discovery_test

import (
"testing"

"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
)

// mockCobraHelp is a cobra-style help output (kubectl-like).
const mockCobraHelp = `kubectl controls the Kubernetes cluster manager.

 Find more information at: https://kubernetes.io/docs/reference/kubectl/

Usage:
  kubectl [command]

Available Commands:
  apply       Apply a configuration to a resource by file name or stdin
  get         Display one or many resources
  delete      Delete resources by file names, stdin, resources
  describe    Show details of a specific resource
  exec        Execute a command in a container
  help        Help about any command
  version     Print the client and server version information

Flags:
      --add-dir-header                   If true, adds the file directory
  -h, --help                             help for kubectl
      --kubeconfig string                Path to the kubeconfig file
      --log-backtrace-at traceLocation   when logging hits line file:N
  -n, --namespace string                 If present, the namespace scope
  -v, --v Level                          number for the log level verbosity

Use "kubectl [command] --help" for more information about a command.
`

// mockGitHelp is a git-style help output.
const mockGitHelp = `usage: git [--version] [--help] [-C <path>] [-c <name>=<value>]
           [--exec-path[=<path>]] [--html-path] [--man-path] [--info-path]
           [-p | --paginate | -P | --no-pager] [--no-replace-objects] [--bare]
           [--git-dir=<path>] [--work-tree=<path>] [--namespace=<name>]
           [--super-prefix=<path>] [--config-env=<name>=<envvar>]
           <command> [<args>]

These are common Git commands used in various situations:

start a working area (see also: git help tutorial)
   clone     Clone a repository into a new directory
   init      Create an empty Git repository

work on the current change (see also: git help everyday)
   add       Add file contents to the index
   commit    Record changes to the repository
   diff      Show changes between commits
   restore   Restore working tree files

examine the history and state (see also: git help revisions)
   log       Show the commit logs
   status    Show the working tree status
`

// mockUnixHelp is a GNU-style help output (ls-like).
const mockUnixHelp = `Usage: ls [OPTION]... [FILE]...
List information about the FILEs (the current directory by default).
Sort entries alphabetically if none of -cftuvSUX nor --sort is specified.

  -a, --all                  do not ignore entries starting with .
  -A, --almost-all           do not list implied . and ..
  -l                         use a long listing format
      --color[=WHEN]         colorize the output
  -h, --human-readable       with -l and -s, print sizes like 1K 234M
  -r, --reverse              reverse order while sorting
  -s, --size                 print the allocated size of each file
`

// mockArgparseHelp is a Python argparse-style help output.
const mockArgparseHelp = `usage: pip [options] <command> [args]

Commands:
  install                     Install packages.
  download                    Download packages.
  uninstall                   Uninstall packages.
  list                        List installed packages.
  show                        Show information about installed packages.
  search                      Search PyPI for packages.

General Options:
  -h, --help                  Show help.
  -v, --verbose               Give more output.
  -q, --quiet                 Give less output.
  --version                   Show version and exit.
  --log <path>                Path to a verbose appending log.
`

func TestParseHelpOutput_cobra_subcommands(t *testing.T) {
p := discovery.ParseHelpOutput(mockCobraHelp)
wantSubs := []string{"apply", "get", "delete", "describe", "exec"}
found := map[string]bool{}
for _, s := range p.Subcommands {
found[s] = true
}
for _, want := range wantSubs {
if !found[want] {
t.Errorf("expected subcommand %q, got %v", want, p.Subcommands)
}
}
// "help" and "version" are legitimate cobra subcommands — expect them
if !found["help"] {
t.Error("expected 'help' subcommand (cobra exposes it)")
}
if !found["version"] {
t.Error("expected 'version' subcommand (cobra exposes it)")
}
}

func TestParseHelpOutput_cobra_flags(t *testing.T) {
p := discovery.ParseHelpOutput(mockCobraHelp)
flags := map[string]models.Flag{}
for _, f := range p.Flags {
flags[f.Name] = f
}

if _, ok := flags["--help"]; !ok {
t.Errorf("expected --help flag, got %v", p.Flags)
}
if _, ok := flags["--kubeconfig"]; !ok {
t.Errorf("expected --kubeconfig flag, got %v", p.Flags)
}
if info, ok := flags["--kubeconfig"]; ok && info.ValueType != "string" {
t.Errorf("--kubeconfig type = %q, want string", info.ValueType)
}
if _, ok := flags["--namespace"]; !ok {
t.Errorf("expected --namespace flag")
}
if info, ok := flags["--namespace"]; ok && info.ShortName != "n" {
t.Errorf("--namespace short = %q, want n", info.ShortName)
}
}

func TestParseHelpOutput_cobra_docsURL(t *testing.T) {
p := discovery.ParseHelpOutput(mockCobraHelp)
if p.DocsURL == "" {
t.Error("expected docs URL to be extracted")
}
if !contains(p.DocsURL, "kubernetes.io") {
t.Errorf("DocsURL = %q, want kubernetes.io URL", p.DocsURL)
}
}

func TestParseHelpOutput_git_subcommands(t *testing.T) {
p := discovery.ParseHelpOutput(mockGitHelp)
wantSubs := []string{"clone", "init", "add", "commit", "diff", "log", "status"}
found := map[string]bool{}
for _, s := range p.Subcommands {
found[s] = true
}
for _, want := range wantSubs {
if !found[want] {
t.Errorf("expected subcommand %q in git help, got %v", want, p.Subcommands)
}
}
}

func TestParseHelpOutput_unix_flags(t *testing.T) {
p := discovery.ParseHelpOutput(mockUnixHelp)
flags := map[string]bool{}
for _, f := range p.Flags {
flags[f.Name] = true
}
wantFlags := []string{"--all", "--almost-all", "--human-readable", "--reverse", "--size"}
for _, want := range wantFlags {
if !flags[want] {
t.Errorf("expected flag %q in ls help, got %v", want, p.Flags)
}
}
}

func TestParseHelpOutput_unix_positionals(t *testing.T) {
p := discovery.ParseHelpOutput(mockUnixHelp)
// Usage: ls [OPTION]... [FILE]... → FILE is a real positional, OPTION is a placeholder
hasFile := false
hasOption := false
for _, pos := range p.Positionals {
switch pos.Name {
case "FILE":
hasFile = true
case "OPTION":
hasOption = true
}
}
if !hasFile {
t.Errorf("expected FILE positional, got %v", p.Positionals)
}
if hasOption {
t.Errorf("OPTION should be filtered as a placeholder, got %v", p.Positionals)
}
}

// mockGNUOptional exercises the [=WHEN] optional-value flag syntax used by GNU coreutils.
const mockGNUOptional = `Usage: demo [OPTION]... [FILE]...

  --color[=WHEN]     colorize output; WHEN can be always, auto, or never
  --hyperlink[=WHEN] hyperlink file names WHEN
  -a, --all          show all entries
`

func TestParseHelpOutput_gnu_optional_value_flags(t *testing.T) {
p := discovery.ParseHelpOutput(mockGNUOptional)
flags := map[string]models.Flag{}
for _, f := range p.Flags {
flags[f.Name] = f
}
for _, want := range []string{"--color", "--hyperlink", "--all"} {
if _, ok := flags[want]; !ok {
t.Errorf("expected flag %q to be parsed, got %v", want, p.Flags)
}
}
}

func TestParseHelpOutput_argparse_subcommands(t *testing.T) {
p := discovery.ParseHelpOutput(mockArgparseHelp)
wantSubs := []string{"install", "download", "uninstall", "list", "show"}
found := map[string]bool{}
for _, s := range p.Subcommands {
found[s] = true
}
for _, want := range wantSubs {
if !found[want] {
t.Errorf("expected subcommand %q in pip help, got %v", want, p.Subcommands)
}
}
}

func TestParseHelpOutput_argparse_flags(t *testing.T) {
p := discovery.ParseHelpOutput(mockArgparseHelp)
flags := map[string]bool{}
for _, f := range p.Flags {
flags[f.Name] = true
}
if !flags["--help"] {
t.Errorf("expected --help, got %v", p.Flags)
}
if !flags["--verbose"] {
t.Errorf("expected --verbose, got %v", p.Flags)
}
}

func TestParseHelpOutput_description(t *testing.T) {
p := discovery.ParseHelpOutput(mockCobraHelp)
if p.Description == "" {
t.Error("expected non-empty description")
}
if !contains(p.Description, "kubectl") && !contains(p.Description, "Kubernetes") {
t.Errorf("description = %q, expected kubectl/Kubernetes mention", p.Description)
}
}

func TestParseHelpOutput_empty(t *testing.T) {
p := discovery.ParseHelpOutput("")
if len(p.Subcommands) != 0 || len(p.Flags) != 0 {
t.Errorf("expected empty results for empty input, got %+v", p)
}
}


func contains(s, substr string) bool {
return len(s) >= len(substr) && (s == substr ||
len(substr) == 0 ||
containsStr(s, substr))
}

func containsStr(s, sub string) bool {
for i := 0; i <= len(s)-len(sub); i++ {
if s[i:i+len(sub)] == sub {
return true
}
}
return false
}

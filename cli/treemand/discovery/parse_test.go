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

func TestParseHelpOutput_manpageDescription(t *testing.T) {
	input := `GIT-CLONE(1)                                          Git Manual                                         GIT-CLONE(1)

NAME
       git-clone - Clone a repository into a new directory

SYNOPSIS
       git clone [--template=<template-directory>] <repository>

OPTIONS
       --depth <depth>
           Create a shallow clone.

       --quiet
           Operate quietly.
`
	parsed := discovery.ParseHelpOutput(input)
	if parsed.Description != "Clone a repository into a new directory" {
		t.Errorf("expected clean description from NAME section, got %q", parsed.Description)
	}
}

func TestParseHelpOutput_manpageHeaderNotDescription(t *testing.T) {
	input := `GIT-COMMIT(1)                  Git Manual                  GIT-COMMIT(1)

NAME
       git-commit - Record changes to the repository

OPTIONS
       --all
           Stage all modified and deleted files.
`
	parsed := discovery.ParseHelpOutput(input)
	if parsed.Description == "GIT-COMMIT(1)                  Git Manual                  GIT-COMMIT(1)" {
		t.Error("manpage header line should not be captured as description")
	}
	if parsed.Description != "Record changes to the repository" {
		t.Errorf("expected clean description, got %q", parsed.Description)
	}
}

// mockGoHelp mimics `go --help` output (tab-indented subcommand list).
const mockGoHelp = `Go is a tool for managing Go source code.

Usage:

	go <command> [arguments]

The commands are:

	bug         start a bug report
	build       compile packages and dependencies
	clean       remove object files and cached files
	doc         show documentation for package or symbol
	env         print Go environment information
	fmt         gofmt (reformat) package sources
	get         add dependencies to current module and install them
	install     compile and install packages and dependencies
	mod         module maintenance
	run         compile and run Go program
	test        test packages
	vet         report likely mistakes in packages

Use "go help <command>" for more information about a command.
`

func TestParseHelpOutput_go_tab_subcommands(t *testing.T) {
	p := discovery.ParseHelpOutput(mockGoHelp)
	wantSubs := []string{"bug", "build", "clean", "doc", "env", "fmt", "get", "install", "mod", "run", "test", "vet"}
	found := map[string]bool{}
	for _, s := range p.Subcommands {
		found[s] = true
	}
	for _, want := range wantSubs {
		if !found[want] {
			t.Errorf("expected subcommand %q in go help, got %v", want, p.Subcommands)
		}
	}
	if p.Description == "" {
		t.Error("expected non-empty description for go")
	}
}

// mockNpmHelp mimics `npm --help` output (comma-separated command list).
const mockNpmHelp = `npm <command>

Usage:

npm install        install all the dependencies in your project
npm test           run this project's tests

All commands:

    access, adduser, audit, bugs, cache, ci, completion,
    config, dedupe, diff, dist-tag, docs, exec,
    help, init, install, link, login, logout, ls,
    outdated, publish, run, search, start, stop, test,
    uninstall, update, version, view, whoami
`

func TestParseHelpOutput_npm_comma_subcommands(t *testing.T) {
	p := discovery.ParseHelpOutput(mockNpmHelp)
	wantSubs := []string{"access", "audit", "ci", "config", "diff", "dist-tag", "exec", "install", "publish", "version"}
	found := map[string]bool{}
	for _, s := range p.Subcommands {
		found[s] = true
	}
	for _, want := range wantSubs {
		if !found[want] {
			t.Errorf("expected subcommand %q in npm help, got %v", want, p.Subcommands)
		}
	}
}

// mockOpensslHelp mimics `openssl help` / `openssl --help` output (multi-column grid).
const mockOpensslHelp = `help:

Standard commands
asn1parse         ca                ciphers           cmp               
cms               crl               crl2pkcs7         dgst              
enc               genpkey           genrsa            help              
list              pkcs12            pkcs7             pkcs8             
req               rsa               s_client          s_server          
verify            version           x509              

Message Digest commands (see the 'dgst' command for more details)
blake2b512        md5               sha1              sha256            
sha512            

Cipher commands (see the 'enc' command for more details)
aes-128-cbc       aes-256-cbc       des3              rc4               
`

func TestParseHelpOutput_openssl_grid_subcommands(t *testing.T) {
	p := discovery.ParseHelpOutput(mockOpensslHelp)
	wantSubs := []string{"asn1parse", "ca", "ciphers", "cmp", "enc", "genrsa", "pkcs12", "req", "rsa", "s_client", "verify", "x509"}
	found := map[string]bool{}
	for _, s := range p.Subcommands {
		found[s] = true
	}
	for _, want := range wantSubs {
		if !found[want] {
			t.Errorf("expected subcommand %q in openssl help, got %v", want, p.Subcommands)
		}
	}
	// Digest and cipher commands should also be found
	for _, want := range []string{"md5", "sha256", "aes-128-cbc", "des3"} {
		if !found[want] {
			t.Errorf("expected subcommand %q (digest/cipher section), got %v", want, p.Subcommands)
		}
	}
}

func TestParseHelpOutputFor_treemand_self(t *testing.T) {
// This is the actual output of `treemand --help`. Previously, the parser
// incorrectly inferred bogus subcommands from the free-text sections in this
// output (h, text, json, yaml, completions, treemand).
helpText := `treemand discovers and visualizes any CLI tool as a command tree.

Point it at any binary and it maps out subcommands, flags, and positionals
by probing the tool's own --help output — no plugins, no config files.

  treemand git            prints a colored ASCII tree of git's commands
  treemand -i aws         opens an interactive TUI to explore aws

Non-interactive output includes inline flags, positional arguments, and
short descriptions. Large CLIs (aws, kubectl) create stub nodes on first
run — use -i to expand them on demand, or increase --depth.

Interactive TUI controls (press ? inside TUI for full help):
  ↑↓ / j k    navigate tree        Space / Enter  add node to command
  h H          toggle help pane     f              pick a flag
  /            fuzzy filter         Ctrl+E         copy / execute
  Esc          quit

Discovery strategies (--strategy):
  help          parse --help output (default, works on nearly every CLI)
  completions   use shell completion data (richer flag metadata)

Output formats (--output):
  text          colored tree (default)
  json          machine-readable full tree with flags and descriptions
  yaml          YAML output (same structure as JSON)

Examples:
  treemand git                        # full git tree
  treemand -i aws                     # interactive aws explorer
  treemand --depth=2 kubectl          # kubectl tree, 2 levels deep
  treemand --commands-only docker     # subcommands only, no flags
  treemand --output=json gh | jq .    # pipe JSON to jq
  treemand --filter=remote git        # only show nodes matching "remote"
  treemand treemand                   # introspect treemand itself

Docs: https://aallbrig.github.io/treemand

Usage:
  treemand <cli> [flags]
  treemand [command]

Available Commands:
  cache       Manage the treemand discovery cache
  completion  Generate shell completion scripts
  help        Help about any command
  version     Print version information

Flags:
      --commands-only        Hide flags and positionals
  -i, --interactive          Launch interactive TUI
  -h, --help                 help for treemand
`

p := discovery.ParseHelpOutputFor(helpText, "treemand")

bogus := []string{"text", "json", "yaml", "h", "completions", "treemand"}
for _, word := range bogus {
for _, sub := range p.Subcommands {
if sub == word {
t.Errorf("bogus subcommand %q should not appear; got subcommands: %v", word, p.Subcommands)
}
}
}

// The real subcommands must still be present.
want := map[string]bool{"cache": false, "completion": false, "version": false}
for _, sub := range p.Subcommands {
if _, ok := want[sub]; ok {
want[sub] = true
}
}
for name, found := range want {
if !found {
t.Errorf("expected real subcommand %q not found; got: %v", name, p.Subcommands)
}
}
}

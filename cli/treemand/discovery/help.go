// Package discovery provides strategies for discovering CLI command hierarchies.
package discovery

import (
"context"
"fmt"
"os/exec"
"regexp"
"strings"
"time"

"github.com/aallbrig/treemand/models"
)

// Discoverer is the interface for CLI hierarchy discovery strategies.
type Discoverer interface {
Name() string
Discover(ctx context.Context, cliName string, args []string) (*models.Node, error)
}

// HelpDiscoverer uses --help output to discover subcommands and flags.
type HelpDiscoverer struct {
MaxDepth int
Timeout  time.Duration
}

// NewHelpDiscoverer creates a HelpDiscoverer with sensible defaults.
func NewHelpDiscoverer(maxDepth int) *HelpDiscoverer {
if maxDepth <= 0 {
maxDepth = 3
}
return &HelpDiscoverer{MaxDepth: maxDepth, Timeout: 5 * time.Second}
}

func (h *HelpDiscoverer) Name() string { return "help" }

// Discover runs the CLI with --help and recursively discovers subcommands.
func (h *HelpDiscoverer) Discover(ctx context.Context, cliName string, args []string) (*models.Node, error) {
return h.discover(ctx, cliName, args, 0)
}

func (h *HelpDiscoverer) discover(ctx context.Context, cliName string, args []string, depth int) (*models.Node, error) {
fullPath := make([]string, 0, 1+len(args))
fullPath = append(fullPath, cliName)
fullPath = append(fullPath, args...)

node := &models.Node{
Name:       fullPath[len(fullPath)-1],
FullPath:   fullPath,
Discovered: true,
}

helpText, err := h.runHelp(ctx, cliName, args)
if err != nil || helpText == "" {
node.Description = fmt.Sprintf("(could not get help: %v)", err)
return node, nil
}
node.HelpText = helpText

parsed := ParseHelpOutput(helpText)
node.Description = parsed.Description
node.Flags = parsed.Flags
node.Positionals = parsed.Positionals

if depth < h.MaxDepth {
for _, sub := range parsed.Subcommands {
subCtx, cancel := context.WithTimeout(ctx, h.Timeout)
child, cerr := h.discover(subCtx, cliName, append(args, sub), depth+1)
cancel()
if cerr != nil {
child = &models.Node{
Name:     sub,
FullPath: append(append([]string{}, fullPath...), sub),
}
}
node.Children = append(node.Children, child)
}
}
return node, nil
}

func (h *HelpDiscoverer) runHelp(ctx context.Context, cliName string, args []string) (string, error) {
for _, flag := range []string{"--help", "-h"} {
cmdArgs := append(append([]string{}, args...), flag)
cmd := exec.CommandContext(ctx, cliName, cmdArgs...) //nolint:gosec
out, _ := cmd.CombinedOutput()
if len(strings.TrimSpace(string(out))) > 0 {
return string(out), nil
}
}
return "", fmt.Errorf("no help output from %s", cliName)
}

// ParsedHelp holds structured results of parsing --help output.
// Exported so tests and other packages can use it directly.
type ParsedHelp struct {
Description string
Flags       []models.Flag
Positionals []models.Positional
Subcommands []string
DocsURL     string
}

// section labels we recognize
const (
secNone     = ""
secFlags    = "flags"
secCommands = "commands"
secUsage    = "usage"
secDesc     = "description"
secExamples = "examples"
secAliases  = "aliases"
)

// sectionHeaders maps lower-cased keywords that appear in section header lines.
var sectionHeaders = map[string]string{
"available commands":  secCommands,
"available command":   secCommands,
"management commands": secCommands,
"management command":  secCommands,
"commands":            secCommands,
"subcommands":         secCommands,
"flags":               secFlags,
"options":             secFlags,
"global flags":        secFlags,
"global options":      secFlags,
"optional arguments":  secFlags,
"arguments":           secFlags,
"usage":               secUsage,
"use":                 secUsage,
"description":         secDesc,
"examples":            secExamples,
"example":             secExamples,
"aliases":             secAliases,
}

// regexes compiled once
var (
// long flag: --flag or --flag=type or --flag <type> or --flag type
longFlagRe = regexp.MustCompile(
`^\s{0,8}` +
`(?:(-[A-Za-z0-9])(?:,\s*|\s+))?` + // optional short: -x, or -x (space)
`(--[A-Za-z][A-Za-z0-9_-]*)` + // long: --flag
`(?:` +
`(?:[= ](?:<([^>]+)>|\[([^\]]+)\]|([A-Za-z][A-Za-z0-9_-]*)))` + // =<type> or =type
`)?` +
`(?:\s{2,}(.*))?$`, // description (2+ spaces gap)
)
// short-only flag: -v or -v <value>
shortOnlyFlagRe = regexp.MustCompile(
`^\s{2,8}(-[A-Za-z0-9])(?:\s+(?:<([^>]+)>|([A-Za-z][A-Za-z0-9_-]*)))?(?:\s{2,}(.*))?$`,
)
// subcommand line: 2–8 leading spaces, lowercase word, 2+ spaces, description
subcmdRe = regexp.MustCompile(`^\s{2,8}([a-z][a-z0-9_-]*)(?:\s{2,}(.+))?$`)
// positional in usage: <required> or [optional]
reqArgRe = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9_.-]*)>`)
optArgRe = regexp.MustCompile(`\[([A-Za-z][A-Za-z0-9_.-]*)(?:\.{3})?\]`)
// URL in help text
urlRe = regexp.MustCompile(`https?://[^\s]+`)
)

// skipSubcmdWords are words that look like subcommands but aren't.
var skipSubcmdWords = map[string]bool{
"help": true, "version": true, "true": true, "false": true,
"none": true, "all": true, "on": true, "off": true,
"yes": true, "no": true, "default": true,
}

// ParseHelpOutput parses --help output into structured ParsedHelp.
// Exported so it can be tested directly.
func ParseHelpOutput(text string) ParsedHelp {
var result ParsedHelp
lines := strings.Split(text, "\n")
section := secNone
seenSubs := map[string]bool{}
seenFlags := map[string]bool{}
usageLines := []string{}

for i, rawLine := range lines {
trimmed := strings.TrimSpace(rawLine)
lower := strings.ToLower(trimmed)

// Detect section header: "Flags:", "Available Commands:", etc.
if sec := detectSection(lower); sec != secNone {
section = sec
continue
}
// A non-indented non-empty line that ends without : resets section
// (but only if we're past the first few lines to avoid clobbering usage)
if i > 0 && trimmed != "" && !strings.HasPrefix(rawLine, " ") &&
!strings.HasPrefix(rawLine, "\t") && section != secNone &&
!strings.HasSuffix(lower, ":") {
section = secNone
}

// Capture first non-empty non-flag non-usage line as description
if result.Description == "" && i < 6 && trimmed != "" &&
!strings.HasPrefix(trimmed, "-") &&
!strings.HasPrefix(lower, "usage") &&
!strings.HasPrefix(lower, "use ") {
result.Description = trimmed
}

// Collect all usage lines for positional parsing
if strings.HasPrefix(lower, "usage:") || strings.HasPrefix(lower, "use:") ||
(section == secUsage && trimmed != "") {
usageLines = append(usageLines, rawLine)
}

// Detect docs URL anywhere in text
if result.DocsURL == "" {
if m := urlRe.FindString(rawLine); m != "" {
result.DocsURL = m
}
}

switch section {
case secFlags:
if f, ok := parseFlag(rawLine); ok && !seenFlags[f.Name] {
seenFlags[f.Name] = true
result.Flags = append(result.Flags, f)
}
case secCommands:
if m := subcmdRe.FindStringSubmatch(rawLine); m != nil {
name := m[1]
if !seenSubs[name] && !skipSubcmdWords[name] {
seenSubs[name] = true
result.Subcommands = append(result.Subcommands, name)
}
}
case secNone, secUsage:
// Outside named sections: still try to pick up flags and subcommands
// with stricter confidence checks.
if f, ok := parseFlag(rawLine); ok && !seenFlags[f.Name] {
seenFlags[f.Name] = true
result.Flags = append(result.Flags, f)
}
// Git-style free-form subcommand lists: indented word + required description.
if m2 := subcmdRe.FindStringSubmatch(rawLine); m2 != nil && m2[2] != "" {
name := m2[1]
if !seenSubs[name] && !skipSubcmdWords[name] {
seenSubs[name] = true
result.Subcommands = append(result.Subcommands, name)
}
}
}
}

// Parse positionals from all collected usage lines
for _, ul := range usageLines {
for _, p := range parsePositionals(ul) {
result.Positionals = append(result.Positionals, p)
}
}
// Deduplicate positionals
result.Positionals = dedupePositionals(result.Positionals)

return result
}

// detectSection returns a section constant if the line is a recognized header.
func detectSection(lower string) string {
if !strings.HasSuffix(lower, ":") {
return secNone
}
candidate := strings.TrimSuffix(lower, ":")
// Exact match: "flags:", "commands:", etc.
if s, ok := sectionHeaders[candidate]; ok {
return s
}
// Prefix match: "available commands:", "global flags:", etc.
// Suffix match: "general options:", "required arguments:", etc.
for kw, sec := range sectionHeaders {
if strings.HasPrefix(candidate, kw) || strings.HasSuffix(candidate, kw) {
return sec
}
}
return secNone
}

// parseFlag tries to parse a flag definition line.
func parseFlag(line string) (models.Flag, bool) {
// Try long flag regex first
if m := longFlagRe.FindStringSubmatch(line); m != nil {
f := models.Flag{
Name:      m[2],
ShortName: strings.TrimLeft(m[1], "-"),
}
switch {
case m[3] != "":
f.ValueType = m[3]
case m[4] != "":
f.ValueType = m[4]
case m[5] != "":
f.ValueType = m[5]
default:
f.ValueType = "bool"
}
f.Description = strings.TrimSpace(m[6])
return f, true
}
// Try short-only flag
if m := shortOnlyFlagRe.FindStringSubmatch(line); m != nil {
f := models.Flag{
Name:      m[1],
ShortName: strings.TrimLeft(m[1], "-"),
}
switch {
case m[2] != "":
f.ValueType = m[2]
case m[3] != "":
f.ValueType = m[3]
default:
f.ValueType = "bool"
}
f.Description = strings.TrimSpace(m[4])
return f, true
}
return models.Flag{}, false
}

// parsePositionals extracts positional args from a usage line.
func parsePositionals(line string) []models.Positional {
var result []models.Positional
seen := map[string]bool{}
// Skip the "usage:" prefix for matching
searchLine := line
if idx := strings.Index(strings.ToLower(line), "usage:"); idx >= 0 {
searchLine = line[idx+6:]
}
for _, m := range reqArgRe.FindAllStringSubmatch(searchLine, -1) {
name := m[1]
if !seen[name] && !strings.ContainsAny(name, "= ") {
seen[name] = true
result = append(result, models.Positional{Name: name, Required: true})
}
}
for _, m := range optArgRe.FindAllStringSubmatch(searchLine, -1) {
name := m[1]
if !seen[name] && !strings.ContainsAny(name, "= ") {
seen[name] = true
result = append(result, models.Positional{Name: name, Required: false})
}
}
return result
}

func dedupePositionals(ps []models.Positional) []models.Positional {
seen := map[string]bool{}
var out []models.Positional
for _, p := range ps {
if !seen[p.Name] {
seen[p.Name] = true
out = append(out, p)
}
}
return out
}

func lastElement(path []string) string {
if len(path) == 0 {
return ""
}
return path[len(path)-1]
}

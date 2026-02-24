// Package discovery provides strategies for discovering CLI command hierarchies.
package discovery

import (
"context"
"fmt"
"os"
"os/exec"
"path/filepath"
"regexp"
"strings"
"sync"
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

if depth < h.MaxDepth && len(parsed.Subcommands) > 0 {
const maxWorkers = 8
sem := make(chan struct{}, maxWorkers)
type result struct {
idx   int
child *models.Node
}
results := make([]result, len(parsed.Subcommands))
var wg sync.WaitGroup
for i, sub := range parsed.Subcommands {
wg.Add(1)
go func(i int, sub string) {
defer wg.Done()
sem <- struct{}{}
defer func() { <-sem }()
subCtx, cancel := context.WithTimeout(ctx, h.Timeout)
defer cancel()
subArgs := append(append([]string{}, args...), sub)
subFull := append(append([]string{}, fullPath...), sub)
childHelp, err := h.runHelp(subCtx, cliName, subArgs)
if err != nil || childHelp == "" {
results[i] = result{i, &models.Node{
Name:        sub,
FullPath:    subFull,
Discovered:  true,
Description: fmt.Sprintf("(could not get help: %v)", err),
}}
return
}
var child *models.Node
if childHelp == helpText {
childParsed := ParseHelpOutput(childHelp)
child = &models.Node{
Name:        sub,
FullPath:    subFull,
Discovered:  true,
HelpText:    childHelp,
Description: childParsed.Description,
Flags:       childParsed.Flags,
Positionals: childParsed.Positionals,
}
} else {
var cerr error
child, cerr = h.discover(subCtx, cliName, subArgs, depth+1)
if cerr != nil {
child = &models.Node{Name: sub, FullPath: subFull}
}
}
results[i] = result{i, child}
}(i, sub)
}
wg.Wait()
for _, r := range results {
if r.child != nil {
node.Children = append(node.Children, r.child)
}
}
} else if len(parsed.Sections) > 1 {
// No real subcommands but multiple named flag sections exist (e.g. Godot).
// Create virtual group children so the tree has meaningful structure.
for _, sec := range parsed.Sections {
slug := sectionSlug(sec.Name)
child := &models.Node{
Name:        slug,
FullPath:    append(append([]string{}, fullPath...), slug),
Description: sec.Name,
Flags:       sec.Flags,
Discovered:  true,
Virtual:     true,
}
node.Children = append(node.Children, child)
}
}
return node, nil
}

// sectionSlug converts a flag-section header like "General options" into a
// lowercase kebab-case identifier used as the virtual node name.
func sectionSlug(name string) string {
name = strings.ToLower(name)
var b strings.Builder
prevHyphen := false
for _, r := range name {
if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
b.WriteRune(r)
prevHyphen = false
} else if !prevHyphen {
b.WriteByte('-')
prevHyphen = true
}
}
return strings.Trim(b.String(), "-")
}

// resolveBinary finds the executable for cliName.
// Tries PATH first, then ./cliName (current dir), then the directory of the
// running executable so that "treemand treemand" works without PATH changes.
func resolveBinary(cliName string) string {
p, _ := resolveBinaryOrError(cliName)
return p
}

// resolveBinaryOrError is like resolveBinary but returns an error when the
// binary cannot be located anywhere on the system.
func resolveBinaryOrError(cliName string) (string, error) {
if p, err := exec.LookPath(cliName); err == nil {
return p, nil
}
// Try the current working directory
if cwd, err := os.Getwd(); err == nil {
candidate := filepath.Join(cwd, cliName)
if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
return candidate, nil
}
}
// Try the directory of the running executable (e.g. ./treemand treemand)
if exe, err := os.Executable(); err == nil {
candidate := filepath.Join(filepath.Dir(exe), cliName)
if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
return candidate, nil
}
}
return cliName, fmt.Errorf("command %q not found in PATH or current directory", cliName)
}

// CheckAvailable returns an error if cliName cannot be resolved to an
// executable. Call this before starting discovery to give the user a clear
// error message instead of a cryptic "no help output" stub node.
func CheckAvailable(cliName string) error {
_, err := resolveBinaryOrError(cliName)
return err
}

// pagerEnv are environment variable overrides appended to every help command
// so that tools that pipe through a pager (AWS, man, etc.) emit plain text.
var pagerEnv = []string{
"AWS_PAGER=",
"PAGER=cat",
"MANPAGER=cat",
"GIT_PAGER=cat",
}

// truncatedHelpRe matches messages that indicate --help output is abbreviated
// and a more complete form is available (e.g. curl's "use --help all").
var truncatedHelpRe = regexp.MustCompile(`(?i)--help all|--help <category>|not the full help`)

// runHelp tries to get help text for args under cliName.
// It attempts --help / -h first, then `help` as a positional fallback
// (needed for tools like aws that use "aws help" instead of "aws --help").
func (h *HelpDiscoverer) runHelp(ctx context.Context, cliName string, args []string) (string, error) {
resolved := resolveBinary(cliName)

// Helper that runs a command with pager env vars and returns trimmed output.
run := func(cmdArgs []string) string {
cmd := exec.CommandContext(ctx, resolved, cmdArgs...) //nolint:gosec
cmd.Env = append(os.Environ(), pagerEnv...)
out, _ := cmd.CombinedOutput()
return strings.TrimSpace(string(out))
}

var firstOut string
for _, flag := range []string{"--help", "-h"} {
cmdArgs := append(append([]string{}, args...), flag)
s := run(cmdArgs)
if s == "" {
continue
}
// Detect truncated help (e.g. curl) and retry with --help all.
if truncatedHelpRe.MatchString(s) {
allArgs := append(append([]string{}, args...), "--help", "all")
if s2 := run(allArgs); s2 != "" {
return s2, nil
}
}
if firstOut == "" {
firstOut = s
}
}

// Fallback: some CLIs (aws, man-page wrappers) use `<cli> [sub...] help`
// as a positional rather than a flag.
if firstOut == "" {
helpArgs := append(append([]string{}, args...), "help")
if s := run(helpArgs); s != "" {
// Avoid returning the parent's "how to get help" message as content.
if !strings.Contains(strings.ToLower(s), "aws help\n  aws") {
firstOut = s
}
}
}

if firstOut != "" {
return firstOut, nil
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
// Sections holds named flag groups (e.g. Godot's "General options:",
// "Debug options:"). Only populated when multiple distinct sections exist.
Sections    []ParsedSection
}

// ParsedSection is a named group of flags found under a section header.
type ParsedSection struct {
Name  string
Flags []models.Flag
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
"available commands":   secCommands,
"available command":    secCommands,
"available services":   secCommands, // AWS man-page style
"available service":    secCommands,
"management commands":  secCommands,
"management command":   secCommands,
"commands":             secCommands,
"subcommands":          secCommands,
"flags":                secFlags,
"options":              secFlags,
"global flags":         secFlags,
"global options":       secFlags,
"optional arguments":   secFlags,
"arguments":            secFlags,
"usage":                secUsage,
"use":                  secUsage,
"description":          secDesc,
"examples":             secExamples,
"example":              secExamples,
"aliases":              secAliases,
}

// awsBulletRe matches AWS man-page bullet list items: "       +o word"
var awsBulletRe = regexp.MustCompile(`^\s{2,}\+o\s+([a-z][a-z0-9_-]*)$`)

// awsFlagRe matches AWS man-page flag lines: "       --flag (type)"
var awsFlagRe = regexp.MustCompile(`^\s{2,}(--[A-Za-z][A-Za-z0-9_-]*)\s+\(([^)]+)\)\s*$`)

// regexes compiled once
var (
// long flag: --flag or --flag=type or --flag <type> or --flag type
longFlagRe = regexp.MustCompile(
`^\s{0,8}` +
`(?:(-[A-Za-z0-9])(?:,\s*|\s+))?` + // optional short: -x, or -x (space)
`(--[A-Za-z][A-Za-z0-9_-]*)` + // long: --flag
`(?:` +
`(?:\[=[A-Za-z][A-Za-z0-9_-]*\])` + // [=WHEN] GNU optional-value style
`|(?:[= ](?:<([^>]+)>|\[([^\]]+)\]|([A-Za-z][A-Za-z0-9_-]*)))` + // =<type> or =type
`)?` +
`(?:\s+(.*))?$`, // description (1+ spaces gap)
)
// short-only flag: -v or -v <value>
shortOnlyFlagRe = regexp.MustCompile(
`^\s{2,8}(-[A-Za-z0-9])(?:\s+(?:<([^>]+)>|([A-Za-z][A-Za-z0-9_-]*)))?(?:\s{2,}(.*))?$`,
)
// subcommand line: 2–8 leading spaces, lowercase word; args like [PATTERN...] may appear
// between name and description (e.g. systemctl's "  list-units [PAT...]   description")
subcmdRe = regexp.MustCompile(`^\s{2,8}([a-z][a-z0-9_-]*)(?:.*?\s{2,}(.+))?$`)
// positional in usage: <required> or [optional]
reqArgRe = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9_.-]*)>`)
optArgRe = regexp.MustCompile(`\[([A-Za-z][A-Za-z0-9_.-]*)(?:\.{3})?\]`)
// URL in help text
urlRe = regexp.MustCompile(`https?://[^\s]+`)
// ANSI escape codes (colors, etc.)
ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHF]`)
)

// buildMarkerRe matches Godot-style build-availability markers at the start of
// a flag description: a single uppercase letter (R/D/X/E) followed by 2+ spaces.
// e.g. "R  Display this help message." → "Display this help message."
var buildMarkerRe = regexp.MustCompile(`^[A-Z]\s{2,}`)

// stripBuildMarker removes a leading build-availability marker if present.
func stripBuildMarker(s string) string {
return strings.TrimSpace(buildMarkerRe.ReplaceAllString(s, ""))
}

// skipSubcmdWords are words that look like subcommands but aren't.
var skipSubcmdWords = map[string]bool{
"true": true, "false": true,
"none": true, "all": true, "on": true, "off": true,
"yes": true, "no": true, "default": true,
}

// stripANSI removes ANSI terminal escape codes from a string.
func stripANSI(s string) string {
return ansiRe.ReplaceAllString(s, "")
}

// ParseHelpOutput parses --help output into structured ParsedHelp.
// Exported so it can be tested directly.
func ParseHelpOutput(text string) ParsedHelp {
text = stripANSI(text)
var result ParsedHelp
lines := strings.Split(text, "\n")
section := secNone
seenSubs := map[string]bool{}
seenFlags := map[string]bool{}
usageLines := []string{}

// Section-grouping state: track the current flag-section header name so we
// can build ParsedSection children for tools like Godot.
currentSectionName := ""
sectionFlagCount := map[string]int{} // section name → flag count added so far

// pendingFlag holds a partially-parsed AWS-style flag whose description is
// on the next non-empty line ("--flag (type)" followed by "   description").
var pendingFlag *models.Flag

// addFlag appends a flag to result.Flags and (if we are in a named section)
// also to the corresponding ParsedSection entry.
addFlag := func(f models.Flag) {
if seenFlags[f.Name] {
return
}
seenFlags[f.Name] = true
result.Flags = append(result.Flags, f)
if currentSectionName != "" {
n := len(result.Sections)
if n == 0 || result.Sections[n-1].Name != currentSectionName {
result.Sections = append(result.Sections, ParsedSection{Name: currentSectionName})
n++
}
result.Sections[n-1].Flags = append(result.Sections[n-1].Flags, f)
sectionFlagCount[currentSectionName]++
}
}

for i, rawLine := range lines {
trimmed := strings.TrimSpace(rawLine)
lower := strings.ToLower(trimmed)

// Flush a pending AWS-style flag when we encounter a non-empty line.
if pendingFlag != nil && trimmed != "" {
// If the next line is another flag, don't use it as a description.
if !strings.HasPrefix(trimmed, "--") && awsFlagRe.FindString(rawLine) == "" {
pendingFlag.Description = trimmed
}
if !seenFlags[pendingFlag.Name] {
addFlag(*pendingFlag)
}
pendingFlag = nil
}

// Detect section header: "Flags:", "Available Commands:", "GLOBAL OPTIONS", etc.
if sec := detectSection(lower); sec != secNone {
// For named flag-group sections (e.g. "General options:", "Debug options:"),
// remember the human-readable name so flags get grouped under it.
if sec == secFlags {
// Capture the raw header text (without trailing colon) as the section name.
currentSectionName = strings.TrimSuffix(trimmed, ":")
} else {
currentSectionName = ""
}
section = sec
continue
}
// A non-indented non-empty line resets the section — EXCEPT for
// man-page headers that are themselves section detectors (handled above).
// We only reset when it's clearly not a section header.
if i > 4 && trimmed != "" && !strings.HasPrefix(rawLine, " ") &&
!strings.HasPrefix(rawLine, "\t") && section != secNone &&
!strings.HasSuffix(lower, ":") &&
detectSection(lower) == secNone {
// Man-page footers like "TOOLNAME()" at end of page shouldn't reset.
if !strings.HasSuffix(trimmed, "()") {
section = secNone
currentSectionName = ""
}
}

// Capture first non-empty non-flag non-usage line as description.
// For man-page format, skip the "NAME\n   cmd -" header lines.
if result.Description == "" && i < 10 && trimmed != "" &&
!strings.HasPrefix(trimmed, "-") &&
!strings.HasPrefix(lower, "usage") &&
!strings.HasPrefix(lower, "use ") &&
!strings.HasPrefix(lower, "name") &&
!strings.HasPrefix(lower, "synopsis") &&
lower != "name" && lower != "synopsis" && lower != "description" {
result.Description = trimmed
}

// Collect all usage / synopsis lines for positional parsing.
if strings.HasPrefix(lower, "usage:") || strings.HasPrefix(lower, "use:") ||
lower == "synopsis" ||
(section == secUsage && trimmed != "") {
usageLines = append(usageLines, rawLine)
}

// Detect docs URL anywhere in text.
if result.DocsURL == "" {
if m := urlRe.FindString(rawLine); m != "" {
result.DocsURL = m
}
}

switch section {
case secFlags:
// AWS man-page flag style: "       --flag (type)"
if m := awsFlagRe.FindStringSubmatch(rawLine); m != nil {
f := models.Flag{Name: m[1], ValueType: m[2]}
if m[2] == "boolean" {
f.ValueType = "bool"
}
pendingFlag = &f
continue
}
if f, ok := parseFlag(rawLine); ok {
addFlag(f)
}
case secCommands:
// AWS man-page bullet: "       +o subcmd"
if m := awsBulletRe.FindStringSubmatch(rawLine); m != nil {
name := m[1]
if !seenSubs[name] && !skipSubcmdWords[name] {
seenSubs[name] = true
result.Subcommands = append(result.Subcommands, name)
}
continue
}
if m := subcmdRe.FindStringSubmatch(rawLine); m != nil {
name := m[1]
if !seenSubs[name] && !skipSubcmdWords[name] {
seenSubs[name] = true
result.Subcommands = append(result.Subcommands, name)
}
}
case secNone, secUsage:
// Outside named sections: pick up flags and subcommands with stricter checks.
if f, ok := parseFlag(rawLine); ok {
addFlag(f)
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

// Flush any trailing pending flag.
if pendingFlag != nil {
addFlag(*pendingFlag)
}

// Drop sections that have fewer than 2 flags — they are noise.
{
filtered := result.Sections[:0]
for _, s := range result.Sections {
if len(s.Flags) >= 2 {
filtered = append(filtered, s)
}
}
result.Sections = filtered
_ = sectionFlagCount // used implicitly via addFlag
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
// Handles "Title Case:" (cobra/click style) and "UPPER CASE" (man/AWS style).
func detectSection(lower string) string {
// Strip optional trailing colon present in cobra/click style.
candidate := strings.TrimSuffix(strings.TrimSpace(lower), ":")

// Exact match handles most cases ("flags", "commands", "global options", etc.)
if s, ok := sectionHeaders[candidate]; ok {
return s
}

// Prefix/suffix match for compound headers like "general options:",
// "required arguments:", "available services:", etc.
// Only apply to short candidates (section headers are rarely > 30 chars);
// this prevents false-positives on lines like "Usage: ls [OPTION]... [FILE]..."
if len(candidate) <= 30 {
for kw, sec := range sectionHeaders {
if strings.HasPrefix(candidate, kw) || strings.HasSuffix(candidate, kw) {
return sec
}
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
f.Description = stripBuildMarker(m[6])
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
f.Description = stripBuildMarker(m[4])
return f, true
}
return models.Flag{}, false
}

// positionalPlaceholders are all-caps words in usage lines that represent
// option/flag slots, not real positional arguments.
var positionalPlaceholders = map[string]bool{
"OPTION": true, "OPTIONS": true, "OPTS": true, "OPT": true,
"FLAG": true, "FLAGS": true, "ARG": true, "ARGS": true,
"ARGUMENTS": true, "PARAMS": true, "PARAMETERS": true,
"SHORT-OPTION": true, "LONG-OPTION": true,
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
canonical := strings.TrimRight(strings.ToUpper(name), ".+")
if !seen[name] && !strings.ContainsAny(name, "= ") && !positionalPlaceholders[canonical] {
seen[name] = true
result = append(result, models.Positional{Name: name, Required: true})
}
}
for _, m := range optArgRe.FindAllStringSubmatch(searchLine, -1) {
name := m[1]
canonical := strings.TrimRight(strings.ToUpper(name), ".+")
if !seen[name] && !strings.ContainsAny(name, "= ") && !positionalPlaceholders[canonical] {
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

package discovery

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

// ParsedHelp holds structured results of parsing --help output.
// Exported so tests and other packages can use it directly.
type ParsedHelp struct {
	Description string
	Flags       []models.Flag
	Positionals []models.Positional
	Subcommands []string
	// SubcommandDescs holds the one-line description each subcommand has in
	// this command's own command list; used when the child's help has none.
	SubcommandDescs map[string]string
	DocsURL         string
	// Sections holds named flag groups (e.g. Godot's "General options:",
	// "Debug options:"). Only populated when multiple distinct sections exist.
	Sections []ParsedSection
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
	secName     = "name" // man-page NAME section
	secExamples = "examples"
	secAliases  = "aliases"
	secValues   = "values" // a list under a header the parser does not know (#154)
)

// ParseHelpOutput parses --help output into structured ParsedHelp.
// Exported so it can be tested directly.
func ParseHelpOutput(text string) ParsedHelp {
	return ParseHelpOutputFor(text, "")
}

// ParseHelpOutputFor parses --help output with knowledge of the CLI name being
// introspected, so self-referential example lines don't create bogus subcommands.
func ParseHelpOutputFor(text, selfName string) ParsedHelp {
	text = stripANSI(text)
	text = stripManpageFormatting(text)
	p := &helpParser{
		lines:           strings.Split(text, "\n"),
		selfName:        selfName,
		seenSubs:        map[string]bool{},
		seenFlags:       map[string]bool{},
		usageEnd:        -1,
		choiceIndent:    -1,
		choiceRowIndent: -1,
	}
	for i, raw := range p.lines {
		trimmed := strings.TrimSpace(raw)
		p.line(helpLine{i: i, raw: raw, trimmed: trimmed, lower: strings.ToLower(trimmed)})
	}
	return p.finish()
}

// helpLine is one line of help text in the forms the rules test.
type helpLine struct {
	i                   int
	raw, trimmed, lower string
}

// helpParser is one pass over a help text. Each rule is a method; line runs
// them in order, and a rule that consumes the line ends it.
type helpParser struct {
	lines    []string
	selfName string
	result   ParsedHelp

	section    string
	bulletSubs []string // AWS man-page "o service" bullets: authoritative when 3+
	seenSubs   map[string]bool
	seenFlags  map[string]bool
	usageLines []string

	// inNameSection: inside the man-page NAME section, whose
	// "   git-clone - Short desc" line is the cleanest description.
	inNameSection bool
	// currentSectionName names the flag group being read ("General options"),
	// so flags are grouped under it (Godot).
	currentSectionName string
	// pendingFlag is an AWS-style "--flag (type)" (or kubectl "--flag=false:")
	// whose description is on the next non-empty line.
	pendingFlag *models.Flag

	// inUsageBlock is true from a "usage:" line through its indented
	// continuation lines, which are synopsis text and never a description.
	inUsageBlock bool
	// descSkipped counts lines that cannot be the description and should not
	// use up the window it is looked for in: banners, and the value lists
	// under unknown headers (rsync's "Capabilities:"). usageEnd is where a
	// usage block ended: the paragraph after a long wrapped usage (argparse)
	// is the description wherever it falls.
	descSkipped, usageEnd int
	// argparse subparser block (#81): a "{a,b,c…}" line, then one row per
	// subcommand at the first row's indent; deeper rows continue a description.
	choiceIndent, choiceRowIndent int
	lastGridRow                   bool
	inGrammar                     bool // inside "OBJECT := { a | b … }" (ip, #160)
}

func (p *helpParser) line(l helpLine) {
	p.trackUsageBlock(l)
	p.flushPendingFlag(l)
	if p.sectionHeader(l) || p.choiceBlock(l) || p.commandGrid(l) || p.unknownHeader(l) {
		return
	}
	p.maybeResetSection(l)
	p.captureDescription(l)
	p.collectUsageAndURL(l)

	switch p.section {
	case secFlags:
		p.flagLine(l)
	case secCommands:
		// No command name starts with "-": a described option row here is a
		// flag. "go help build" wraps "...shared by the build, clean, get,
		// install, list, run,\nand test commands:" over its flag list (#66).
		// An undescribed one is a wrapped example ("    --lint=none").
		if f, ok := parseFlag(l.raw); ok && strings.HasPrefix(l.trimmed, "-") {
			if f = p.withNextLineDescription(l, f); f.Description != "" {
				p.addFlag(f)
			}
			return
		}
		p.commandLine(l)
	case secExamples, secAliases, secDesc, secValues:
		// Narrative text, examples or aliases: example usage may reference
		// flags worth surfacing, but these never list subcommands.
		p.addParsedFlag(l)
	case secNone, secUsage:
		// Outside named sections: flags, and subcommands with stricter checks.
		p.addParsedFlag(l)
		p.proseFlags(l)
		p.freeFormSubcommand(l.raw)
	}
}

func (p *helpParser) trackUsageBlock(l helpLine) {
	switch {
	case strings.HasPrefix(l.lower, "usage:"):
		p.inUsageBlock = true
	case l.trimmed == "" || (l.raw[0] != ' ' && l.raw[0] != '\t'):
		if p.inUsageBlock {
			p.usageEnd = l.i
		}
		p.inUsageBlock = false
	}
	if p.result.Description == "" && l.trimmed != "" &&
		(bannerRe.MatchString(l.trimmed) || (p.section == secValues && l.raw[0] == ' ') || versionRowRe.MatchString(l.raw)) {
		p.descSkipped++
	}
}

// flushPendingFlag completes a pending flag at the next non-empty line, which
// is its description unless it is another flag.
func (p *helpParser) flushPendingFlag(l helpLine) {
	if p.pendingFlag == nil || l.trimmed == "" {
		return
	}
	if !strings.HasPrefix(l.trimmed, "--") && awsFlagRe.FindString(l.raw) == "" {
		p.pendingFlag.Description = l.trimmed
	}
	if !p.seenFlags[p.pendingFlag.Name] {
		p.addFlag(*p.pendingFlag)
	}
	p.pendingFlag = nil
}

// sectionHeader detects "Flags:", "Available Commands:", "GLOBAL OPTIONS"…
// Bullet items ("o appconfig") are never headers, even when the name ends in
// a header keyword such as "config" or "cache". Inside a command list, a line
// indented 4+ spaces belongs to the previous row's description (kingpin's
// "    examples:"), and so does an indented bare word (a cleo namespace label).
func (p *helpParser) sectionHeader(l helpLine) bool {
	describesRow := p.section == secCommands &&
		(indentOf(l.raw) >= 4 || (indentOf(l.raw) >= 1 && !strings.ContainsAny(l.trimmed, " :")))
	sec := detectSection(l.lower)
	if sec == secNone || awsBulletRe.MatchString(l.raw) || describesRow {
		return false
	}
	// A named flag group ("General options:") groups the flags under it.
	p.currentSectionName = ""
	if sec == secFlags {
		p.currentSectionName = strings.TrimSuffix(l.trimmed, ":")
	}
	p.section = sec
	p.inNameSection = sec == secName
	return true
}

// choiceBlock reads subcommand lists that are not rows: a grammar of the
// command word ("OBJECT := { address | link … }", possibly wrapped; only OBJECT
// and COMMAND, as "OPTIONS := {…}" lists options) and argparse's "{a,b,c}"
// choices with their rows.
func (p *helpParser) choiceBlock(l helpLine) bool {
	if p.inGrammar {
		p.addGrammar(l.trimmed)
		return true
	}
	if m := grammarRe.FindStringSubmatch(l.trimmed); m != nil && (m[1] == "OBJECT" || m[1] == "COMMAND") {
		p.addGrammar(m[2])
		return true
	}
	if strings.HasPrefix(l.trimmed, "{") && !p.inUsageBlock && !strings.Contains(l.trimmed, " ") {
		p.choiceIndent, p.choiceRowIndent = indentOf(l.raw), -1
		if strings.HasSuffix(l.trimmed, "}") {
			for _, n := range strings.Split(strings.Trim(l.trimmed, "{}"), ",") {
				p.addChoice(strings.TrimSpace(n), "")
			}
		}
		return true
	}
	if p.choiceIndent < 0 || l.trimmed == "" {
		return false
	}
	ind := indentOf(l.raw)
	if ind <= p.choiceIndent {
		p.choiceIndent = -1
		return false
	}
	if p.choiceRowIndent < 0 {
		p.choiceRowIndent = ind
	}
	if m := argparseRowRe.FindStringSubmatch(l.raw); m != nil && ind == p.choiceRowIndent {
		p.addChoice(m[1], strings.TrimSpace(m[2]))
	}
	return true
}

// commandGrid reads flush-left rows inside a commands section before they can
// reset it: openssl's multi-column grid of names, and kingpin's (#165)
// "check config [<flags>] <files>..." rows, each followed by an indented
// description, whose first word is the subcommand.
func (p *helpParser) commandGrid(l helpLine) bool {
	if p.section != secCommands || l.trimmed == "" || strings.HasPrefix(l.raw, " ") || strings.HasPrefix(l.raw, "\t") {
		p.lastGridRow = false
		return false
	}
	parts := strings.Fields(l.raw)
	// A grid's last row may hold a single entry (openssl "x509", #162).
	if (len(parts) >= 2 || (len(parts) == 1 && p.lastGridRow)) && allGridEntries(parts) {
		for _, name := range parts {
			if !p.seenSubs[name] && !skipSubcmdWords[name] {
				p.seenSubs[name] = true
				p.result.Subcommands = append(p.result.Subcommands, name)
			}
		}
		p.lastGridRow = true
		return true
	}
	p.lastGridRow = false
	if m := kingpinRowRe.FindStringSubmatch(l.raw); m != nil && nextLineIndented(p.lines, l.i) {
		p.addChoice(m[1], "")
		return true
	}
	return false
}

// unknownHeader handles a header the parser does not know ("Features:",
// "Dump List:", " FORMAT is one of the following:"): it introduces a list of
// values, not of commands (#154), so flags are read there but no subcommands.
// One that names commands ("These are common Git commands …:", stripe's
// "Available Operations:") stays free-form, and a multi-call binary's function
// list (busybox "Currently defined functions:") is a command list (#158).
func (p *helpParser) unknownHeader(l helpLine) bool {
	if l.trimmed == "" || !strings.HasSuffix(l.trimmed, ":") || len(l.raw)-len(strings.TrimLeft(l.raw, " ")) > 2 ||
		strings.HasPrefix(l.trimmed, "-") || p.inUsageBlock {
		return false
	}
	if strings.Contains(l.lower, "functions") {
		p.section = secCommands
		return true
	}
	if namesCommands(l.lower) {
		return false
	}
	p.section = secValues
	p.currentSectionName = ""
	p.inNameSection = false
	return true
}

// maybeResetSection: a flush-left line that is not a header ends the section,
// except a man-page footer like "TOOLNAME()".
func (p *helpParser) maybeResetSection(l helpLine) {
	if l.i > 4 && l.trimmed != "" && !strings.HasPrefix(l.raw, " ") &&
		!strings.HasPrefix(l.raw, "\t") && p.section != secNone &&
		!strings.HasSuffix(l.lower, ":") && detectSection(l.lower) == secNone &&
		!strings.HasSuffix(l.trimmed, "()") {
		p.section = secNone
		p.currentSectionName = ""
		p.inNameSection = false
	}
}

// captureDescription: first the man-page NAME line ("   git-clone - Short
// description"), else the first prose line near the top.
func (p *helpParser) captureDescription(l helpLine) {
	if p.result.Description != "" || l.trimmed == "" {
		return
	}
	if p.inNameSection {
		if m := nameSectionDescRe.FindStringSubmatch(l.raw); m != nil {
			p.result.Description = strings.TrimSpace(m[1])
		}
	}
	inWindow := l.i < 10+p.descSkipped || (p.usageEnd >= 0 && l.i <= p.usageEnd+2)
	if p.result.Description == "" && inWindow && p.isProse(l) {
		p.result.Description = l.trimmed
	}
}

// isProse: the line can be the description: not a header, flag, usage,
// banner, list row, argparse choices or grammar.
func (p *helpParser) isProse(l helpLine) bool {
	return !p.inUsageBlock &&
		!bannerRe.MatchString(l.trimmed) && !versionRowRe.MatchString(l.raw) &&
		!nameVersionRe.MatchString(l.trimmed) && !authorLineRe.MatchString(l.trimmed) &&
		!inNonProseList(p.section, l.raw) &&
		!strings.HasPrefix(l.trimmed, "{") && // argparse choices: "{init,run,…}"
		!strings.Contains(l.trimmed, ":= ") && strings.Count(l.trimmed, "|") < 3 && // grammar: "OBJECT := { a | b }"
		!introducesIndentedList(p.lines, l.i) && !listRowRe.MatchString(l.raw) &&
		!manpageHeaderRe.MatchString(l.trimmed) && // "GIT-CLONE(1)   Git Manual"
		!strings.HasPrefix(l.trimmed, "-") &&
		!isUsageErrorLine(l.lower) &&
		!strings.HasSuffix(l.trimmed, " -") && // empty man NAME body: "aws -"
		!isUsageLine(l.trimmed, p.selfName) &&
		!strings.HasSuffix(l.trimmed, ":") && // section header, not prose
		!strings.HasPrefix(l.lower, "usage") &&
		!strings.HasPrefix(l.lower, "use ") &&
		!strings.HasPrefix(l.lower, "name") &&
		!strings.HasPrefix(l.lower, "synopsis") &&
		l.lower != "description"
}

// isUsageErrorLine: the line is an argument parser rejecting an option
// ("ls: unrecognized option `--help'", "error: unknown flag"), which BSD and
// getopt tools print before their usage. It is an error, never a description.
// The marker must open the line or follow a "prog: " prefix, so prose that
// mentions one ("Ignore unknown options") still counts.
func isUsageErrorLine(lower string) bool {
	for _, m := range usageErrorMarkers {
		if strings.HasPrefix(lower, m) || strings.Contains(lower, ": "+m) {
			return true
		}
	}
	return false
}

// collectUsageAndURL keeps usage / synopsis lines, with the indented lines
// that continue a "usage:" line (ssh, tmux, a second BSD synopsis), for
// positional and flag parsing, and the first docs URL anywhere in the text.
func (p *helpParser) collectUsageAndURL(l helpLine) {
	if strings.HasPrefix(l.lower, "usage:") || strings.HasPrefix(l.lower, "use:") ||
		l.lower == "synopsis" || (p.section == secUsage && l.trimmed != "") ||
		(p.inUsageBlock && l.trimmed != "") {
		p.usageLines = append(p.usageLines, l.raw)
	}
	if p.result.DocsURL == "" {
		if m := urlRe.FindString(l.raw); m != "" {
			p.result.DocsURL = m
		}
	}
}

func (p *helpParser) flagLine(l helpLine) {
	// AWS man-page flag style: "       --flag (type)"
	if m := awsFlagRe.FindStringSubmatch(l.raw); m != nil {
		f := models.Flag{Name: m[1], ValueType: m[2]}
		if m[2] == "boolean" {
			f.ValueType = "bool"
		}
		p.pendingFlag = &f
		return
	}
	f, ok := parseFlag(l.raw)
	if !ok {
		return
	}
	// kubectl style: "--flag=false:" then the description on the next line.
	if f.Description == "" && strings.HasSuffix(l.trimmed, ":") {
		p.pendingFlag = &f
		return
	}
	p.addFlag(p.withNextLineDescription(l, f))
}

// commandLine reads one row of a commands section; the first shape that
// matches wins.
func (p *helpParser) commandLine(l helpLine) {
	// Thor (#167): "  thor install NAME    # Install …": the row starts with
	// the program's own name, the command is the next word.
	if m := thorRowRe.FindStringSubmatch(l.raw); m != nil && m[1] == p.selfName {
		p.addChoice(m[2], strings.TrimSpace(m[3]))
		return
	}
	// Man-page command reference (#159): "bundle install(1) …", "git-clone(1)".
	if m := manRefRowRe.FindStringSubmatch(l.raw); m != nil && m[1] == p.selfName {
		p.addChoice(m[2], "")
		return
	}
	// Namespaced command (cleo, #166): "  cache clear   Clears …": the
	// namespace is the subcommand, its words below it.
	if m := namespaceRowRe.FindStringSubmatch(l.raw); m != nil {
		p.addChoice(m[1], "")
		return
	}
	// Bullet list (yarn 1, #157): "    - access".
	if m := bulletCmdRe.FindStringSubmatch(l.raw); m != nil {
		p.addChoice(m[1], "")
		return
	}
	// Tab-indented subcommand (Go toolchain style): "\tbug  start a bug report"
	if m := goTabSubcmdRe.FindStringSubmatch(l.raw); m != nil {
		p.addSub(m[1])
		return
	}
	if p.commaList(l) {
		return
	}
	// AWS man-page bullet: "       +o subcmd"
	if m := awsBulletRe.FindStringSubmatch(l.raw); m != nil {
		if !slices.Contains(p.bulletSubs, m[1]) {
			p.bulletSubs = append(p.bulletSubs, m[1])
		}
		p.addSub(m[1])
		return
	}
	if m := subcmdRe.FindStringSubmatch(l.raw); m != nil && p.addSub(m[1]) && m[2] != "" {
		p.setSubDesc(m[1], m[2])
	}
}

// commaList reads an npm-style "  access, adduser, audit, ..." row, only when
// every comma-delimited token is a bare command name, so description text like
// "by file names, stdin, resources" is not read as commands.
func (p *helpParser) commaList(l helpLine) bool {
	if !strings.Contains(l.raw, ",") || strings.HasPrefix(l.trimmed, "-") {
		return false
	}
	names, ok := commaCommandNames(l.raw)
	if !ok || len(names) == 0 {
		return false
	}
	for _, name := range names {
		p.addSub(name)
	}
	return true
}

func commaCommandNames(raw string) ([]string, bool) {
	var names []string
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		switch {
		case name == "":
		case validCmdNameRe.MatchString(name):
			names = append(names, name)
		case !strings.ContainsFunc(name, unicode.IsLetter):
			// symbol-named entries ("[", "[[" in busybox) are skipped
		default:
			return nil, false
		}
	}
	return names, true
}

// freeFormSubcommand reads git-style free-form lists: an indented word and a
// required description that reads as words (a version row "libavutil  59.  8.100"
// is not a command). A name ending in "-" is a fragment of "bundler-<command>".
func (p *helpParser) freeFormSubcommand(raw string) {
	m := subcmdRe.FindStringSubmatch(raw)
	if m == nil || m[2] == "" || startsWithDigit(m[2]) || strings.HasSuffix(m[1], "-") {
		return
	}
	p.addSub(m[1])
}

func (p *helpParser) addParsedFlag(l helpLine) {
	if f, ok := parseFlag(l.raw); ok {
		p.addFlag(p.withNextLineDescription(l, f))
	}
}

// proseFlagRe matches a paragraph that opens by naming flags, as "go help
// <cmd>" documents them (#66): "The -fmt flag reformats ...", "The
// -require=path@version and -droprequire=path flags ...".
var (
	proseFlagRe      = regexp.MustCompile(`^The (-{1,2}[A-Za-z][A-Za-z0-9_-]*(?:=\S+)?(?:(?:,? (?:and|or) |, )-{1,2}[A-Za-z][A-Za-z0-9_-]*(?:=\S+)?)*) flags?\b`)
	proseFlagTokenRe = regexp.MustCompile(`(-{1,2}[A-Za-z][A-Za-z0-9_-]*)(?:=(\S+))?`)
)

// proseFlags adds the flags a "The -x flag ..." paragraph opens with. The
// sentence must start the line, so a mention mid-paragraph ("Note that
// -require overrides ...") is never a flag.
func (p *helpParser) proseFlags(l helpLine) {
	m := proseFlagRe.FindStringSubmatch(l.raw)
	if m == nil {
		return
	}
	for _, t := range proseFlagTokenRe.FindAllStringSubmatch(m[1], -1) {
		f := models.Flag{Name: t[1], ValueType: strings.TrimRight(t[2], ",")}
		if f.ValueType == "" {
			f.ValueType = "bool"
		}
		if len(f.Name) == 2 {
			f.ShortName = f.Name[1:]
		}
		p.addFlag(f)
	}
}

// withNextLineDescription gives a flag row with no description of its own the
// next line's text when that line is indented deeper and is not another flag:
// Go's flag package ("  -cpuprofile string" over "    \twrite cpu profile")
// and "go help <cmd>" ("\t-race" over "\t\tenable data race detection.").
func (p *helpParser) withNextLineDescription(l helpLine, f models.Flag) models.Flag {
	if f.Description != "" || l.i+1 >= len(p.lines) {
		return f
	}
	next := p.lines[l.i+1]
	t := strings.TrimSpace(next)
	if t == "" || strings.HasPrefix(t, "-") || leadingWidth(next) <= leadingWidth(l.raw) {
		return f
	}
	f.Description = t
	return f
}

// leadingWidth is a line's indentation with a tab counted as eight columns.
func leadingWidth(s string) int {
	w := 0
	for _, r := range s {
		switch r {
		case ' ':
			w++
		case '\t':
			w += 8 - w%8
		default:
			return w
		}
	}
	return w
}

// addFlag appends a flag once, and to the named section being read, if any.
func (p *helpParser) addFlag(f models.Flag) {
	if p.seenFlags[f.Name] {
		return
	}
	p.seenFlags[f.Name] = true
	p.result.Flags = append(p.result.Flags, f)
	if p.currentSectionName == "" {
		return
	}
	n := len(p.result.Sections)
	if n == 0 || p.result.Sections[n-1].Name != p.currentSectionName {
		p.result.Sections = append(p.result.Sections, ParsedSection{Name: p.currentSectionName})
		n++
	}
	p.result.Sections[n-1].Flags = append(p.result.Sections[n-1].Flags, f)
}

// addSub records a subcommand once; it reports whether the name was new.
func (p *helpParser) addSub(name string) bool {
	if p.seenSubs[name] || skipSubcmdWords[name] || name == p.selfName {
		return false
	}
	p.seenSubs[name] = true
	p.result.Subcommands = append(p.result.Subcommands, name)
	return true
}

func (p *helpParser) setSubDesc(name, desc string) {
	if p.result.SubcommandDescs == nil {
		p.result.SubcommandDescs = map[string]string{}
	}
	p.result.SubcommandDescs[name] = strings.TrimSpace(desc)
}

// addChoice records a subcommand from a structured list, when it is a valid
// command name.
func (p *helpParser) addChoice(name, desc string) {
	if name == "" || !validCmdNameRe.MatchString(name) || !p.addSub(name) {
		return
	}
	if desc != "" {
		p.setSubDesc(name, desc)
	}
}

func (p *helpParser) addGrammar(text string) {
	body, closed := text, false
	if j := strings.Index(body, "}"); j >= 0 {
		body, closed = body[:j], true
	}
	for _, n := range strings.Split(body, "|") {
		p.addChoice(strings.TrimSpace(n), "")
	}
	p.inGrammar = !closed
}

func (p *helpParser) finish() ParsedHelp {
	r := &p.result
	if p.pendingFlag != nil {
		p.addFlag(*p.pendingFlag)
	}
	// Sections with fewer than 2 flags are noise.
	filtered := r.Sections[:0]
	for _, s := range r.Sections {
		if len(s.Flags) >= 2 {
			filtered = append(filtered, s)
		}
	}
	r.Sections = filtered
	for _, ul := range p.usageLines {
		r.Positionals = append(r.Positionals, parsePositionals(ul)...)
	}
	// Flags the synopsis names but no row lists ("go mod tidy" documents them
	// in prose) come after the listed ones, which carry descriptions, and
	// belong to no flag section.
	p.currentSectionName = ""
	for _, f := range synopsisFlags(p.usageLines, r.Flags) {
		p.addFlag(f)
	}
	r.Description = strings.Join(strings.Fields(r.Description), " ")
	// A man-page bullet list ("o service") is authoritative: prose elsewhere in
	// the page ("... help each ...") must not add phantom subcommands.
	if len(p.bulletSubs) >= 3 {
		r.Subcommands = p.bulletSubs
	}
	r.Positionals = dedupePositionals(r.Positionals)
	return *r
}

// sectionRank orders sections when a header matches more than one keyword;
// lower wins.
var sectionRank = map[string]int{
	secCommands: 0, secFlags: 1, secUsage: 2, secName: 3, secAliases: 4, secExamples: 5, secDesc: 6,
}

// betterSection reports whether (sec, kw) should replace (bestSec, bestKw).
func betterSection(sec, kw, bestSec, bestKw string) bool {
	if sectionRank[sec] != sectionRank[bestSec] {
		return sectionRank[sec] < sectionRank[bestSec]
	}
	if len(kw) != len(bestKw) {
		return len(kw) > len(bestKw)
	}
	return kw < bestKw
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

	// Handle headers with parenthetical suffixes, e.g. openssl's
	// "Cipher commands (see the 'enc' command for more details)".
	// Strip everything from " (" onward and re-check.
	// The header ends at its closing parenthesis; a wrapped prose line such as
	// "name (like 'C++' or 'LaTeX') … Use" runs on past it and is not a header.
	if idx := strings.Index(candidate, " ("); idx > 0 && strings.HasSuffix(candidate, ")") {
		prefix := strings.TrimSpace(candidate[:idx])
		if s, ok := sectionHeaders[prefix]; ok {
			return s
		}
	}

	// Prefix/suffix match for compound headers like "general options:",
	// "required arguments:", "available services:", etc.
	// Only apply to short candidates (section headers are rarely > 30 chars);
	// this prevents false-positives on lines like "Usage: ls [OPTION]... [FILE]..."
	// A real header is plain words: it never starts with "-" (a flag line) or
	// uses column spacing (a "name   description" row). Without this guard
	// "-v, --verbose   verbose output" is taken as an "output" header and the
	// flag, and the rest of its section, is lost.
	if strings.HasPrefix(candidate, "-") || strings.Contains(candidate, "  ") {
		return secNone
	}
	if len(candidate) <= 30 {
		// A header can match several keywords ("environment commands" matches
		// both "commands" and "environment"). Choose deterministically: sections
		// that hold lists (commands, flags) beat free-text ones, then the longer
		// keyword, then the keyword itself, so the result never depends on map order.
		bestSec, bestKw := secNone, ""
		for kw, sec := range sectionHeaders {
			if !strings.HasPrefix(candidate, kw) && !strings.HasSuffix(candidate, kw) {
				continue
			}
			if bestSec == secNone || betterSection(sec, kw, bestSec, bestKw) {
				bestSec, bestKw = sec, kw
			}
		}
		return bestSec
	}
	return secNone
}

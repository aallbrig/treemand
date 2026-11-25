package discovery

import (
	"regexp"
	"strings"
)

// sectionHeaders maps lower-cased keywords that appear in section header lines.
var sectionHeaders = map[string]string{
	"available commands":      secCommands,
	"available command":       secCommands,
	"available services":      secCommands, // AWS man-page style
	"available service":       secCommands,
	"management commands":     secCommands,
	"management command":      secCommands,
	"commands":                secCommands,
	"subcommands":             secCommands,
	"the commands are":        secCommands, // Go toolchain style: "The commands are:"
	"all commands":            secCommands, // npm style: "All commands:"
	"standard commands":       secCommands, // openssl style: "Standard commands"
	"message digest commands": secCommands, // openssl: "Message Digest commands"
	"cipher commands":         secCommands, // openssl: "Cipher commands"
	"utilities":               secCommands, // man pages: bundle "UTILITIES"
	"flags":                   secFlags,
	"options":                 secFlags,
	"global flags":            secFlags,
	"global options":          secFlags,
	"optional arguments":      secFlags,
	"arguments":               secFlags,
	"usage":                   secUsage,
	"use":                     secUsage,
	"name":                    secName,
	"description":             secDesc,
	"examples":                secExamples,
	"example":                 secExamples,
	"aliases":                 secAliases,
	// free-text sections that contain non-command content
	"supported formats":        secDesc,
	"supported engines":        secDesc,
	"supported types":          secDesc,
	"supported protocols":      secDesc,
	"supported backends":       secDesc,
	"supported drivers":        secDesc,
	"output formats":           secDesc,
	"output modes":             secDesc,
	"discovery strategies":     secDesc,
	"interactive tui controls": secDesc,
	"tui controls":             secDesc,
	"interactive":              secDesc,
	"output":                   secDesc,
	"notes":                    secDesc,
	"note":                     secDesc,
	"see also":                 secDesc,
	"environment variables":    secDesc,
	"environment":              secDesc,
	"configuration":            secDesc,
	"config":                   secDesc,
	"caching":                  secDesc,
	"cache":                    secDesc,
}

// awsBulletRe matches AWS man-page bullet list items.
// After stripping man-page bold formatting, "+o word" bullets appear as either
// "+o word" (plain text) or "o word" (when the "o" was rendered bold via backspace).
var awsBulletRe = regexp.MustCompile(`^\s{2,}(?:\+o|o)\s+([a-z][a-z0-9_-]*)$`)

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
			`|(?:[= ](?:<([^>]+)>|\[([^\]]+)\]|([A-Za-z][A-Za-z0-9_-]*|'[^']*'|\[\]|\d+(?:\.\d+)?)))` + // =<type>, =type, kubectl =false / ='' / =[] / =500
			`)?` +
			`:?` + // kubectl: "--flag=false:" with the description on the next line
			`(?:\s+(.*))?$`, // description (1+ spaces gap)
	)
	// short-only flag: -v or -v <value>
	shortOnlyFlagRe = regexp.MustCompile(
		`^\s{2,8}(-[A-Za-z0-9])(?:\s+(?:<([^>]+)>|([A-Za-z][A-Za-z0-9_-]*)))?(?:\s{2,}(.*))?$`,
	)
	// manpageHeaderRe matches lines like "GIT-CLONE(1)   Git Manual   GIT-CLONE(1)".
	// These appear at the top of man-formatted --help output and should never be
	// used as the command description.
	manpageHeaderRe = regexp.MustCompile(`^[A-Z][A-Z0-9_.-]+\(\d*\)\s`)
	// nameSectionDescRe matches the NAME section body: "   git-clone - Short desc"
	nameSectionDescRe = regexp.MustCompile(`^[\s\t]+\S.*?\s+[-\x{2013}\x{2014}]\s+(.+)$`)
	// subcommand line: 2–8 leading spaces, lowercase word; args like [PATTERN...] may appear
	// between name and description (e.g. systemctl's "  list-units [PAT...]   description")
	subcmdRe = regexp.MustCompile(`^\s{2,8}([a-z][a-z0-9_-]*)(?:.*?\s{2,}(.+))?$`)
	// positional in usage: <required> or [optional]
	reqArgRe = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9_.-]*)>`)
	optArgRe = regexp.MustCompile(`\[([A-Za-z][A-Za-z0-9_.-]*)(?:\.{3})?\]`)
	// URL in help text
	urlRe = regexp.MustCompile(`https?://[^\s]+`)
)

// ansiRe matches ANSI terminal escape codes (colors, etc.)
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHF]`)

// manpageBoldRe matches man-page backspace-based bold/underline sequences.
// Man pages encode bold as "char\bchar" and underline as "_\bchar".
// Replacing "char\b" with "" leaves only the visible character.
var manpageBoldRe = regexp.MustCompile(`.\x08`)

// buildMarkerRe matches Godot-style build-availability markers at the start of
// a flag description: a single uppercase letter (R/D/X/E) followed by 2+ spaces.
// e.g. "R  Display this help message." → "Display this help message."
var buildMarkerRe = regexp.MustCompile(`^[A-Z]\s{2,}`)

// goTabSubcmdRe matches Go toolchain style tab-indented subcommand lines:
//
//	\tbug         start a bug report
var goTabSubcmdRe = regexp.MustCompile(`^\t([a-z][a-z0-9_-]*)(?:\s{2,}(.+))?$`)

// validCmdNameRe matches a bare command name (npm comma-separated list entries).
var validCmdNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// gridCmdRe matches a multi-column grid entry (openssl style): lowercase word, min 2 chars.
var gridCmdRe = regexp.MustCompile(`^[a-z][a-z0-9_-]+$`)

// stripBuildMarker removes a leading build-availability marker if present.
func stripBuildMarker(s string) string {
	return strings.TrimSpace(buildMarkerRe.ReplaceAllString(s, ""))
}

// allGridEntries reports whether every token in parts looks like a command name.
// Used to detect openssl-style multi-column grids of subcommands.
func allGridEntries(parts []string) bool {
	for _, p := range parts {
		if !gridCmdRe.MatchString(p) {
			return false
		}
	}
	return true
}

// isErrorOutput reports whether s looks like a CLI error message rather than
// help text. Used to skip error output from CLIs (e.g. aws, go) that print an
// error instead of help when --help is passed without a required argument.
// Checks for Contains("unknown command") rather than HasPrefix because Go-style
// CLIs emit "<name>: unknown command" (e.g. "go mod: unknown command").
func isErrorOutput(s string) bool {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return false
	}
	// A complete help body is help, even after a warning line: flask --help
	// with no app prints "Error: Could not locate a Flask application" and
	// then its full help.
	if hasHelpBody(lines) {
		return false
	}
	first := strings.ToLower(lines[0])
	if strings.Contains(first, "[error]") ||
		strings.HasPrefix(first, "error:") ||
		strings.Contains(first, "unknown command") ||
		strings.Contains(first, "invalid command") {
		return true
	}
	// A short answer that names a rejected option is a usage error, even when
	// it starts with a usage line (flask -h: "Usage: … Error: No such option").
	if len(lines) <= 6 {
		low := strings.ToLower(strings.Join(lines, "\n"))
		for _, m := range usageErrorMarkers {
			if strings.Contains(low, m) {
				return true
			}
		}
	}
	return false
}

// usageErrorMarkers are how argument parsers say an option was rejected.
var usageErrorMarkers = []string{
	"no such option",         // click
	"unknown shorthand flag", // cobra
	"unknown flag",           // cobra, pflag
	"unknown option",         // many
	"invalid option",         // getopt: "invalid option -- 'h'"
	"unrecognized option",    // getopt_long
	"unrecognized arguments", // argparse
	"illegal option",         // BSD getopt
}

// helpSectionHeaders are section titles that only real help has.
var helpSectionHeaders = map[string]bool{
	"options:": true, "commands:": true, "flags:": true, "global flags:": true,
	"available commands:": true, "subcommands:": true,
	"positional arguments:": true, "optional arguments:": true,
}

// hasHelpBody reports whether the output is a full help page: long enough and
// with at least one section a usage error never has.
func hasHelpBody(lines []string) bool {
	if len(lines) < 8 {
		return false
	}
	for _, l := range lines[1:] {
		if helpSectionHeaders[strings.ToLower(strings.TrimSpace(l))] {
			return true
		}
	}
	return false
}

// skipSubcmdWords are words that look like subcommands but aren't.
var skipSubcmdWords = map[string]bool{
	"true": true, "false": true,
	"none": true, "all": true, "on": true, "off": true,
	"yes": true, "no": true, "default": true,
	// "or" starts a usage continuation ("   or  java [options] -jar …").
	"or": true,
	// Single-letter "commands" are almost always hotkey hints in help text, not subcommands.
	"a": true, "b": true, "c": true, "d": true, "e": true, "f": true,
	"g": true, "h": true, "i": true, "j": true, "k": true, "l": true,
	"m": true, "n": true, "o": true, "p": true, "q": true, "r": true,
	"s": true, "t": true, "u": true, "v": true, "w": true, "x": true,
	"y": true, "z": true,
}

// stripANSI removes ANSI terminal escape codes from a string.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// stripManpageFormatting removes backspace-based bold/underline sequences used
// by groff/man output. Example: "+\bo" (bold o rendered on top of +) → "o".
func stripManpageFormatting(s string) string {
	return manpageBoldRe.ReplaceAllString(s, "")
}

// agentHintRe matches machine hints some CLIs print when they detect an AI
// agent (e.g. <claude-code-hint v="1" ... />); they are not help text.
var agentHintRe = regexp.MustCompile(`(?m)[ \t]*<[a-z-]+-hint\b[^>]*/>[ \t]*\r?\n?`)

// SanitizeHelp strips terminal escapes, man-page overstrike sequences and
// agent hint tags from raw help output, then trims surrounding whitespace.
func SanitizeHelp(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = manpageBoldRe.ReplaceAllString(s, "")
	s = agentHintRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// isUsageLine reports whether line is a bare usage synopsis such as
// "stripe charges <operation> [parameters...]" rather than prose.
// listRowRe matches an indented "name   text" row of a command list. Needs 3+
// spaces so double-spaced justified man-page prose is not mistaken for a row.
var listRowRe = regexp.MustCompile(`^[ \t]+[a-z][a-z0-9_-]*[ \t]{3,}\S`)

// introducesIndentedList reports whether line i is a heading for the indented
// block right below it (git's "start a working area (...)" followed by its
// commands), which makes it a section title rather than a description.
func introducesIndentedList(lines []string, i int) bool {
	if i+1 >= len(lines) || lines[i+1] == "" {
		return false
	}
	next := lines[i+1]
	return (next[0] == ' ' || next[0] == '\t') && strings.TrimSpace(next) != "" &&
		(lines[i][0] != ' ' && lines[i][0] != '\t')
}

func isUsageLine(line, cliName string) bool {
	return strings.HasPrefix(line, cliName+" ") && strings.ContainsAny(line, "<[")
}

// startsWithDigit reports whether s begins with a digit, as version numbers do.
func startsWithDigit(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && s[0] >= '0' && s[0] <= '9'
}

// commandWords are the words CLIs use for their subcommands in list headers.
var commandWords = []string{"command", "operation", "action", "topic", "verb", "service", "function"}

// namesCommands reports whether a header line introduces a list of commands.
func namesCommands(lower string) bool {
	for _, w := range commandWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// splitTopLevelCommas splits an option spec on commas outside [] and {}:
// "--disable={jit|rubyopt}[,...]" is one option, "-n, --quiet" two.
func splitTopLevelCommas(spec string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range spec {
		switch r {
		case '[', '{':
			depth++
		case ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, spec[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, spec[start:])
}

// bannerRe matches the banner lines some CLIs print before their help: name
// and version, copyright, licence, build details. They are not a description.
var bannerRe = regexp.MustCompile(`(?i)\bversion\s+v?\d|\bv\d+\.\d+|copyright|\(c\)\s|licen[cs]ed under|built with|^configuration:|^web site:|no warranty|free software|redistribute it|general public licen|source distribution`)

// nameVersionRe matches clap's first help line, a bare "name version"
// ("fakeclap-add 1.0", "ripgrep 14.1.0").
var nameVersionRe = regexp.MustCompile(`^[A-Za-z][\w.+-]*\s+v?\d+(\.\d+)+([-+][\w.+-]*)?$`)

// authorLineRe matches clap's author line ("Andrew Gallant <jam@example.com>").
var authorLineRe = regexp.MustCompile(`^[^<>]*<[^<>\s]+@[^<>\s]+>$`)

// versionRowRe matches library version rows ("  libavutil      59.  8.100 / …").
var versionRowRe = regexp.MustCompile(`^\s+[A-Za-z][\w.-]*\s{2,}\d+\.`)

// inNonProseList reports whether rawLine is an entry of a list that is never
// a description: a value list under an unknown header (rsync "Capabilities:"),
// or an alias or example ("ALIASES\n  gh variable remove").
func inNonProseList(section, rawLine string) bool {
	if rawLine == "" || rawLine[0] != ' ' {
		return false
	}
	return section == secValues || section == secAliases || section == secExamples
}

// argparseRowRe is a subparser row: "    install             Install a package".
var argparseRowRe = regexp.MustCompile(`^\s+([a-z][a-z0-9_-]*)(?:\s{2,}(\S.*))?$`)

var (
	// kingpinRowRe is a kingpin command row: lowercase words, then arguments.
	kingpinRowRe = regexp.MustCompile(`^([a-z][a-z0-9_-]*)(?: [a-z][a-z0-9_-]*)*(?: [\[<-].*)?$`)
	// thorRowRe is a thor row: "  <program> <command> [ARGS]  # description".
	thorRowRe = regexp.MustCompile(`^\s+(\S+)\s+([a-z][a-z0-9_:-]*)\b[^#]*#\s*(.*)$`)
	// manRefRowRe is a man-page command reference: "bundle install(1) …".
	manRefRowRe = regexp.MustCompile(`^\s+(\S+?)[ -]([a-z][a-z0-9_-]*)\(\d+[a-z]*\)`)
	// cleoListHelpRe recognizes the help of a cleo app's "list" command:
	// "Usage:" followed by "  list [options] …" (what "poetry --help" prints).
	cleoListHelpRe = regexp.MustCompile(`(?m)^Usage:\s*\n\s+list(?:\s|$)`)
	// grammarRe is a grammar rule: "where  OBJECT := { address | …".
	grammarRe = regexp.MustCompile(`^(?:where\s+)?([A-Z][A-Z_]*)\s*:=\s*\{(.*)$`)
	// namespaceRowRe is a namespaced command row: "  cache clear        Clears …".
	namespaceRowRe = regexp.MustCompile(`^\s+([a-z][a-z0-9_-]*) [a-z][a-z0-9_-]*\s{2,}\S`)
	// bulletCmdRe is a bulleted command: "    - access".
	bulletCmdRe = regexp.MustCompile(`^\s+- ([a-z][a-z0-9_-]*)\s*$`)
)

// nextLineIndented reports whether the next non-empty line after i is indented.
func nextLineIndented(lines []string, i int) bool {
	for _, l := range lines[i+1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		return l[0] == ' ' || l[0] == '\t'
	}
	return false
}

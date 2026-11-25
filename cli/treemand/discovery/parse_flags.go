package discovery

import (
	"regexp"
	"strings"

	"github.com/aallbrig/treemand/cli/treemand/models"
)

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
			f.ValueType = normalizeFlagDefault(m[5])
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
	if f, ok := parseOptionSpec(line); ok {
		return f, true
	}
	return parseSingleDashOption(line)
}

// singleDashRe matches single-dash options (#156), as Go's flag package
// ("  -cpuprofile string" with the description on the next line, "  -d<TAB>…"),
// "go help <cmd>" ("<TAB>-C dir", #66) and mitchellh/cli ("  -chdir=DIR    Switch …")
// print them. After the name and an optional " type" or "=VALUE" comes a tab,
// a 2+ space gap or the end of the line, so "-v verbose output" (prose after
// one space) never matches.
var singleDashRe = regexp.MustCompile(`^(?: {2,4}|\t)(-[A-Za-z][A-Za-z0-9_.-]*)(?:[ =]([A-Za-z<][^\s]*))?(?:\t+| {2,}|$)(.*)$`)

func parseSingleDashOption(line string) (models.Flag, bool) {
	m := singleDashRe.FindStringSubmatch(line)
	if m == nil {
		return models.Flag{}, false
	}
	value := m[2]
	if strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") && strings.Count(value, "<") == 1 {
		value = value[1 : len(value)-1] // "<file>" -> "file"; "<var>[:<type>]=<value>" stays whole
	}
	f := models.Flag{Name: m[1], ValueType: value, Description: strings.TrimSpace(m[3])}
	if f.ValueType == "" {
		f.ValueType = "bool"
	}
	if len(m[1]) == 2 {
		f.ShortName = m[1][1:]
	}
	return f, true
}

// parseOptionSpec reads option lines the single-short-then-long regex does
// not cover (#161). The spec is everything before the first gap of two or more
// spaces; it is a comma-separated list of option names, each optionally
// followed by a value, in any order:
//
//	-C DIRECTORY, --directory=DIRECTORY   (value after the short name)
//	-n, --quiet, --silent                 (several aliases)
//	-i[SUFFIX], --in-place[=SUFFIX]       (attached and optional values)
//	--json, -j                            (long name first)
//	-pl,--projects <arg>                  (multi-letter short, no space)
//	-p, --plain...                        (repeatable)
//
// Every part must be an option and one must be a --long name, so prose never
// matches. Single-dash long options without one (gofmt, terraform) are #156.
func parseOptionSpec(line string) (models.Flag, bool) {
	t := strings.TrimLeft(line, " \t")
	if len(line)-len(t) > 12 || !strings.HasPrefix(t, "-") {
		return models.Flag{}, false
	}
	// kingpin negatable booleans: "--[no-]help" is the option --help.
	t = strings.ReplaceAll(t, "--[no-]", "--")
	spec, desc := t, ""
	if loc := specGapRe.FindStringIndex(t); loc != nil {
		spec, desc = t[:loc[0]], strings.TrimSpace(t[loc[1]:])
	}
	var long, short, value string
	for _, part := range splitTopLevelCommas(spec) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m := optionTokenRe.FindStringSubmatch(part)
		if m == nil {
			return models.Flag{}, false
		}
		v, ok := optionValue(m[3])
		if !ok {
			return models.Flag{}, false
		}
		if value == "" {
			value = v
		}
		switch {
		case m[1] == "--":
			if long == "" {
				long = "--" + m[2]
			}
		case short == "":
			short = m[2]
		}
	}
	if long == "" {
		return models.Flag{}, false
	}
	if value == "" {
		value = "bool"
	}
	return models.Flag{Name: long, ShortName: short, ValueType: value, Description: stripBuildMarker(desc)}, true
}

var (
	specGapRe     = regexp.MustCompile(`\s{2,}|\t`)
	optionTokenRe = regexp.MustCompile(`^(--?)([A-Za-z0-9][A-Za-z0-9_-]*)(.*)$`)
	valueWordRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_:.-]*$`)
)

// optionValue normalizes what follows an option name: " FILE", "=FILE",
// "[=N]", "[N]", " <arg>", "=..." or nothing. A trailing "..." alone marks a
// repeatable option, not a value. ok is false when the text is not a value.
func optionValue(rest string) (string, bool) {
	// A repeatable list suffix ("={a|b}[,...]") says nothing about the value.
	r := strings.TrimSuffix(strings.TrimSpace(rest), "[,...]")
	switch {
	case r == "" || r == "...":
		return "", true
	case strings.HasPrefix(r, "[=") && strings.HasSuffix(r, "]"):
		r = r[2 : len(r)-1]
	case strings.HasPrefix(r, "[") && strings.HasSuffix(r, "]"):
		r = r[1 : len(r)-1]
	case strings.HasPrefix(r, "="):
		r = r[1:]
	}
	r = strings.TrimSuffix(strings.TrimPrefix(r, "<"), ">")
	if r == "" || strings.ContainsAny(r, " \t") {
		return "", false
	}
	if r == "..." || strings.HasPrefix(r, "[") || valueWordRe.MatchString(r) ||
		(strings.HasPrefix(r, "{") && strings.HasSuffix(r, "}")) { // choices: {jit|rubyopt|...}
		return r, true
	}
	return "", false
}

// normalizeFlagDefault turns a kubectl-style default shown in place of a type
// (=false, =500, =”, =[]) into a value type; real type names pass through.
func normalizeFlagDefault(v string) string {
	switch {
	case v == "true" || v == "false":
		return "bool"
	case v == "[]":
		return "list"
	case strings.HasPrefix(v, "'"):
		return "string"
	case v != "" && v[0] >= '0' && v[0] <= '9':
		return "int"
	}
	return v
}

// positionalPlaceholders are all-caps words in usage lines that represent
// option/flag slots, not real positional arguments.
var positionalPlaceholders = map[string]bool{
	"OPTION": true, "OPTIONS": true, "OPTS": true, "OPT": true,
	"FLAG": true, "FLAGS": true, "ARG": true, "ARGS": true,
	"ARGUMENTS": true, "PARAMS": true, "PARAMETERS": true,
	"SHORT-OPTION": true, "LONG-OPTION": true,
	// cobra/click/common framework placeholders (lowercased inputs are uppercased before lookup)
	"COMMAND": true, "COMMANDS": true,
	"SUBCOMMAND": true, "SUBCOMMANDS": true,
	"CMD": true, "CMDS": true,
	"RESOURCE": true, "NAME": true, "TYPE": true, "OBJECT": true,
}

// bracketOptionRe matches bracket-enclosed option patterns that should not be
// treated as positional arguments: [--long-opt...] and [-X <value>] forms.
var bracketOptionRe = regexp.MustCompile(`\[-[^\]]+\]`)

// usageOptionRe matches one option alone in a usage-line bracket, with an
// optional value: "[-v]", "[--verbose]", "[-o output]", "[-go=version]",
// "[--config <file>]". Brackets holding alternatives ("[-v | --version]")
// or nested optional values ("[--exec-path[=<path>]]") do not match.
var usageOptionRe = regexp.MustCompile(`\[(--?[A-Za-z][A-Za-z0-9_-]*)(?:[ =](<[A-Za-z][^<>\[\]|]*>|[A-Za-z][A-Za-z0-9_.,-]*))?\]`)

// wordOptionRe matches a single-dash word option, which is lowercase
// ("-json", "-testcache"); "[-Olevel]" (GNU find) is -O with a value.
var wordOptionRe = regexp.MustCompile(`^-[a-z][a-z0-9-]+$`)

// usageLineFlags reads the options a usage line names one per bracket (#66):
// "usage: go mod tidy [-e] [-v] [-diff] [-go=version]". BSD tools cluster
// single-letter options in one bracket ("[-pv]" is -p and -v), so a
// multi-letter single-dash option without a value counts only when the same
// line also lists a single letter alone ("[-e]"), which a clustering tool
// would have merged.
func usageLineFlags(line string) []models.Flag {
	ms := usageOptionRe.FindAllStringSubmatch(line, -1)
	lonePerLetter := false
	for _, m := range ms {
		lonePerLetter = lonePerLetter || (len(m[1]) == 2 && m[2] == "")
	}
	var flags []models.Flag
	for _, m := range ms {
		if f, ok := usageOptionFlag(m[1], strings.Trim(m[2], "<>"), lonePerLetter); ok {
			flags = append(flags, f)
		}
	}
	return flags
}

// usageOptionFlag turns one bracketed usage option into a flag, or reports
// false for a possible BSD letter cluster (see usageLineFlags).
func usageOptionFlag(name, value string, lonePerLetter bool) (models.Flag, bool) {
	long := strings.HasPrefix(name, "--")
	word := len(name) > 2 && !long
	if word && value == "" && (!lonePerLetter || !wordOptionRe.MatchString(name)) {
		return models.Flag{}, false
	}
	f := models.Flag{Name: name, ValueType: value}
	if f.ValueType == "" {
		f.ValueType = "bool"
	}
	if len(name) == 2 {
		f.ShortName = name[1:]
	}
	return f, true
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
	// Strip [--option] bracket patterns to avoid false positives from
	// lines like "git [--exec-path[=<path>]]".
	searchLine = bracketOptionRe.ReplaceAllString(searchLine, "")
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

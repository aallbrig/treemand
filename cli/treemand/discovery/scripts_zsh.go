package discovery

import (
	"regexp"
	"strings"

	"github.com/aallbrig/treemand/models"
)

// zshOptionNameRe matches the option name at the start of an _arguments spec
// (after any exclusion list and repeat marker).
var zshOptionNameRe = regexp.MustCompile(`^--?[A-Za-z0-9?][A-Za-z0-9_.?-]*`)

// parseZshScript reads a zsh completion script for cliName: root flags from
// the first _arguments call, and subcommands from the 'name:description'
// arrays in _<cli>_commands and _<cli>__<sub>_commands functions, as clap
// and many hand-written scripts lay them out.
func parseZshScript(src, cliName string) *scriptTree {
	t := newScriptTree(cliName)
	cmds := scriptCommands(src, dialectPOSIX)
	t.addZshRootFlags(zshRootArguments(cmds, cliName))
	t.addZshCommandArrays(cmds, cliName)
	return t
}

// addZshRootFlags adds the options of the root's _arguments call.
func (t *scriptTree) addZshRootFlags(words []string) {
	if len(words) == 0 {
		return
	}
	var flags []models.Flag
	for _, spec := range zshSpecs(words[1:]) {
		for _, s := range expandBraces(spec) {
			if f, ok := zshFlag(s); ok {
				flags = append(flags, f)
			}
		}
	}
	for _, f := range pairShortLong(flags) {
		t.addFlag(nil, f)
	}
}

// zshRootArguments returns the first _arguments call in the _<cli> function,
// or, when the script has none, the first one before any function is
// defined (a script whose body is the completion function). Calls in other
// functions describe subcommands or helpers, not the root.
func zshRootArguments(cmds [][]string, cliName string) []string {
	fn, sawFunction := "", false
	var loose []string
	for _, words := range cmds {
		if name, ok := zshFunctionName(words); ok {
			fn, sawFunction = name, true
			continue
		}
		switch {
		case words[0] != "_arguments":
		case fn == "_"+cliName:
			return words
		case !sawFunction && loose == nil:
			loose = words
		}
	}
	return loose
}

// zshFunctionName reads a function header: "name() {" (lexed as "name(",
// ")") or "function name {".
func zshFunctionName(words []string) (string, bool) {
	if words[0] == "function" && len(words) > 1 {
		return strings.TrimSuffix(words[1], "()"), true
	}
	if len(words) > 1 && words[1] == ")" && len(words[0]) > 1 && strings.HasSuffix(words[0], "(") && !strings.HasPrefix(words[0], "(") {
		return strings.TrimSuffix(words[0], "("), true
	}
	return "", false
}

// zshSpecs returns the spec words of an _arguments call: those after a lone
// ":" when there is one, else those that look like an option spec.
func zshSpecs(args []string) []string {
	for i, a := range args {
		if a == ":" {
			return args[i+1:]
		}
	}
	var specs []string
	for _, a := range args {
		if strings.ContainsAny(a, "[:"+string(braceOpen)) || strings.HasPrefix(a, "(") {
			specs = append(specs, a)
		}
	}
	return specs
}

// zshFlag reads one _arguments option spec such as
// '(-q --quiet)*--env=[Add env vars]:KEY=VALUE:_default'.
func zshFlag(spec string) (models.Flag, bool) {
	if strings.HasPrefix(spec, "(") {
		end := strings.IndexByte(spec, ')')
		if end < 0 {
			return models.Flag{}, false
		}
		spec = spec[end+1:]
	}
	spec = strings.TrimLeft(spec, "*!")
	name := zshOptionNameRe.FindString(spec)
	if name == "" {
		return models.Flag{}, false
	}
	rest := spec[len(name):]
	takesValue := strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, "+")
	rest = strings.TrimLeft(rest, "=+-")
	desc := ""
	if strings.HasPrefix(rest, "[") {
		desc, rest = zshBracketed(rest)
	}
	takesValue = takesValue || strings.HasPrefix(rest, ":")
	long, short := "", ""
	if strings.HasPrefix(name, "--") {
		long = name[2:]
	} else {
		short = name[1:]
	}
	return scriptFlag(long, short, desc, takesValue)
}

// zshBracketed splits "[description]rest", honouring \] inside.
func zshBracketed(s string) (inside, rest string) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case s[i] == ']':
			return b.String(), s[i+1:]
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String(), ""
}

// pairShortLong joins a one-letter flag and the long flag next to it that
// has the same description, as zsh scripts list them separately.
func pairShortLong(flags []models.Flag) []models.Flag {
	var out []models.Flag
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		if i+1 < len(flags) {
			if joined, ok := joinPair(f, flags[i+1]); ok {
				out = append(out, joined)
				i++
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

func joinPair(a, b models.Flag) (models.Flag, bool) {
	if len(b.Name) == 2 && strings.HasPrefix(a.Name, "--") {
		a, b = b, a
	}
	if len(a.Name) != 2 || !strings.HasPrefix(b.Name, "--") || b.ShortName != "" || a.Description != b.Description {
		return models.Flag{}, false
	}
	b.ShortName = a.Name[1:]
	return b, true
}

// addZshCommandArrays adds the subcommands listed in each _<cli>_commands
// function: _ruff_commands holds ruff's, _ruff__analyze_commands analyze's.
func (t *scriptTree) addZshCommandArrays(cmds [][]string, cliName string) {
	var path []string
	inCommands := false
	for _, words := range cmds {
		if p, ok := zshCommandsFunction(words, cliName); ok {
			path, inCommands = p, true
			continue
		}
		if inCommands && zshArrayStart(words) >= 0 {
			for _, item := range zshArrayItems(words) {
				name, desc := zshItem(item)
				t.addCommand(path, name, desc)
			}
			inCommands = false
		}
	}
}

// zshCommandsFunction reads a function header such as
// "_ruff__analyze__graph_commands() {" (lexed as "..._commands(", ")") as
// the path [analyze graph].
func zshCommandsFunction(words []string, cliName string) ([]string, bool) {
	if len(words) < 2 || words[1] != ")" {
		return nil, false
	}
	name, ok := strings.CutSuffix(words[0], "(")
	if !ok {
		return nil, false
	}
	name, ok = strings.CutPrefix(name, "_"+cliName)
	if !ok {
		return nil, false
	}
	name, ok = strings.CutSuffix(name, "_commands")
	if !ok {
		return nil, false
	}
	if name == "" {
		return []string{}, true
	}
	if !strings.HasPrefix(name, "__") {
		return nil, false
	}
	path := strings.Split(name[2:], "__")
	for _, p := range path {
		if !scriptCommandNameRe.MatchString(p) {
			return nil, false
		}
	}
	return path, true
}

// zshArrayStart returns the index of the "name=(" word of an array
// assignment, or -1.
func zshArrayStart(words []string) int {
	for i, w := range words {
		if strings.HasSuffix(w, "=(") {
			return i
		}
	}
	return -1
}

// zshItem splits a 'name:description' item on its first unescaped colon, so
// 'run\:all:Run everything' names run:all.
func zshItem(item string) (name, desc string) {
	for i := 0; i < len(item); i++ {
		switch item[i] {
		case '\\':
			i++
		case ':':
			return strings.ReplaceAll(item[:i], `\`, ""), item[i+1:]
		}
	}
	return strings.ReplaceAll(item, `\`, ""), ""
}

// zshArrayItems returns the 'name:description' items of an array assignment.
func zshArrayItems(words []string) []string {
	var items []string
	for _, w := range words[zshArrayStart(words)+1:] {
		if w == ")" {
			break
		}
		for _, item := range expandBraces(w) {
			if strings.Contains(item, ":") {
				items = append(items, item)
			}
		}
	}
	return items
}

package discovery

import (
	"path/filepath"
	"slices"
	"strings"
)

// fishEntry is one `complete` command of a fish completion script.
type fishEntry struct {
	commands   []string
	conditions []string
	shorts     []string
	longs      []string
	olds       []string
	args       string
	desc       string
	takesValue bool
	erase      bool
}

// fishValueOptions maps each `complete` option that takes an argument to the
// field it sets.
var fishValueOptions = map[string]string{
	"c": "command", "command": "command",
	"p": "command", "path": "command",
	"s": "short", "short-option": "short",
	"l": "long", "long-option": "long",
	"o": "old", "old-option": "old",
	"a": "args", "arguments": "args",
	"d": "desc", "description": "desc",
	"n": "cond", "condition": "cond",
	"w": "wraps", "wraps": "wraps",
	"C": "do", "do-complete": "do",
}

func (e *fishEntry) set(field, value string) {
	switch field {
	case "command":
		e.commands = append(e.commands, filepath.Base(value))
	case "short":
		e.shorts = append(e.shorts, value)
	case "long":
		e.longs = append(e.longs, value)
	case "old":
		e.olds = append(e.olds, value)
	case "args":
		e.args = strings.TrimSpace(e.args + " " + value)
	case "desc":
		e.desc = value
	case "cond":
		e.conditions = append(e.conditions, value)
	}
}

func (e *fishEntry) setSwitch(name string) {
	switch name {
	case "r", "require-parameter", "x", "exclusive":
		e.takesValue = true
	case "e", "erase":
		e.erase = true
	}
}

// parseFishComplete reads the options of one `complete` command.
func parseFishComplete(words []string) fishEntry {
	var e fishEntry
	for i := 1; i < len(words); i++ {
		w := words[i]
		switch {
		case strings.HasPrefix(w, "--") && len(w) > 2:
			i += e.longOption(w[2:], words, i)
		case strings.HasPrefix(w, "-") && len(w) > 1:
			i += e.shortOptions(w[1:], words, i)
		case len(e.commands) == 0:
			e.set("command", w) // old syntax: complete <cmd> ...
		}
	}
	return e
}

// longOption applies --name[=value] and returns how many extra words it used.
func (e *fishEntry) longOption(opt string, words []string, i int) int {
	name, value, hasValue := strings.Cut(opt, "=")
	field, takes := fishValueOptions[name]
	switch {
	case !takes:
		e.setSwitch(name)
	case hasValue:
		e.set(field, value)
	case i+1 < len(words):
		e.set(field, words[i+1])
		return 1
	}
	return 0
}

// shortOptions applies a cluster such as -fa or -sV and returns how many
// extra words it used.
func (e *fishEntry) shortOptions(cluster string, words []string, i int) int {
	for j := 0; j < len(cluster); j++ {
		name := cluster[j : j+1]
		field, takes := fishValueOptions[name]
		if !takes {
			e.setSwitch(name)
			continue
		}
		if rest := cluster[j+1:]; rest != "" {
			e.set(field, rest)
			return 0
		}
		if i+1 < len(words) {
			e.set(field, words[i+1])
			return 1
		}
	}
	return 0
}

// fishPlace is where an entry's condition puts it in the tree.
type fishPlace struct {
	path []string
	// commandSlot: the condition says the word being completed is a
	// subcommand name (no subcommand yet, or not one of a listed set).
	commandSlot bool
	excluded    map[string]bool
}

// fishCondition reads an entry's -n conditions. ok is false when a clause
// cannot be placed: an `or`, or a positive test of a function it does not
// know, which could mean anything.
func fishCondition(conditions []string, cliName string) (fishPlace, bool) {
	p := fishPlace{excluded: map[string]bool{}}
	for _, cond := range conditions {
		for clause := range strings.SplitSeq(strings.ReplaceAll(cond, "&&", ";"), ";") {
			if !p.clause(clause, cliName) {
				return p, false
			}
		}
	}
	return p, true
}

// clause applies one clause of a condition.
func (p *fishPlace) clause(clause, cliName string) bool {
	if strings.Contains(clause, "||") {
		return false
	}
	words := firstCommand(clause)
	if len(words) > 0 && words[0] == "and" {
		words = words[1:]
	}
	if len(words) == 0 {
		return true
	}
	if words[0] == "or" {
		return false
	}
	negated := words[0] == "not" || words[0] == "!"
	if negated {
		words = words[1:]
	}
	if len(words) == 0 {
		return true
	}
	if negated {
		p.negatedTest(words, cliName)
		return true
	}
	return p.test(words, cliName)
}

func firstCommand(src string) []string {
	cmds := scriptCommands(src, dialectFish)
	if len(cmds) == 0 {
		return nil
	}
	return cmds[0]
}

// negatedTest applies `not <fn> ...`.
func (p *fishPlace) negatedTest(words []string, cliName string) {
	fn, args := words[0], words[1:]
	switch {
	case fn == "__fish_seen_subcommand_from":
		p.commandSlot = true
		for _, a := range args {
			p.excluded[a] = true
		}
	case len(args) == 0 && isFishCommandTest(fn, cliName):
		p.commandSlot = true // not __fish_brew_command: no command yet
	}
}

// test applies a positive `<fn> ...` clause.
func (p *fishPlace) test(words []string, cliName string) bool {
	fn, args := words[0], words[1:]
	switch {
	case isFishRootTest(fn):
		p.commandSlot = true
	case fn == "__fish_seen_subcommand_from" || isFishCommandTest(fn, cliName):
		if len(args) > 0 {
			if !scriptCommandNameRe.MatchString(args[0]) {
				return false
			}
			p.path = append(p.path, args[0])
		}
	case strings.HasPrefix(fn, "__fish_") || strings.HasPrefix(fn, "_"):
		return false
	}
	return true // a builtin such as `set -q VAR` or `test`: no placement
}

// isFishRootTest reports whether fn is a "no subcommand yet" predicate, as
// clap (__fish_<cli>_needs_command), cleo (..._no_subcommand) and fish itself
// (__fish_use_subcommand) name them.
func isFishRootTest(fn string) bool {
	for _, s := range []string{"needs_command", "no_subcommand", "use_subcommand", "no_command", "is_first_arg", "is_first_token"} {
		if strings.HasSuffix(fn, s) {
			return true
		}
	}
	return false
}

// isFishCommandTest reports whether fn tests the subcommand already typed:
// clap's __fish_<cli>_using_subcommand, or __fish_<cli>_command and
// __fish_<cli>_using_command in hand-written scripts.
func isFishCommandTest(fn, cliName string) bool {
	if strings.HasSuffix(fn, "_using_subcommand") || strings.HasSuffix(fn, "_using_command") {
		return true
	}
	return fn == "__fish_"+cliName+"_command" || fn == "__fish_"+cliName+"_subcommand"
}

// parseFishScript reads a fish completion script for cliName.
func parseFishScript(src, cliName string) *scriptTree {
	t := newScriptTree(cliName)
	for _, words := range scriptCommands(src, dialectFish) {
		if words[0] != "complete" {
			continue
		}
		e := parseFishComplete(words)
		if e.erase || !slices.Contains(e.commands, cliName) {
			continue
		}
		place, ok := fishCondition(e.conditions, cliName)
		if !ok {
			continue
		}
		t.addFishEntry(e, place)
	}
	return t
}

func (t *scriptTree) addFishEntry(e fishEntry, place fishPlace) {
	if len(e.shorts)+len(e.longs)+len(e.olds) > 0 {
		t.addFishFlag(e, place.path)
		return
	}
	if e.args == "" || strings.ContainsAny(e.args, "($") || !place.commandSlot {
		return // file or value completions, or generated at completion time
	}
	words := firstCommand(e.args)
	for _, w := range words {
		name, desc, hasDesc := strings.Cut(w, "\t")
		if !hasDesc && len(words) == 1 {
			desc = e.desc
		}
		if len(place.excluded) > 0 && len(place.path) > 0 && !place.excluded[name] {
			continue // a value for the command at path, not a subcommand
		}
		t.addCommand(place.path, name, desc)
	}
}

func (t *scriptTree) addFishFlag(e fishEntry, path []string) {
	long, short := first(e.longs), first(e.shorts)
	if long == "" && short == "" {
		short = first(e.olds)
	}
	takesValue := e.takesValue || e.args != "" // a value list, fixed or generated
	if f, ok := scriptFlag(long, short, e.desc, takesValue); ok {
		t.addFlag(path, f)
	}
}

func first(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

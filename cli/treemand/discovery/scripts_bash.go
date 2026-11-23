package discovery

import (
	"regexp"
	"strings"

	"github.com/aallbrig/treemand/models"
)

// bashFirstWordRe matches a test that the word being completed is the first
// argument: COMP_CWORD -eq 1, cword == 1, or a reference to COMP_WORDS[1].
var bashFirstWordRe = regexp.MustCompile(`(COMP_CWORD|cword)\}?"?\s*(-eq|==|=)\s*"?1\b|(COMP_WORDS|words)\[1\]`)

// parseBashScript reads a bash completion script for cliName. Only the first
// argument's words can be read statically: the opts list in the "<cli>)"
// case arm clap writes, or a literal `compgen -W "..."` list completing the
// first argument. Words starting with "-" are flags, the rest subcommands.
func parseBashScript(src, cliName string) *scriptTree {
	t := newScriptTree(cliName)
	cmds := scriptCommands(src, dialectPOSIX)
	for _, words := range bashRootWordLists(cmds, cliName) {
		for w := range strings.FieldsSeq(words) {
			t.addBashWord(w)
		}
	}
	return t
}

func (t *scriptTree) addBashWord(w string) {
	if strings.HasPrefix(w, "-") {
		if len(w) > 1 && zshOptionNameRe.MatchString(w) {
			t.addFlag(nil, models.Flag{Name: w, ValueType: "bool"})
		}
		return
	}
	t.addCommand(nil, w, "")
}

// bashRootWordLists returns the literal word lists that complete cliName's
// first argument.
func bashRootWordLists(cmds [][]string, cliName string) []string {
	if opts, ok := bashClapRootOpts(cmds, cliName); ok {
		return []string{opts}
	}
	var lists []string
	for i, words := range cmds {
		list, ok := bashCompgenWords(words)
		if !ok {
			continue
		}
		var context []string
		for _, near := range cmds[max(0, i-2) : i+1] {
			context = append(context, strings.Join(near, " "))
		}
		if bashFirstWordRe.MatchString(strings.Join(context, " ")) {
			lists = append(lists, list)
		}
	}
	return lists
}

// bashClapRootOpts finds `opts="..."` right after the "<cli>" ")" case arm.
func bashClapRootOpts(cmds [][]string, cliName string) (string, bool) {
	for i, words := range cmds {
		if len(words) < 2 || words[0] != cliName || words[1] != ")" || i+1 >= len(cmds) {
			continue
		}
		for _, w := range cmds[i+1] {
			if opts, ok := strings.CutPrefix(w, "opts="); ok && isLiteralWords(opts) {
				return opts, true
			}
		}
	}
	return "", false
}

// bashCompgenWords returns the literal list of a `compgen -W "<list>"`.
func bashCompgenWords(words []string) (string, bool) {
	for i := 0; i+2 < len(words); i++ {
		if strings.HasSuffix(words[i], "compgen") && words[i+1] == "-W" && isLiteralWords(words[i+2]) {
			return words[i+2], true
		}
	}
	return "", false
}

// isLiteralWords reports whether s is a fixed word list: no expansion or
// substitution that would need the shell (or the CLI) to run.
func isLiteralWords(s string) bool {
	return strings.TrimSpace(s) != "" && !strings.ContainsAny(s, "$`(")
}

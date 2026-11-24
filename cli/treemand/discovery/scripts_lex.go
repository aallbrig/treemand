package discovery

import "strings"

// scriptDialect picks the quoting rules scriptCommands applies.
type scriptDialect int

const (
	// dialectFish: '\'' and '\\' escape inside single quotes, and an unquoted
	// \t or \n is a tab or newline.
	dialectFish scriptDialect = iota
	// dialectPOSIX (bash, zsh): single quotes are literal, $'...' is ANSI-C
	// quoting, unquoted {a,b} braces expand and ')' ends a word.
	dialectPOSIX
)

// Brace expansion markers that scriptCommands leaves in a word for
// expandBraces; they cannot appear in a completion script's text.
const (
	braceOpen  = '\x01'
	braceComma = '\x02'
	braceClose = '\x03'
)

// scriptCommands splits a shell script into simple commands, each a list of
// words with quotes removed. It only reads the text: nothing is expanded,
// substituted or run. Newlines inside unquoted parentheses do not end a
// command, so a multi-line array assignment stays one command.
func scriptCommands(src string, dialect scriptDialect) [][]string {
	l := &scriptLexer{src: src, dialect: dialect}
	for l.i < len(l.src) {
		l.step()
	}
	l.endWord()
	l.endCommand()
	return l.cmds
}

type scriptLexer struct {
	src     string
	i       int
	dialect scriptDialect
	word    strings.Builder
	inWord  bool
	braces  int // unquoted brace depth within the current word
	vars    int // depth of ${...} parameter braces within the current word
	parens  int // unquoted parenthesis depth
	words   []string
	cmds    [][]string
}

func (l *scriptLexer) step() {
	c := l.src[l.i]
	switch {
	case c == '\'':
		l.single()
	case c == '"':
		l.double()
	case c == '\\':
		l.backslash()
	case c == '#' && !l.inWord:
		l.comment()
	case c == '\n' || c == ';':
		l.endWord()
		if l.parens == 0 {
			l.endCommand()
		}
		l.i++
	case c == ' ' || c == '\t' || c == '\r':
		l.endWord()
		l.i++
	default:
		l.plain(c)
	}
}

// plain handles an unquoted character that is not whitespace or a quote.
func (l *scriptLexer) plain(c byte) {
	l.i++
	if l.dialect == dialectFish {
		l.trackParens(c)
		l.add(c)
		return
	}
	switch {
	case c == '$' && l.i < len(l.src) && l.src[l.i] == '\'':
		l.ansiC()
	case c == ')':
		l.endWord()
		l.trackParens(c)
		l.words = append(l.words, ")")
	case c == '(':
		l.trackParens(c)
		l.add(c)
		if strings.HasSuffix(l.word.String(), "=(") {
			l.endWord() // name=( starts an array: its items are separate words
		}
	default:
		l.add(l.braceMarker(c))
	}
}

func (l *scriptLexer) trackParens(c byte) {
	switch {
	case c == '(':
		l.parens++
	case c == ')' && l.parens > 0:
		l.parens--
	}
}

// braceMarker swaps an unquoted brace-expansion character for its marker.
// The braces of a ${name} parameter stay as they are.
func (l *scriptLexer) braceMarker(c byte) byte {
	switch {
	case c == '{' && strings.HasSuffix(l.word.String(), "$"):
		l.vars++
	case c == '}' && l.vars > 0 && l.braces == 0:
		l.vars--
	case c == '{':
		l.braces++
		return braceOpen
	case c == ',' && l.braces > 0:
		return braceComma
	case c == '}' && l.braces > 0:
		l.braces--
		return braceClose
	}
	return c
}

func (l *scriptLexer) add(c byte) {
	l.word.WriteByte(c)
	l.inWord = true
}

// single reads a '...' string.
func (l *scriptLexer) single() {
	l.inWord = true
	l.i++
	for l.i < len(l.src) {
		c := l.src[l.i]
		l.i++
		if c == '\'' {
			return
		}
		if c == '\\' && l.dialect == dialectFish && l.i < len(l.src) && (l.src[l.i] == '\'' || l.src[l.i] == '\\') {
			c = l.src[l.i]
			l.i++
		}
		l.word.WriteByte(c)
	}
}

// double reads a "..." string; a backslash escapes only " \ $ ` and newline.
func (l *scriptLexer) double() {
	l.inWord = true
	l.i++
	for l.i < len(l.src) {
		c := l.src[l.i]
		l.i++
		switch {
		case c == '"':
			return
		case c == '\\' && l.i < len(l.src) && strings.IndexByte("\"\\$`\n", l.src[l.i]) >= 0:
			if l.src[l.i] != '\n' {
				l.word.WriteByte(l.src[l.i])
			}
			l.i++
		default:
			l.word.WriteByte(c)
		}
	}
}

// ansiC reads a $'...' string (l.i is on the opening quote).
func (l *scriptLexer) ansiC() {
	l.inWord = true
	l.i++
	for l.i < len(l.src) {
		c := l.src[l.i]
		l.i++
		if c == '\'' {
			return
		}
		if c == '\\' && l.i < len(l.src) {
			c = unescapeChar(l.src[l.i])
			l.i++
		}
		l.word.WriteByte(c)
	}
}

// backslash reads an unquoted escape; backslash-newline continues the line.
func (l *scriptLexer) backslash() {
	l.i++
	if l.i >= len(l.src) {
		return
	}
	c := l.src[l.i]
	l.i++
	if c == '\n' {
		return
	}
	if l.dialect == dialectFish {
		c = unescapeChar(c)
	}
	l.add(c)
}

func unescapeChar(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	}
	return c
}

func (l *scriptLexer) comment() {
	for l.i < len(l.src) && l.src[l.i] != '\n' {
		l.i++
	}
}

func (l *scriptLexer) endWord() {
	if l.inWord {
		l.words = append(l.words, l.word.String())
	}
	l.word.Reset()
	l.inWord = false
	l.braces = 0
	l.vars = 0
}

func (l *scriptLexer) endCommand() {
	if len(l.words) > 0 {
		l.cmds = append(l.cmds, l.words)
	}
	l.words = nil
}

// expandBraces expands the first {a,b} group of a word scriptCommands marked,
// recursively: "{-h,--help}x" gives "-hx" and "--helpx". Unmatched markers
// read as the literal braces.
func expandBraces(word string) []string {
	open := strings.IndexByte(word, braceOpen)
	closeAt := matchingBrace(word, open)
	if open < 0 || closeAt < 0 {
		return []string{restoreBraces(word)}
	}
	var out []string
	for _, alt := range splitBraceAlternatives(word[open+1 : closeAt]) {
		out = append(out, expandBraces(word[:open]+alt+word[closeAt+1:])...)
	}
	return out
}

func matchingBrace(word string, open int) int {
	if open < 0 {
		return -1
	}
	depth := 0
	for i := open; i < len(word); i++ {
		switch word[i] {
		case braceOpen:
			depth++
		case braceClose:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitBraceAlternatives splits the inside of a brace group on its top-level
// commas.
func splitBraceAlternatives(inner string) []string {
	var alts []string
	depth, start := 0, 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case braceOpen:
			depth++
		case braceClose:
			depth--
		case braceComma:
			if depth == 0 {
				alts = append(alts, inner[start:i])
				start = i + 1
			}
		}
	}
	return append(alts, inner[start:])
}

func restoreBraces(word string) string {
	return strings.NewReplacer(string(braceOpen), "{", string(braceComma), ",", string(braceClose), "}").Replace(word)
}

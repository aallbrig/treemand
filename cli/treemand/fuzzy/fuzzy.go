// Package fuzzy provides a small subsequence fuzzy matcher and a searchable
// index over a models.Node command graph.
package fuzzy

import (
	"unicode"
)

// Match scores pattern against text as a case-insensitive subsequence match.
// It returns the score, the rune indices in text that matched, and whether
// the pattern matched at all. Higher scores are better.
func Match(pattern, text string) (score int, idx []int, ok bool) {
	p := []rune(pattern)
	if len(p) == 0 {
		return 0, nil, true
	}
	t := []rune(text)
	if len(p) > len(t) {
		return 0, nil, false
	}
	idx = make([]int, 0, len(p))
	pi := 0
	prev := -2
	for ti := 0; ti < len(t) && pi < len(p); ti++ {
		if unicode.ToLower(t[ti]) != unicode.ToLower(p[pi]) {
			continue
		}
		s := 1
		switch {
		case ti == 0:
			s += 8
		case isBoundary(t[ti-1]):
			s += 6
		case unicode.IsUpper(t[ti]) && unicode.IsLower(t[ti-1]):
			s += 4
		}
		if prev == ti-1 {
			s += 5
		}
		if t[ti] == p[pi] {
			s++ // exact-case bonus
		}
		score += s
		idx = append(idx, ti)
		prev = ti
		pi++
	}
	if pi < len(p) {
		return 0, nil, false
	}
	// Prefer tighter, shorter matches.
	score -= (idx[len(idx)-1] - idx[0] + 1 - len(p))
	score -= len(t) / 8
	return score, idx, true
}

func isBoundary(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

package main

import (
	"strconv"
	"strings"
)

// run is a stretch of rendered text drawn with one SGR state. A nil fg or bg
// is the terminal's own default colour.
type run struct {
	text                 string
	fg, bg               *rgb
	bold, faint, reverse bool
	line                 int
}

type sgrState struct {
	fg, bg               *rgb
	bold, faint, reverse bool
}

// scan splits a rendered frame into runs, applying every SGR sequence the way
// a terminal does (including resets an inner style emits inside an outer one).
func scan(frame string) []run {
	var runs []run
	var st sgrState
	var buf strings.Builder
	line := 0
	flush := func() {
		if buf.Len() > 0 {
			runs = append(runs, run{text: buf.String(), fg: st.fg, bg: st.bg,
				bold: st.bold, faint: st.faint, reverse: st.reverse, line: line})
			buf.Reset()
		}
	}
	for i := 0; i < len(frame); i++ {
		switch end := csiEnd(frame, i); {
		case frame[i] == '\n':
			flush()
			line++
		case end < 0:
			buf.WriteByte(frame[i])
		case end >= len(frame):
			i = end
		default:
			if frame[end] == 'm' {
				flush()
				st = apply(st, frame[i+2:end])
			}
			i = end
		}
	}
	flush()
	return runs
}

// csiEnd returns the index of the final byte of the CSI sequence starting at
// i, len(frame) for an unterminated one, or -1 when none starts at i.
func csiEnd(frame string, i int) int {
	if frame[i] != 0x1b || i+1 >= len(frame) || frame[i+1] != '[' {
		return -1
	}
	end := i + 2
	for end < len(frame) && (frame[end] < 0x40 || frame[end] > 0x7e) {
		end++
	}
	return end
}

// attribute applies a non-colour SGR parameter, reporting whether p was one.
func attribute(st *sgrState, p int) bool {
	switch p {
	case 0:
		*st = sgrState{}
	case 1:
		st.bold = true
	case 2:
		st.faint = true
	case 22:
		st.bold, st.faint = false, false
	case 7:
		st.reverse = true
	case 27:
		st.reverse = false
	case 39:
		st.fg = nil
	case 49:
		st.bg = nil
	default:
		return false
	}
	return true
}

// basic resolves the 16-colour SGR parameters (30-37, 90-97, 40-47, 100-107).
func basic(p int) (c rgb, fg, ok bool) {
	switch {
	case p >= 30 && p <= 37:
		return xterm256(p - 30), true, true
	case p >= 90 && p <= 97:
		return xterm256(p - 90 + 8), true, true
	case p >= 40 && p <= 47:
		return xterm256(p - 40), false, true
	case p >= 100 && p <= 107:
		return xterm256(p - 100 + 8), false, true
	}
	return rgb{}, false, false
}

// extended resolves 38/48;2;r;g;b and 38/48;5;n starting at ps[k], returning
// the colour, how many parameters it consumed, and whether it was valid.
func extended(ps []int, k int) (rgb, int, bool) {
	at := func(j int) int {
		if j < len(ps) {
			return ps[j]
		}
		return 0
	}
	switch at(k + 1) {
	case 2:
		return rgb{u8(at(k + 2)), u8(at(k + 3)), u8(at(k + 4))}, 4, true
	case 5:
		return xterm256(at(k + 2)), 2, true
	}
	return rgb{}, 0, false
}

func apply(st sgrState, params string) sgrState {
	var ps []int
	for _, f := range strings.Split(params, ";") {
		n, _ := strconv.Atoi(f)
		ps = append(ps, n)
	}
	set := func(c rgb, fg bool) {
		if fg {
			st.fg = &c
		} else {
			st.bg = &c
		}
	}
	for k := 0; k < len(ps); k++ {
		p := ps[k]
		if attribute(&st, p) {
			continue
		}
		if c, fg, ok := basic(p); ok {
			set(c, fg)
			continue
		}
		if p == 38 || p == 48 {
			c, used, ok := extended(ps, k)
			if ok {
				set(c, p == 38)
			}
			k += used
		}
	}
	return st
}

// terminal is the background and default foreground the TUI is drawn on
// wherever it leaves a colour to the terminal.
type terminal struct {
	name   string
	fg, bg rgb
}

// effective resolves the colours a run is actually seen in on term.
func effective(r run, term terminal) (fg, bg rgb) {
	fg, bg = term.fg, term.bg
	if r.fg != nil {
		fg = *r.fg
	}
	if r.bg != nil {
		bg = *r.bg
	}
	if r.reverse {
		fg, bg = bg, fg
	}
	if r.faint {
		fg = blend(fg, bg, 0.5)
	}
	return fg, bg
}

// hasColour reports whether a frame sets any foreground or background colour.
func hasColour(frame string) bool {
	for _, r := range scan(frame) {
		if r.fg != nil || r.bg != nil {
			return true
		}
	}
	return false
}

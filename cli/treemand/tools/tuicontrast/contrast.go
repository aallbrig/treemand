package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// rgb is an sRGB colour with 8-bit channels.
type rgb struct{ r, g, b uint8 }

func (c rgb) hex() string { return fmt.Sprintf("#%02X%02X%02X", c.r, c.g, c.b) }

// str renders an optional colour; nil means "the terminal's default".
func (c *rgb) str() string {
	if c == nil {
		return ""
	}
	return c.hex()
}

func mustHex(s string) rgb {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	v, err := strconv.ParseUint(s[:6], 16, 32)
	if err != nil {
		panic("bad hex colour " + s)
	}
	n := int(v)
	return rgb{u8(n >> 16 & 0xff), u8(n >> 8 & 0xff), u8(n & 0xff)}
}

// luminance is the WCAG 2.x relative luminance.
func luminance(c rgb) float64 {
	ch := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.r) + 0.7152*ch(c.g) + 0.0722*ch(c.b)
}

// ratio is the WCAG 2.x contrast ratio, from 1 to 21, independent of order.
func ratio(a, b rgb) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// blend mixes fg toward bg; t=0.5 models how terminals draw faint (SGR 2)
// text: the foreground at half intensity over the background.
func blend(fg, bg rgb, t float64) rgb {
	mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*(1-t) + float64(b)*t)) }
	return rgb{mix(fg.r, bg.r), mix(fg.g, bg.g), mix(fg.b, bg.b)}
}

// suggest returns the colour closest to fg, keeping its hue, that reaches min
// contrast against bg: fg mixed toward black or white, whichever bg is not.
func suggest(fg, bg rgb, min float64) rgb {
	if ratio(fg, bg) >= min {
		return fg
	}
	target := rgb{0, 0, 0}
	if luminance(bg) < 0.18 {
		target = rgb{255, 255, 255}
	}
	for t := 0.02; t <= 1.0; t += 0.02 {
		if c := blend(fg, target, t); ratio(c, bg) >= min {
			return c
		}
	}
	return target
}

// ansi16 is the xterm default palette for SGR 30-37 and 90-97.
var ansi16 = [16]string{
	"#000000", "#CD0000", "#00CD00", "#CDCD00", "#0000EE", "#CD00CD", "#00CDCD", "#E5E5E5",
	"#7F7F7F", "#FF0000", "#00FF00", "#FFFF00", "#5C5CFF", "#FF00FF", "#00FFFF", "#FFFFFF",
}

// xterm256 resolves an SGR 38;5;n / 48;5;n index with the xterm palette.
func xterm256(n int) rgb {
	switch {
	case n < 16:
		return mustHex(ansi16[n])
	case n < 232:
		n -= 16
		level := func(v int) uint8 {
			if v == 0 {
				return 0
			}
			return u8(55 + 40*v)
		}
		return rgb{level(n / 36), level(n / 6 % 6), level(n % 6)}
	default:
		v := u8(8 + 10*(n-232))
		return rgb{v, v, v}
	}
}

// u8 clamps a channel value into a byte.
func u8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

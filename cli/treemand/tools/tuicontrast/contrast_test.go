package main

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestRatioMatchesWCAGReferenceValues(t *testing.T) {
	cases := []struct {
		fg, bg string
		want   float64
	}{
		{"#000000", "#FFFFFF", 21},
		{"#FFFFFF", "#FFFFFF", 1},
		{"#777777", "#FFFFFF", 4.48}, // the classic "just fails AA" grey
		{"#FFFFFF", "#777777", 4.48}, // order does not matter
		{"#0000FF", "#FFFFFF", 8.59},
	}
	for _, c := range cases {
		got := ratio(mustHex(c.fg), mustHex(c.bg))
		if !near(got, c.want) {
			t.Errorf("ratio(%s, %s) = %.2f, want %.2f", c.fg, c.bg, got, c.want)
		}
	}
}

func TestFaintBlendsHalfwayTowardTheBackground(t *testing.T) {
	got := blend(mustHex("#FFFFFF"), mustHex("#000000"), 0.5)
	if got.hex() != "#808080" {
		t.Errorf("blend = %s, want #808080", got.hex())
	}
}

func TestSuggestReachesTheThresholdOnTheGivenBackground(t *testing.T) {
	for _, bg := range []string{"#FFFFFF", "#1E1E1E"} {
		s := suggest(mustHex("#50FA7B"), mustHex(bg), 4.5)
		if r := ratio(s, mustHex(bg)); r < 4.5 {
			t.Errorf("suggest on %s = %s with ratio %.2f, want >= 4.5", bg, s.hex(), r)
		}
	}
	// A colour that already passes is returned unchanged.
	if s := suggest(mustHex("#000000"), mustHex("#FFFFFF"), 4.5); s.hex() != "#000000" {
		t.Errorf("suggest changed a passing colour to %s", s.hex())
	}
}

func TestXterm256MapsCubeAndGreys(t *testing.T) {
	if got := xterm256(196).hex(); got != "#FF0000" {
		t.Errorf("196 = %s, want #FF0000", got)
	}
	if got := xterm256(244).hex(); got != "#808080" {
		t.Errorf("244 = %s, want #808080", got)
	}
}

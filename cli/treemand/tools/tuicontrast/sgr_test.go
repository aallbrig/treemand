package main

import "testing"

func TestScanTracksColoursAttributesAndResets(t *testing.T) {
	frame := "\x1b[1;38;2;255;0;0mred bold\x1b[0m plain \x1b[2mdim\x1b[22m \x1b[48;2;0;191;255;38;2;0;0;0msel\x1b[m\n" +
		"\x1b[7mrev\x1b[0m ──"
	runs := scan(frame)
	want := []struct {
		text        string
		fg, bg      string
		bold, faint bool
		line        int
	}{
		{"red bold", "#FF0000", "", true, false, 0},
		{" plain ", "", "", false, false, 0},
		{"dim", "", "", false, true, 0},
		{" ", "", "", false, false, 0},
		{"sel", "#000000", "#00BFFF", false, false, 0},
		{"rev", "", "", false, false, 1},
		{" ──", "", "", false, false, 1},
	}
	if len(runs) != len(want) {
		t.Fatalf("got %d runs %+v, want %d", len(runs), runs, len(want))
	}
	for i, w := range want {
		r := runs[i]
		if r.text != w.text || r.fg.str() != w.fg || r.bg.str() != w.bg || r.bold != w.bold || r.faint != w.faint || r.line != w.line {
			t.Errorf("run %d = %q fg=%s bg=%s bold=%v faint=%v line=%d, want %+v",
				i, r.text, r.fg.str(), r.bg.str(), r.bold, r.faint, r.line, w)
		}
	}
	if !runs[5].reverse {
		t.Error("run 5 should be reversed")
	}
}

func TestEffectiveColoursUseTheTerminalDefaultsAndFaint(t *testing.T) {
	term := terminal{name: "light", fg: mustHex("#000000"), bg: mustHex("#FFFFFF")}
	// Faint default text on a light terminal is grey on white.
	fg, bg := effective(run{faint: true}, term)
	if fg.hex() != "#808080" || bg.hex() != "#FFFFFF" {
		t.Errorf("faint default = %s on %s", fg.hex(), bg.hex())
	}
	// Reverse swaps the resolved colours.
	red := mustHex("#FF0000")
	fg, bg = effective(run{fg: &red, reverse: true}, term)
	if fg.hex() != "#FFFFFF" || bg.hex() != "#FF0000" {
		t.Errorf("reverse = %s on %s", fg.hex(), bg.hex())
	}
}

func TestColouredCountsOnlyColourSequences(t *testing.T) {
	if hasColour("\x1b[1mbold\x1b[0m \x1b[2mdim\x1b[22m") {
		t.Error("bold and faint are not colour")
	}
	if !hasColour("\x1b[38;2;1;2;3mx\x1b[0m") || !hasColour("\x1b[44mx\x1b[0m") {
		t.Error("truecolour foreground and basic background are colour")
	}
}

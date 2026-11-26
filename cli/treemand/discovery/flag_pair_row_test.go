package discovery_test

import (
	"testing"

	"github.com/aallbrig/treemand/cli/treemand/discovery"
	"github.com/aallbrig/treemand/cli/treemand/models"
)

// bsdtar (#277) lists its modes as several "-x Word" pairs on one row, and
// separates a short flag from a one-word description by spaces only.
const bsdtarHelp = `tar(bsdtar): manipulate archive files
First option must be a mode specifier:
  -c Create  -r Add/Replace  -t List  -u Update  -x Extract
Common Options:
  -b #  Use # 512-byte records per I/O block
  -f <filename>  Location of archive
  -v    Verbose
  -w    Interactive
`

func TestParseHelp_bsdtarFlagPairRowAndOneWordDescriptions(t *testing.T) {
	p := discovery.ParseHelpOutput(bsdtarHelp)
	byName := map[string]models.Flag{}
	for _, f := range p.Flags {
		byName[f.Name] = f
	}
	want := map[string]struct{ value, desc string }{
		"-c": {"bool", "Create"},
		"-r": {"bool", "Add/Replace"},
		"-t": {"bool", "List"},
		"-u": {"bool", "Update"},
		"-x": {"bool", "Extract"},
		"-v": {"bool", "Verbose"},
		"-w": {"bool", "Interactive"},
		"-f": {"filename", "Location of archive"},
	}
	for name, w := range want {
		f, ok := byName[name]
		if !ok {
			t.Errorf("%s not parsed; got %+v", name, p.Flags)
			continue
		}
		if f.ValueType != w.value || f.Description != w.desc {
			t.Errorf("%s = (%q, %q), want (%q, %q)", name, f.ValueType, f.Description, w.value, w.desc)
		}
	}
}

package treeload

import (
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

// The help discoverers a load builds carry the preset's help-form hint, since
// discovery does not read presets itself.
func TestDiscoverers_passThePresetHelpHint(t *testing.T) {
	for _, req := range []Request{
		{CLI: "aws", Crawl: true, Strategies: []string{"help"}},
		{CLI: "aws", Strategies: []string{"help", "man"}, Depth: 2},
		{CLI: "git", Strategies: []string{"help"}, Depth: 2},
	} {
		var helps int
		for _, d := range (Loader{}).discoverers(req) {
			h, ok := d.(*discovery.HelpDiscoverer)
			if !ok {
				continue
			}
			helps++
			if want := req.CLI == "aws"; h.HelpSubcommand != want {
				t.Errorf("%+v: HelpSubcommand = %v, want %v", req, h.HelpSubcommand, want)
			}
		}
		if helps != 1 {
			t.Errorf("%+v: %d help discoverers, want 1", req, helps)
		}
	}
}

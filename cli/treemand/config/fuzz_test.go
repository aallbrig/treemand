package config_test

import (
	"bytes"
	"reflect"
	"sort"
	"testing"

	"github.com/spf13/viper"

	"github.com/aallbrig/treemand/config"
)

// Fuzz targets for the user's config file: loading it through viper onto the
// defaults (what every treemand run does) and `treemand config validate`.
// `go test` runs the seeds; the fuzz probe (task measure -- fuzz, skill
// treemand-fuzz-campaign) mutates them.

func addConfigSeeds(f *testing.F) {
	f.Helper()
	if def, err := config.ToYAML(config.DefaultConfig()); err == nil {
		f.Add([]byte(def))
	}
	for _, s := range []string{
		"",
		"icons: nerd\ndepth: 3\nno_color: true\n",
		"desc_line_length: -5\nstub_threshold: abc\ncache_ttl_hours: -1\n",
		"colors:\n  base: \"#FF0000\"\n  subcmd: red\n  selected_text: [1, 2]\n",
		"tree_style: columns\nstrategies: help,completions\n",
		"colrs: 1\ncolors:\n  nested:\n    deeper: x\n",
		"a: &a [*a]\n",
		"? [complex, key]\n: value\n1: 2\n",
		"- just\n- a\n- list\n",
		"depth: 99999999999999999999999\n",
		"colors: null\n",
		"\t:bad yaml",
	} {
		f.Add([]byte(s))
	}
}

// sortedDiagnostics is the result's diagnostics in a fixed order: ValidateYAML
// walks a map, so its order is not part of the contract.
func sortedDiagnostics(r *config.ValidationResult) []config.Diagnostic {
	d := append([]config.Diagnostic(nil), r.Diagnostics...)
	sort.Slice(d, func(i, j int) bool { return d[i].String() < d[j].String() })
	return d
}

func FuzzValidateYAML(f *testing.F) {
	addConfigSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		a, errA := config.ValidateYAML(data)
		b, errB := config.ValidateYAML(data)
		if (errA == nil) != (errB == nil) {
			t.Fatalf("ValidateYAML is not deterministic: %v vs %v", errA, errB)
		}
		if errA != nil {
			return
		}
		if !reflect.DeepEqual(sortedDiagnostics(a), sortedDiagnostics(b)) {
			t.Fatalf("ValidateYAML is not deterministic:\n%+v\n%+v", a.Diagnostics, b.Diagnostics)
		}
		for _, d := range a.Diagnostics {
			if d.Level != "warning" && d.Level != "error" {
				t.Fatalf("diagnostic %+v has level %q", d, d.Level)
			}
		}
		if a.HasErrors() != (len(a.Errors()) > 0) {
			t.Fatalf("HasErrors disagrees with Errors: %+v", a.Diagnostics)
		}
	})
}

func FuzzLoadConfig(f *testing.F) {
	addConfigSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		viper.Reset()
		t.Cleanup(viper.Reset)
		viper.SetConfigType("yaml")
		if err := viper.ReadConfig(bytes.NewReader(data)); err != nil {
			return // treemand reports an unreadable config and stops
		}
		cfg := config.DefaultConfig()
		config.ApplyViper(cfg)
		if cfg.DescLineLength <= 0 || cfg.StubThreshold <= 0 {
			t.Fatalf("loaded a non-positive limit: desc_line_length %d, stub_threshold %d", cfg.DescLineLength, cfg.StubThreshold)
		}
		if cfg.CacheTTL < 0 {
			t.Fatalf("loaded a negative cache TTL %v", cfg.CacheTTL)
		}
		if len(cfg.Strategies) == 0 {
			t.Fatalf("loaded no discovery strategies")
		}
	})
}

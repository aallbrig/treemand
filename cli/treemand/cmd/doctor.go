package cmd

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/presets"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor <cli>",
	Short: "Show how a CLI was parsed and how complete the result is",
	Long: `Discover a CLI afresh (the cache is neither read nor written) and report how
it went: the strategy and help form used, how many commands were found, which
lack a description or flags, which were left undiscovered, which failed, and the
slowest help probes.

The text report is meant to be pasted into a bug report. Your home directory is
shown as ~. --output=json (or yaml) prints the same report for scripts.
Exits 0 whenever a report is printed, even when some commands could not be read.`,
	Example: `  treemand doctor git
  treemand doctor --output=json kubectl | jq .slowest_probes`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeCLIName,
	RunE:              runDoctor,
}

// slowestProbeCount is how many of the slowest probes a report lists.
const slowestProbeCount = 5

// doctorReport is what treemand doctor prints.
type doctorReport struct {
	CLI             string          `json:"cli" yaml:"cli"`
	TreemandVersion string          `json:"treemand_version" yaml:"treemand_version"`
	Platform        string          `json:"platform" yaml:"platform"`
	Binary          string          `json:"binary" yaml:"binary"`
	Strategies      []string        `json:"strategies" yaml:"strategies"`
	HelpForm        string          `json:"help_form,omitempty" yaml:"help_form,omitempty"`
	Preset          string          `json:"preset,omitempty" yaml:"preset,omitempty"`
	Depth           int             `json:"depth" yaml:"depth"`
	ElapsedMS       int64           `json:"elapsed_ms" yaml:"elapsed_ms"`
	Error           string          `json:"error,omitempty" yaml:"error,omitempty"`
	Nodes           int             `json:"nodes" yaml:"nodes"`
	NoDescription   []string        `json:"no_description" yaml:"no_description"`
	NoFlags         []string        `json:"no_flags" yaml:"no_flags"`
	Stubs           []string        `json:"stubs" yaml:"stubs"`
	DiscoveryErrors []doctorNodeErr `json:"discovery_errors" yaml:"discovery_errors"`
	Probes          probeCounts     `json:"probes" yaml:"probes"`
	SlowestProbes   []doctorProbe   `json:"slowest_probes" yaml:"slowest_probes"`
}

type doctorNodeErr struct {
	Command string `json:"command" yaml:"command"`
	Error   string `json:"error" yaml:"error"`
}

type probeCounts struct {
	Run         int `json:"run" yaml:"run"`
	OK          int `json:"ok" yaml:"ok"`
	NonZeroExit int `json:"non_zero_exit" yaml:"non_zero_exit"`
	TimedOut    int `json:"timed_out" yaml:"timed_out"`
	Cancelled   int `json:"cancelled" yaml:"cancelled"`
	Failed      int `json:"failed" yaml:"failed"`
	Truncated   int `json:"truncated" yaml:"truncated"`
}

type doctorProbe struct {
	Command    string `json:"command" yaml:"command"`
	DurationMS int64  `json:"duration_ms" yaml:"duration_ms"`
	Outcome    string `json:"outcome" yaml:"outcome"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	setupLogging()
	cliName := args[0]
	if err := checkOutputFormat(cfgOutput); err != nil {
		return err
	}
	if err := checkTimeout(cfgTimeout); err != nil {
		return err
	}
	configureDiscovery()
	if err := checkAvailable(cliName); err != nil {
		return err
	}
	cfg := configFromFlags()
	preset, _ := presets.Lookup(cliName)
	applyPresetDepth(cmd, cfg, preset)

	rep := diagnose(cliName, cfg, config.ParseStrategies(cfgStrategy), preset)
	return writeDoctorReport(cmd.OutOrStdout(), rep, cfgOutput)
}

// diagnose discovers cliName with every probe recorded and builds the report.
func diagnose(cliName string, cfg *config.Config, strategies []string, preset *presets.Preset) doctorReport {
	ctx, cancel := discoveryContext(cfgTimeout)
	defer cancel()
	var probes discovery.ProbeLog
	ctx = discovery.WithProbeLog(ctx, &probes)

	discoverers := discovery.BuildDiscoverersWithThreshold(strategies, crawlDepth(cfg), cfg.StubThreshold)
	start := time.Now()
	var (
		node *models.Node
		err  error
	)
	withSpinner("diagnosing "+cliName+"…", func() {
		node, err = discovery.Run(ctx, discoverers, cliName)
		if err == nil && node != nil && preset != nil && preset.Seed != nil {
			seedPreset(ctx, node, cliName, preset)
		}
	})

	rep := doctorReport{
		CLI:             cliName,
		TreemandVersion: Version,
		Platform:        runtime.GOOS + "/" + runtime.GOARCH,
		Binary:          homeRelative(discovery.ResolveBinary(cliName)),
		Strategies:      strategies,
		HelpForm:        helpFormOf(discoverers),
		Depth:           cfg.Depth,
		ElapsedMS:       time.Since(start).Milliseconds(),
	}
	if preset != nil {
		rep.Preset = preset.Name
	}
	if err != nil {
		rep.Error = err.Error()
	}
	rep.addTree(node)
	rep.addProbes(probes.Records())
	return rep
}

// seedPreset grafts the preset's own map of the CLI onto the tree, as a
// normal run does, so the report describes the tree users see.
func seedPreset(ctx context.Context, node *models.Node, cliName string, preset *presets.Preset) {
	if seed, err := preset.Seed(ctx, cliName); err == nil {
		presets.Graft(node, seed)
	}
}

// helpFormOf is the help form the help strategy learned, or "".
func helpFormOf(discoverers []discovery.Discoverer) string {
	for _, d := range discoverers {
		if h, ok := d.(*discovery.HelpDiscoverer); ok {
			return h.HelpForm()
		}
	}
	return ""
}

// addTree counts the tree's nodes and lists the ones that are incomplete.
// Virtual group nodes are display-only and not counted.
func (r *doctorReport) addTree(root *models.Node) {
	r.NoDescription, r.NoFlags, r.Stubs = []string{}, []string{}, []string{}
	r.DiscoveryErrors = []doctorNodeErr{}
	if root == nil {
		return
	}
	root.Walk(func(n *models.Node) {
		if n.Virtual {
			return
		}
		r.Nodes++
		path := n.FullCommand()
		if n.Stub {
			r.Stubs = append(r.Stubs, path)
			return // never asked: missing details are expected
		}
		if n.DiscoveryErr != "" {
			r.DiscoveryErrors = append(r.DiscoveryErrors, doctorNodeErr{Command: path, Error: n.DiscoveryErr})
		}
		if n.Description == "" {
			r.NoDescription = append(r.NoDescription, path)
		}
		if len(n.Flags) == 0 {
			r.NoFlags = append(r.NoFlags, path)
		}
	})
}

// addProbes counts the probes by outcome and keeps the slowest.
func (r *doctorReport) addProbes(records []discovery.ProbeRecord) {
	r.Probes.Run = len(records)
	for _, p := range records {
		r.Probes.count(p)
	}
	slices.SortStableFunc(records, func(a, b discovery.ProbeRecord) int { return cmp.Compare(b.Duration, a.Duration) })
	r.SlowestProbes = []doctorProbe{}
	for _, p := range records[:min(len(records), slowestProbeCount)] {
		r.SlowestProbes = append(r.SlowestProbes, doctorProbe{
			Command:    probeCommand(p),
			DurationMS: p.Duration.Milliseconds(),
			Outcome:    probeOutcomeLabel(p),
		})
	}
}

func (c *probeCounts) count(p discovery.ProbeRecord) {
	if p.Truncated {
		c.Truncated++
	}
	switch p.Outcome {
	case discovery.ProbeOK:
		c.OK++
	case discovery.ProbeNonZero:
		c.NonZeroExit++
	case discovery.ProbeTimeout:
		c.TimedOut++
	case discovery.ProbeCancelled:
		c.Cancelled++
	default:
		c.Failed++
	}
}

// probeCommand is the probe as a command line, with the executable's base
// name so no directory of the user's machine is shown.
func probeCommand(p discovery.ProbeRecord) string {
	name := p.Name
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, ".exe")
	return strings.Join(append([]string{name}, p.Args...), " ")
}

func probeOutcomeLabel(p discovery.ProbeRecord) string {
	if p.Outcome == discovery.ProbeNonZero {
		return fmt.Sprintf("exit %d", p.ExitCode)
	}
	return p.Outcome
}

// homeRelative shows a path under the user's home directory as ~/..., so a
// pasted report does not carry the user's name.
func homeRelative(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || !strings.HasPrefix(path, home) {
		return path
	}
	return "~" + strings.TrimPrefix(path, home)
}

func writeDoctorReport(w io.Writer, rep doctorReport, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	case "yaml":
		return yaml.NewEncoder(w).Encode(rep)
	}
	_, err := io.WriteString(w, rep.text())
	return err
}

// doctorListLimit is how many commands the text report lists per problem.
const doctorListLimit = 10

// text renders the report as plain text, ready to paste into an issue.
func (r doctorReport) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "treemand doctor: %s\n\n", r.CLI)
	row := func(k, v string) { fmt.Fprintf(&b, "  %-22s %s\n", k, v) }
	row("treemand", r.TreemandVersion+" ("+r.Platform+")")
	row("binary", r.Binary)
	row("strategy", strings.Join(r.Strategies, ","))
	row("help form", cmp.Or(r.HelpForm, "none answered"))
	if r.Preset != "" {
		row("preset", r.Preset)
	}
	row("depth", fmt.Sprint(r.Depth))
	row("elapsed", (time.Duration(r.ElapsedMS) * time.Millisecond).String())
	if r.Error != "" {
		row("error", r.Error)
	}

	b.WriteString("\nTree\n")
	row("nodes", fmt.Sprint(r.Nodes))
	r.list(&b, "without description", r.NoDescription)
	r.list(&b, "without flags", r.NoFlags)
	r.list(&b, "undiscovered stubs", r.Stubs)
	errs := make([]string, len(r.DiscoveryErrors))
	for i, e := range r.DiscoveryErrors {
		errs[i] = e.Command + ": " + e.Error
	}
	r.list(&b, "discovery errors", errs)

	p := r.Probes
	b.WriteString("\nProbes\n")
	row("run", fmt.Sprintf("%d (ok %d, non-zero exit %d, timed out %d, cancelled %d, failed %d, truncated %d)",
		p.Run, p.OK, p.NonZeroExit, p.TimedOut, p.Cancelled, p.Failed, p.Truncated))
	b.WriteString("  slowest probes\n")
	for _, s := range r.SlowestProbes {
		fmt.Fprintf(&b, "    %8s  %-10s %s\n", (time.Duration(s.DurationMS) * time.Millisecond).String(), s.Outcome, s.Command)
	}
	return b.String()
}

// list writes a count and up to doctorListLimit items under it.
func (r doctorReport) list(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "  %-22s %d\n", label, len(items))
	for _, it := range items[:min(len(items), doctorListLimit)] {
		fmt.Fprintf(b, "    %s\n", it)
	}
	if extra := len(items) - doctorListLimit; extra > 0 {
		fmt.Fprintf(b, "    ... and %d more (--output=json lists all)\n", extra)
	}
}

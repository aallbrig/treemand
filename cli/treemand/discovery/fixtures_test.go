package discovery_test

import (
	"os"
	"testing"

	"github.com/aallbrig/treemand/discovery"
)

func parseFixture(t *testing.T, name, cli string) discovery.ParsedHelp {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return discovery.ParseHelpOutputFor(discovery.SanitizeHelp(string(b)), cli)
}

// aws root help is a man page whose service bullets include names ending in
// section keywords ("appconfig", "elasticache").
func TestFixture_awsRoot_allServices(t *testing.T) {
	p := parseFixture(t, "aws_root.txt", "aws")
	if len(p.Subcommands) < 400 {
		t.Fatalf("got %d services, want >= 400", len(p.Subcommands))
	}
	have := map[string]bool{}
	for _, s := range p.Subcommands {
		have[s] = true
	}
	for _, want := range []string{"s3", "ec2", "appconfig", "elasticache"} {
		if !have[want] {
			t.Errorf("missing service %q", want)
		}
	}
	for _, bad := range []string{"each"} {
		if have[bad] {
			t.Errorf("phantom subcommand %q", bad)
		}
	}
	if p.Description == "" || p.Description[:3] != "The" {
		t.Errorf("description = %q, want the DESCRIPTION text", p.Description)
	}
}

func TestFixture_awsSQS_commands(t *testing.T) {
	p := parseFixture(t, "aws_sqs.txt", "aws")
	if len(p.Subcommands) < 20 {
		t.Errorf("got %d sqs commands, want >= 20", len(p.Subcommands))
	}
}

func TestFixture_stripeCharges_operations(t *testing.T) {
	p := parseFixture(t, "stripe_charges.txt", "stripe")
	want := []string{"capture", "create", "list", "retrieve", "search", "update"}
	if len(p.Subcommands) != len(want) {
		t.Fatalf("subs = %v, want %v", p.Subcommands, want)
	}
}

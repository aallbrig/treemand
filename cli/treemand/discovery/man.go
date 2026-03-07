package discovery

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/aallbrig/treemand/models"
)

// ManDiscoverer uses the system man page to discover flags and subcommands.
// It runs `man <cli>`, strips groff formatting with `col -bx`, then feeds the
// plain text through the shared ParseHelpOutput parser.  Man pages typically
// describe top-level flags and subcommands well; deeper nesting is not
// attempted here — the help strategy handles that once it has a name list.
type ManDiscoverer struct {
	Timeout time.Duration
}

// NewManDiscoverer creates a ManDiscoverer with sensible defaults.
func NewManDiscoverer() *ManDiscoverer {
	return &ManDiscoverer{Timeout: 10 * time.Second}
}

func (m *ManDiscoverer) Name() string { return "man" }

// Discover fetches the man page for cliName and parses it into a Node.
// args is ignored for the man strategy (man pages are always top-level).
func (m *ManDiscoverer) Discover(ctx context.Context, cliName string, _ []string) (*models.Node, error) {
	plain, err := m.fetchManPage(ctx, cliName)
	if err != nil {
		return nil, err
	}
	if plain == "" {
		return nil, nil //nolint:nilnil // no man page available is a normal case
	}

	parsed := ParseHelpOutput(plain)

	node := &models.Node{
		Name:        cliName,
		FullPath:    []string{cliName},
		Description: parsed.Description,
		Flags:       parsed.Flags,
		Positionals: parsed.Positionals,
		HelpText:    plain,
		Discovered:  true,
	}

	for _, sub := range parsed.Subcommands {
		node.Children = append(node.Children, &models.Node{
			Name:       sub,
			FullPath:   []string{cliName, sub},
			Discovered: false,
		})
	}

	if len(parsed.Sections) > 0 && len(node.Flags) == 0 {
		for _, sec := range parsed.Sections {
			node.Flags = append(node.Flags, sec.Flags...)
		}
	}

	return node, nil
}

// fetchManPage runs `man <cliName>` and strips groff bold/underline encoding
// via `col -bx`.  Returns the plain-text output or ("", nil) if no man page
// is installed for the tool.
func (m *ManDiscoverer) fetchManPage(ctx context.Context, cliName string) (string, error) {
	// Sanitise: cliName must be a single token (no spaces, no shell metacharacters).
	if strings.ContainsAny(cliName, " \t\r\n;|&`$<>(){}") {
		return "", nil
	}

	tctx, cancel := context.WithTimeout(ctx, m.Timeout)
defer cancel()

	manCmd := exec.CommandContext(tctx, "man", cliName)
	// MANPAGER=cat prevents man from invoking a pager; TERM=dumb keeps output plain.
	manCmd.Env = append(manCmd.Environ(), "MANPAGER=cat", "TERM=dumb")
	var manOut bytes.Buffer
	manCmd.Stdout = &manOut
	manCmd.Stderr = nil // discard stderr (e.g. "No manual entry for X")

	if err := manCmd.Run(); err != nil {
		// Exit code 1 means no man page — not a hard error.
		return "", nil
	}

	raw := manOut.Bytes()
	if len(raw) == 0 {
		return "", nil
	}

	// col -bx: strip backspace-based bold/underline and expand tabs to spaces.
	colCmd := exec.CommandContext(tctx, "col", "-bx")
	colCmd.Stdin = bytes.NewReader(raw)
	var colOut bytes.Buffer
	colCmd.Stdout = &colOut
	if err := colCmd.Run(); err != nil {
		// col not available — fall back to our built-in regex stripper.
		return manpageBoldRe.ReplaceAllString(string(raw), ""), nil
	}

	return colOut.String(), nil
}

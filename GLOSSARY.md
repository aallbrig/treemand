# Glossary

The words this project uses for its own concepts. Use these terms in issues, PRs,
skills and code names; avoid the listed synonyms.

## Product

**Target CLI**: a command-line program treemand explores and renders as a tree
(git, aws, kubectl). _Avoid_: tool, binary (ambiguous with treemand itself).

**Command tree**: the subcommands, flags and descriptions treemand discovered for
a target CLI. It is the core thing treemand produces.

**Help-text family**: target CLIs whose help output shares a shape (cobra, clap,
argparse, man-page, BSD utilities). Parser quality is judged per family.

**Discovery**: reading a target CLI's own help to build its command tree.
_Avoid_: introspection, scanning, crawling.

**Strategy**: one source discovery reads: help output, shell completions, or the
man page.

**Help attempt**: one run of a target CLI with one help form (`--help`, `-h`,
`help`, `help <sub>`). It ends read, rejected (the output was an error, not
help), timed out, or failed.

**Complete tree**: a command tree in which every node that discovery tried has
a help attempt that was read. Stub nodes and a depth limit do not make a tree
incomplete.

**Partial tree**: a command tree with at least one node whose help attempts all
ended unread, or one cut short by the run deadline. It always says so, and why.

**Failed discovery**: discovery whose root node could not be read at all. There
is no tree to show, only the reason. _Avoid_: empty tree, one-node tree.

**Discovery event**: something that happened during discovery (help attempted,
node discovered, node stubbed, discovery finished), which views of the tree can
follow as it happens.

**Node**: one command in a command tree, with its flags, positionals and
children. _Avoid_: entry, item.

**Stub node**: a node that is listed but not yet discovered; it is discovered
when someone expands it. _Avoid_: placeholder, lazy node.

**Virtual node**: a display-only group of flags that adds nothing to an
assembled command.

**Inherited flag**: a flag a node accepts because an ancestor defines it.
_Avoid_: global flag (unless the target CLI itself says global).

**Static output**: a command tree printed once, as a tree, JSON or YAML.

**The TUI**: the interactive terminal view of a command tree (`-i`).
_Avoid_: interactive mode, -i mode.

**Assembled command**: the command line built by picking a node, flags and
values in the TUI. _Avoid_: built command, preview.

**Try page**: the TUI running in the browser on treemand.dev.
_Avoid_: demo page, playground.

**Agent surface**: what treemand offers programs rather than people: the
schema-versioned JSON and YAML output and the MCP server (`treemand mcp`).
_Avoid_: API, integration.

## Measurement

**Probe**: a deterministic measurement of one aspect of treemand that emits
numbers and needs no judgement to run.

**Finding**: a probe result that breaks a rule (over budget, or worse than the
last recorded value). A finding is evidence, not an opinion.

**Corpus**: the committed help texts of real target CLIs, with ground truth for
what a correct command tree contains. It does not depend on which CLIs a host has
installed.

**Zoo**: a disposable container holding target CLIs from help-text families a
host lacks, so treemand can run them without touching a developer's machine.

## Hosts and the loop

**Dev host**: any machine a developer works on treemand from (today Linux or
macOS). No dev host is primary. Which OSes treemand is developed on is separate
from which it supports: users on every OS are supported. Each has its own installed target CLIs and
its own host-bound measurements.

**Loop host**: whichever dev host is running the dev loop at a given time; only
one at a time.

**Loop home**: the git worktree the loop runs from, beside the main checkout
(`treemand.loop`), detached at `origin/main`. It holds the loop's state
(`.loop/`) and its per-issue worktrees; a developer's checkout is never used.

**Cycle**: one pass of the dev loop that takes one piece of work from selected to
shipped (or parked), in one fresh agent session.

**Work**: a trusted, ready issue: one filed by the maintainer, either from a
conversation with the agent or from a finding. The loop never invents its own
work.

**Stand down**: the loop finding no work and exiting on its own. It stays
_enabled_: the next start or login checks again.

**Stop**: the maintainer turning the loop off. A stopped loop stays off across
reboots until it is started again. _Avoid_: pause.

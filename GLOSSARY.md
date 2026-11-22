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

**Dev host**: a machine a developer works on treemand from (the MacBook, the
ThinkPad). Each has its own installed target CLIs and its own host-bound
measurements.

**Loop host**: the one dev host running the dev loop at a given time.

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

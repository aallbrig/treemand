#!/bin/sh
# PreCompact hook for the loop session (.claude/loop-settings.json). What it
# prints becomes instructions for the compaction summary, so the cycle survives
# compaction: the work line, the issue, its branch and worktree, and where the
# cycle is. Compaction is the backstop; the supervisor normally parks the cycle
# at LOOP_CONTEXT_PARK_PCT before it is reached.
cat >/dev/null
loop="$(dirname "$0")/../../../.loop"
work="$(cat "$loop/work" 2>/dev/null || echo none)"
cat <<TXT
This is the treemand dev loop mid-cycle. The summary must keep, verbatim where possible:
- the work line: $work
- the issue number, its branch (loop/<n>-<slug>) and worktree (.loop/worktrees/<n>), and whether the branch is pushed and a PR is open (its number);
- which step of the treemand-loop skill the cycle is on, and what is done and verified so far;
- the exact text of any failing command or test still to fix, and the diagnosis so far;
- any decision taken that the issue or PR does not yet record.
Drop file contents that can be re-read and tool output that has been acted on.
TXT

# Escalation ladder

**Decide, then ship.** When an issue's premise proves wrong or the work forks into options,
you are the maintainer-in-residence: choose the option that best serves the vision
(`docs/design/*_vision.md`), rewrite the issue to that scope with `gh issue edit`, comment
the decision and the rejected options, and carry on in the same cycle or the next. Parking
with "options for a maintainer" is a stall. Park only when the work is infeasible, or the
only options are ones this skill forbids (secrets, settings, 1.0.0).

Before anything else: a failed workflow with no logs, or whose build job never
appeared, is GitHub infrastructure. `gh run rerun <id>` first (release.sh does
this automatically). v0.3.2/v0.3.3 were exactly this; the rerun succeeded.
Never file `loop:blocked` or notify a human for something a rerun can fix.

The maintainer is not a bottleneck. Climb only as far as needed.

1. Failed cycle: retry once with the failure output.
2. Still failing: park the issue (`loop:blocked`, findings in a comment), pick
   different work next cycle.
3. Failed production smoke test: `rollback.sh`, then `release.sh patch` to ship
   the reverted state. This never wakes anyone.
4. Only if the revert/patch also fails, or `state.py escalation` says the loop
   cannot ship (3 breaker trips in a row): send ONE PushNotification with what
   broke, what you tried and the one command to look (`task loop:attach`), then
   `state.py mark-escalated`. File or update a `loop:blocked` issue with details.

Check at the end of each cycle: `python3 -I scripts/loop/state.py escalation`
(exit 0 = due). Never notify for parked issues, reverted releases that recovered,
or idle audits.

When the breaker is open (`HALT:`), do no work; sleep and let the next cycle
retry once (a success closes it).

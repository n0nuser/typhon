# 012 — A benchmark arm that compared a configuration with itself

**Found:** re-reading `chooseActors` before designing the four-snake harness.
**Changed:** the `opponents` row in `BENCHMARK.md`, from a result to a
non-result, and [ADR 0005](../adr/0005-paranoid-with-opponent-reduction.md)'s
evidence section.

## What was published

The Phase A sweep ran an arm labelled `model 2 opponents vs 1` and reported
**14-16, p=0.855, not separated**. Both `BENCHMARK.md` and ADR 0005 carried it
as weak evidence: underpowered, but pointing at nothing in particular.

## What it actually measured

Nothing. `Searcher.chooseActors` ranks the live rivals by board distance and
searches the nearest `min(cfg.Opponents, live rivals)` of them. The sweep runs
duels, so `live rivals` is 1 for the whole game and the minimum is 1 whichever
way the flag is set. The two arms are not close configurations. They are the
same configuration.

Re-running the arm and the floor on the same seed block shows it without any
appeal to the source:

```
floor      alpha  turns=12206 mean_depth=5.47 nodes=48254201 aborted=11947 all_losing=16 deaths=16
           beta   turns=12208 mean_depth=5.45 nodes=48213457 aborted=11931 all_losing=14 deaths=14
opponents  two    turns=12206 mean_depth=5.47 nodes=48254201 aborted=11947 all_losing=16 deaths=16
           one    turns=12208 mean_depth=5.45 nodes=48213457 aborted=11931 all_losing=14 deaths=14
```

Identical in every field, down to the node count. The `opponents` run replayed
the floor run's thirty games with the arms relabelled. Its 14-16 is the floor's
own 14-16, and its p-value is the floor's p-value.

## Why it survived

Not because the check was missing. `docs/agents/rules.md` §5 says *verify a
feature fires before benchmarking it*, the harness prints per-path counters for
exactly that purpose, and the suite even prints a `DIFF` line naming what the
two arms differ in - which said `opponents 2 vs 1`, truthfully, about a flag
that resolves to the same thing.

It survived because every one of those checks is *within* a run. `oneply`
averaging 1.00 ply against `full`'s 5.08 was noticed because both numbers sit in
one block of output. Two arms that are identical to each other are also
perfectly consistent with each other, and the counters that prove it inert only
look wrong next to a *different* run's counters. Nobody put the floor block and
the `opponents` block side by side, because there was no reason to: the DIFF
line said they differed.

## The shape of it

This is [001](001-a-passing-test-that-tested-nothing.md) again. There, a
differential test passed on its first run while exercising almost none of the
rules; the pass was real and meant nothing. Here a benchmark arm ran to
completion, produced a plausible split and a plausible p-value, and compared
nothing.

Both failures share a signature worth naming: **a green result whose evidence is
that a procedure completed, not that the procedure touched the thing under
test.** A p-value cannot tell you its two samples came from the same
configuration. It will report 0.855 quite happily.

## What it does not change

The bot. `Opponents: 2` is still the default and is still a judgement rather
than a measurement - it was that before, just with a spurious 14-16 attached to
it. What changes is the size of the gap: the project believed it had a weak
answer and a strong one outstanding. It has only the outstanding one.

The measurement that would settle it needs a four-snake harness. `playGame`
builds two slots, and a four-snake floor run has to come first, because the
1,000-game start-position result was measured in duels and says nothing about
four start squares.

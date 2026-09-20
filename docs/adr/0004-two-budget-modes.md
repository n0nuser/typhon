# 0004 — Give the search a deadline budget or a node budget

**Status:** accepted

## Context

In play the constraint is wall-clock: the engine allows 500ms including the
round trip, and a late answer is scored as no answer.

In a benchmark, wall-clock is a liability. A game played under a deadline
depends on what else the machine was doing, so two runs of one configuration
differ by scheduler noise, and no number of games sees through that. The
predecessor's conclusions died of exactly this kind of variance.

## Decision

`search.Budget` carries **either** a `Deadline` **or** a `Nodes` count. Production
sets the deadline. The tournament harness sets nodes.

## Why it matters more than it looks

Under a node budget a game is **bit-for-bit reproducible from its seed**. Every
remaining source of variance is seed sampling, which is the thing more games
actually fix. Concretely it buys:

- Two runs of one configuration produce identical results, asserted by
  `TestAGameIsReproducibleFromItsSeed`.
- Games run eight at a time with no distortion, asserted by
  `TestParallelismDoesNotChangeTheResults`. Without this, parallelism would
  corrupt every timing-dependent decision, and running sequentially would make
  n=200 take most of a day - which in practice means it does not get run.
- A node budget reads **no clock at all**, so the search under it is a pure
  function of the position and the configuration.

## The gap it opens, and how it is bridged

A result at 4,000 nodes is a result about a bot nobody is running. So
`typhon-bench -calibrate` reports how many nodes a wall-clock budget buys on the
machine at hand, as a command rather than a number in a document, and the ratio
is recorded in `BENCHMARK.md`.

Currently: about 4.5µs a node in a duel, so the suites' 4,000 nodes is roughly
**18ms of thinking against the ~400ms a deployed turn allows** - about a
twentieth. Conclusions drawn there are conclusions at that depth, and the file
says so.

## What would make this wrong

If the effects being measured reversed with depth. Search beating one ply 191-9
at a twentieth of the budget should if anything understate the gap, but that is
reasoning and not measurement.

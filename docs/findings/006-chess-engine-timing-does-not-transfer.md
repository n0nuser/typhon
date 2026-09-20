# 006 — Reading the clock every 1024 nodes overran a 2ms budget to 86ms

**Found:** by a test, under `-race`, on a machine that was busy.
**Changed:** `internal/search/budget.go` and the budget check in `Search`.

## The inherited assumption

Chess engines check the clock every N nodes, typically 1024 or 4096, because
`time.Now()` costs tens of nanoseconds and a node costs tens of nanoseconds, so
checking every node would be a double-digit percentage of the search.

That reasoning was copied into this search without being re-derived, and the
constant carried a confident comment explaining why 1024 was safe.

## What the measurement said

`TestTheDeadlineIsHonoured` on an unloaded machine: passes. Under `-race` while
a benchmark suite was saturating eight cores:

```
budget 2ms:  took 86.569348ms
budget 10ms: took 51.629296ms
```

Forty-three times over on the tightest budget.

## Why the assumption did not transfer

A node in a chess engine is a make-move, an unmake, and some bookkeeping. A node
here rebuilds an occupancy map (`Passable`), hashes a whole position including
every snake's body, and at a leaf evaluates a Voronoi partition. The measured
cost is **about 4.5µs in a duel and 9µs with four snakes** - two orders of
magnitude more than the case the 1024 figure comes from.

At 4.5µs a node, 1024 nodes is 4.6ms between clock readings on an idle machine,
and far more under contention and the race detector. The granularity of the
guarantee was larger than most of the budgets being guaranteed.

## The fix

A deadline is now checked at **every node**. A node budget reads no clock at all,
which is what keeps a benchmarked game reproducible; the check interval only
applies when no deadline is set.

Cost, measured: about 1.7% of a node. Bought: the guarantee outright.

## Verification that matters

The interesting deployment is Render's free tier, which is shared CPU. So the
live games were re-run deliberately *while* a benchmark suite was running, at a
load average of 13 on eight cores:

**2,018 turns across eight snake-games, not one of them late.** `max_think`
428-451ms against a 500ms budget throughout, which is the search spending its
allowance and stopping rather than finishing early by luck.

## The general shape

The constant was not wrong. The reasoning behind it was right for the domain it
came from, and nobody checked whether this was that domain. A comment confidently
explaining a number is not evidence the number was measured here - which is
what `docs/agents/go-review-checklist.md` R9.5 asks about every stated rationale.

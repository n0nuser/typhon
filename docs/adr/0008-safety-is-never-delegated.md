# 0008 — Compute the safe move first and never delegate safety

**Status:** accepted

## Context

The brief is explicit: *never return an unsafe move, never return late; compute
a safe fallback first and hold it; every path out must already have an answer.*

The tempting structure is to let the search own the guarantee: it always returns
something, so what is a fallback for?

## Decision

Before the search starts, a one-ply safe move is computed and stored in the
result. Every subsequent step can only improve on it.

Concretely:

1. `safeMove` runs first and its answer is already in the `Result`.
2. Only a **completed** depth is ever promoted. A half-finished iteration has
   examined some of our options and not others, which is worse than not
   searching.
3. The search runs in the calling goroutine. **No goroutine on the move path** —
   nothing may be the reason a move is late or absent.
4. `panic` does not appear on the move path, and `log.Fatal` appears nowhere
   outside `main`.
5. If the board cannot even be built, the handler returns a direction anyway.

## Why the fallback is not redundant

Because the search's guarantee is conditional on the search running. A budget
too small to complete depth 1, an unrepresentable board, a snake already
eliminated — in each the search has nothing to say and something must still be
returned within the deadline. The engine moves a silent snake `up`, and `up` is
usually into something.

## Where it nearly went wrong anyway

The interaction between the fallback and the search is subtler than "prefer the
search". An early version let the one-ply check *overrule* a completed search
whenever every neighbouring square was contested — discarding the search's
knowledge that one loss arrives five turns later than another.

See [finding 005](../findings/005-counting-rivals-can-prefer-certain-death.md).
The rule now is that the fallback answers when the search has *nothing* to say,
never when it merely says something uncomfortable.

## Verified

- `TestNeverReturnsAMoveIntoAWallOrABody`, and a property test over 60 random
  positions requiring a legal move whenever one exists.
- `TestTheDeadlineIsHonoured`, plus **2,018 live turns under load with zero
  overruns**.

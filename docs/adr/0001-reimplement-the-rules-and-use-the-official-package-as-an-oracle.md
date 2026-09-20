# 0001 — Reimplement the rules; use the official package as a test oracle

**Status:** accepted

## Context

`github.com/BattlesnakeOfficial/rules` is the engine's own implementation and is
importable as a Go library. The obvious move is to call it from the search: it
is correct by definition and removes a whole class of divergence.

## Decision

Write our own forward simulator in `internal/rules`, and use the official
package as the **test oracle** rather than as the engine.

## Why

`rules.Execute` clones the whole board and allocates fresh slices on every call.
That is the right design for a referee that runs a few hundred times a game, and
it is unusable inside a search that wants to run it a hundred thousand times a
turn. Measured, our simulator applies and reverses a turn in **210ns with zero
allocations**; the official pipeline is orders of magnitude off that, and a GC
pause inside a 400ms budget is a missed deadline.

## What makes it safe

The usual objection to reimplementing rules is that the copy drifts. The answer
is that the original is still present, as the thing the copy is checked against:

- Random playouts run under both implementations and the boards are compared
  **after every turn**, for all four rulesets.
- Fourteen single-step scenarios pin the specific rules the brief warned about -
  stacked tails, equal-length head-to-heads, food cancelling hazard damage.
- The generator's own reach is asserted, so the comparison cannot go quiet
  (see [finding 001](../findings/001-a-passing-test-that-tested-nothing.md)).

This is stronger than hand-written property tests would have been, because a
hand-written expectation of what the rules say encodes the same misreading twice.

## Consequences

The module depends on the official package for tests only. The version is pinned
to v1.2.3, the same version the installed `battlesnake` CLI was built from, so
the oracle and the end-to-end harness agree.

Three divergences are deliberate and documented at their site: food spawning and
royale hazard expansion are seeded-random and are not modelled, and a hazard
square listed twice deals damage twice in the engine while this models hazards as
a set.

## What would make this wrong

A rules change that the differential test does not cover - a new stage, or a
map whose furniture is not hazards. The mitigation is that the oracle is a
dependency, so bumping it and re-running the tests is the check.

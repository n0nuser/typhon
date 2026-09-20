# 0010 — A game state belongs to one goroutine

**Status:** accepted

## Context

`State` is mutated in place ([ADR 0006](0006-ring-buffer-bodies-and-counted-occupancy.md)),
so `Apply` and `Unapply` obviously need exclusive access. Less obviously, so does
everything else: `Passable()` rebuilds state-owned scratch, so two apparently
read-only callers racing on one `State` is a bug.

## Decision

A `State` is owned by one goroutine. So is an `Evaluator` and a `Searcher`, each
of which owns mutable scratch. In practice:

- The server holds **one searcher per (game id + snake id)**, behind a mutex held
  for the whole turn.
- The harness builds **one state, one evaluator and one searcher per game**.

## Why this is load-bearing for the benchmarks, not just for correctness

A transposition table shared between games running in parallel makes each game's
result depend on what the *other* games happened to look up. That destroys the
reproducibility the whole n≥200 plan rests on — and it does so while every unit
test stays green, because nothing about a single game is wrong.

`TestParallelismDoesNotChangeTheResults` runs the same four games one at a time
and four at a time and requires them identical. That is the assertion that would
fail.

## Why the key is game id *and* snake id

One server can back several snakes in the same match; that is how a
one-against-three test is arranged. Keying on the game alone would have those
snakes share a search table and a latency estimate, and quietly make each one's
move depend on the others'.

## How the contract was discovered

Not by design. Two parallel subtests shared a `State` and the race detector
pointed at `Passable`. The contract was then written onto the type, where it
should have been from the start.

## Consequences

The store is bounded: evicted on `/end`, swept on a TTL for matches the engine
abandons. An unbounded map keyed by game id is a leak with a timer on it.

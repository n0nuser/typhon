# 0006 — Ring-buffer bodies, counted occupancy, explicit undo

**Status:** accepted

## Context

A search walks one state and needs to undo every move it makes. The alternative
is copying the board per node, which is what the official rules do and what
[ADR 0001](0001-reimplement-the-rules-and-use-the-official-package-as-an-oracle.md)
rejects.

## Decision

Three linked choices:

**Bodies are ring buffers of cell indices.** Moving is a single decrement of the
head index: the new head goes into the slot that opens and the old tail falls off
the end. Nothing is copied. A slice-based body shifts every segment on every
move, which at a hundred thousand nodes a turn is the difference between
searching six plies and three.

**Occupancy counts segments per square rather than flagging them.** This is what
makes a stacked snake - one whose body holds the same coordinate twice, at the
start of a game or the turn after it eats - keep that square blocked when only
one of its two segments leaves. The brief named this as a bug that cost the
predecessor games; counting removes the possibility rather than guarding against
it.

**`Apply` returns an `Undo` that holds no slices.** Popped tails, growths,
clobbered ring slots, health and elimination cause are all fixed-size. The one
variable-length part, eaten food, lives on a stack the state owns and the `Undo`
indexes into.

## Measured

`BenchmarkApplyUnapply`: **210.5 ns/op, 0 B/op, 0 allocs/op** — about 4.7 million
simulator steps a second.

## The subtlety that needed checking empirically

Growth duplicates the tail, and in a ring the slot it must write is the one the
discarded tail is still sitting in. The reasoning is easy to get backwards, and
the stacked-start case hides the error because the two values are equal. It is
verified against the official ruleset rather than by argument: the differential
test compares bodies coordinate by coordinate from a `[A,A,A]` opening.

## Consequences

A `State` is mutable and owned by one goroutine — see
[ADR 0010](0010-state-is-single-goroutine.md).

## What would make this wrong

A profile showing the simulator is no longer the bottleneck. It already is not:
one evaluation costs about thirty simulator nodes, so further work belongs in
`internal/eval`, not here.

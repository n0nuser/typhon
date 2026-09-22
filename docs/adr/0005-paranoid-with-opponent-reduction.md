# 0005 — Paranoid opponents, and model only the nearest few

**Status:** accepted

## Context

[ADR 0003](0003-simultaneous-move-nodes.md) makes branching multiplicative in the
number of snakes modelled. Three opponents searched exhaustively is 4⁴ = 256
joint moves per ply, most of them spent on snakes that cannot reach us inside the
horizon.

## Decision

Search the `Opponents` nearest rivals properly (default 2); advance the rest with
a cheap greedy one-ply move. Opponents are assumed to play the move worst for us.

Nearest is by the **board's own metric**, from `Topology.Distance`.

## Why nearest-by-topology and not by coordinate

On a wrapped board the snake at the far edge is one step away. A ranking that
used raw coordinate difference would ignore the one snake about to take our
square, in precisely the ruleset the predecessor already got wrong. Every
distance in this module goes through one function for this reason.

## The evidence

There is none, again, and this section has now been wrong in both directions.

`opponents=2` against `opponents=1`, four snakes, once read **123-71, p=0.0003**
for modelling two. That run was made on a harness that paid arm A a bias worth
9.5 points, and `two` was arm A; see
[findings/015](../findings/015-the-board-was-choosing-the-winner.md). On the
mirrored harness the same comparison came back the *other* way at p=0.0402, and
then did not replicate on a second seed block: **p=0.41, not separated.**

So: one result through a known bias, one unreplicated result against it, and
nothing that survives both. The default stands on the reasoning below and on no
measurement at all.

This replaces the evidence this section used to cite. An arm labelled the same
thing was run at n=30 in a **duel**, where `chooseActors` takes
`min(Opponents, live rivals)` and the live rival count is 1: both arms searched
one opponent and played bit-identical games. Its 14-16, p=0.855 was the floor
run wearing different labels. See
[findings/012](../findings/012-an-arm-that-compared-a-flag-with-itself.md), and
[findings/013](../findings/013-breadth-buys-what-depth-buys.md) for why breadth
beats the ply it costs.

The case where it matters is four snakes, and the calibration says what is at
stake: 400ms buys **depth 7 against one rival and depth 3 against three**.
Whether depth 3 with two opponents modelled beats depth 5 with one is an open
question and is recorded as such in `BENCHMARK.md`.

So this default is a **judgement, not a measurement**, which is what it was at
the end of the first build and what it has returned to being. Three attempts
have now been made on it: one that compared the flag with itself, one that was
measured through a harness bias, and one that did not replicate.

## What would make this wrong

A four-snake n=200 run showing `opponents=1` ahead. That run has now been made
and shows the opposite, so what would make this wrong has narrowed: an
`Opponents: 3` arm beating 2 by a margin worth the branching, or the 2-vs-1
result failing to reproduce at a deployed node budget rather than the twentieth
of one it was measured at.

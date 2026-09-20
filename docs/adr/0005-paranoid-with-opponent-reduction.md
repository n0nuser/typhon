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

## The evidence, such as it is

`opponents=2` against `opponents=1`, n=30 in a duel: **14-16, p=0.855, not
separated.** That run is close to uninformative - in a duel there is only one
opponent to model, so the flag barely does anything.

The case where it matters is four snakes, and the calibration says what is at
stake: 400ms buys **depth 7 against one rival and depth 3 against three**.
Whether depth 3 with two opponents modelled beats depth 5 with one is an open
question and is recorded as such in `BENCHMARK.md`.

So this default is a **judgement, not a measurement**, and the honest position is
that the arm that would settle it was run in the wrong game size.

## What would make this wrong

A four-snake n=200 run showing `opponents=1` ahead. That run is the single most
valuable outstanding measurement in the project.

# 0003 — Advance a node only once every snake has committed

**Status:** accepted

## Context

Battlesnake is a **simultaneous-move** game: every snake commits before anyone
sees the result, and collisions resolve against where everybody moved to. Almost
every available search technique - alpha-beta, iterative deepening,
transposition tables - is written for alternating play.

The tempting shortcut is to treat it as alternating: we move, the board updates,
then the opponent chooses.

## Decision

A node collects a move from us and from each modelled opponent into a pending
buffer, and the rules step runs **once**, at the leaf of that joint move.

## Why

The shortcut makes the opponent omniscient. If the board updates before the
opponent chooses, the opponent is choosing with knowledge of our move, which it
does not have. The resulting evaluation is pathologically pessimistic and the
bot refuses moves that are perfectly safe - every square a rival *could* reach
becomes a square it *will* reach, because in the model it has already seen us go
there.

It also gets head-to-heads wrong in the one direction that matters. A head-to-head
resolves against post-move positions of both snakes; an alternating model
resolves ours against the rival's *old* body.

## Cost

Branching is multiplicative within a ply rather than additive: our four moves
times each modelled opponent's four. With two opponents modelled that is 64 joint
moves per ply, against 4 for an alternating model at the same nominal depth.

The calibration shows what that buys and costs: **depth 7 in a duel at 400ms,
depth 3 with four snakes**. See [ADR 0005](0005-paranoid-with-opponent-reduction.md)
for the mitigation.

## What is still paranoid

Opponents are assumed to pick the move worst for us. That is conservative rather
than accurate - a real opponent is optimising for itself, not against us - and in
a game where one mistake is terminal it is the right bias. It is also why the
root needs a tie-break for the all-contested case
([finding 005](../findings/005-counting-rivals-can-prefer-certain-death.md)).

## What would make this wrong

Evidence that the paranoid assumption costs more than the simultaneity buys -
for instance if a max-n or opponent-modelling variant measurably outplayed it.
That comparison has not been run.

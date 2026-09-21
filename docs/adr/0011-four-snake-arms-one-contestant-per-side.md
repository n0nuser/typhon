# 0011 — Four-snake arms run one contestant per side, against neutrals

**Status:** accepted — built, and used for the run in `BENCHMARK.md`'s
four-snake section.

## Context

[ADR 0005](0005-paranoid-with-opponent-reduction.md) sets `Opponents: 2` as a
judgement, and says what would make it wrong: a four-snake n=200 run showing
`opponents=1` ahead. That run has never been possible.
[findings/012](../findings/012-an-arm-that-compared-a-flag-with-itself.md)
records why the run that claimed to be it was not: `chooseActors` searches
`min(Opponents, live rivals)` rivals, and a duel has one live rival, so both arms
played bit-identical games.

`cmd/typhon-bench` is built for duels. `playGame` hardcodes two ids, `perArm` is
a `[2]counters`, slot assignment is an `aFirst bool`, and `outcome` has three
values. `main.go`, `report.go` and `stats.go` all assume two arms in two slots.

## Decision

A four-snake run places **one snake per arm and two neutral snakes**, where the
neutrals play the published default configuration.

Scoring is **"A outlived B"**, decided from elimination turn rather than from
last-snake-standing.

Slot assignment becomes a rotation index rather than a boolean, so that each arm
occupies each of the four start squares equally often across the seed block.

## The alternative rejected: two snakes per arm

Two of each doubles the exposure per game, which is the obvious appeal. It also
breaks the thing the numbers rest on.

With 2×A and 2×B, "who won" stops being binary — A and A' can be the last two
alive, and A can eliminate A. The paired McNemar test in `stats.go` takes two
discordant cells and nothing else; there is no cell for "arm A took first and
third". Every published result in `BENCHMARK.md` is a McNemar on a binary
outcome, so a four-snake result scored any other way would not be comparable to
any of them, and the machinery would have to be rebuilt to produce a number
nobody could place beside the existing ones.

One contestant per arm keeps the comparison binary and `stats.go` untouched.

The cost is honest and should be stated in the run header: the neutrals play a
stated reference configuration, so a four-snake result is "A beats B in a field
containing two default snakes", not "A beats B in the general four-snake case".

## Why elimination turn and not last-standing

Four-snake games reach the turn cap far more often than duels, and a
last-standing rule scores every capped game a draw, discarding most of the
sample. Elimination turn keeps them.

`counters.deaths` and `counters.deathTurnSum` already carry what is needed, with
one trap: **`deathTurnSum` is zero for a snake that never died**, so a naive
comparison reads a survivor as having died on turn 0 and hands the game to the
loser. The three cases are branched explicitly: both survive is a draw, exactly
one survives is a win for that arm, both die goes to the later death and ties to
a draw.

## Why the floor run comes first, and is not optional

`BENCHMARK.md` puts the starting slot at 49.3%, [46.2%, 52.4%], over a thousand
decisive games. **Every one of those games is a duel.** Four start squares on
11x11 is a larger asymmetry than two, not a smaller one, and nothing measured
here bears on it.

So a four-snake floor run — all four snakes identical, n=200, per-slot split
reported — lands before any four-snake arm result is read.
[findings/003](../findings/003-the-floor-that-was-never-there.md) is the
precedent: the predecessor's whole noise story rested on a floor it never
established, and this project's first Phase B run was a floor for that reason.

## What the floor said

The floor run was made before any arm result was read, as this record requires.
All four start squares came back inside their 25% null — 24.9%, 22.8%, 26.9% and
25.4% — so rotation across four squares absorbs whatever asymmetry exists at
n=200, and the mirrored design held in reserve below was not needed. Draws were
3 in 200, so the paired test keeps essentially the full sample.

## What would make this wrong

A four-snake floor run whose per-slot split is large enough that rotation across
four squares cannot absorb it. If one start square is worth ten points, an arm
comparison needs either many more games or a mirrored design — the same seed
played twice with the two contestants exchanging squares. That did not happen
here, and the check is worth repeating on any board or ruleset where the start
squares are laid out differently.

## Cost

Two n=200 runs at roughly 30-45 minutes each — four snakes searching per turn,
over longer games, against the duel suite's eleven minutes — plus the harness
change across four files and the pre-merge walk of
[`go-review-checklist.md`](../agents/go-review-checklist.md).

# 001 — A differential test that passed while testing almost nothing

**Found:** while building `internal/rules`.
**Changed:** the playout generator, and added `TestPlayoutsExerciseEveryRule`.

## What happened

The forward simulator is checked against `BattlesnakeOfficial/rules` by playing
random games under both and comparing the board after every turn. The first
version picked each snake's move uniformly at random from the four directions.

It passed on the first run. That is the moment the repository's own rule -
*when a result confirms what you expected, that is precisely when to check it a
second way* - earned its place.

Instrumenting the generator showed what it had actually exercised, across sixty
games per ruleset:

| Ruleset | Turns | Growths | Head-to-heads | Starvations | Hazard deaths |
| --- | --- | --- | --- | --- | --- |
| standard | 213 | 2 | **0** | 0 | — |
| wrapped | 292 | 3 | **0** | 0 | — |
| royale | 213 | 2 | **0** | 0 | **0** |
| constrictor | 208 | 278 | **0** | — | — |

**3.5 turns per game.** Two growths across sixty standard games. And not one
head-to-head in any ruleset - the rule that decides the opening of every
Battlesnake game, and the one the brief singled out as having cost the
predecessor real matches.

The test could not have caught the bug it was written to catch.

## Why

Four snakes moving uniformly at random on an 11x11 board kill themselves almost
immediately. Three of four directions from a starting position walk into your
own stacked body, so a game is over before the rules that matter are reached.

## The fix

Pick at random among the moves that are not *immediately* fatal - not into a
wall, not into an occupied square other than a vacating tail. Still random, still
unsophisticated, but alive long enough to be interesting:

| Ruleset | Turns | Growths | Head-to-heads |
| --- | --- | --- | --- |
| standard | 3,395 | 168 | 2,541 |
| wrapped | 5,312 | 211 | 3,057 |
| royale | 1,564 | 108 | 415 |
| constrictor | 848 | 1,867 | 66 |

Every elimination cause now fires, including hazard deaths and starvation.

## What was kept

`TestPlayoutsExerciseEveryRule` asserts the reach rather than assuming it: it
requires each ruleset to produce every cause that ruleset can produce, at least
one growth, and at least 500 turns across the sixty games. Wrapped is exempted
from out-of-bounds because a torus has no walls, which is the point of it.

If someone later changes the move policy and the generator stops producing
head-to-heads, that test fails. Without it the differential test would simply go
quiet and keep passing.

## The general shape

A test that exercises nothing passes. It is indistinguishable, from the outside,
from a test that exercises everything and finds no fault. The only defence is to
measure the test's own reach and assert it - which is the same discipline this
project applies to benchmark arms in [004](004-how-search-actually-wins.md), and
which the predecessor's log records failing to apply when it benchmarked a
feature whose threshold was never crossed.

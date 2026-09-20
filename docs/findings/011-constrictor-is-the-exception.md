# 011 — Search beats one ply in three rulesets out of four

**Found:** running the per-ruleset arms.
**Changed:** a claim in `BENCHMARK.md` that had been written from the wrong
evidence.

## The result

Full search against the same evaluation capped at one ply, n=30 each:

| Ruleset | Full | One ply | Paired p | Reading |
| --- | --- | --- | --- | --- |
| standard (n=200) | 191 | 9 | <0.0001 | search pays |
| royale | 29 | 1 | <0.0001 | search pays |
| wrapped | 28 | 2 | <0.0001 | search pays |
| **constrictor** | **18** | **11** (1 draw) | **0.265** | **not separated** |

Royale and wrapped reproduce the standard result at the same magnitude, which is
some evidence the effect is about lookahead rather than one ruleset's quirks.

Constrictor does not.

## The claim it corrected

The live-game section had recorded constrictor reaching **depth 20** - deeper
than any other ruleset, because there is no food, the board fills, and the
branching collapses - and had continued:

> It is the ruleset where lookahead should matter most and the search behaves
> accordingly.

That is reasoning from *a depth was reached* to *that depth was worth something*.
They are different claims and only the first had been measured. The per-ruleset
arm then measured the second and did not support it.

The text now says so, and says which of the two was measured.

## A hypothesis, labelled as one

In constrictor every snake grows every turn and health is pinned at maximum.
There is no food to contest, no starvation to time, and the board fills steadily
until most positions are close to forced. There may simply be less for lookahead
to find: when three of your four moves are walls, depth eight tells you what
depth one already did.

That is a story, not a result. 18-11-1 at n=30 is not separated, and the honest
statement is that this run does not know. Settling it needs n=200, which has not
been run.

## Why it is worth a file

Two reasons. It is the one place where the project's headline claim has a known
exception, and an exception stated up front is worth more than one discovered by
a reader. And the correction it forced is a textbook R9.5 failure - a stated
rationale that had not been checked against the artifact it claimed - committed
in the document that argues for checking them.

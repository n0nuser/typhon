# 013 — Breadth buys what depth buys, and in a crowd it buys more

> **Withdrawn.** Every number below was measured on a harness that handed arm A
> a bias worth 9.5 points, with `two` sitting in arm A — see
> [015](015-the-board-was-choosing-the-winner.md). Re-run on the mirrored
> harness the same comparison came back `one` ahead at p=0.0402, and then
> **failed to replicate** on a second seed block at p=0.41. The honest position
> is that nothing separates `opponents=2` from `opponents=1`, and that this file
> records how a bias and a single unreplicated run can agree with each other.
>
> It is kept rather than deleted because a finding that corrected an earlier one
> says so rather than replacing it, and because the mechanism it describes —
> that the counter explaining depth also explains breadth — is the part that was
> never the problem. What it lacks is any evidence that the effect is real.

**Found:** running the four-snake `opponents` arm, the measurement
[findings/012](012-an-arm-that-compared-a-flag-with-itself.md) showed had never
been made.
**Changed:** `Opponents: 2` from a judgement to a measurement, and
[ADR 0005](../adr/0005-paranoid-with-opponent-reduction.md)'s evidence section.

## The result

Two hundred four-snake games, one snake per arm and two default snakes filling
the field, 4,000 nodes a turn, seeds 9100-9299:

```
RESULT opponents-4p: two=123 one=71 draw=6 of 200 games in 22m53s
SHARE  two took 63.4% of decisive games, 95% CI [56.4%, 69.9%]
PAIRED McNemar on 194 discordant games: chi2=13.41 p=0.0003 -> two is better
```

**123-71, p=0.0003.** Modelling two opponents beats modelling one.

This is the first structural arm in the project that separates. Voronoi control,
tail reachability and opponent confinement all came back null, two of them at
n=200. This one is not close to the line.

## Why it is not the obvious trade

Modelling a second opponent multiplies the branching factor by four, and it buys
exactly what you would expect it to cost: the arm modelling two rivals reached a
mean depth of **4.20**, the arm modelling one reached **5.28**. It searched more
than a ply shallower and won anyway.

The counter that explains it is the same one that explained why depth beats one
ply in [004](004-how-search-actually-wins.md) — `all_losing`, the turns on which
every legal move was contested by an equal-or-longer rival:

| | mean depth | all-losing turns | rate | mean death turn |
| --- | --- | --- | --- | --- |
| models 2 rivals | 4.20 | 107 of 53,631 | 0.20% | 219 |
| models 1 rival | 5.28 | 163 of 35,309 | 0.46% | 135 |

The shallower arm walks into hopeless positions **less than half as often per
turn**, and survives about eighty turns longer.

So the mechanism is the one already on record, arriving by the other road.
Finding 004 said depth does not help a snake survive a lost position, it stops
it reaching one. Breadth does the same job. With three rivals on the board, the
snake that is about to take your square is frequently not the one you chose to
model, and no amount of depth against the wrong snake sees it coming.

## What it does not say

**Not that more is always better.** This is 2 against 1. Three rivals modelled
exhaustively is 4⁴ = 256 joint moves a ply, and nothing here bears on whether
that pays. `Opponents: 3` remains untested.

**Not a claim about any four-snake game.** The field was two snakes running the
shipped configuration. A four-snake result is a comparison made inside a field,
and the field is part of the claim.

**Not necessarily true at the deployed budget.** The run is at 4,000 nodes,
about a twentieth of what a 400ms turn buys. Both arms get deeper with more
budget and the breadth factor is multiplicative either way, so the trade should
hold — but that is reasoning, not measurement, and it is the same reasoning the
one-ply result carries.

## The part worth keeping

The arm that claimed to have tested this reported **14-16, p=0.855, not
separated** and was cited twice as evidence for leaving the default alone. It
had compared a configuration with itself. Run properly, the same question
answers **123-71, p=0.0003** in the same direction the default already pointed.

The default was right. The evidence for it was worthless, and the two facts are
unrelated — which is the entire reason this project measures things.

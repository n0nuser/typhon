# What the rules engine actually does

Read from `github.com/BattlesnakeOfficial/rules@v1.2.3` in the module cache, not
from the prose documentation. Every claim here cites the file it came from, and
every one of them is exercised by the differential test.

The prose docs are not wrong so much as incomplete: several of the details below
decide games and appear nowhere in them.

## The stage order

`standard.go:8`:

```
GameOverStandard -> MovementStandard -> StarvationStandard
  -> HazardDamageStandard -> FeedSnakesStandard -> EliminationStandard
```

Variants differ only at the edges:

- **wrapped** (`wrapped.go:3`) swaps `MovementStandard` for
  `MovementWrapBoundaries`, which runs standard movement and then wraps the head.
- **constrictor** (`constrictor.go:3`) appends `SpawnFoodNoFood` and
  `ModifySnakesAlwaysGrow`.
- **royale** (`royale.go:7`) appends `SpawnHazardsShrinkMap`. Note this exists
  *both* as a ruleset stage and as `maps/royale.go`'s `PostUpdateBoard` — pair
  `-g royale` with the royale map or the hazards never appear.

**GameOver runs first.** With one snake left, `Execute` returns immediately and
applies no moves. A simulator that moves anyway diverges at the end of every
duel.

## Movement always pops the tail

`MoveSnakesStandard` appends the new head and pops the tail *unconditionally*.
Growth is `growSnake`, called from `FeedSnakesStandard`, and it appends a copy of
the **current last element**.

This single ordering is the whole mechanism behind two things the brief warns
about:

- "A tail frees up as the snake moves, except when it just ate."
- "`body` contains duplicate coordinates while a snake is stacked."

Model the pop and the duplication and neither needs a heuristic. The practical
consequence is easy to state backwards: after a snake eats, the square that stays
occupied is the **second-to-last** segment, not the one that was the tail.

## Hazard damage, with two surprises

`DamageHazardsStandard`:

- **Food on the square cancels the damage outright** — not reduces it, cancels
  it, via a `continue`. This is not in the prose docs.
- It loops `b.Hazards` per snake, so a square listed **twice deals damage
  twice**.
- The default is `settings.Int(ParamHazardDamagePerTurn, 0)` — **zero**. The CLI
  passes 14. A missing field is not "14 damage".

## Parameter names are not wire names

`constants.go`: `ParamHazardDamagePerTurn = "damagePerTurn"`. The **wire** JSON
field is `hazardDamagePerTurn`. They are different strings; passing the wire name
to `rulesetBuilder.WithParams` leaves royale at its default while looking
configured.

Similarly `shrinkEveryNTurns` defaults to **20** in the library and **25** in the
CLI flag.

## Elimination is two passes, and the difference is visible

`EliminateSnakesStandard`:

1. **Pass one**, applied immediately: out of health (`Health <= 0`), then out of
   bounds (any body point outside). Order matters — a snake that both starves and
   leaves the board is recorded as out-of-health.
2. **Pass two**, collected and applied *afterwards*: self-collision, then body
   collision, then head-to-head.

The consequence: a snake eliminated in pass one **blocks nobody** in pass two, so
a rival moving into the square it just vacated survives. A snake eliminated *in*
pass two still blocks, so a rival moving into its body dies with it. One pass
with immediate removal gets both wrong.

Two details inside pass two:

- `snakeHasBodyCollided` skips `other.Body[0]`, so head-on-head is handled
  **only** by the head-to-head rule. That is what lets a longer snake survive one.
- `snakeHasLostHeadToHead` is `len(s) <= len(other)`, so **equal lengths kill
  both** — each satisfies it against the other.

## Constrictor applies to the dead

`GrowSnakesConstrictor` does **not** check `EliminatedCause`. It sets health to
maximum and grows snakes that have already left the board. A differential test
must therefore compare only the living, or it will chase a phantom on every
constrictor game with a death in it.

It also reads `Body[len-2]` directly, so it would panic on a length-one snake.
Not reachable in normal play; reachable from a search.

## What is deliberately not modelled

Food spawning and royale hazard expansion are driven by a seed this process never
sees. Predicting them would be fiction, so both sets are held static across a
search horizon, and the differential test runs with `foodSpawnChance=0` so the
two implementations stay comparable.

# 008 — The harness's `fallbacks` column was the win column in disguise

**Found:** noticing that a number was suspiciously round.
**Changed:** `cmd/typhon-bench/game.go`.

## Two counters, both wrong, found the same way

### `mean_first_death`

The n=30 search-pays run reported the winning arm's mean first death as **turn
11**, while it was winning 28 games of 30. The number was averaged over every
game including the ones it survived, so two deaths at turn 165 divided by thirty
games gave 11.

Fixed by reporting a count and a mean over the deaths: `deaths=2
mean_death_turn=170`.

### `fallbacks`

Then the random control at n=200 reported `fallbacks=200` for the searching arm.
Exactly 200, in 200 games. The floor run reported 98 and 102 - against deaths of
102 and 98. The search-pays run reported 191 and 9, which were precisely the two
arms' win totals.

`fallbacks` counts turns where the search completed no depth. It was also
counting the turns where the game was already decided and the last snake standing
was asked to move - one per game won. **The failure column was the win column.**

Fixed by only counting a turn that is genuinely undecided.

## Why this matters more than the arithmetic

These counters exist for one reason: the predecessor benchmarked a feature whose
threshold was never crossed, and the fix is to print how often each path was
reached beside the result. That only works if the counters mean what their names
say.

A counter that reads backwards is worse than no counter, because it will be read.
`fallbacks=200` in a 200-game run looks like a bot failing to search on every
game; it was a bot winning every game.

## Not backdated

The Phase B numbers recorded in `BENCHMARK.md` were produced before the fix, and
their `fallbacks` figures are left as they were with a note explaining how to
read them. `docs/agents/rules.md` forbids quietly editing a recorded
measurement, and that rule does not have an exception for measurements that are
embarrassing.

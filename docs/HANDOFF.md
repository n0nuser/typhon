# Handoff

Written at the end of the first build. It says what exists, what is known, what
is believed but unmeasured, and what the next person should do first.

## What this is

A Battlesnake that searches, in Go, at `github.com/n0nuser/typhon`. It replaces
`n0nuser/battlesnake-jev`, which scored the four squares next to its head and
nothing beyond. Typhon reaches a mean depth of 7-8 in a duel inside the same
500ms turn.

## State: working, measured, not deployed

| | |
| --- | --- |
| Gates | `make check` — gofmt+gofumpt, vet, golangci-lint v2, `go test -race -cover`, build. Green. Enforced by a pre-push hook that has blocked pushes. |
| Tests | 7,200 lines of Go; coverage 82-97% per package; simulator differentially tested against the official rules on every turn of thousands of random playouts |
| Live play | Deployed on Render free tier. One game against `det`: 203 turns, length 24, mean depth 8, 75,240 nodes a turn, zero overruns - and still lost, to a timeout the bot's clock could not see |
| Benchmarks | Rewritten from nothing on the mirrored harness: floors at both parities, two headline arms, six evaluation terms each replicated on a second seed block. See `BENCHMARK.md` |
| Deployed | **Yes**, Render free tier, 0.1 CPU. Sleeps after 15 minutes; leaderboard games run once a day in a batch, so a cron needs to warm it for that window only. |

## What is established

- **Search pays**: full search took **192 of 200 boards** from a one-ply arm,
  and a random control **200 of 200**. Duel, 400 games.
- **Three evaluation terms were costing games**, each replicated on a second
  seed block: centre (off wins 103-14, 107-21), tail reachability (69-27,
  77-33) and the Voronoi/confine pair (69-29, 64-32). They are now zero.
- **Two earn their place**: food (79-16, 77-24) and space (57-34, 64-27).
  Length does not separate either way and is kept on "not shown to hurt".
- **The Voronoi partition was 85% of the whole search**, and the evaluation is
  **8.6x cheaper** without it. Live, that took the bot from 20,498 nodes a turn
  to 75,240, and from mean depth 6 to 8.
- **There is no start-square effect left to worry about**, because the harness
  now cancels it: every board is played twice with the contestants exchanged,
  and all four floor runs return zero decided boards.

Royale, wrapped and constrictor have **not been re-measured** since the harness
was rebuilt. Neither has the duel case for any evaluation term. Everything in
the list above is four snakes on 11x11 standard.

## What is believed but not measured

Read these before changing anything, because each is a place where the code
embodies a guess:

1. **The magnitudes of the three surviving weights.** Each term has been tested
   on its own against absent, and replicated. Weight *space* has not been
   searched: `space` at 12 instead of 6 changes almost nothing, and `food` at 12
   looked like a win at p=0.0003 and then reversed on a second block. There is
   very likely a better set of numbers than `space=6, length=30, food=4`.
2. **`Opponents: 2`.** Measured three times, wrong twice: once comparing the
   flag with itself, once through the harness bias, once unreplicated. Nothing
   separates it from `Opponents: 1`. `Opponents: 3` untried.
3. **Every ruleset but standard, and the duel case.** Each evaluation arm above
   is four snakes on 11x11 standard. Royale, wrapped and constrictor have not
   been re-measured since the harness was rebuilt, and neither has the duel.
4. **That any of this holds at the deployed budget.** The arms run at 4,000
   nodes; the live instance does about 75,000 a turn.
5. **That the deadline holds in production.** Better than it was - one live game
   went 203 turns with zero overruns after the ceiling landed - but that same
   game ended on a timeout the bot's own clock could not see. See
   [findings/014](findings/014-the-budget-modelled-two-of-three-costs.md).

## What to do first

In order of value:

1. **Play it and watch the logs.** The evaluation lost four of its seven terms
   and the deadline gained a ceiling, and no live game has been played since.
   `mean_depth` should be well above 8 now; `timeout_overruns` should stay at
   zero. If it does not, drop `TYPHON_BUDGET_CEILING` below 0.60 — no rebuild.
2. **Sweep the three surviving weights.** They were each tested at one or two
   magnitudes, never searched. A 400-game arm is about eight minutes now, so
   this is affordable in a way it was not before the search got 8.6x cheaper.
3. **Re-measure the other three rulesets and the duel.** Everything established
   above is four snakes on 11x11 standard.
4. **Keep the cron warm window.** Leaderboard games run once a day in a batch, so
   the service needs waking for that window and not around the clock — 750 free
   instance hours a month against 720 in a 30-day month.
5. **Typhon vs `battlesnake-jev`.** The headline nobody has run. It needs the old
   binary over HTTP at ~0.5s a turn — 8+ hours — and the in-process harness
   cannot drive an external server, so it needs a separate driver.

And one standing obligation rather than a task:

**Any change to `internal/search` or `internal/eval` invalidates every number in
`BENCHMARK.md`.** Re-run the arms in the same change, or delete the entry. This
is `docs/agents/rules.md`'s rule about regenerated artifacts, and it is the one
most likely to be skipped, because the numbers are already there and look
authoritative.

**And one rule that is newer and earned its place immediately: anything that
separates gets replicated on a second seed block before it is believed.** It was
committed to before the arms were run and it caught three results out of nine,
including `food=12` at p=0.0003 — a larger margin than several things now
recorded as established — which reversed on the second block.

## How to run things

```sh
make check                    # the gate
make phase-a                  # n=30 sweep, diagnostic only
make phase-b                  # n=200, the three arms that answer a question
make e2e                      # one live game per ruleset (needs ./typhon running)
go run ./cmd/typhon-bench -calibrate    # what a wall-clock budget buys here
go run ./cmd/typhon-bench -help-spec    # what an arm can vary
```

## Traps

The tooling ones are in [`docs/agents/gotchas.md`](agents/gotchas.md) — things
that silently do the wrong thing rather than failing. The two that bear on the
bot itself:

- **Node budgets make results immune to CPU contention**; wall-clock budgets do
  not. Never benchmark on a clock.
- **`you.latency` includes your own think time.** See
  [findings/007](findings/007-latency-is-not-round-trip.md).

## Where to read

- `BENCHMARK.md` — every measured number, with the negative results kept
- `docs/findings/` — fifteen things that would not have been guessed
- `docs/adr/` — eleven decisions, each with the alternative it rejected
- `docs/research/` — the rules engine's real semantics; the turn budget; a
  re-analysis of the predecessor's conclusions
- `.review/main-checklist.md` — the pre-merge pass, 58 rows, four of which failed

## The one thing worth internalising

`docs/findings/015`. A bot beat an identical copy of itself, 120-80, at
p=0.0058. Every safeguard this project has was running at the time — the floor
run, the per-slot rate printed beside every arm, the rule against publishing
below 200 games — and not one of them could see it. The per-slot number was
level, and correctly so: across the run each slot won about half the games. The
imbalance was not slot-against-slot, it was arm-against-board, and no statistic
computed over a run as a whole contains that.

**A control that aggregates cannot detect a bias that correlates.** That is the
third time this repository has met that shape: a differential test that passed
while exercising almost none of the rules, a benchmark arm that compared a
setting with itself and reported a p-value about it, and now a harness in which
the board picked the winner and the arm took the credit.

The diagnosis took one command — shift the seed base by one, and the result
mirrored exactly. A property of the bots cannot invert when the first seed
changes by one; a property of the boards can.

The cost was every number this project had published. The gate was green
throughout.

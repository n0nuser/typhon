# Typhon

A Battlesnake that searches.

Typhon is the serpent-headed monster of Greek myth — the one storm-giant that
made the Olympians run. The name is a statement of intent about the decision
procedure, not the win rate.

## Why this exists

[`battlesnake-jev`](https://github.com/n0nuser/battlesnake-jev) is deployed and
works, but its decision is **one ply**: it scores the four squares next to its
head and picks the best one. It cannot see a trap closing three moves out.

Meanwhile the turn budget goes almost entirely unspent. Battlesnake allows
500ms per turn *including* the round trip; the engine reports 44ms of round
trip from Frankfurt, and the old bot's decision takes 0.14ms at p50. Roughly
**400ms of compute per turn is going unused**.

Typhon spends it on iterative-deepening search over a forward simulator of the
real rules. Fully deterministic — no model inference anywhere in the move loop.

## What it does

**Iterative-deepening search**, not a scoring heuristic. Typhon deepens one ply
at a time until the budget is nearly spent, keeping the best move from the last
*completed* depth so it can be stopped at any moment and still answer. In a duel
at 500ms it reaches a mean depth of 8.

Battlesnake is a **simultaneous-move** game, so a node advances the board only
once every snake has committed. Modelling it as alternating turns makes the
opponent omniscient and the evaluation pathologically pessimistic, and a bot
built that way refuses moves that are perfectly safe.

The evaluation is Voronoi control, reachable space against our own length, tail
reachability, length advantage, hunger against distance to food, centre control
and cutting rivals off. Every term has a weight, and a weight of zero switches
its term off — which is what lets the harness vary exactly one thing.

**Safety is never delegated.** The one-ply fallback is computed and held before
the search starts, only a completed depth is ever promoted, and when every move
loses at the same distance the choice falls to how many rivals contest the
square.

Four rulesets, properly: `standard`, `royale`, `constrictor`, `wrapped`.
Anything else is played with standard logic **and says so in the log**.

## Status

Playing. `BENCHMARK.md` has what has actually been measured, including the
results that did not go the way we hoped.

## Running it

```sh
make tools    # pinned golangci-lint, gofumpt, and the official battlesnake CLI
make hooks    # install the pre-push gate (.git/hooks is not versioned)
make check    # gofmt + gofumpt, go vet, golangci-lint, go test -race, go build
make run      # serve on :8080
```

A local game against the real engine, once the server is up:

```sh
make e2e          # one game per supported ruleset
```

And the tournament harness, which runs games in process rather than over HTTP:

```sh
make phase-a      # the n=30 suite: shakes out bugs, publishes nothing
make phase-b      # the n=200 suite, which BENCHMARK.md is written from

go run ./cmd/typhon-bench -help-spec   # what an arm can vary
```

## Deploying

`render.yaml` is a Render Blueprint: **New → Blueprint**, pointed at this repo.

Two things about it are deliberate.

**The build command names the package path.** Render's default Go build command
does not, which fails on a `cmd/` layout — a build at the repo root finds no Go
files.

**The `free` plan spins down** after 15 minutes without traffic and takes about
a minute to wake. A Battlesnake that is asleep when a game starts returns
nothing, the engine moves it `up`, and it usually dies. That is fine for a demo
and wrong for a leaderboard. The two ways round it are to move to a paid
instance, or to keep the service warm with an external pinger — a cron that
hits `/` every few minutes. Neither is configured here.

**The region is a measurement, not a preference.** Frankfurt was measured at
44ms of engine-reported round trip. Every game logs `engine_rtt_mean_ms` taken
from `you.latency`; if that number climbs, move the service and compare rather
than reasoning about it.

## Layout

Per [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) — `cmd/` for
binaries, `internal/` for everything else, no `pkg/`.

```
cmd/typhon/           the server
cmd/typhon-bench/     the tournament harness
internal/api/         wire types
internal/board/       geometry, occupancy, flood fill, Voronoi
internal/rules/       the forward simulator
internal/eval/        the evaluation and its weights
internal/search/      iterative deepening, alpha-beta, transposition table
internal/server/      handlers and per-game state
```

The board, rules, search and eval packages are **pure functions over a state**:
no I/O, no clock beyond an injected budget, no logging. That is what makes them
table-testable against the official rules and benchmarkable against a deadline.

The simulator is not a wrapper around `BattlesnakeOfficial/rules` — that
allocates a fresh board per call, which is correct and far too slow for a
search. It is a reimplementation, and the official package is the **test
oracle**: random playouts are compared against it after every turn, for every
ruleset. Hand-written expectations would encode the same misreading twice.

## Measurement

`BENCHMARK.md` is the record. The discipline behind it, in short:

- **n=20 is worthless.** The predecessor ran the same configuration on two seed
  blocks and got 8-10-2 and 25-9-6. Nothing below 200 games is published here.
- **The floor is measured before anything is compared.** Two identical bots
  went 29-23-8 over there, so one starting slot was worth about six points
  before anyone changed anything. The arms alternate slots and per-slot rates
  are reported separately.
- **The comparison is paired.** Both arms play the same seeds and the test is
  McNemar's on the games where they disagreed, so the seed's own difficulty
  cancels instead of being averaged over.
- **A feature is verified to fire before it is benchmarked.** A whole batch was
  once measured against a feature whose threshold was never crossed, so the
  per-path counts print beside the win column.
- **There is a random control arm.** Without a floor, a component that
  contributes nothing and one that contributes a lot look alike.
- **Negative results are reported.** They were the most interesting findings
  last time.

## Documentation

| Where | What |
| --- | --- |
| [`BENCHMARK.md`](BENCHMARK.md) | Every number that was measured, including the ones that did not go the way we hoped |
| [`docs/findings/`](docs/findings/) | Things learned that would not have been guessed - eleven of them, including two bugs a green gate never saw |
| [`docs/adr/`](docs/adr/) | Decisions that were not forced, each with the alternative it rejected |
| [`docs/research/`](docs/research/) | What the rules engine actually does, where the 500ms goes, and what survives of the predecessor's conclusions |
| [`.review/`](.review/) | The filled pre-merge checklist |

If you read one, read
[findings/002](docs/findings/002-a-suggestive-result-that-evaporated.md): the
largest weight in the evaluation looked worth 63% at thirty games and 52.5% at
two hundred, with nothing changed but the sample size.

## Contributing

`docs/agents/rules.md` is the baseline — change discipline, the complexity
rules, Go conventions, testing, and the measurement discipline that this project
exists to get right.

`docs/agents/go-review-checklist.md` is the pre-merge pass. Copy it to
`.review/<branch>-checklist.md` and fill every row; the completeness guarantee
lives in the file, not in attention on the day.

`AGENTS.md` governs while a step in `.orchestrator/TODO_PLAN.md` is `ACTIVE`.

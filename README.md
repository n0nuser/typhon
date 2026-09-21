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
its term off — which is what lets the harness vary exactly one thing. The
weights themselves are guesses; see below.

**Only the nearest rivals are modelled properly.** Three opponents searched
exhaustively is 256 joint moves a ply, most of them spent on snakes that cannot
reach us inside the horizon, so the two nearest get a real search and the rest
get a cheap greedy move. With four snakes that beats modelling one — 123-71 at
n=200, p=0.0003 — and it wins while searching more than a ply shallower.

**Safety is never delegated.** The one-ply fallback is computed and held before
the search starts, only a completed depth is ever promoted, and when every move
loses at the same distance the choice falls to how many rivals contest the
square.

Four rulesets, properly: `standard`, `royale`, `constrictor`, `wrapped`.
Anything else is played with standard logic **and says so in the log**.

## What it does not do

Stated plainly, because a capability list that only lists capabilities is an
advertisement.

**Not deployed.** `render.yaml` is written and its build command is verified;
the Render dashboard step has not been done. Every number below was measured
locally.

**Not measured against its predecessor.** The headline nobody has run is Typhon
against `battlesnake-jev`. It needs the old binary answering over HTTP at about
half a second a turn — two hundred games is upwards of eight hours — and the
in-process harness cannot drive an external server, so it needs a driver that
does not exist.

**The weights are guesses.** `eval.Default()` encodes an ordering, not a
measurement. Two of the seven have been put to two hundred games each and
neither separated from its own absence: tail reachability, the largest weight in
the evaluation, at 105-95, and opponent confinement at 99-101. The other five
have never been tested. Nothing here is tuned; there was budget for structural
questions and none for searching weight space.

**No learning, no opening book, no endgame table.** The move loop is a search
and nothing else. No model inference, no training, no persistence between games
beyond one transposition table per game.

**Four rulesets, not all of them.** `standard`, `royale`, `constrictor` and
`wrapped` are implemented against the real semantics. Anything else — solo,
hazard-pits maps, custom variants — is played with standard logic and says so in
the log rather than failing quietly. Board maps other than the ruleset's own
default are not modelled.

**Constrictor is unresolved.** Search beats one ply decisively in the other
three rulesets and does not clearly do so in constrictor (18-11-1, p=0.27, and
n=30 at that). There may simply be less for lookahead to find when the board
fills regardless.

**The deadline is proven on one laptop, not in production.** 2,018 turns
against the real engine with zero late turns, under synthetic load — but on
different silicon, on a shared CPU, on a plan that sleeps, that is a different
claim. The first turn after a cold start is the one to watch.

**No multi-snake tuning beyond one flag.** `Opponents: 2` is now measured as
better than 1 with four snakes. Whether 3 would be better is untested, and the
cost rises from 64 joint moves a ply to 256.

## Configuration

Everything is an environment variable with a working default, so the deployed
service needs none of them set.

| Variable | Default | What it is |
| --- | --- | --- |
| `PORT` | `8080` | listen port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `TYPHON_AUTHOR` | `n0nuser` | shown on the snake's profile |
| `TYPHON_COLOR` | `#8A0303` | deep volcanic red |
| `TYPHON_HEAD` | `lantern-fish` | head sprite |
| `TYPHON_TAIL` | `cosmic-horror` | tail sprite |
| `TYPHON_VERSION` | `0.1.0` | reported in `/` |
| `TYPHON_OPPONENTS` | `2` | rivals modelled properly; the rest get a greedy move |
| `TYPHON_TABLE_BITS` | `20` | transposition table sized `1<<N` entries |
| `TYPHON_NO_TABLE` | unset | set to disable the transposition table |

The appearance is themed rather than arbitrary: Typhon is the serpent-headed
monster buried under Etna, so volcanic red, with a lantern-fish head and a
cosmic-horror tail for the thing that lives down there. Head and tail names come
from the Battlesnake customization list.

## Status

Built, measured, gated and pushed — **not deployed**. `BENCHMARK.md` has what
has actually been measured, including the results that did not go the way we
hoped.

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
nothing, the engine moves it `up`, and it usually dies.

The fix is narrower than it first looks, because **leaderboard games are not
continuous**. Battlesnake runs them once a day: *"Once a day, at the same time
every day, each Leaderboard will initiate a series of competitive games among
Leaderboard participants."* So the service does not need to be awake all day —
it needs to be awake for one window.

That matters because Render grants **750 Free instance hours per workspace per
month**, and a spun-down service consumes none of them. Staying awake 24/7 costs
720 hours in a 30-day month and 744 in a 31-day one: it fits, with nothing left
for any other free service, and exhausting the budget suspends *every* Free web
service in the workspace until the next month. Warming a two-hour window instead
costs about 60 hours a month.

So: an external cron hitting `GET /` every 10 minutes, for a window around the
daily batch. [cron-job.org](https://cron-job.org) is free and does this. A
self-ping from inside the service does not work — it keeps the service from
sleeping but cannot wake it, so a single redeploy or hiccup takes it down until
something external arrives.

The batch time is not documented and has to be observed: register, join a
leaderboard, let a day pass, and read the timestamps in the game history.

None of this is configured in the repo, because the window is account-specific.

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
- **The floor is measured before anything is compared.** The predecessor read
  29-23-8 as "one starting slot is worth about six points". Re-analysed, that
  interval contains 50%; measured properly here it is 49.3% over a thousand
  duel games, and 25% per square over two hundred four-snake ones. The arms
  rotate through the slots and per-slot rates are reported separately.
- **A comparison that cannot vary its own flag is refused.** An arm ran for a
  whole build reporting 14-16 and p=0.855 about a setting it never varied. The
  harness now errors rather than printing a split.
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
| [`docs/findings/`](docs/findings/) | Things learned that would not have been guessed - thirteen of them, including two bugs a green gate never saw and a benchmark arm that compared a setting with itself |
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

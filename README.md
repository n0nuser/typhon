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

## Status

Quality gates and the service skeleton. The engine is being built on top.

## Running it

```sh
make tools    # pinned golangci-lint, gofumpt, and the official battlesnake CLI
make hooks    # install the pre-push gate (.git/hooks is not versioned)
make check    # gofmt + gofumpt, go vet, golangci-lint, go test -race, go build
make run      # serve on :8080
```

A local game against the real engine, once the server is up:

```sh
make e2e
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

The board, rules, search and eval packages are **pure functions over a state**:
no I/O, no clock beyond an injected budget, no logging. That is what makes them
table-testable against the official rules and benchmarkable against a deadline.

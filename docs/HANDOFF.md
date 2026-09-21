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
| Live play | 2,018 turns against the real engine over HTTP, four rulesets, **zero late turns** |
| Benchmarks | Seven n=200 paired runs, two of them four-snake, plus a full n=30 sweep; see `BENCHMARK.md` |
| Deployed | **No.** `render.yaml` is written and the build command is verified; the Render dashboard step has not been done. |

## What is established

- **Search pays: 191-9** at n=200 paired, p<0.0001, CI [91.7%, 97.6%], against a
  measured floor of 51%. Standard ruleset, 11x11.
- **It is not a coin: 200-0**, n=200.
- **There is no start-position bias**: 49.3% [46.2%, 52.4%] over 1,000 games —
  which contradicts the predecessor's stated explanation for its own noise.
- **Opponent confinement neither helps nor hurts**: 99-101 at n=200, p=0.94.
  The n=30 sweep's 19-11 against the term was noise; weight 6 is not paying to
  lose, and it is also not paying for anything.
- **Modelling two rivals beats modelling one, with four snakes**: 123-71 at
  n=200, p=0.0003, [56.4%, 69.9%]. It wins while searching a ply shallower
  (4.20 against 5.28), reaching hopeless positions less than half as often per
  turn. The first structural arm in the project that separates.
- **No four-snake start-square bias**: all four squares inside their 25% null
  over 200 games. This is a separate measurement from the duel one above; four
  squares is not two.
- **The deadline holds on this hardware under synthetic load**: 2,018 turns at
  load average 13 on eight cores, zero late. See the scope note below.

Royale (29-1) and wrapped (28-2) point the same way but are **n=30**, and this
project's own rule is that nothing below 200 games is a finding. They belong in
the section below, not this one.

## What is believed but not measured

Read these before changing anything, because each is a place where the code
embodies a guess:

1. **Every weight in `eval.Default()`.** They encode an ordering, not a
   measurement. Two have now been put to n=200 and neither separated from its
   own absence: tail reachability, the largest of them, at 105-95 (p=0.52), and
   opponent confinement at 99-101 (p=0.94). Both are kept because "not shown to
   help" is not "shown to hurt". The other five have not been tested at all.
2. **`Opponents: 3`.** `Opponents: 2` is now measured — 123-71 at n=200 with
   four snakes, p=0.0003. Whether modelling the third rival also pays is not,
   and the cost rises from 64 joint moves a ply to 256.
3. **Constrictor.** The one ruleset where capping at one ply did not clearly
   lose (18-11-1, p=0.27).
4. **That search pays in royale and wrapped.** 29-1 and 28-2 are large margins
   and they agree with the standard result, which is reassuring and is not
   evidence. n=30.
5. **That the deadline holds in production.** It holds here: Typhon against
   Typhon, one machine, contention supplied by a benchmark suite. Render is
   different silicon, a shared and unpredictable CPU, and a free plan that
   sleeps. The mechanism is sound - the clock is read every node and only a
   completed depth is promoted - but "measured on this laptop" and "measured in
   production" are different claims. The first turn after a cold start is the
   one to watch.

## What to do first

In order of value:

1. **Deploy it.** `render.yaml` → New → Blueprint. The region is Frankfurt on a
   measured 44ms round trip; verify against `engine_rtt` in the logs rather than
   trusting it. Note the free plan sleeps after 15 minutes and a sleeping snake
   dies.
2. **Typhon vs `battlesnake-jev`, n=200.** The headline nobody has run. It needs
   the old binary over HTTP at ~0.5s a turn — 8+ hours — and the in-process
   harness cannot drive an external server, so it needs a separate driver.

And one standing obligation rather than a task:

**Any change to `internal/search` or `internal/eval` invalidates every number in
`BENCHMARK.md`.** Re-run `make phase-b` in the same change, or delete the entry.
This is `docs/agents/rules.md`'s rule about regenerated artifacts, and it is the
one most likely to be skipped, because the numbers are already there and look
authoritative. It has already been violated three times in this repository — the
calibration table went stale twice under load and once against an older search,
and each time the derived claims in the prose went stale with it.

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
- `docs/findings/` — thirteen things that would not have been guessed
- `docs/adr/` — eleven decisions, each with the alternative it rejected
- `docs/research/` — the rules engine's real semantics; the turn budget; a
  re-analysis of the predecessor's conclusions
- `.review/main-checklist.md` — the pre-merge pass, 58 rows, four of which failed

## The one thing worth internalising

`docs/findings/001`, `002` and `012`, together. A differential test passed while
exercising almost none of the rules; an evaluation term looked worth 63% at
thirty games and 52.5% at two hundred; and a benchmark arm ran to completion,
printed a p-value, and had compared a configuration with itself. All three were
caught by measuring the *measurement* rather than trusting it, and the third was
caught long after it had been cited twice as evidence.

The gate was green through all of it, and through two search bugs that silently
preferred a worse move. A green gate is evidence that nothing crashed. It is not
evidence that anything works.

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
| Benchmarks | Four n=200 paired runs plus a full n=30 sweep; see `BENCHMARK.md` |
| Deployed | **No.** `render.yaml` is written and the build command is verified; the Render dashboard step has not been done. |

## What is established

- **Search pays: 191-9** at n=200 paired, p<0.0001, CI [91.7%, 97.6%], against a
  measured floor of 51%. Reproduced in royale (29-1) and wrapped (28-2).
- **It is not a coin: 200-0.**
- **The deadline holds**, including on a machine at load average 13.
- **There is no start-position bias**: 48.8% [45.3%, 52.2%] over 800 games —
  which contradicts the predecessor's stated explanation for its own noise.

## What is believed but not measured

Read these before changing anything, because each is a place where the code
embodies a guess:

1. **Every weight in `eval.Default()`.** They encode an ordering, not a
   measurement. Tail reachability — the largest of them — was specifically
   tested at n=200 and **did not** demonstrate value (105-95, p=0.52). It is
   kept because "not shown to help" is not "shown to hurt".
2. **`Opponents: 2`.** The arm that would settle it was run in a duel, where the
   flag barely does anything. The calibration says 400ms buys depth 7 against one
   rival and depth 3 against three, so the four-snake case is where it matters.
3. **Opponent confinement may be harmful.** Switching it off won 19-11 at n=30.
   Not significant, but the sign is wrong and weight 6 may be paying to lose.
4. **Constrictor.** The one ruleset where capping at one ply did not clearly
   lose (18-11-1, p=0.27).

## What to do first

In order of value:

1. **Deploy it.** `render.yaml` → New → Blueprint. The region is Frankfurt on a
   measured 44ms round trip; verify against `engine_rtt` in the logs rather than
   trusting it. Note the free plan sleeps after 15 minutes and a sleeping snake
   dies.
2. **Four-snake `opponents=1` vs `2`, n=200.** The single most valuable
   outstanding measurement; it decides a default that is currently a judgement.
3. **`confine` at n=200.** Cheap, and it may remove a term that costs games.
4. **Typhon vs `battlesnake-jev`, n=200.** The headline nobody has run. It needs
   the old binary over HTTP at ~0.5s a turn — 8+ hours — and the in-process
   harness cannot drive an external server, so it needs a separate driver.

## How to run things

```sh
make check                    # the gate
make phase-a                  # n=30 sweep, diagnostic only
make phase-b                  # n=200, the three arms that answer a question
make e2e                      # one live game per ruleset (needs ./typhon running)
go run ./cmd/typhon-bench -calibrate    # what a wall-clock budget buys here
go run ./cmd/typhon-bench -help-spec    # what an arm can vary
```

## Traps, all of them learned the hard way

- **`pkill -f` self-matches** and will kill your own shell. Kill by PID. This is
  documented in `AGENTS.md` and was then done anyway.
- **Do not edit a shell script while it is running.** bash re-reads from a byte
  offset; a Phase A died three runs in with an unbound variable that did not
  exist when the run started. Freeze a copy — `ROOT` is overridable for this.
- **A driver with `set -u` but not `set -e`** turns a suite that stopped early
  into one that reports success.
- **Node budgets make results immune to CPU contention**; wall-clock budgets do
  not. Never benchmark on a clock.
- **`you.latency` includes your own think time.** See
  `docs/findings/007-latency-is-not-round-trip.md`.

## Where to read

- `BENCHMARK.md` — every measured number, with the negative results kept
- `docs/findings/` — eleven things that would not have been guessed
- `docs/adr/` — ten decisions, each with the alternative it rejected
- `docs/research/` — the rules engine's real semantics; the turn budget; a
  re-analysis of the predecessor's conclusions
- `.review/main-checklist.md` — the pre-merge pass, 58 rows, four of which failed

## The one thing worth internalising

`docs/findings/002` and `docs/findings/001`, together. A differential test passed
while exercising almost none of the rules, and an evaluation term looked worth
63% at thirty games and 52.5% at two hundred. Both were caught by measuring the
*measurement* rather than trusting it.

The gate was green through all of it, and through two search bugs that silently
preferred a worse move. A green gate is evidence that nothing crashed. It is not
evidence that anything works.

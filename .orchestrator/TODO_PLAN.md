# Orchestrator plan

Shared state for the loop defined in `AGENTS.md` → *Orchestrator runtime*.
Written by the LLM only. Committed — this is the reviewable artifact.

**This file is the state.** If the LLM's belief and this file disagree, the file
wins.

**The loop is dormant** (see `AGENTS.md` → *Status*), so no step is marked
`ACTIVE` and the work is being done directly. The steps, their scope and their
acceptance checks are unchanged — this is still the plan of record, and a step
still only reaches `DONE` when its acceptance check has been run and the
decisive line quoted.

## Status legend

| Status | Meaning |
| --- | --- |
| `ACTIVE` | The step being executed right now. **Exactly one, ever.** While one exists, the LLM must not touch application source. |
| `TODO` | Queued, not started. |
| `DONE` | Diff reviewed, acceptance check passed, decisive log line quoted. |
| `BLOCKED` | Correction budget (3 attempts) exhausted. Escalated to the user. |

## Goal

Build Typhon: a deterministic Battlesnake whose move comes from an
iterative-deepening search over a forward simulator of the real rules, spending
the ~400ms per turn that the predecessor (`n0nuser/battlesnake-jev`, one ply,
0.14ms per decision) leaves unused.

Two deliverables, weighted equally:

1. **The engine.** Simultaneous-move paranoid alpha-beta with a transposition
   table, opponent reduction and a Voronoi-based evaluation, under a budget
   derived from `game.timeout` on every request. Four rulesets: `standard`,
   `royale`, `constrictor`, `wrapped`; anything else plays standard **and says
   so in the log**.
2. **A harness that can actually settle a question.** In-process, node-budgeted
   so games are bit-for-bit reproducible and parallelisable, paired across a
   shared seed set, reporting McNemar and a Wilson interval. The previous
   project's conclusions came from n=20 and were noise; nothing here is
   published below n=200.

Branch: `main`, continuing from `4eb99f6` (the quality gates).

### Established before planning, do not re-derive

Read from `rules@v1.2.3` in the module cache, not from the prose docs:

- Stage order is `GameOver → Movement → Starvation → HazardDamage → Feed →
  Elimination`. Movement **always** pops the tail; `Feed` re-duplicates it. That
  single ordering is the whole mechanism behind "the tail frees up except when
  it just ate" and behind duplicate coordinates in `body`.
- `DamageHazardsStandard` skips hazard damage **entirely** when the hazard
  square also holds food.
- `ParamHazardDamagePerTurn` is the string `"damagePerTurn"`. The wire JSON
  field is `hazardDamagePerTurn`. They are not the same name, and passing the
  wrong one leaves royale silently at its default.
- That default is **0** in the library; the CLI passes 14. A missing field is
  not "14 damage".
- Food spawning and royale hazard expansion are seeded-random, so the search
  does not model them. Both sets are held static across the horizon.

## Steps

### Step 1 — `internal/api` and `internal/board` — `DONE`

Wire types, and the bitset/topology/flood-fill/Voronoi primitives.

- **Scope:** `internal/api/`, `internal/board/`.
- **Acceptance:** `go test ./internal/api/... ./internal/board/... -race -count=1`
- **Note:** every neighbour operation goes through one `Topology` type. A
  `c.X + 1` written anywhere else is how `wrapped` ends up subtly wrong while
  the standard tests stay green.
- **Result:** `ok github.com/n0nuser/typhon/internal/board 1.036s coverage:
  96.6% of statements`, `golangci-lint: 0 issues`, and the benchmarks that the
  deadline depends on:

  ```
  BenchmarkReach-8      1306947    918.0 ns/op    0 B/op    0 allocs/op
  BenchmarkVoronoi-8     192918   6231   ns/op    0 B/op    0 allocs/op
  ```

  Both allocation-free, which is the bar §13 of the review checklist sets.
  Voronoi at 6.2µs is the number to watch: it puts a hard ceiling near 64k
  evaluations inside a 400ms budget, so if the search turns out to be
  evaluation-bound, this is where a profile should look first. Not optimised
  now — `rules.md` §2 says a profile comes before the optimisation.

### Step 2 — `internal/rules` and the differential tests — `DONE`

The forward simulator, make/unmake, and the differential test against the
official rules module in two modes (strict with food spawning off; re-sync with
it on).

- **Scope:** `internal/rules/`, `go.mod`.
- **Acceptance:** `go test ./internal/rules/... -race -count=1`
- **Result:** `ok github.com/n0nuser/typhon/internal/rules 3.369s coverage:
  94.8% of statements`, and `BenchmarkApplyUnapply-8 5652693 210.5 ns/op
  0 B/op 0 allocs/op` - about 4.7M simulator steps a second.

  Against Voronoi's 6.2µs that settles where the search's time will go: one
  evaluation costs thirty nodes. If the search turns out evaluation-bound, the
  structural fixes are to evaluate only at leaves that survive the cutoff, or
  to cache the partition in the transposition table - both cheaper than
  micro-optimising the bitset loop, and neither worth doing before a profile
  says so.
- **The finding worth keeping:** the differential test passed on its first run
  while testing almost nothing. Uniformly random moves gave 3.5 turns a game,
  two growths across sixty games, and **not one head-to-head** - the rule the
  opening of every game turns on. Picking at random among the moves that are
  not immediately fatal took standard from 213 turns to 3,395 and made every
  elimination cause fire. `TestPlayoutsExerciseEveryRule` now asserts that
  reach, so a future change to the move policy fails loudly instead of
  quietly making the differential test vacuous.

### Step 3 — `internal/eval` — `DONE`

Weight struct with one flag per term; terminal scoring; Voronoi control, space
vs length, tail reachability, length advantage, health vs food, centre control,
opponent confinement.

- **Scope:** `internal/eval/`.
- **Acceptance:** `go test ./internal/eval/... -race -count=1`
- **Result:** `ok github.com/n0nuser/typhon/internal/eval 1.013s coverage: 94.4%
  of statements`, `BenchmarkEvaluate-8 147020 8097 ns/op 0 B/op 0 allocs/op`.

  8.1µs is the number that sets the search's shape: about 49,000 evaluations
  fit in a 400ms budget, so the search must evaluate at leaves and prune well
  rather than score everything it touches. In a duel that is roughly depth 6
  to 8, against the predecessor's one.
- **Two bugs the tests caught, both silent:** the reachable-space term was
  always zero, because a flood fill refuses to start on a blocked square and a
  snake's own head is occupied - it still produced a number, always the same
  wrong one. And `State` owns mutable scratch, so evaluating one from two
  goroutines is a race; the contract is now written on the type and the
  harness gives every game its own.

### Step 4 — `internal/search` — `DONE`

Iterative deepening, simultaneous-move paranoid alpha-beta, TT, move ordering,
opponent reduction, and the dual time/node budget.

- **Scope:** `internal/search/`.
- **Acceptance:** `go test ./internal/search/... -race -count=1` and
  `go test -run '^$' -bench . -benchmem ./internal/search` reporting nodes/sec
  and `0 allocs/op` on the hot path.
- **Result:** `ok github.com/n0nuser/typhon/internal/search 10.014s coverage:
  96.8% of statements`, and

  ```
  BenchmarkSearch-8       18   60299485 ns/op   18 B/op   0 allocs/op
  BenchmarkSearchNode-8   12  101157366 ns/op   5050 ns/node   0 allocs/op
  ```

  **Depth 7 in a duel at 40,000 nodes**, against the predecessor's one ply.
  At 5µs a node a 400ms budget is worth about 80,000 of them.
- **Two things that had to be measured rather than assumed:**
  - The deadline was being missed by thirty-fold - a 2ms budget took 67ms -
    because the clock was read every 1024 nodes on the usual chess-engine
    reasoning that `time.Now` is too expensive per node. That reasoning does
    not transfer: a node here rebuilds an occupancy map and hashes a position,
    so it costs a microsecond and up, not tens of nanoseconds. At an interval
    of 64 the clock is under a percent of a node.
  - Move ordering returned `out[:n]` from a local array, which escapes. Once
    per actor per node, that was **11,404 allocations a turn** - a GC pause
    waiting to land inside the budget. Filling a caller-owned array took the
    whole search to zero.

### Step 5 — `internal/server` and `cmd/typhon` — `DONE`

The four webhooks, per-(game id + snake id) state with TTL eviction, the
overhead-corrected budget, and the held safe fallback.

- **Scope:** `internal/server/`, `cmd/typhon/`.
- **Acceptance:** `go test ./internal/server/... ./cmd/... -race -count=1`
- **Result:** both green, and a real 270-turn game against the official
  `battlesnake` CLI, Typhon against Typhon, 11x11 standard, `-t 500`:

  ```
  turns=270 alive=true  mean_depth=8 max_depth=12 nodes=27288070
    fallbacks=1 max_think=425.675392ms timeout_overruns=0
  turns=270 alive=false mean_depth=7 max_depth=11 nodes=26762113
    fallbacks=0 max_think=425.945523ms timeout_overruns=0
  ```

  **Mean depth 8 against the predecessor's one ply, and no turn late in 540.**
  The predecessor's own log records first deaths at turn 8 to 12; this game
  ran to 270.

  `engine_overhead=0s` is the budget model working rather than failing. Played
  locally there is no network to pay for, so the engine's reported latency is
  almost entirely our own think time, and the overhead it leaves is nothing -
  which is correct, and leaves the whole timeout available. Subtracting the
  raw reported latency instead would have computed a 75ms budget from a 500ms
  turn and collapsed the search to one ply, turn after turn.

### Step 6 — `cmd/typhon-bench` — `DONE`

In-process harness over the official rules loop, alternating slots, paired
McNemar plus Wilson, per-path counters, random control arm.

- **Scope:** `cmd/typhon-bench/`.
- **Acceptance:** the same seed run twice produces an identical move log.
- **Result:** `ok github.com/n0nuser/typhon/cmd/typhon-bench 36.899s coverage:
  49.2% of statements`. Reproducibility is asserted as a property rather than
  against a stored transcript: `TestAGameIsReproducibleFromItsSeed` replays
  each seed and compares outcome, turn count and every per-arm counter, and
  `TestParallelismDoesNotChangeTheResults` runs the same four games one at a
  time and four at a time and requires them identical.

  A first smoke run, full search against one ply at 3,000 nodes:

  ```
  RESULT smoke: full=4 oneply=0 draw=0 of 4 games
  PAIRED McNemar on 4 discordant games: chi2=2.25 p=0.1336 -> not separated
  PATHS  full    mean_depth=4.80 max_depth=10 nodes=2228353 all_losing=0
  PATHS  oneply  mean_depth=1.00 max_depth=1  nodes=7244    all_losing=4
  ```

  4-0 and the test still says *not separated*, which is the discipline
  working. The path counts are the other half of it: they show the two arms
  really are searching to different depths, so whatever the win column later
  says, it is not measuring an inert flag.

### Also done — end to end against the real engine

One game per supported ruleset, Typhon against Typhon, through the official
`battlesnake` CLI over HTTP at `-t 500`:

| Ruleset | Turns |
| --- | --- |
| standard | 343 |
| royale | 97 |
| constrictor | 49 |
| wrapped | 505 |

The predecessor silently played `wrapped` with standard logic, treating the
edge of the board as fatal. This one plays 505 turns of it.

### Step 7 — Phase A, n=30 — `DONE`

Throwaway runs to shake out bugs and confirm every decision path fires at least
once. Not for publication.

- **Acceptance:** every counter non-zero; one clean run per ruleset.
- **Result:** the three headline arms are in and written up in `BENCHMARK.md`.
  The structural and per-ruleset arms were skipped by a bug in the suite script
  and are re-running.

  Phase A did its job twice over. It confirmed the arms differ - one ply
  averages exactly 1.00 ply against the full search's 5.08, so nothing here is
  measuring an inert flag - and it produced the mechanism: the one-ply bot
  does not die sooner, it dies more often, having reached positions where every
  move loses **31 times against the searching bot's once**.

  It also caught two bugs in its own scaffolding, which is what a smoke phase
  is for: a path counter that read backwards, and a suite script that stopped
  three runs in and reported success, because the driver had `set -u` but not
  `set -e`. A run that stops early and says it finished is worse than one that
  crashes.

### Step 8 — Phase B, n=200, and `BENCHMARK.md` — `DONE`

The floor, the random control, depth-1 vs full, and the structural arms.

- **Acceptance:** McNemar and Wilson printed; the floor measured, not assumed.
- **Result:** three arms at n=200, written up in `BENCHMARK.md`.

  | Run | Result | Paired p |
  | --- | --- | --- |
  | floor, identical configs | 98-102 | 0.832, not separated |
  | random control | search 200, coin 0 | <0.0001 |
  | **search pays** | **full 191, one ply 9** | **<0.0001** |

  95.5% of decisive games, CI [91.7%, 97.6%], against a measured floor of
  51.0% [44.1%, 57.8%].

  **The negative result is the floor.** There is no detectable start-position
  advantage: 51.0% [44.1%, 57.8%], and the random control agrees independently
  at exactly 100-100. The predecessor named position bias as the likely
  explanation for its noise and recorded the floor as "~56%"; re-analysed, its
  29-23 of 52 decisive games is 55.8% with an interval of [42.3%, 68.4%], which
  contains 50%. The floor it cited was itself never established.

  **And the mechanism is not the obvious one.** The one-ply arm does not die
  much sooner - turn 186 against 231. It reaches positions where every legal
  move is contested **213 times against the searching arm's 7**, over the same
  number of turns. Depth does not help a snake survive a lost position; it
  stops it walking into one.

  The structural arms all came back **not separated** at n=30, which is the
  only honest reading thirty games allows. Two are worth following up:
  opponent confinement points the *wrong* way (19-11 for switching it off),
  and tail reachability points the right way at exactly the same p while
  carrying the largest weight in the evaluation. The latter is being re-run at
  n=200; the former is recorded as an open question rather than acted on.

  Per ruleset, search beats one ply in royale (29-1) and wrapped (28-2) at the
  same magnitude as standard. **Constrictor is the exception** - 18-11-1,
  p=0.27, not separated - which also corrected a claim in `BENCHMARK.md` that
  had reasoned from "reaches depth 20" to "lookahead matters most there".

### Step 9 — the review checklist — `DONE`

Walk `docs/agents/go-review-checklist.md` into `.review/`, every row filled.

- **Acceptance:** zero blank verdict cells; findings derived from the ledger.
- **Result:** `.review/main-checklist.md`, 58 rows, zero blank. **Four FAIL
  rows, three of them CRITICAL**, all fixed with tests:

  - A completed search was overruled by the one-ply safety check whenever every
    neighbour was contested - throwing away the search's knowledge that one
    loss arrives five turns later than another.
  - The tie-break itself preferred certain death: ranking by contester count
    puts a self-collision first, because nobody competes for our own neck.
  - The deadline was missed under load - a 2ms budget running to 86ms - at a
    64-node clock interval. A shared CPU is the realistic deployment.
  - A path counter read backwards, printing a mean death turn of 11 for an arm
    that died twice in thirty games at turn 165.

  **`make check` was green throughout the period every one of those was
  present.** Two were silent preferences for a worse move, with nothing in any
  log to point at. That is the whole argument for §0's rule that the loop
  suspends this review rather than replacing it.
- **Note:** this is the step §0 exists for. The loop suspended the checklist, so
  by construction it has never run against the branch the loop just built, and
  the green gate that ends the loop is not a substitute for it.

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

### Step 2 — `internal/rules` and the differential tests — `TODO`

The forward simulator, make/unmake, and the differential test against the
official rules module in two modes (strict with food spawning off; re-sync with
it on).

- **Scope:** `internal/rules/`, `go.mod`.
- **Acceptance:** `go test ./internal/rules/... -race -count=1`

### Step 3 — `internal/eval` — `TODO`

Weight struct with one flag per term; terminal scoring; Voronoi control, space
vs length, tail reachability, length advantage, health vs food, centre control,
opponent confinement.

- **Scope:** `internal/eval/`.
- **Acceptance:** `go test ./internal/eval/... -race -count=1`

### Step 4 — `internal/search` — `TODO`

Iterative deepening, simultaneous-move paranoid alpha-beta, TT, move ordering,
opponent reduction, and the dual time/node budget.

- **Scope:** `internal/search/`.
- **Acceptance:** `go test ./internal/search/... -race -count=1` and
  `go test -run '^$' -bench . -benchmem ./internal/search` reporting nodes/sec
  and `0 allocs/op` on the hot path.

### Step 5 — `internal/server` and `cmd/typhon` — `TODO`

The four webhooks, per-(game id + snake id) state with TTL eviction, the
overhead-corrected budget, and the held safe fallback.

- **Scope:** `internal/server/`, `cmd/typhon/`.
- **Acceptance:** `go test ./internal/server/... ./cmd/... -race -count=1`

### Step 6 — `cmd/typhon-bench` — `TODO`

In-process harness over the official rules loop, alternating slots, paired
McNemar plus Wilson, per-path counters, random control arm.

- **Scope:** `cmd/typhon-bench/`.
- **Acceptance:** the same seed run twice produces an identical move log.

### Step 7 — Phase A, n=30 — `TODO`

Throwaway runs to shake out bugs and confirm every decision path fires at least
once. Not for publication.

- **Acceptance:** every counter non-zero; one clean run per ruleset.

### Step 8 — Phase B, n=200, and `BENCHMARK.md` — `TODO`

The floor, the random control, depth-1 vs full, and the structural arms.

- **Acceptance:** McNemar and Wilson printed; the floor measured, not assumed.

### Step 9 — the review checklist — `TODO`

Walk `docs/agents/go-review-checklist.md` into `.review/`, every row filled.

- **Acceptance:** zero blank verdict cells; findings derived from the ledger.
- **Note:** this is the step §0 exists for. The loop suspended the checklist, so
  by construction it has never run against the branch the loop just built, and
  the green gate that ends the loop is not a substitute for it.

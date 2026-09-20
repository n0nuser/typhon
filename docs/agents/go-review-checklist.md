# Go review checklist

Ported from `deadchannel`'s Python review checklist. The rows that had no Go
analogue were **dropped rather than translated into filler** — carrying dead
rows across is what trains a reviewer to skim. Three rows are new, because this
project has failure modes that one did not.

This must be walked before merging any new Go code. It complements
`docs/agents/rules.md` rather than replacing it: rules.md covers this repo's
baseline conventions, this file is the deeper, itemized pass.

## How to use this file (read before reviewing)

1. **Copy, don't regenerate.** Copy this file to a working path for the diff
   under review — `.review/<branch-or-pr>-checklist.md`. Never rebuild the list
   from memory: the completeness guarantee lives in *this file*, not in
   attention on any given run.
2. **Fill every row.** For each rule set `Verdict` to `PASS`, `FAIL`, or `N/A`:
   - `PASS` — the rule applies to this diff and the code satisfies it. Cite the
     strongest `file:line` you checked in `Evidence`.
   - `FAIL` — the rule applies and the code violates it. Cite `file:line` and
     state the fix in `Note`.
   - `N/A` — the rule cannot apply to this diff (no code of that kind changed)
     **or** it is a `[LINT]` rule already covered by a green `golangci-lint`.
     Say which in `Note`. `N/A` is a conscious decision, never a silent skip.
3. **Completion gate — do not produce findings until zero `Verdict` cells are
   blank.** A blank cell means the rule was not considered.
4. **Derive the findings from the ledger, not from memory.** The review summary
   is built by filtering the `FAIL` rows into `[CRITICAL]` then `[STANDARD]`. If
   a rule is not in the ledger as `FAIL`, it is not a finding.

`[LINT]` rows: if `make lint` is green and covers the rule, mark `N/A` with note
`lint-gate-green`. Only flag a `[LINT]` rule `FAIL` when the gate is missing that
rule.

IDs are stable (`R<section>.<n>`). If a rule here changes, keep the ID.

## §1 Package structure & architecture [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R1.1 | STANDARD | Layout per go.dev/doc/modules/layout — `cmd/<binary>/main.go` plus `internal/`; no `pkg/` | | | |
| R1.2 | STANDARD | `cmd/*/main.go` stays thin — environment, logger, wiring, shutdown only; no domain logic | | | |
| R1.3 | STANDARD | A package is named after what it owns; no `util`/`helpers`/`common`/`misc` | | | |
| R1.4 | STANDARD | No stuttering — `search.State`, never `search.SearchState`; no repeated package name in exported identifiers | | | |
| R1.5 | **CRITICAL** | Dependencies point inward: `board`, `rules`, `search`, `eval` do not import `api`, `server`, `net/http`, `log/slog` or anything with I/O in it | | | |
| R1.6 | **CRITICAL** | `board`/`rules`/`search`/`eval` are pure over a state — no globals, no clock beyond an injected budget, no logging. This is what makes them table-testable and benchmarkable | | | |
| R1.7 | STANDARD | SOLID; composition over embedding; single responsibility per unit | | | |
| R1.8 | STANDARD | YAGNI bounds abstraction — no generalization until reused/non-trivial; trivial single-use logic stays inline | | | |
| R1.9 | STANDARD | Behaviour lives with the state it owns; a method moves to a higher layer only when it spans several types or needs an injected collaborator | | | |
| R1.10 | STANDARD | No pass-through wrapper — a method whose whole body forwards to a collaborator's identically-named method is deleted and the caller calls the owner; validating/adapting/narrowing wrappers are fine | | | |

## §2 Typing [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R2.1 | **CRITICAL** | No `any` plus a type assertion at an internal boundary. `any` appears at the JSON boundary and nowhere else | | | |
| R2.2 | STANDARD | A struct whenever the key set is known and fixed; `map[string]X` only for open/dynamic keys with uniform `X` | | | |
| R2.3 | STANDARD | No stringly-typed returns — `Direction`, `Variant`, `H2HRisk` are named types with a `String()`, not bare strings the caller must compare | | | |
| R2.4 | STANDARD | Named types over bare `int` where the value carries a meaning (square index, score, ply); a `bool` parameter at a call site is usually a named type waiting to happen | | | |
| R2.5 | STANDARD | Environment configuration is read in one place and passed as a struct, not re-read with `os.Getenv` scattered through the code | | | |

## §4 State & construction [mixed]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R4.1 | **CRITICAL** | **No package-level mutable state.** This is the rule that protects reproducibility: a transposition table shared across parallel harness games makes results order-dependent while every unit test still passes. One TT per decider, one decider per game | | | |
| R4.1a | STANDARD | A frozen, computed-once package constant (a Zobrist key table, a direction table) may be package-level — confirm it is never written after `init` and that nothing derives per-run state from it | | | |
| R4.2 | **CRITICAL** | No logic in a constructor — field assignment and cheap validation only. I/O, parsing or heavy compute moves to an explicit `Load`/`Build` function that can return an error | | | |
| R4.3 | STANDARD | Inject boundary collaborators (HTTP client, clock, store) as constructor params; instantiate at point of use otherwise | | | |

## §5 Naming & interfaces [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R5.1 | **CRITICAL** | Auto-explicative code — names carry meaning; a reviewer understands intent without comments; prefer renaming over commenting | | | |
| R5.2 | STANDARD | Everything the code says is in English — identifiers, comments, log messages, shouts | | | |
| R5.3 | STANDARD | No initiative codenames, issue numbers, sprint or phase labels in identifiers, comments or test names; traceability is the commit message | | | |
| R5.4 | STANDARD | No term borrowed from another trade's meaning — every identifier checked against its established industry sense | | | |
| R5.5 | STANDARD | **Accept interfaces, return structs**, and declare the interface at the *consumer*. An interface earns its place as a real substitution seam (2+ implementations, or a boundary a test must replace), not as decoration beside the implementation | | | |
| R5.6 | STANDARD | Access data by field name, not by index, when both are available | | | |
| R5.7 | STANDARD | No `reflect` outside encoding boundaries | | | |

## §7 Errors [mixed]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R7.1 | **CRITICAL** | Errors are never swallowed; propagate with `fmt.Errorf("...: %w", err)` | | | |
| R7.2 | STANDARD | Sentinel and typed errors declared in one `errors.go` per package, never inline at the raise site | | | |
| R7.3 | **CRITICAL** | No `panic` on the move path, and no `log.Fatal` outside `main`. Nothing may be the reason a move is late or absent | | | |
| R7.4 | LINT | Every returned error is checked (`errcheck`); every opened body closed (`bodyclose`) | | | |

## §9 Comments & docs [mixed]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R9.1 | STANDARD | No forced doc comments on auto-explicative code; write one only when it adds what code cannot say. When present it begins with the identifier name | | | |
| R9.2 | STANDARD | Comments explain the *why* (a decision, a tradeoff, a rule of the game that reads wrong), never the *what*; no AI-filler narration | | | |
| R9.3 | **CRITICAL** | No point-in-time status — no "currently", "as of this writing", "not yet built", "see the owning PR", work-item numbers or phase labels. A comment must survive loss of context | | | |
| R9.4 | STANDARD | No section-banner comments (`// ---- helpers ----`); ordering and naming carry it. A banner usually means the file does too much | | | |
| R9.5 | **CRITICAL** | **Every stated rationale checked against the artifact it claims.** A comment asserting a stage order, a default value or a parameter name is checked against the rules module in the module cache; a comment citing a benchmark is checked against `BENCHMARK.md`. A false reason is worse than none — this is the row that caught a docstring asserting the opposite of its own code | | | |
| R9.6 | STANDARD | Every package has a package comment stating what it owns (`revive: package-comments`) | | | |

## §10 Testing [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R10.1 | **CRITICAL** | Table tests for variations, not near-duplicate `TestMoveUp` / `TestMoveDown` functions | | | |
| R10.2 | **CRITICAL** | No isolated constructor tests. If a constructor needs one, it has logic that belongs elsewhere (R4.2) | | | |
| R10.3 | STANDARD | Whole-object assertions over field-picking (field-level only when one field is the behaviour under test) | | | |
| R10.4 | STANDARD | Mock at the boundary (HTTP, clock, store); real objects for internal structures; stub over mock when behaviour is simple | | | |
| R10.5 | STANDARD | Test layout mirrors source; helpers are `t.Helper()` and fixtures live in `testdata/` | | | |
| R10.6 | **CRITICAL** | Deterministic tests — no wall-clock dependence, no RNG without a fixed seed, no network. A computed table asserts its case set is non-empty | | | |
| R10.7 | **CRITICAL** | The simulator is differentially tested against `github.com/BattlesnakeOfficial/rules`, not against hand-written expectations of what the rules say — hand-written expectations encode the same misreading twice | | | |
| R10.8 | STANDARD | Determinism is tested as a *property* (run twice, compare), never as a stored golden move list | | | |

## §11 Concurrency & race conditions [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R11.1 | **CRITICAL** | Any change touching shared state (the per-game store, counters, caches, the TT) is explicitly reasoned about for races | | | |
| R11.2 | **CRITICAL** | Handlers are stateless or mutex-guarded; no check-then-act split across a lock release | | | |
| R11.3 | **CRITICAL** | Per-game state is keyed by **game id + snake id**. One server backs several snakes in one match; keying on the game alone makes them share a budget estimate and a search table | | | |
| R11.4 | **CRITICAL** | Every other reader/writer of the same state identified and the interaction confirmed | | | |
| R11.5 | LINT | The change is covered by `go test -race` and the gate is green | | | |

## §12 Resource lifecycle [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R12.1 | **CRITICAL** | The per-game store is bounded and evicted on `/end`, with a TTL sweep for games that never send one. An unbounded map keyed by game id is a leak with a timer on it | | | |
| R12.2 | STANDARD | The HTTP server carries explicit timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`); an unbounded server eventually hangs | | | |
| R12.3 | STANDARD | Goroutines started outside the move path have an owner and a stop condition; nothing is detached without one | | | |

## §13 Search & deadline [REVIEW] — new in this repo

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R13.1 | **CRITICAL** | **Zero allocations in the search hot path**, evidenced by `-benchmem` reporting `0 allocs/op`. A GC pause inside a 400ms budget is a missed deadline | | | |
| R13.2 | **CRITICAL** | **No `map` iteration on any path whose output must be deterministic.** Go randomises it, and it will pass every test while making the harness irreproducible | | | |
| R13.3 | **CRITICAL** | **No goroutine on the move path.** The safe fallback is computed first and held; every return already has an answer; only a completed depth is promoted; safety is never delegated | | | |
| R13.4 | **CRITICAL** | The deadline derives from `game.timeout` on every request, never a constant, and the overhead estimate subtracts our own think time from `you.latency` rather than treating it as network RTT | | | |
| R13.5 | STANDARD | Any claim that a change is faster carries a before/after `testing.B` number or a profile. A shipped "optimisation" without one is a guess (rules.md §2) | | | |
| R13.6 | STANDARD | A ruleset this repo does not implement is played with standard logic **and logged loudly**, never silently | | | |

## Dropped sections

Recorded so the next reader knows they were considered, not forgotten.

| From the source checklist | Why it is not here |
| --- | --- |
| §3 Imports | `goimports` is in the formatter set and Go has no relative imports. Nothing to review by hand |
| §6 Validation | Pydantic-specific. The Go equivalent is R2.2 plus decoding the request at the wire boundary |
| §8 Type annotations | `Optional`/`Union` have no Go analogue; the useful part became R2.4 |

## Findings summary (build only after the ledger is complete)

Filter the `FAIL` rows above into the two groups below, preserving `ID`,
`file:line`, and fix.

### [CRITICAL] — must fix

_(one bullet per CRITICAL FAIL, with rule ID and file:line)_

### [STANDARD] — should fix / discuss

_(one bullet per STANDARD FAIL, with rule ID and file:line)_

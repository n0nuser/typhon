# Go review checklist — `main`, commits 4eb99f6..HEAD

Filled by walking `docs/agents/go-review-checklist.md` against the whole tree,
as AGENTS.md §0 requires before a merge. The loop was never entered, so this
is the review of code written directly — which makes it more necessary, not
less: the gate was green the entire time and a green gate is not this review.

Findings are at the end, derived from the FAIL rows and from nothing else.


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
| R1.1 | STANDARD | Layout per go.dev/doc/modules/layout — `cmd/<binary>/main.go` plus `internal/`; no `pkg/` | PASS | cmd/typhon/main.go, internal/ | cmd/ plus internal/, no pkg/ |
| R1.2 | STANDARD | `cmd/*/main.go` stays thin — environment, logger, wiring, shutdown only; no domain logic | PASS | cmd/typhon/main.go:56 | environment, logger, routes, shutdown; the decision lives in internal/server |
| R1.3 | STANDARD | A package is named after what it owns; no `util`/`helpers`/`common`/`misc` | PASS | internal/ | board, rules, eval, search, server, api - each named for what it owns |
| R1.4 | STANDARD | No stuttering — `search.State`, never `search.SearchState`; no repeated package name in exported identifiers | PASS | internal/search/search.go:66 | search.Searcher, search.Result, board.Point, rules.State |
| R1.5 | **CRITICAL** | Dependencies point inward: `board`, `rules`, `search`, `eval` do not import `api`, `server`, `net/http`, `log/slog` or anything with I/O in it | PASS | internal/board, internal/rules, internal/eval, internal/search | imports checked: only errors, fmt, math/bits and each other. search imports time - see R1.6 |
| R1.6 | **CRITICAL** | `board`/`rules`/`search`/`eval` are pure over a state — no globals, no clock beyond an injected budget, no logging. This is what makes them table-testable and benchmarkable | PASS | internal/search/search.go:327 | the one clock read is `time.Now` against an injected deadline, and it is skipped entirely under a node budget (`s.dead.IsZero()`), which is why a benchmarked game is reproducible and a played one need not be |
| R1.7 | STANDARD | SOLID; composition over embedding; single responsibility per unit | PASS | internal/eval/eval.go:96 | Evaluator owns its scratch; Searcher owns its table; neither reaches into the other |
| R1.8 | STANDARD | YAGNI bounds abstraction — no generalization until reused/non-trivial; trivial single-use logic stays inline | PASS | internal/board/voronoi.go:47 | VoronoiScratch exists because the search needs it allocation-free, not speculatively |
| R1.9 | STANDARD | Behaviour lives with the state it owns; a method moves to a higher layer only when it spans several types or needs an injected collaborator | PASS | internal/rules/state.go:126 | Snake.Cell, Head, Tail are on the type that owns the ring |
| R1.10 | STANDARD | No pass-through wrapper — a method whose whole body forwards to a collaborator's identically-named method is deleted and the caller calls the owner; validating/adapting/narrowing wrappers are fine | PASS | - | no forwarding-only method found |

## §2 Typing [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R2.1 | **CRITICAL** | No `any` plus a type assertion at an internal boundary. `any` appears at the JSON boundary and nowhere else | PASS | internal/server/server.go:217 | the single `any` is writeJSON's value, at the JSON boundary |
| R2.2 | STANDARD | A struct whenever the key set is known and fixed; `map[string]X` only for open/dynamic keys with uniform `X` | PASS | internal/rules/state.go:186 | Config, State, Snake are structs; the only maps are in the harness's dedupe and tests |
| R2.3 | STANDARD | No stringly-typed returns — `Direction`, `Variant`, `H2HRisk` are named types with a `String()`, not bare strings the caller must compare | PASS | internal/board/topology.go:33, internal/rules/errors.go | Direction, Variant, Cause, Risk are named types with String() |
| R2.4 | STANDARD | Named types over bare `int` where the value carries a meaning (square index, score, ply); a `bool` parameter at a call site is usually a named type waiting to happen | PASS | internal/eval/eval.go:21 | Score is a named type; cells are uint16 indices; Budget carries its own meaning |
| R2.5 | STANDARD | Environment configuration is read in one place and passed as a struct, not re-read with `os.Getenv` scattered through the code | PASS | cmd/typhon/main.go:60 | environment read once in run(), passed as search.Config |

## §4 State & construction [mixed]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R4.1 | **CRITICAL** | **No package-level mutable state.** This is the rule that protects reproducibility: a transposition table shared across parallel harness games makes results order-dependent while every unit test still passes. One TT per decider, one decider per game | PASS | internal/search/table.go:33, internal/rules/state.go:170 | no package-level mutable state. One table per Searcher, one Searcher per game, asserted by TestParallelismDoesNotChangeTheResults |
| R4.1a | STANDARD | A frozen, computed-once package constant (a Zobrist key table, a direction table) may be package-level — confirm it is never written after `init` and that nothing derives per-run state from it | PASS | internal/search/table.go:120 | the Zobrist key table is built from a fixed seed and never written after construction; board.Directions is a fixed array |
| R4.2 | **CRITICAL** | No logic in a constructor — field assignment and cheap validation only. I/O, parsing or heavy compute moves to an explicit `Load`/`Build` function that can return an error | PASS | internal/rules/state.go:186 | NewState allocates and validates; NewTopology validates and returns an error rather than panicking |
| R4.3 | STANDARD | Inject boundary collaborators (HTTP client, clock, store) as constructor params; instantiate at point of use otherwise | PASS | internal/server/server.go:33 | the logger and the config are injected; the store's clock is a field so a test can move it |

## §5 Naming & interfaces [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R5.1 | **CRITICAL** | Auto-explicative code — names carry meaning; a reviewer understands intent without comments; prefer renaming over commenting | PASS | internal/rules/sim.go:129 | damageHazards, feed, eliminate, constrict read as what they do |
| R5.2 | STANDARD | Everything the code says is in English — identifiers, comments, log messages, shouts | PASS | - | all identifiers, comments and log messages in English |
| R5.3 | STANDARD | No initiative codenames, issue numbers, sprint or phase labels in identifiers, comments or test names; traceability is the commit message | PASS | - | no issue numbers or phase labels in identifiers; the loop's brief numbers live in .orchestrator/, not in code |
| R5.4 | STANDARD | No term borrowed from another trade's meaning — every identifier checked against its established industry sense | PASS | internal/board/voronoi.go:31 | Contested and Unowned are named, not magic -1/-2 at call sites |
| R5.5 | STANDARD | **Accept interfaces, return structs**, and declare the interface at the *consumer*. An interface earns its place as a real substitution seam (2+ implementations, or a boundary a test must replace), not as decoration beside the implementation | PASS | - | no interface is declared anywhere in the module, so none is unjustified |
| R5.6 | STANDARD | Access data by field name, not by index, when both are available | PASS | internal/rules/state.go:126 | Snake.Cell(i) rather than ring indexing at call sites |
| R5.7 | STANDARD | No `reflect` outside encoding boundaries | PASS | - | no use of reflect |

## §7 Errors [mixed]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R7.1 | **CRITICAL** | Errors are never swallowed; propagate with `fmt.Errorf("...: %w", err)` | PASS | internal/server/convert.go:47 | 13 wrapped returns; none swallowed |
| R7.2 | STANDARD | Sentinel and typed errors declared in one `errors.go` per package, never inline at the raise site | FAIL | internal/board/errors.go, internal/rules/errors.go | was declared in topology.go and state.go. **Fixed in this pass** - moved to errors.go, matching internal/server |
| R7.3 | **CRITICAL** | No `panic` on the move path, and no `log.Fatal` outside `main`. Nothing may be the reason a move is late or absent | PASS | - | no panic and no log.Fatal outside main anywhere in the module |
| R7.4 | LINT | Every returned error is checked (`errcheck`); every opened body closed (`bodyclose`) | N/A | - | lint-gate-green: errcheck and bodyclose are enabled and `make lint` reports 0 issues |

## §9 Comments & docs [mixed]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R9.1 | STANDARD | No forced doc comments on auto-explicative code; write one only when it adds what code cannot say. When present it begins with the identifier name | PASS | internal/board/topology.go:66 | doc comments begin with the identifier |
| R9.2 | STANDARD | Comments explain the *why* (a decision, a tradeoff, a rule of the game that reads wrong), never the *what*; no AI-filler narration | PASS | internal/search/budget.go:36 | the clock-interval comment explains why 1024 was wrong here, not what the constant is |
| R9.3 | **CRITICAL** | No point-in-time status — no "currently", "as of this writing", "not yet built", "see the owning PR", work-item numbers or phase labels. A comment must survive loss of context | PASS | - | no 'currently', no 'not yet', no PR references in code comments |
| R9.4 | STANDARD | No section-banner comments (`// ---- helpers ----`); ordering and naming carry it. A banner usually means the file does too much | PASS | - | no section-banner comments |
| R9.5 | **CRITICAL** | **Every stated rationale checked against the artifact it claims.** A comment asserting a stage order, a default value or a parameter name is checked against the rules module in the module cache; a comment citing a benchmark is checked against `BENCHMARK.md`. A false reason is worse than none — this is the row that caught a docstring asserting the opposite of its own code | PASS | internal/rules/sim.go:29 | the stage order in the comment was read from rules@v1.2.3/standard.go:8 and is asserted by the differential test; the hazard-damage default of 0 and the parameter key `damagePerTurn` were both read from constants.go and are exercised by cmd/typhon-bench |
| R9.6 | STANDARD | Every package has a package comment stating what it owns (`revive: package-comments`) | N/A | - | lint-gate-green: revive's package-comments rule is enabled and passing |

## §10 Testing [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R10.1 | **CRITICAL** | Table tests for variations, not near-duplicate `TestMoveUp` / `TestMoveDown` functions | PASS | internal/board/topology_test.go:27 | tables throughout; no TestStepUp/TestStepDown split |
| R10.2 | **CRITICAL** | No isolated constructor tests. If a constructor needs one, it has logic that belongs elsewhere (R4.2) | PASS | - | no constructor-only tests |
| R10.3 | STANDARD | Whole-object assertions over field-picking (field-level only when one field is the behaviour under test) | PASS | cmd/typhon-bench/bench_test.go:44 | per-arm counters compared as whole structs |
| R10.4 | STANDARD | Mock at the boundary (HTTP, clock, store); real objects for internal structures; stub over mock when behaviour is simple | PASS | internal/server/server_test.go:181 | the store's clock is stubbed; nothing else is mocked |
| R10.5 | STANDARD | Test layout mirrors source; helpers are `t.Helper()` and fixtures live in `testdata/` | PASS | - | layout mirrors source; helpers call t.Helper() |
| R10.6 | **CRITICAL** | Deterministic tests — no wall-clock dependence, no RNG without a fixed seed, no network. A computed table asserts its case set is non-empty | FAIL | cmd/typhon-bench/report.go:120 | **Fixed in this pass.** Not a determinism failure but a counter that read backwards: the mean first-death turn was averaged over every game including the ones the arm survived, so two deaths at turn 165 in thirty games printed as 11. Deaths and their mean turn are reported separately now |
| R10.7 | **CRITICAL** | The simulator is differentially tested against `github.com/BattlesnakeOfficial/rules`, not against hand-written expectations of what the rules say — hand-written expectations encode the same misreading twice | PASS | internal/rules/differential_test.go:26 | random playouts compared against the official ruleset every turn, plus 14 single-step scenarios |
| R10.8 | STANDARD | Determinism is tested as a *property* (run twice, compare), never as a stored golden move list | PASS | internal/board/voronoi_test.go:118, cmd/typhon-bench/bench_test.go:15 | determinism run twice and compared, never stored |

## §11 Concurrency & race conditions [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R11.1 | **CRITICAL** | Any change touching shared state (the per-game store, counters, caches, the TT) is explicitly reasoned about for races | PASS | internal/rules/state.go:170 | State's single-goroutine contract is on the type, found by the race detector rather than assumed |
| R11.2 | **CRITICAL** | Handlers are stateless or mutex-guarded; no check-then-act split across a lock release | PASS | internal/server/server.go:78 | one mutex per game, held across the whole turn; the store has its own |
| R11.3 | **CRITICAL** | Per-game state is keyed by **game id + snake id**. One server backs several snakes in one match; keying on the game alone makes them share a budget estimate and a search table | PASS | internal/server/server.go:196 | stateKey is game id + '/' + snake id, asserted by TestGamesAreKeyedByGameAndSnake |
| R11.4 | **CRITICAL** | Every other reader/writer of the same state identified and the interaction confirmed | PASS | internal/server/store.go:44 | the game struct is reached only through store.get and store.end, both locked |
| R11.5 | LINT | The change is covered by `go test -race` and the gate is green | N/A | - | lint-gate-green: `make check` runs go test -race and it passes |

## §12 Resource lifecycle [REVIEW]

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R12.1 | **CRITICAL** | The per-game store is bounded and evicted on `/end`, with a TTL sweep for games that never send one. An unbounded map keyed by game id is a leak with a timer on it | PASS | internal/server/store.go:110 | evicted on /end, swept on ttl, asserted by TestAbandonedGamesAreSweptOut |
| R12.2 | STANDARD | The HTTP server carries explicit timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`); an unbounded server eventually hangs | PASS | cmd/typhon/main.go:78 | ReadHeaderTimeout, ReadTimeout, WriteTimeout and IdleTimeout all set |
| R12.3 | STANDARD | Goroutines started outside the move path have an owner and a stop condition; nothing is detached without one | PASS | cmd/typhon/main.go:88 | the one goroutine is the listener, ended by Shutdown |

## §13 Search & deadline [REVIEW] — new in this repo

| ID | Sev | Rule | Verdict | Evidence (file:line) | Note |
|---|---|---|---|---|---|
| R13.1 | **CRITICAL** | **Zero allocations in the search hot path**, evidenced by `-benchmem` reporting `0 allocs/op`. A GC pause inside a 400ms budget is a missed deadline | PASS | internal/search/search_test.go:284 | BenchmarkSearch and BenchmarkSearchNode both report 0 allocs/op. Was 11,404 per turn until move ordering stopped returning a slice of a local array |
| R13.2 | **CRITICAL** | **No `map` iteration on any path whose output must be deterministic.** Go randomises it, and it will pass every test while making the harness irreproducible | PASS | - | no map is constructed or ranged over in board, rules, eval or search |
| R13.3 | **CRITICAL** | **No goroutine on the move path.** The safe fallback is computed first and held; every return already has an answer; only a completed depth is promoted; safety is never delegated | FAIL | internal/search/search.go:140 | **Two real bugs, both fixed in this pass.** (1) A completed search was overruled by the one-ply check whenever every neighbour was contested - but the search may have found one loss arriving five turns later than another, and dying later is strictly better. The tie-break now fires on the search's own verdict. (2) Ranking tied moves purely by contester count put certain death first: a self-collision has no contesters, so our own neck beat a contested square. Only enterable squares are considered now |
| R13.4 | **CRITICAL** | The deadline derives from `game.timeout` on every request, never a constant, and the overhead estimate subtracts our own think time from `you.latency` rather than treating it as network RTT | FAIL | internal/search/budget.go:36, internal/search/search.go:118 | **Fixed in this pass.** The budget derivation was always right, but the deadline was not being honoured under load: with `-race` on a busy machine a 2ms budget overran to 86ms at a 64-node check interval. A loaded machine is the realistic case - the free Render tier is shared CPU - so a deadline is now checked at every node, about 1.7% of a node's cost. A node budget still reads no clock, which is what keeps a benchmarked game reproducible |
| R13.5 | STANDARD | Any claim that a change is faster carries a before/after `testing.B` number or a profile. A shipped "optimisation" without one is a guess (rules.md §2) | PASS | - | both optimisations made carry before/after numbers: the clock interval (2ms budget overrunning to 67ms) and the ordering allocation (11,404 to 0) |
| R13.6 | STANDARD | A ruleset this repo does not implement is played with standard logic **and logged loudly**, never silently | PASS | internal/server/convert.go:29 | variantFor logs a warning naming the ruleset; asserted by TestUnsupportedRulesetIsAnnounced |

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

Four rows failed. All four are fixed, in the commits named below, and the
fixes carry tests. `make check` was green for the entire period in which every
one of them was present, which is the point AGENTS.md §0 makes about this
review not being replaceable by a gate.

### [CRITICAL] — must fix

- **R13.3** `internal/search/search.go:140` — a completed search was overruled
  by the one-ply safety check whenever every neighbouring square was contested.
  The check looks one square ahead; the search may have found that one of those
  losses arrives five turns later than another, and dying later is strictly
  better. Fixed: the tie-break fires on the search's own verdict - every root
  move terminal, at the same distance - not on the one-ply check.
  *(commit "Fix two ways the tie-break could hand back a worse move")*

- **R13.3** `internal/search/search.go:398` — and the tie-break itself preferred
  certain death. Ranking purely by how many rivals contest a square puts a
  self-collision first, because nobody is competing for our own neck. Walking
  into ourselves is a certainty; a contested square is a coin flip. Fixed: only
  enterable squares are ranked, with a test asserting the chosen move is one.
  *(same commit)*

- **R13.4** `internal/search/search.go:118` — the deadline was missed under
  load. At a 64-node check interval, with `-race` on a busy machine, a 2ms
  budget ran to 86ms. The realistic deployment is a shared CPU, so this is not
  a test artefact. Fixed: a deadline is checked every node, at about 1.7% of a
  node's cost; a node budget reads no clock at all.
  *(same commit)*

### [STANDARD] — should fix / discuss

- **R7.2** `internal/board/errors.go`, `internal/rules/errors.go` — sentinel
  errors were declared in `topology.go` and `state.go` rather than in an
  `errors.go`. The substance was fine - package-level var blocks, not inline at
  the raise site - so this was the letter of the rule. Fixed by moving them.
  *(commit "Move sentinel errors into errors.go")*

- **R10.6** `cmd/typhon-bench/report.go:120` — a path counter read backwards:
  the mean first-death turn averaged over every game including the survivals,
  so two deaths at turn 165 in thirty games printed as 11. The whole reason
  those counters print beside the win column is that a claim should be
  checkable against how often the thing happened, and a counter that reads
  backwards is worse than none. Fixed.
  *(commit "Report deaths and their turn separately")*

## What this review says about the gate

Nothing here was caught by `gofmt`, `go vet`, `golangci-lint`, `go test -race`
or a 96% coverage figure. Two of the five were silent preferences for a worse
move, and the bot would have gone on playing and losing without anything in a
log to point at. That is the argument for walking the list rather than trusting
the gate, and it is the argument the predecessor's project records having
learned the same way.

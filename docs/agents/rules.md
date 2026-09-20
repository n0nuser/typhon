# Working rules

Ported from `deadchannel`'s rule set and adapted to this repository. They apply
to every contributor, human or agent.

Rules that were specific to that project's stack — Pydantic conventions, Ruff
codes, image-comparison testing — are **not** reproduced here: this is a Go
service with a search engine in it. Carrying them across would be noise that
trains readers to skim. What follows is the part that transfers, plus the local
equivalents where this project has its own answer.

---

## 1. Change discipline

These exist because each failure below has actually happened.

### Verify before you claim

**Never report work as done, working, or passing without having run something
that proves it.** "The tests should pass" is not a result. Run the command, read
the output, quote the decisive line.

Verification exercises the real path, not a proxy:

- Moved a file? Run the tests **and** build the binary that imports it.
- Changed a default? Print the resolved value; don't trust the source line.
- Fixed a bug? Reproduce it first, then show that reproduction passing.
- Edited a doc? Check the links resolve and the paths exist.
- Encoded a rule of the game? Read it in
  `$(go env GOPATH)/pkg/mod/github.com/!battlesnake!official/rules@v1.2.3/`,
  not in the prose docs and not from memory. The stage order, the parameter
  key `damagePerTurn`, the hazard-damage default of `0` and the
  food-cancels-hazard-damage branch are all things the prose does not say.

Caching and process state will lie to you. `go test` caches: a suite that
"passed" may not have run at all, which is what `-count=1` is for when a result
surprises you. When a result confirms what you expected, that is precisely when
to check it a second way.

If you could not verify something, say so plainly and name what is unverified.

### Never suppress a diagnostic to make it quiet

`//nolint`, widening `.golangci.yml`, deleting an assertion, or `t.Skip` all
make a signal disappear without fixing the cause.

Legitimate suppression is rare, narrow, and **explained on the same line**:

```go
// ✅ specific linter, real reason, minimal scope
resp, err := c.Do(req) //nolint:bodyclose // closed by the caller; see Warm's contract

// ❌ blanket, unexplained, hides whatever else breaks
resp, err := c.Do(req) //nolint
```

- Suppress **one specific linter**, never a bare `//nolint`.
- A suppression without a stated reason is a bug.
- If a suppression existed only for a workaround you removed, delete it too.
- Never edit `.golangci.yml`, the `Makefile` gate or a coverage threshold to get
  green.

A silently empty table test is the same failure wearing a different hat: `go
test` reports a table with zero cases as a pass. Assert the case set is
non-empty when it is computed rather than literal.

### Delete workarounds when the reason is gone

Shims, aliases, compatibility wrappers and build tags live only as long as the
constraint that forced them. When you fix the root cause, remove the workaround
**and** what it dragged along — the `//nolint` it needed, the doc line
describing it, the path that only existed to route around it. Leaving both the
fix and the workaround is worse than either alone: the next reader cannot tell
which path is real.

### Don't hardcode paths that depend on the working directory

`go test` runs with the package directory as the working directory, so a test
fixture is `testdata/` relative to the package and a scratch file is
`t.TempDir()` — never a path that assumes the repo root.

The wrong version does not crash. It quietly reads or writes somewhere else,
which is why it survives review.

### Stay inside the scope you were given

Fix what was asked. If you notice something else wrong, **say so** rather than
silently fixing it in the same change — unrequested edits bury the actual one.
Two exceptions: fix it inline when leaving it would make your own change
incorrect, or when it is a one-line consequence of what you touched.

Never narrow scope silently either. If part of the task is blocked, finish
everything else and state explicitly what you left undone and why.

### Regenerated artifacts are not free to overwrite

Committed evidence encodes a measurement. Re-running a generator and committing
whatever falls out replaces reviewed evidence with an unreviewed run. Diff
first; if only noise changed, revert.

This binds hardest on **`BENCHMARK.md`**. Every number in it came from a run of
`cmd/typhon-bench` at a stated seed block, sample size and configuration. A
change that alters the search or the eval invalidates those numbers, and they
must be re-measured in the same change or the entry is removed — a stale table
read as current is worse than no table.

---

## 2. Complexity — the grug rules

Complexity is the apex predator. Every decision should reduce it.

- **Say no.** The default answer to a new feature, abstraction or indirection is
  no. If forced to yes, build the 80/20 version.
- **Factor late.** Wait for cut points — narrow interfaces that trap complexity
  behind a small surface. Wrong abstractions cost more than duplication.
- **Chesterton's fence.** Understand code before removing it. If you cannot
  explain why it exists, you do not have permission to delete it.
- **Name intermediate conditions.** Break dense conditionals into named
  variables; they are easier to read and to breakpoint.
- **DRY, with balance.** Simple repetition often beats a complex abstraction.
- **Locality of behaviour.** Put code on the thing that does the thing.
- **Fear concurrency.** Prefer simple models.
- **Never optimize without a profile.** In this codebase the corollary is sharp,
  because the whole project is a search engine under a deadline: "make it
  faster" is justified by a `testing.B` number or a `pprof` profile, never by a
  hunch about what is hot. A change that claims to be an optimisation and ships
  without a before/after benchmark is not an optimisation, it is a guess.
- **Generics and closures are salt.** A little goes a long way.
- **No fear of looking dumb.** Say "this is too complex" out loud.

---

## 3. Go conventions

### Architecture

- SOLID, composition over embedding, single responsibility.
- Dependency injection over package-level state. **No package-level mutable
  state at all** — see the checklist's §4, which is `CRITICAL` here because a
  shared transposition table silently destroys reproducibility.
- YAGNI governs abstractions: don't extract until the logic is reused,
  non-trivial, or clutters the main flow.
- **Accept interfaces, return structs**, and declare the interface at the
  *consumer*, not beside the implementation. A one-method interface named for
  what the caller needs is the right size.
- No `util`, `helpers`, `common` or `misc` packages. A package is named after
  what it owns.
- No stuttering: `search.State`, never `search.SearchState`.
- `cmd/<binary>/main.go` is wiring only — environment, logger, routes, shutdown.

### Purity

`internal/board`, `internal/rules`, `internal/search` and `internal/eval` are
**pure functions over a state**: no I/O, no logging, no clock beyond an injected
budget, no globals. That is not a style preference, it is what makes them
table-testable against the official rules and benchmarkable against a deadline.

### Style

- Readable over clever, predictable over magic.
- Self-documenting names over comments. A doc comment begins with the
  identifier it documents.
- Named types over bare `int` where the value has a meaning — `Direction`,
  `Variant`, `Score`, square indices. A `bool` parameter at a call site is
  usually a named type waiting to happen.
- `any` appears at the JSON boundary and nowhere else.
- Everything the code says is in English — identifiers, comments, log messages.

### Errors

- Wrap with `%w`; never swallow. `if err != nil { return fmt.Errorf("...: %w", err) }`.
- Specific errors, declared in one `errors.go` per package, never inline at the
  raise site.
- `panic` never appears on the move path. Nothing may be the reason a move is
  late or absent.

### Returns

- Slice-returning functions return an empty slice, never `nil`, unless absence
  is semantically distinct from empty.
- No anonymous multi-returns where the positions carry distinct meanings and
  there are more than two — use a small struct.

### Determinism

- **No `map` iteration on any path whose output must be deterministic.** Go
  randomises it. Sort the keys, or use a slice.
- No RNG in the move path. Ties break on a fixed, documented order.

### Business-decision annotations

When a decision is not obvious from the code — a workaround, a rule of the game
that reads wrong, a deliberate deviation — say **why** inline. For temporary
decisions include a `TODO` with the removal condition.

---

## 4. Testing

- Tests validate behaviour and flow, not a coverage number.
- Read whole test files before editing them — spot the existing helpers.
- Each test verifies one behaviour, with a name that says which.
- **Use table tests for variations.** Never `TestMoveUp`, `TestMoveDown`,
  `TestMoveLeft`; one table.
- Assert whole objects, not field by field, where the whole object is the claim.
- Reproduce a bug with a failing regression test first, then fix it.
- Avoid mocking unless necessary; mock at coarse boundaries only.
- No isolated constructor tests — if a constructor needs a test, it has logic in
  it that belongs elsewhere.
- Test layout mirrors source: `internal/board/topology.go` →
  `internal/board/topology_test.go`.

### Local equivalents

- **Determinism is the headline contract.** Same seed, same config, same node
  budget must produce a bit-identical move log. Test the *property* by running
  twice and comparing — never by pinning a stored golden move list, which
  asserts today's moves rather than the property.
- **The official rules are the oracle.** The simulator is differentially tested
  against `github.com/BattlesnakeOfficial/rules`, not against hand-written
  expectations of what the rules say. Hand-written expectations encode the same
  misreading twice.
- **`-race` is in the gate**, and every test that touches the per-game store
  runs under it.
- **Benchmarks report nodes/sec and `allocs/op`.** A search benchmark without
  `-benchmem` is not measuring the thing that misses deadlines.

---

## 5. Measurement discipline

This project exists partly because the previous one drew conclusions from noise.
These are not suggestions.

- **n=20 is worthless.** Two runs of an identical configuration on different
  seed blocks went 8-10-2 and 25-9-6. Nothing below **n=200** is published.
- **Measure the floor before comparing anything.** Two identical bots went
  29-23-8, so one slot's baseline is ~56%, not 50%. Start-position bias is real.
  Alternate slot assignment across seeds and report per-slot rates separately.
- **Verify a feature fires before benchmarking it.** A whole batch once measured
  an inert feature whose threshold was never crossed. Print per-path counts per
  run and check them.
- **Keep a random control arm.** Breaking ties with a coin lost 2-18; without
  that floor you cannot tell a contributing component from a useless one.
- **One flag per variable.** Two runs differ in exactly one thing, and the run
  header states which.
- **Report negative results.** They were the most interesting findings last
  time.

---

## 6. Documentation

- Settled decisions land in `docs/` — not only in a commit message or a chat log.
- A document that cites evidence makes a **live promise**. If the evidence lives
  in the repo, it stays runnable; if you move it, update the citation in the
  same change.
- Comments explain the **why**, never the what. No AI-filler narration.
- No point-in-time status in a comment ("currently", "as of this writing", "not
  yet built") and no issue numbers or phase labels in identifiers. Traceability
  is the commit message.

---

## 7. Communication

After making changes: **bullet points, a few sentences.** What changed and why.
No preamble, no essay, no summary of the summary. Expand only when asked, when
introducing a new architectural pattern, or when making a breaking change.

You are the executor. Do not read AGENTS.md or `.orchestrator/`. Do not run
opencode. Do not edit `.orchestrator/*`.

# Task: the wire types and the board primitives

Create two packages in the Go module `github.com/n0nuser/typhon`:
`internal/api` and `internal/board`. Nothing else. Do not create
`internal/rules`, `internal/search`, `internal/eval` or `internal/server`, and
do not touch `cmd/`.

## Ground rules for this repo

- **Do not use `rtk`.** Run `go`, `gofmt` and `make` directly.
- **Do not touch `.golangci.yml`, `Makefile`, `go.mod` or `render.yaml`.** If a
  linter fires, fix the code. No bare `//nolint` — if a suppression is truly
  needed it names one linter and states the reason on the same line.
- Every package needs a package comment (`revive: package-comments` is on).
- Every exported identifier needs a doc comment starting with its own name
  (`revive: exported` is on).
- Comments explain **why**, never what. No narration, no "currently", no TODO
  without a removal condition.
- `gofumpt` is in the gate, so run `make fmt-fix` before you finish.

## Acceptance check

Run, and paste the real output:

```
go test ./internal/... -race -count=1
make check
```

Both must pass. `make check` needs `$(go env GOPATH)/bin` on PATH.

---

## Package `internal/api`

File `internal/api/types.go`. The Battlesnake HTTP wire types, mirroring
<https://docs.battlesnake.com/api/example-move>. Plain structs with JSON tags,
no logic:

- `Coord{X, Y int}`
- `RulesetSettings{FoodSpawnChance, MinimumFood, HazardDamagePerTurn int}` with
  JSON tags `foodSpawnChance`, `minimumFood`, `hazardDamagePerTurn`
- `Ruleset{Name, Version string; Settings RulesetSettings}`
- `Game{ID, Map, Source string; Ruleset Ruleset; Timeout int}`
- `Customizations{Color, Head, Tail string}`
- `Battlesnake{ID, Name string; Health int; Body []Coord; Latency string;
  Head Coord; Length int; Shout string; Customizations Customizations}`
- `Board{Height, Width int; Food, Hazards []Coord; Snakes []Battlesnake}`
- `GameRequest{Game Game; Turn int; Board Board; You Battlesnake}`
- `InfoResponse{APIVersion, Author, Color, Head, Tail, Version string}` —
  `apiversion` is one lowercase word in JSON; the rest are `omitempty`
- `MoveResponse{Move string; Shout string omitempty}`

Two facts that belong in the package or field doc comments because they are not
obvious and they cost the predecessor real games:

- The board origin `(0,0)` is the **bottom-left** corner and y increases upward,
  so "up" is y+1.
- `Game.Timeout` is the per-move budget in milliseconds and it **includes** the
  round trip. Derive deadlines from it, never from a constant 500.
- `Battlesnake.Body` is ordered head-first and **may contain duplicate
  coordinates** while a snake is stacked — at game start, and on the turn after
  it eats, when the engine duplicates the tail.

Do **not** write an adapter from `GameRequest` to a board state in this step.
`internal/api` must not import `internal/board` yet.

---

## Package `internal/board`

Pure functions over a board. **No I/O, no logging, no globals, no clock.** This
package must not import `internal/api`, `net/http`, `log/slog`, or anything with
side effects. It is the package the search runs inside, so it is written to be
table-testable and benchmarkable.

### `internal/board/topology.go`

The single most important constraint in this step: **every neighbour
calculation and every distance calculation goes through this file.** A `x + 1`
written anywhere else in the repo is a bug, because that is exactly how the
`wrapped` ruleset ends up subtly wrong while all the `standard` tests stay
green. Say so in the file's doc comment.

Define:

- `type Point struct{ X, Y int }` — the board's own coordinate type. It is
  deliberately separate from `api.Coord` so that this package does not depend on
  the wire format.
- `type Direction uint8` with constants `Up, Down, Left, Right` in that order,
  a `Directions [4]Direction` array in that stable order, and
  `func (d Direction) String() string` returning the wire words `"up"`,
  `"down"`, `"left"`, `"right"`. The fixed order is load-bearing: it is how
  exact ties break deterministically, so do not sort it or iterate a map.
- `type Topology struct { Width, Height int; Wrapped bool }`
- `func NewTopology(width, height int, wrapped bool) (Topology, error)` —
  returns an error for a non-positive dimension and for `width > MaxWidth`.
- `const MaxWidth = 64` with a comment explaining the choice: the bitset holds
  one `uint64` per row, and the largest official board is 25 wide, so 64 is
  headroom rather than a limit anyone reaches. A board wider than this is
  rejected at the boundary rather than played wrong.
- `func (t Topology) Cells() int`
- `func (t Topology) Index(p Point) int` — `p.Y*Width + p.X`
- `func (t Topology) At(index int) Point`
- `func (t Topology) InBounds(p Point) bool` — on a wrapped board every
  normalised point is in bounds; this reports whether the *raw* point is inside
  the rectangle.
- `func (t Topology) Step(p Point, d Direction) (Point, bool)` — the one place a
  move is applied. On a bounded board the second result is false when the step
  leaves the rectangle. On a wrapped board it wraps and is always true.
- `func (t Topology) Distance(a, b Point) int` — Manhattan on a bounded board.
  On a wrapped board it is toroidal:
  `min(|dx|, Width-|dx|) + min(|dy|, Height-|dy|)`. Both the food logic and the
  head-to-head logic use this, which is why it must not be duplicated.

### `internal/board/bitset.go`

`type Bitset` holding one `uint64` per row: bit `x` of word `y` is set when
`(x, y)` is occupied. This makes flood fill a few word operations per row
instead of a queue.

- `func (t Topology) NewBitset() Bitset`
- `Set`, `Clear`, `Has` taking a `Point`, and `Count() int` via
  `math/bits.OnesCount64`.
- In-place set operations writing into a destination that the caller already
  owns, so the hot path never allocates: `Or`, `And`, `AndNot`, `CopyFrom`,
  `Clone`, `Reset`, `Equal`, `IsEmpty`.
- `func (t Topology) rowMask() uint64` — `(1<<Width)-1`, with the `Width == 64`
  case correct. Every operation that shifts left must mask with it, or bits
  escape past the right edge of the board.
- `func (t Topology) Dilate(dst, src Bitset)` — the core primitive: `dst`
  becomes every cell of `src` plus each of its four neighbours. Bounded:
  `row>>1`, `(row<<1)&mask`, and the rows above and below, with the edge rows
  simply having nothing beyond them. Wrapped: the horizontal shifts become
  rotations within the width mask, and the vertical ones wrap modulo height.
  `dst` and `src` must be allowed to be the same Bitset only if you handle it —
  if you do not, document that they must differ and make the callers obey it.
- `Points() []Point` and/or an iterator, for tests and for the slow paths.

### `internal/board/fill.go`

- `func (t Topology) Reach(blocked Bitset, start Point, scratch *FillScratch) Bitset`
  — the set of cells reachable from `start` without entering `blocked`.
  Implement it as repeated `Dilate` masked by `^blocked`, iterated to a
  fixpoint. Return an empty set when `start` itself is blocked.
- `FillScratch` is a caller-owned struct of preallocated Bitsets so that a
  repeated fill in the search allocates nothing. Provide
  `func (t Topology) NewFillScratch() *FillScratch`.
- `func (t Topology) ReachableCount(blocked Bitset, start Point, scratch *FillScratch) int`
- `func (t Topology) Reaches(blocked Bitset, start, target Point, scratch *FillScratch) bool`
  — whether `target` is in the reachable set. Document *why* this matters and
  not just what it does: a snake that can still path to its own tail can
  survive by following it, because the tail keeps vacating squares ahead of the
  head; a large open area with no route back to the tail is how a snake walks
  into a trap several turns before the trap closes, which raw square-counting
  cannot see.

### `internal/board/voronoi.go`

Multi-source ownership: which snake reaches each cell first.

```go
type Source struct {
    Head   Point
    Length int
}

type Ownership struct {
    // Owner[i] is the index into the sources slice that owns cell i,
    // or Unowned, or Contested.
    Owner []int16
}

const (
    Unowned   int16 = -1
    Contested int16 = -2
)

func (t Topology) Voronoi(blocked Bitset, sources []Source, scratch *VoronoiScratch) Ownership
```

Expand every source's frontier one step at a time, simultaneously. A cell newly
reached at step `n` by exactly one source belongs to that source. A cell reached
at step `n` by more than one source goes to the **longest** source, because that
is the snake that would win the head-to-head there; if the longest is tied, the
cell is `Contested` and belongs to nobody. Cells already owned at an earlier
step are never reassigned. Provide `func (o Ownership) CountFor(i int) int`.

`VoronoiScratch` is caller-owned and preallocated, same reason as `FillScratch`.

**Do not use a `map` anywhere in this package.** Go randomises map iteration
order, and a search whose output depends on it cannot be reproduced, which
breaks the benchmark harness this project is built around.

---

## Tests

`internal/board/topology_test.go`, `bitset_test.go`, `fill_test.go`,
`voronoi_test.go`. **Table tests** — one table per behaviour, never
`TestStepUp`/`TestStepDown`/`TestStepLeft` as separate functions. Do not write
tests for plain structs or constructors that only assign fields.

Cover at minimum:

- `Step` off each of the four edges on a bounded board returns `ok == false`;
  the same steps on a wrapped board return the opposite edge.
- `Distance` on a wrapped 11x11: `(0,0)` to `(10,0)` is **1**, not 10. Include
  the bounded case in the same table for contrast.
- `Dilate` at a corner on a bounded board produces 3 cells, and on a wrapped
  board produces 5.
- `rowMask` at `Width == 64` does not overflow, and a left shift at the right
  edge does not leak into the next row.
- `Reach` on an empty bounded board from any cell reaches `Width*Height`; with
  a full-height wall splitting the board it reaches only its own side; on a
  wrapped board a vertical wall does **not** split it, because the players can
  go round.
- `Reach` from a blocked start is empty.
- `Voronoi` with two equal-length sources on an empty 11x11 gives each the same
  count and leaves the middle column `Contested`; with one source longer, the
  contested cells go to the longer one.
- A determinism test: run `Voronoi` and `Reach` twice on the same input and
  require identical results. This is a property test, not a golden file — do
  not store an expected cell list.

Add `testing.B` benchmarks for `Reach` and `Voronoi` on a populated 11x11, and
report `allocs/op`. With the scratch structs reused across iterations these
should be **0 allocs/op**; if they are not, fix the allocation rather than
lowering the bar, and say in the benchmark's comment what the measured number
is.

Start writing code immediately. Do not spend the run on further orientation.

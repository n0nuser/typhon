package search

import (
	"math/rand/v2"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/eval"
	"github.com/n0nuser/typhon/internal/rules"
)

// entry is one transposition table slot.
type entry struct {
	key   uint64
	score eval.Score
	depth int16
	gen   uint16
	move  board.Direction
	exact bool
}

// table caches positions the search has already valued.
//
// One table belongs to one searcher and one game. Sharing a table between
// games running in parallel makes each game's result depend on what the others
// happened to look up, which would quietly destroy the reproducibility the
// whole benchmark plan rests on while leaving every unit test green.
type table struct {
	entries []entry
	mask    uint64
	gen     uint16
}

func newTable(bits uint) *table {
	if bits < 10 {
		bits = 10
	}
	size := uint64(1) << bits
	return &table{entries: make([]entry, size), mask: size - 1}
}

// newSearch ages the table so that this search's entries replace older ones.
func (t *table) newSearch() { t.gen++ }

// probe returns a stored exact score for the position if one was stored at
// least this deep.
func (t *table) probe(key uint64, depth int) (eval.Score, board.Direction, bool) {
	e := &t.entries[key&t.mask]
	if e.key != key || !e.exact || int(e.depth) < depth {
		return 0, board.Up, false
	}
	return e.score, e.move, true
}

// probeMove returns the best move stored for a position, whatever its depth.
// A move from a shallow search is still worth trying first.
func (t *table) probeMove(key uint64) (eval.Score, board.Direction, bool) {
	e := &t.entries[key&t.mask]
	if e.key != key {
		return 0, board.Up, false
	}
	return e.score, e.move, true
}

// store keeps an exact score, preferring deeper results and this search's own.
func (t *table) store(key uint64, score eval.Score, depth int, move board.Direction) {
	e := &t.entries[key&t.mask]
	if e.key == key && e.exact && int(e.depth) > depth && e.gen == t.gen {
		return
	}
	*e = entry{key: key, score: score, depth: int16(depth), gen: t.gen, move: move, exact: true}
}

// storeMove keeps only the move, for ordering, without claiming a score.
func (t *table) storeMove(key uint64, move board.Direction) {
	e := &t.entries[key&t.mask]
	if e.key == key {
		e.move = move
		return
	}
	*e = entry{key: key, move: move, gen: t.gen}
}

// zobrist holds the random keys a position hash is built from.
//
// The keys come from a fixed seed, so two runs of the same search hash the same
// way. A table seeded from the clock would make every game unreproducible for
// no benefit whatsoever.
type zobrist struct {
	body    [][]uint64
	food    []uint64
	hazard  []uint64
	health  [][]uint64
	alive   []uint64
	variant []uint64
}

const healthBuckets = 13

func newZobrist(topo board.Topology) *zobrist {
	rng := rand.New(rand.NewPCG(0x7479_7068_6f6e_0001, 0x5eed_c0ff_ee00_d1ce))
	cells := topo.Cells()

	z := &zobrist{
		body:    make([][]uint64, rules.MaxSnakes),
		food:    make([]uint64, cells),
		hazard:  make([]uint64, cells),
		health:  make([][]uint64, rules.MaxSnakes),
		alive:   make([]uint64, rules.MaxSnakes),
		variant: make([]uint64, 4),
	}
	for i := range rules.MaxSnakes {
		z.body[i] = make([]uint64, cells)
		for c := range cells {
			z.body[i][c] = rng.Uint64()
		}
		z.health[i] = make([]uint64, healthBuckets)
		for b := range healthBuckets {
			z.health[i][b] = rng.Uint64()
		}
		z.alive[i] = rng.Uint64()
	}
	for c := range cells {
		z.food[c] = rng.Uint64()
		z.hazard[c] = rng.Uint64()
	}
	for i := range z.variant {
		z.variant[i] = rng.Uint64()
	}
	return z
}

// hash mixes the whole position, not just the heads.
//
// Bodies are included segment by segment. Two positions with the same heads and
// different bodies are different positions, and a hash that collapsed them
// would have the search return a cached value for a board it never looked at.
func (z *zobrist) hash(s *rules.State) uint64 {
	h := z.variant[s.Variant&3]

	for i := range s.Snakes {
		sn := &s.Snakes[i]
		if !sn.Alive() {
			continue
		}
		h ^= z.alive[i]
		for j := range sn.Len() {
			h ^= z.body[i][sn.Cell(j)]
		}
		bucket := sn.Health / 8
		if bucket < 0 {
			bucket = 0
		}
		if bucket >= healthBuckets {
			bucket = healthBuckets - 1
		}
		h ^= z.health[i][bucket]
	}

	for y := range s.Food {
		row := s.Food[y]
		for row != 0 {
			x := trailingZeros(row)
			row &= row - 1
			h ^= z.food[y*s.Topo.Width+x]
		}
	}
	return h
}

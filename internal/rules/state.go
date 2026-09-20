// Package rules holds the forward simulator: the game state the search runs on
// and the rule that advances it by one turn.
//
// It is a reimplementation of github.com/BattlesnakeOfficial/rules, not a
// wrapper around it. The official ruleset allocates a fresh board state with
// fresh slices on every call, which is correct and far too slow to sit inside
// a search; this one mutates a single state and records an undo. The official
// package is the test oracle instead, which is a stronger guarantee than the
// hand-written property tests it replaces.
//
// Two things the engine does are deliberately not modelled, because both are
// driven by a seed this process never sees, so predicting them would be
// fiction: food spawning, and royale's hazard expansion. The food and hazard
// sets are held static across a search horizon.
package rules

import (
	"errors"
	"math/bits"

	"github.com/n0nuser/typhon/internal/board"
)

// MaxSnakes is the largest number of snakes a state holds.
const MaxSnakes = 8

// MaxHealth is the health a snake is restored to when it eats.
const MaxHealth = 100

// Errors returned when a state cannot be built.
var (
	// ErrTooManySnakes reports a board with more snakes than MaxSnakes.
	ErrTooManySnakes = errors.New("rules: too many snakes")
	// ErrEmptySnake reports a snake with no body, which the engine treats as
	// an error rather than as a dead snake.
	ErrEmptySnake = errors.New("rules: zero-length snake")
	// ErrDuplicateHazard reports a hazard square listed more than once. The
	// engine deals its damage once per listing; this package models hazards as
	// a set and deals it once, so a duplicate is refused rather than played
	// with quietly different damage.
	ErrDuplicateHazard = errors.New("rules: duplicate hazard square")
)

// Variant is the ruleset being played.
type Variant uint8

// The rulesets this package implements. Anything else is played as Standard by
// the caller, which must say so in its log rather than doing it silently.
const (
	// Standard is the default ruleset.
	Standard Variant = iota
	// Royale adds hazard squares that drain health.
	Royale
	// Constrictor removes food, pins health at maximum and grows every snake
	// every turn, so the board fills until only one snake can move.
	Constrictor
	// Wrapped makes the board a torus: leaving one edge arrives at the other.
	Wrapped
)

// String returns the ruleset's wire name.
func (v Variant) String() string {
	switch v {
	case Royale:
		return "royale"
	case Constrictor:
		return "constrictor"
	case Wrapped:
		return "wrapped"
	default:
		return "standard"
	}
}

// Cause records why a snake left the board, mirroring the engine's own reasons
// so that a differential test can compare them.
type Cause uint8

// Why a snake was eliminated.
const (
	// Alive means the snake is still playing.
	Alive Cause = iota
	// OutOfHealth means health reached zero.
	OutOfHealth
	// OutOfBounds means the snake left a bounded board.
	OutOfBounds
	// SelfCollision means the snake ran into its own body.
	SelfCollision
	// BodyCollision means the snake ran into another snake's body.
	BodyCollision
	// HeadToHead means the snake met another head on and did not win.
	HeadToHead
	// Hazard means health reached zero from hazard damage.
	Hazard
)

// String returns the engine's own spelling of the cause.
func (c Cause) String() string {
	switch c {
	case OutOfHealth:
		return "out-of-health"
	case OutOfBounds:
		return "wall-collision"
	case SelfCollision:
		return "snake-self-collision"
	case BodyCollision:
		return "snake-collision"
	case HeadToHead:
		return "head-collision"
	case Hazard:
		return "hazard"
	default:
		return ""
	}
}

// Snake is one snake's body, health and fate.
//
// The body is a ring buffer of cell indices with the head at ring[head] and the
// tail at ring[head+length-1]. Moving is then a single decrement of head: the
// new head is written into the slot that opens up, and the old tail falls off
// the end without anything being copied. Growing writes one more slot.
//
// That is the whole reason for the ring. A slice-based body has to shift every
// segment on every move, which at a hundred thousand nodes a turn is the
// difference between searching six plies and searching three.
type Snake struct {
	ID     string
	ring   []uint16
	head   int
	length int
	Health int
	Cause  Cause
}

// Alive reports whether the snake is still playing.
func (s *Snake) Alive() bool { return s.Cause == Alive }

// Len returns the snake's length in body segments, counting duplicates.
func (s *Snake) Len() int { return s.length }

// Cell returns the flat board index of the i'th body segment, head first.
func (s *Snake) Cell(i int) uint16 { return s.ring[s.slot(i)] }

// Head returns the flat board index of the snake's head.
func (s *Snake) Head() uint16 { return s.ring[s.head] }

// Tail returns the flat board index of the snake's last body segment.
//
// It is the square that frees up as the snake moves - unless the snake just
// ate, in which case the engine has stacked a duplicate there and it stays
// blocked for one more turn.
func (s *Snake) Tail() uint16 { return s.ring[s.slot(s.length-1)] }

func (s *Snake) slot(i int) int {
	n := s.head + i
	if n >= len(s.ring) {
		n -= len(s.ring)
	}
	return n
}

// State is the whole board for one turn.
//
// It is mutated in place by Apply and restored by Unapply, so a search holds
// exactly one of these and walks it, rather than allocating a board per node.
//
// A State is owned by one goroutine. Not only Apply and Unapply mutate it:
// Passable rebuilds state-owned scratch, so even two apparently read-only
// callers racing on one State is a bug. The tournament harness therefore gives
// every game its own State, its own evaluator and its own transposition table,
// which is also what makes a run reproducible rather than dependent on the
// order games happen to interleave.
type State struct {
	Topo         board.Topology
	Variant      Variant
	HazardDamage int
	Turn         int

	Snakes []Snake

	Food    board.Bitset
	Hazards board.Bitset

	// counts holds how many body segments stand on each cell. Counting rather
	// than flagging is what makes a stacked snake - one whose body holds the
	// same square twice, at the start of a game or the turn after it eats -
	// keep that square blocked when only one of its segments leaves.
	counts   []uint8
	occupied board.Bitset

	// passable is Occupied with vacating tails released, rebuilt on demand.
	passableCounts []uint8
	passable       board.Bitset

	// foodStack holds the food cells removed by each nested Apply, so that
	// Unapply can put them back without Undo carrying a slice of its own.
	foodStack []uint16
}

// Occupied returns the squares a snake may not move into, which is every square
// holding at least one body segment.
//
// The set is owned by the state and changes under the caller on the next Apply.
func (s *State) Occupied() board.Bitset { return s.occupied }

// AliveCount returns how many snakes are still playing.
func (s *State) AliveCount() int {
	n := 0
	for i := range s.Snakes {
		if s.Snakes[i].Alive() {
			n++
		}
	}
	return n
}

// Over reports whether the game has ended.
//
// The engine checks this before it applies any move, so a state with one snake
// left never advances - which matters for the search as much as for the
// differential test, since it is the terminal condition of every duel.
func (s *State) Over() bool { return s.AliveCount() <= 1 }

// SnakeSpec is one snake's starting position, as the caller reads it off the
// wire.
type SnakeSpec struct {
	ID     string
	Health int
	// Body is head-first and may repeat a coordinate while the snake is
	// stacked, which the engine does at the start of a game and on the turn
	// after a snake eats.
	Body []board.Point
}

// Config is everything needed to build a state.
type Config struct {
	Topology     board.Topology
	Variant      Variant
	HazardDamage int
	Turn         int
	Snakes       []SnakeSpec
	Food         []board.Point
	Hazards      []board.Point
}

// NewState builds a state from a board as the engine described it.
func NewState(cfg Config) (*State, error) {
	if len(cfg.Snakes) > MaxSnakes {
		return nil, ErrTooManySnakes
	}

	t := cfg.Topology
	// A body can outgrow the board only by stacking, and stacking adds at most
	// one segment per snake per turn, so the board plus a margin is enough.
	ringSize := t.Cells() + MaxSnakes + 2

	s := &State{
		Topo:           t,
		Variant:        cfg.Variant,
		HazardDamage:   cfg.HazardDamage,
		Turn:           cfg.Turn,
		Snakes:         make([]Snake, len(cfg.Snakes)),
		Food:           t.NewBitset(),
		Hazards:        t.NewBitset(),
		counts:         make([]uint8, t.Cells()),
		occupied:       t.NewBitset(),
		passableCounts: make([]uint8, t.Cells()),
		passable:       t.NewBitset(),
		foodStack:      make([]uint16, 0, 64),
	}

	for _, p := range cfg.Food {
		s.Food.Set(p)
	}
	for _, p := range cfg.Hazards {
		if s.Hazards.Has(p) {
			return nil, ErrDuplicateHazard
		}
		s.Hazards.Set(p)
	}

	for i, spec := range cfg.Snakes {
		if len(spec.Body) == 0 {
			return nil, ErrEmptySnake
		}
		sn := &s.Snakes[i]
		sn.ID = spec.ID
		sn.Health = spec.Health
		sn.ring = make([]uint16, ringSize)
		sn.head = 0
		sn.length = len(spec.Body)
		for j, p := range spec.Body {
			sn.ring[j] = uint16(t.Index(p))
		}
		s.addBody(i)
	}

	return s, nil
}

// addBody puts every segment of a snake back onto the occupancy counts.
func (s *State) addBody(i int) {
	sn := &s.Snakes[i]
	for j := range sn.length {
		s.occupy(sn.Cell(j))
	}
}

// removeBody takes a snake off the board, which is what elimination does.
func (s *State) removeBody(i int) {
	sn := &s.Snakes[i]
	for j := range sn.length {
		s.vacate(sn.Cell(j))
	}
}

func (s *State) occupy(cell uint16) {
	s.counts[cell]++
	if s.counts[cell] == 1 {
		s.occupied[int(cell)/s.Topo.Width] |= 1 << uint(int(cell)%s.Topo.Width)
	}
}

func (s *State) vacate(cell uint16) {
	s.counts[cell]--
	if s.counts[cell] == 0 {
		s.occupied[int(cell)/s.Topo.Width] &^= 1 << uint(int(cell)%s.Topo.Width)
	}
}

// trailingZeros is math/bits.TrailingZeros64, named locally so the bit-walking
// loops read as board scanning rather than as arithmetic.
func trailingZeros(v uint64) int { return bits.TrailingZeros64(v) }

// Passable returns the squares a snake may move into on the next turn.
//
// It is Occupied with each living snake's tail released, because a tail vacates
// as its snake moves. The release is one count, not one square, which is the
// whole point: a snake that ate last turn carries its tail twice, so releasing
// one leaves the square blocked and the snake behind it does not walk into a
// body that has not moved.
//
// The result is owned by the state and is rebuilt by the next call.
func (s *State) Passable() board.Bitset {
	copy(s.passableCounts, s.counts)
	for i := range s.Snakes {
		sn := &s.Snakes[i]
		if !sn.Alive() {
			continue
		}
		if tail := sn.Tail(); s.passableCounts[tail] > 0 {
			s.passableCounts[tail]--
		}
	}

	s.passable.Reset()
	for cell, n := range s.passableCounts {
		if n > 0 {
			s.passable[cell/s.Topo.Width] |= 1 << uint(cell%s.Topo.Width)
		}
	}
	return s.passable
}

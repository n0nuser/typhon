package search

import (
	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
)

// Risk is how a head-to-head on a square would go for us.
type Risk uint8

// Head-to-head outcomes, ordered best to worst so they compare.
const (
	// NoContest means no rival head can reach the square next turn.
	NoContest Risk = iota
	// WinsContest means every rival that can reach it is strictly shorter.
	WinsContest
	// TiesContest means an equally long rival can reach it and both would die.
	TiesContest
	// LosesContest means a longer rival can reach it and we would die.
	LosesContest
)

// SafeMove returns the best move available from one ply of looking, and reports
// whether every move loses.
//
// This is the answer the bot holds before it starts searching, so that every
// path out of a turn already has a move in hand. Nothing is allowed to delegate
// safety to the search: the search improves on this answer, it is not permitted
// to be the reason there isn't one.
//
// The ranking is: a move that does not kill us beats one that does; among
// survivable moves, fewer ways to lose a head-to-head beats more; among equal
// ones, more reachable space beats less. When nothing survives, the moves are
// ranked by how many rivals contest the square - which is the whole point of
// counting them rather than only recording the worst outcome. One rival on the
// square is a coin flip and three is a certainty, and the two are worth very
// different amounts when they are all that is left.
func SafeMove(s *rules.State, me int) (board.Direction, bool) {
	return safeMove(s, me, s.Topo.NewFillScratch(), s.Topo.NewBitset())
}

// safeMove is SafeMove over scratch the caller already owns, so that a search
// does not allocate once per turn for something it computes before every one.
func safeMove(s *rules.State, me int, fill *board.FillScratch, scratch board.Bitset) (board.Direction, bool) {
	self := &s.Snakes[me]
	if !self.Alive() {
		return board.Up, true
	}

	topo := s.Topo
	head := topo.At(int(self.Head()))
	blocked := s.Passable()

	best := board.Up
	bestRank := worstRank
	anySafe := false

	for _, d := range board.Directions {
		next, inside := topo.Step(head, d)
		if !inside || blocked.Has(next) {
			continue
		}

		risk, contesters := Contest(s, me, next)

		scratch.CopyFrom(blocked)
		scratch.Clear(next)
		space := topo.ReachableCount(scratch, next, fill)

		r := rank{
			fatal:      risk == TiesContest || risk == LosesContest,
			risk:       risk,
			contesters: contesters,
			// A pocket smaller than we are is a self-collision that has not
			// happened yet, so space is capped at our length: beyond that,
			// more room is not more survival.
			space: min(space, self.Len()),
		}
		if !r.fatal {
			anySafe = true
		}
		if r.better(bestRank) {
			bestRank, best = r, d
		}
	}

	if bestRank == worstRank {
		// Every direction is a wall or a body. Something still has to be
		// returned, and the engine will move us up if we return nothing.
		return board.Up, true
	}
	return best, !anySafe
}

// Contest reports the worst head-to-head outcome on a square and how many
// rivals could reach it.
func Contest(s *rules.State, me int, target board.Point) (Risk, int) {
	self := &s.Snakes[me]
	worst := NoContest
	contesters := 0

	for i := range s.Snakes {
		other := &s.Snakes[i]
		if i == me || !other.Alive() {
			continue
		}
		if s.Topo.Distance(s.Topo.At(int(other.Head())), target) != 1 {
			continue
		}
		contesters++

		var risk Risk
		switch {
		case other.Len() > self.Len():
			risk = LosesContest
		case other.Len() == self.Len():
			risk = TiesContest
		default:
			risk = WinsContest
		}
		if risk > worst {
			worst = risk
		}
	}
	return worst, contesters
}

// rank orders one-ply moves.
type rank struct {
	fatal      bool
	risk       Risk
	contesters int
	space      int
}

// worstRank is the sentinel for "no move considered yet".
var worstRank = rank{fatal: true, risk: LosesContest, contesters: 1 << 30, space: -1}

// better reports whether r is preferable to other.
func (r rank) better(other rank) bool {
	if r.fatal != other.fatal {
		return !r.fatal
	}
	if r.contesters != other.contesters {
		return r.contesters < other.contesters
	}
	if r.risk != other.risk {
		return r.risk < other.risk
	}
	return r.space > other.space
}

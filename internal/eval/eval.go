// Package eval scores a board from one snake's point of view.
//
// Every term is weighted, and a weight of zero switches its term off without
// paying for it. That is what lets the tournament harness vary exactly one
// thing between two arms, which is the only way any of the numbers in
// BENCHMARK.md mean anything.
//
// The weights are hand-set and untuned. With two hundred games needed per
// honest comparison there is budget for a handful of structural questions -
// does the Voronoi term pay, does tail reachability pay - and none at all for
// searching weight space, so no weight here should be read as measured.
package eval

import (
	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
)

// Score is a position's value in centipawn-like units, from one snake's view.
type Score int32

// The bounds of the scale.
//
// Win and Loss sit far enough outside any heuristic sum that no combination of
// terms can reach them, so a search can tell "winning" from "very good".
const (
	// Win is the value of a position where every rival is gone.
	Win Score = 1 << 20
	// Loss is the value of a position where we are gone.
	Loss Score = -Win
	// Unknown is the zero value, used where no score has been computed.
	Unknown Score = 0
)

// Weights sets what the evaluation cares about. One field per term, so two
// harness arms differ in exactly one number.
type Weights struct {
	// Voronoi values squares we reach before any rival does. It is the main
	// space signal: a flood fill says how much room we have alone, this says
	// how much we have against the other snakes.
	Voronoi Score
	// Space values raw reachable squares, and punishes a pocket smaller than
	// we are - a self-collision that has not happened yet.
	Space Score
	// TailReach values still being able to path to our own tail, which is what
	// makes survival in a tight space possible at all.
	TailReach Score
	// Length values being longer than the longest rival, which decides every
	// head-to-head.
	Length Score
	// Food values closing on food, scaled by how hungry we are.
	Food Score
	// Centre values the middle of the board, which keeps escape routes open.
	Centre Score
	// Confine values taking space away from the nearest rival.
	Confine Score
}

// Default returns the weights the bot plays with.
//
// Untuned, and deliberately so. They encode an ordering rather than a
// measurement: staying alive beats having room, having room beats being long,
// and being long beats standing anywhere in particular.
func Default() Weights {
	return Weights{
		Voronoi:   10,
		Space:     6,
		TailReach: 40,
		Length:    30,
		Food:      4,
		Centre:    1,
		Confine:   6,
	}
}

// Evaluator scores positions. It owns its scratch space, so one belongs to one
// search and is never shared between goroutines.
type Evaluator struct {
	Weights Weights

	topo    board.Topology
	fill    *board.FillScratch
	voronoi *board.VoronoiScratch
	sources []board.Source
	blocked board.Bitset
}

// New returns an evaluator sized for a board.
func New(topo board.Topology, w Weights) *Evaluator {
	return &Evaluator{
		Weights: w,
		topo:    topo,
		fill:    topo.NewFillScratch(),
		voronoi: topo.NewVoronoiScratch(rules.MaxSnakes),
		sources: make([]board.Source, 0, rules.MaxSnakes),
		blocked: topo.NewBitset(),
	}
}

// Evaluate scores the state from snake me's point of view, at the given ply.
//
// Terminal scores carry the ply so that the search prefers dying later and
// winning sooner: a loss ten moves away is worth more than a loss now, which is
// what makes a bot play on instead of giving up when every line looks lost.
func (e *Evaluator) Evaluate(s *rules.State, me, ply int) Score {
	self := &s.Snakes[me]

	if !self.Alive() {
		// A mutual kill is scored a hair above a plain loss. It is still a
		// loss - a tie eliminates us exactly as dead - but where both are
		// available, taking the rival along is worth the fraction.
		if s.AliveCount() == 0 {
			return Loss + Score(ply) + 1
		}
		return Loss + Score(ply)
	}
	if s.AliveCount() == 1 {
		return Win - Score(ply)
	}

	w := e.Weights
	head := e.topo.At(int(self.Head()))

	// Our own head is occupied, and a flood fill refuses to start on a blocked
	// square, so it has to be freed first or the reachable region comes back
	// empty every single time. That bug is invisible from the outside: the
	// space term still produces a number, and the number is always the same
	// wrong one.
	e.blocked.CopyFrom(s.Passable())
	e.blocked.Clear(head)
	blocked := e.blocked

	var score Score

	reach := e.topo.Reach(blocked, head, e.fill)
	space := reach.Count()

	if w.Space != 0 {
		// Room beyond our own length is worth little; room short of it is a
		// countdown. The asymmetry is the point.
		if space < self.Len() {
			score -= w.Space * Score(self.Len()-space) * 4
		} else {
			score += w.Space * Score(min(space, self.Len()*2))
		}
	}

	if w.TailReach != 0 && s.Variant != rules.Constrictor {
		// Meaningless in constrictor, where nothing ever vacates.
		if reach.Has(e.topo.At(int(self.Tail()))) {
			score += w.TailReach
		}
	}

	if w.Voronoi != 0 || w.Confine != 0 {
		score += e.control(s, me)
	}

	if w.Length != 0 {
		longest := 0
		for i := range s.Snakes {
			if other := &s.Snakes[i]; i != me && other.Alive() && other.Len() > longest {
				longest = other.Len()
			}
		}
		score += w.Length * Score(self.Len()-longest)
	}

	if w.Food != 0 && s.Variant != rules.Constrictor {
		score += e.hunger(s, self, head)
	}

	if w.Centre != 0 && !e.topo.Wrapped {
		// A torus has no middle, so the term is off there rather than wrong.
		centre := board.Point{X: e.topo.Width / 2, Y: e.topo.Height / 2}
		score -= w.Centre * Score(e.topo.Distance(head, centre))
	}

	return score
}

// control scores the Voronoi partition: squares we reach first, less the
// squares the best-placed rival reaches first.
func (e *Evaluator) control(s *rules.State, me int) Score {
	e.sources = e.sources[:0]
	mine := -1
	for i := range s.Snakes {
		sn := &s.Snakes[i]
		if !sn.Alive() {
			continue
		}
		if i == me {
			mine = len(e.sources)
		}
		e.sources = append(e.sources, board.Source{
			Head:   e.topo.At(int(sn.Head())),
			Length: sn.Len(),
		})
	}
	if mine < 0 || len(e.sources) < 2 {
		return 0
	}

	own := s.Passable()
	partition := e.topo.Voronoi(own, e.sources, e.voronoi)

	ours := partition.CountFor(mine)
	best := 0
	for i := range e.sources {
		if i == mine {
			continue
		}
		if n := partition.CountFor(i); n > best {
			best = n
		}
	}

	return e.Weights.Voronoi*Score(ours) - e.Weights.Confine*Score(best)
}

// hunger scores distance to the nearest food, scaled by how badly we need it.
//
// A snake at full health should not chase food across the board; a snake at ten
// should think about nothing else. Scaling by the health deficit rather than
// switching at a threshold avoids the predecessor's problem of a rule that
// never fired.
func (e *Evaluator) hunger(s *rules.State, self *rules.Snake, head board.Point) Score {
	nearest := -1
	for y := range s.Food {
		row := s.Food[y]
		for row != 0 {
			x := trailingZeros(row)
			row &= row - 1
			if d := e.topo.Distance(head, board.Point{X: x, Y: y}); nearest < 0 || d < nearest {
				nearest = d
			}
		}
	}
	if nearest < 0 {
		return 0
	}

	deficit := rules.MaxHealth - self.Health
	return -e.Weights.Food * Score(nearest) * Score(deficit) / rules.MaxHealth
}

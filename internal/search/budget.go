// Package search picks a move by looking ahead, under a budget it is not
// allowed to exceed.
//
// Battlesnake is a simultaneous-move game: every snake commits before anyone
// sees the result. Modelling that as alternating turns - we move, then the
// opponent chooses knowing what we did - makes the opponent omniscient and the
// evaluation pathologically pessimistic, and a bot built on it refuses moves
// that are perfectly safe. So a node here advances the board only once every
// snake has committed, and collisions resolve against the positions everyone
// moved to.
//
// The opponents are still assumed to play the move worst for us, which is
// paranoid and conservative. In a game where one mistake is terminal that is
// the right bias.
package search

import "time"

// Budget bounds a search. Either limit may be set; the first to bite wins.
//
// The two modes exist for two different jobs. In play the deadline is what
// matters, because the engine counts wall-clock and a late answer is no answer.
// In the tournament harness the node count is what matters, because it makes a
// game bit-for-bit reproducible from its seed: the same position searched twice
// returns the same move, on a loaded machine or an idle one. Without that,
// every A/B comparison measures scheduler noise alongside the change, and 200
// games are not enough to see through it.
type Budget struct {
	// Deadline stops the search at a wall-clock time. Zero means no deadline.
	Deadline time.Time
	// Nodes stops the search after this many nodes. Zero means no node limit.
	Nodes int64
	// MaxDepth stops the search after this many plies. Zero means no limit
	// beyond the other two.
	MaxDepth int
}

// clockCheckInterval is how many nodes pass between readings of the clock.
//
// It was 1024, chosen by the usual chess-engine reasoning that time.Now is too
// expensive to call per node. That reasoning does not transfer: a node here
// rebuilds an occupancy map and hashes a position, and a leaf evaluates a
// Voronoi partition, so nodes cost a microsecond and up rather than tens of
// nanoseconds. A measured 2ms budget overran to 67ms.
//
// At 64 the clock costs well under a percent of a node and the overshoot is
// bounded by sixty-four of them. Returning late is scored as returning nothing,
// so this is the one number that is not allowed to be approximately right.
const clockCheckInterval = 64

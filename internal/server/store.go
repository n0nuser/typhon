package server

import (
	"sync"
	"time"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/search"
)

// seedOverhead is what the round trip is assumed to cost before anything has
// been measured.
//
// Conservative on purpose: the first turn of a game has no previous latency to
// learn from, and guessing low there means the very first move is the one that
// arrives late.
const seedOverhead = 50 * time.Millisecond

// safetyMargin is held back from every turn budget on top of the measured
// overhead and overshoot, for whatever this turn does that the last one did
// not.
const safetyMargin = 25 * time.Millisecond

// budgetCeiling is the most of the engine's timeout the search may ever be
// given, whatever the estimates say.
//
// The estimates cannot see the whole cost of a turn. Our clock starts on the
// first line of the handler, so the TCP accept, the HTTP parse and the wait for
// the Go scheduler to reach our goroutine are invisible to us and only surface
// as `overhead` on the *following* turn. On a CPU-throttled instance that gap
// is not small and it is not stable: one live game had it oscillating between
// 48ms and 122ms, and the turn that died had budget 327ms, a measured 397ms of
// thinking, and a round trip the engine recorded as the full 500ms - it lost
// because the spike was bigger than the peak the estimate had decayed to.
//
// A ceiling bounds that exposure without having to predict it. The estimates
// still shrink the budget when they can see a reason to; this stops the budget
// growing back to where a single spike is fatal.
//
// 0.60 of a 500ms turn is 300ms. Measured live at 330ms the bot did about
// 75,000 nodes and reached depth 8; iterative deepening costs roughly 4x per
// ply, so the ceiling gives up well under half a ply. A late answer is not a
// worse move, it is no move: the engine plays `getDefaultMove`, which continues
// in the direction the neck implies, straight into whatever is there.
const budgetCeiling = 0.60

// overheadDecay and overshootDecay set how fast a peak is forgotten, as the
// reciprocal of the weight given to each new sample.
//
// Overhead decays far more slowly than overshoot because its spikes recur. In
// the game that died they arrived every twenty turns or so, and a peak forgotten
// in thirty is a peak that is never holding when it is needed.
const (
	overheadDecay  = 64
	overshootDecay = 16
)

// seedOvershoot is the first turn's guess at what a turn costs after the
// search has already stopped.
//
// Conservative for the same reason as seedOverhead, and it matters more: the
// instance that needs this term is a cold one, and the first turn of the first
// game is the turn it is least able to answer quickly.
const seedOvershoot = 50 * time.Millisecond

// game is the per-snake scratch space for one match.
//
// Keyed by game id *and* snake id, because one server can back several snakes
// in the same match - which is exactly how a one-against-three test is set up.
// Keying on the game alone would have those snakes share a search table and a
// latency estimate, and quietly make each one's move depend on the others'.
type game struct {
	mu       sync.Mutex
	searcher *search.Searcher
	topo     board.Topology

	// overhead is an EWMA of the round trip minus our own think time.
	overhead time.Duration
	// overshoot is an EWMA of the time a turn spent past the budget it was
	// given - the encode, the write, and on a CPU-throttled instance the
	// scheduler freezing the process mid-encode until its next period.
	//
	// Without this term the loop closes on the wrong number. The search stops
	// on time, `overhead` correctly excludes our own compute, and the reply
	// still lands late, because nothing subtracted the gap between the search
	// stopping and the bytes leaving. On an idle machine that gap is under a
	// millisecond and the omission is invisible; on Render's 0.1-CPU free tier
	// it is tens of milliseconds and every turn is late by about that much.
	overshoot time.Duration
	// ceiling is the fraction of the timeout this game may spend searching.
	ceiling  float64
	lastSeen time.Time

	turns     int
	depthSum  int
	maxDepth  int
	nodes     int64
	fallbacks int
	aborted   int
	allLosing int
	overruns  int
	maxThink  time.Duration
}

// noteTurn folds one turn's measurements into the game's running estimate.
//
// The engine reports the round trip it measured for our *previous* reply, and
// that measurement includes our own think time. Subtracting the raw figure from
// the budget therefore charges our compute twice and ratchets the budget down
// turn after turn. What is wanted is the part we cannot see - the network and
// the engine's own handling - which is the reported latency less what we know
// we spent.
func (g *game) noteTurn(engineLatency, thought, budget time.Duration) {
	if engineLatency <= 0 {
		return
	}

	// A turn that did not spend its budget teaches nothing about where the time
	// went, and under a peak-hold estimate it does active harm.
	//
	// The search runs to its deadline on every real turn, so `thought` under
	// half the budget means the turn was trivial - the game was already over,
	// or we were the last snake asked to move. Those return in microseconds,
	// which makes `engineLatency - thought` charge the entire round trip as
	// overhead: one live game logged 472ms that way. A mean would have shrugged
	// it off; a peak holds it, and one such sample drops the budget from 395ms
	// to the 1ms floor for the thirty turns it takes to decay.
	if thought*2 < budget {
		return
	}

	over := thought - budget
	if over < 0 {
		over = 0
	}
	g.overshoot = peakHold(g.overshoot, over, overshootDecay)

	overhead := engineLatency - thought
	if overhead < 0 {
		overhead = 0
	}
	g.overhead = peakHold(g.overhead, overhead, overheadDecay)
}

// peakHold folds one sample into a running estimate that rises at once and
// falls slowly.
//
// A mean is the wrong estimator for a deadline, and the live logs say why. The
// cost a turn pays after its search stops is **bimodal** on a throttled
// instance: measured over one game it was either under a millisecond or
// 76-85ms, with almost nothing in between, because the scheduler's freeze
// either lands after the deadline or it does not. An average of that sits near
// 40ms and is wrong both ways - too generous on the turns that freeze, so the
// reply is late, and too cautious on the turns that do not, so the search is
// short-changed for nothing.
//
// What a deadline needs is an upper bound. Rising immediately means one late
// turn is enough to learn from; decaying slowly means a single outlier does not
// pin the budget for the rest of the game.
func peakHold(current, sample time.Duration, decay int) time.Duration {
	if sample > current {
		return sample
	}
	n := time.Duration(decay)
	return (current*(n-1) + sample) / n
}

// budget returns how long the search may run this turn.
func (g *game) budget(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	overhead := g.overhead
	if overhead == 0 {
		overhead = seedOverhead
	}
	overshoot := g.overshoot
	if overshoot == 0 {
		overshoot = seedOvershoot
	}
	budget := timeout - overhead - overshoot - safetyMargin
	frac := g.ceiling
	if frac <= 0 || frac > 1 {
		frac = budgetCeiling
	}
	if ceiling := time.Duration(float64(timeout) * frac); budget > ceiling {
		budget = ceiling
	}
	if budget < time.Millisecond {
		// Something is badly wrong with the estimate, but a turn still has to
		// be answered, and the held fallback answers it.
		budget = time.Millisecond
	}
	return budget
}

// store holds one game struct per snake per match.
//
// It is bounded by eviction on /end and by a sweep for the games that never
// send one, which the engine does whenever a match is abandoned. An unbounded
// map keyed by game id is a leak with a timer on it.
type store struct {
	mu      sync.Mutex
	games   map[string]*game
	ttl     time.Duration
	ceiling float64
	now     func() time.Time
}

func newStore(ttl time.Duration, ceiling float64) *store {
	if ceiling <= 0 || ceiling > 1 {
		ceiling = budgetCeiling
	}
	return &store{games: make(map[string]*game), ttl: ttl, ceiling: ceiling, now: time.Now}
}

// get returns the game for a key, creating it if this is the first sight of it.
func (s *store) get(key string) *game {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	g, ok := s.games[key]
	if !ok {
		g = &game{ceiling: s.ceiling}
		s.games[key] = g
	}
	g.lastSeen = now
	s.sweepLocked(now)
	return g
}

// end drops a game and returns it for one last log line.
func (s *store) end(key string) *game {
	s.mu.Lock()
	defer s.mu.Unlock()

	g := s.games[key]
	delete(s.games, key)
	return g
}

// count reports how many games are held, for tests and diagnostics.
func (s *store) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.games)
}

// sweepLocked drops games that stopped talking to us.
func (s *store) sweepLocked(now time.Time) {
	for key, g := range s.games {
		if now.Sub(g.lastSeen) > s.ttl {
			delete(s.games, key)
		}
	}
}

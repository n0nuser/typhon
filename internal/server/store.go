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
// overhead, for the encode, the write and whatever the network does that the
// last turn did not.
const safetyMargin = 25 * time.Millisecond

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
func (g *game) noteTurn(engineLatency, thought time.Duration) {
	if engineLatency <= 0 {
		return
	}
	overhead := engineLatency - thought
	if overhead < 0 {
		overhead = 0
	}
	if g.overhead == 0 {
		g.overhead = overhead
		return
	}
	g.overhead = (g.overhead*3 + overhead) / 4
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
	budget := timeout - overhead - safetyMargin
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
	mu    sync.Mutex
	games map[string]*game
	ttl   time.Duration
	now   func() time.Time
}

func newStore(ttl time.Duration) *store {
	return &store{games: make(map[string]*game), ttl: ttl, now: time.Now}
}

// get returns the game for a key, creating it if this is the first sight of it.
func (s *store) get(key string) *game {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	g, ok := s.games[key]
	if !ok {
		g = &game{}
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

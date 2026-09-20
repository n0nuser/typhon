package search

import (
	"time"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/eval"
	"github.com/n0nuser/typhon/internal/rules"
)

// Config sets how the search behaves. One field per structural question, so the
// harness can vary exactly one of them between two arms.
type Config struct {
	// Weights is the evaluation this search optimises.
	Weights eval.Weights
	// Opponents is how many rivals are searched properly. The rest are
	// advanced with a cheap greedy move.
	//
	// Three rivals searched exhaustively is twenty-seven branches for every
	// one of ours, most of them spent on snakes that cannot reach us inside
	// the horizon. Depth against the snake contesting our space is worth more
	// than breadth across the ones that are not.
	Opponents int
	// UseTable enables the transposition table.
	UseTable bool
	// TableBits sizes the table as 1<<TableBits entries.
	TableBits uint
}

// DefaultConfig returns the configuration the bot plays with.
func DefaultConfig() Config {
	return Config{
		Weights:   eval.Default(),
		Opponents: 2,
		UseTable:  true,
		TableBits: 20,
	}
}

// Result is what a search found.
type Result struct {
	// Move is the move to play. It is always set, even when the search was
	// stopped before it finished anything.
	Move board.Direction
	// Score is the value of Move at Depth.
	Score eval.Score
	// Depth is the deepest iteration that completed. A depth of zero means
	// nothing completed and Move is the one-ply fallback.
	Depth int
	// Nodes counts positions advanced.
	Nodes int64
	// Aborted reports whether an iteration was cut short by the budget.
	Aborted bool
	// AllLosing reports that every legal move was a loss at the same distance,
	// so the move was chosen by how many rivals contest it rather than by
	// score.
	AllLosing bool
}

// Searcher searches one game. It owns mutable scratch and a transposition
// table, so it belongs to one goroutine and one game: sharing one across
// parallel games makes results depend on the order they interleave, which
// silently destroys the reproducibility every benchmark rests on.
type Searcher struct {
	cfg  Config
	eval *eval.Evaluator
	topo board.Topology
	tt   *table
	zob  *zobrist

	me      int
	state   *rules.State
	nodes   int64
	limit   int64
	dead    time.Time
	stopped bool

	actors  []int
	killers [][2]board.Direction

	safeFill    *board.FillScratch
	safeScratch board.Bitset
}

// New returns a searcher for a board.
func New(topo board.Topology, cfg Config) *Searcher {
	s := &Searcher{
		cfg:         cfg,
		eval:        eval.New(topo, cfg.Weights),
		topo:        topo,
		zob:         newZobrist(topo),
		actors:      make([]int, 0, rules.MaxSnakes),
		killers:     make([][2]board.Direction, 64),
		safeFill:    topo.NewFillScratch(),
		safeScratch: topo.NewBitset(),
	}
	if cfg.UseTable {
		s.tt = newTable(cfg.TableBits)
	}
	return s
}

// Search returns the best move it can find for snake me within the budget.
//
// It deepens one ply at a time and keeps the best move of the last *completed*
// iteration, so it can be stopped at any moment and still answer. A move from a
// half-finished depth is never promoted: a partial search has looked at some of
// our options and not others, which is worse than not searching at all.
func (s *Searcher) Search(state *rules.State, me int, budget Budget) Result {
	s.me = me
	s.state = state
	s.nodes = 0
	s.limit = budget.Nodes
	s.dead = budget.Deadline
	s.stopped = false
	for i := range s.killers {
		s.killers[i] = [2]board.Direction{}
	}
	if s.tt != nil {
		s.tt.newSearch()
	}

	// The fallback is computed first and held, so that every path out of this
	// function already has an answer. Nothing below is allowed to be the reason
	// a move is missing.
	fallback, allLosing := safeMove(state, me, s.safeFill, s.safeScratch)
	result := Result{Move: fallback, AllLosing: allLosing}

	if !state.Snakes[me].Alive() || state.Over() {
		return result
	}

	maxDepth := budget.MaxDepth
	if maxDepth <= 0 {
		maxDepth = len(s.killers) - 1
	}

	best := fallback
	for depth := 1; depth <= maxDepth; depth++ {
		move, score, ok := s.rootSearch(depth, best)
		if !ok {
			result.Aborted = true
			break
		}
		best = move
		result.Move = move
		result.Score = score
		result.Depth = depth

		// A forced result will not change with more depth, so spending the
		// rest of the budget confirming it is waste.
		if score >= eval.Win-Score(maxDepth) || score <= eval.Loss+Score(maxDepth) {
			break
		}
	}

	result.Nodes = s.nodes

	// When every move loses at the same distance the score cannot separate
	// them, and the choice falls through to whatever order the moves happened
	// to be tried in. Counting contesters is what ranks them: one rival on the
	// square is a coin flip, three is a certainty.
	if result.Depth == 0 || allLosing {
		result.Move = fallback
		result.AllLosing = allLosing
	}

	s.state = nil
	return result
}

// Score is eval.Score, re-exported for readability in this package's bounds.
type Score = eval.Score

// rootSearch runs one full-depth iteration and reports whether it finished.
func (s *Searcher) rootSearch(depth int, first board.Direction) (board.Direction, Score, bool) {
	st := s.state
	actors := s.chooseActors(st)

	var moves [rules.MaxSnakes]board.Direction
	alpha, beta := eval.Loss*2, eval.Win*2
	bestMove := first
	bestScore := alpha

	var order [4]board.Direction
	n := s.orderMoves(st, s.me, 0, first, &order)
	for _, d := range order[:n] {
		moves[s.me] = d
		score := s.opponentLayer(st, actors, 1, depth, 0, alpha, beta, &moves)
		if s.stopped {
			return bestMove, bestScore, false
		}
		if score > bestScore {
			bestScore, bestMove = score, d
		}
		if bestScore > alpha {
			alpha = bestScore
		}
	}
	return bestMove, bestScore, true
}

// opponentLayer has each searched opponent commit a move, minimising our score.
//
// The state does not advance until every actor has chosen. That is what makes
// this a simultaneous-move search rather than an alternating one, and it is why
// a head-to-head resolves against where both snakes moved to rather than
// against where one of them used to be.
func (s *Searcher) opponentLayer(st *rules.State, actors []int, i, depth, ply int,
	alpha, beta Score, moves *[rules.MaxSnakes]board.Direction,
) Score {
	if i >= len(actors) {
		return s.advance(st, actors, depth, ply, alpha, beta, moves)
	}

	actor := actors[i]
	best := eval.Win * 2
	var order [4]board.Direction
	n := s.orderMoves(st, actor, ply, board.Up, &order)
	for _, d := range order[:n] {
		moves[actor] = d
		score := s.opponentLayer(st, actors, i+1, depth, ply, alpha, beta, moves)
		if s.stopped {
			return best
		}
		if score < best {
			best = score
		}
		if best < beta {
			beta = best
		}
		if alpha >= beta {
			break
		}
	}
	return best
}

// advance applies the committed joint move and searches the position it leads
// to.
func (s *Searcher) advance(st *rules.State, actors []int, depth, ply int,
	alpha, beta Score, moves *[rules.MaxSnakes]board.Direction,
) Score {
	s.fillUnsearched(st, actors, moves)

	undo := st.Apply(moves)
	score := s.node(st, depth-1, ply+1, alpha, beta)
	st.Unapply(undo)
	return score
}

// node searches one position: our move, then the opponents', then recurse.
func (s *Searcher) node(st *rules.State, depth, ply int, alpha, beta Score) Score {
	s.nodes++
	if s.nodes&(clockCheckInterval-1) == 0 && s.overBudget() {
		s.stopped = true
		return alpha
	}
	if s.stopped {
		return alpha
	}

	if depth <= 0 || st.Over() || !st.Snakes[s.me].Alive() {
		return s.eval.Evaluate(st, s.me, ply)
	}

	var key uint64
	if s.tt != nil {
		key = s.zob.hash(st)
		if score, move, ok := s.tt.probe(key, depth); ok {
			_ = move
			return score
		}
	}

	actors := s.chooseActors(st)
	var moves [rules.MaxSnakes]board.Direction

	ttMove := board.Up
	if s.tt != nil {
		if _, move, ok := s.tt.probeMove(key); ok {
			ttMove = move
		}
	}

	best := eval.Loss * 2
	bestMove := ttMove
	localAlpha := alpha
	var order [4]board.Direction
	n := s.orderMoves(st, s.me, ply, ttMove, &order)
	for _, d := range order[:n] {
		moves[s.me] = d
		score := s.opponentLayer(st, actors, 1, depth, ply, localAlpha, beta, &moves)
		if s.stopped {
			return best
		}
		if score > best {
			best, bestMove = score, d
		}
		if best > localAlpha {
			localAlpha = best
		}
		if localAlpha >= beta {
			s.recordKiller(ply, d)
			break
		}
	}

	if s.tt != nil && !s.stopped {
		// Only exact scores are stored. A bound stored from inside a window
		// can be re-read under a different window and give a different answer
		// for the same position, which is a real source of search bugs and
		// costs more to get right than it is worth at this depth.
		if best > alpha && best < beta {
			s.tt.store(key, best, depth, bestMove)
		} else {
			s.tt.storeMove(key, bestMove)
		}
	}
	return best
}

func (s *Searcher) overBudget() bool {
	if s.limit > 0 && s.nodes >= s.limit {
		return true
	}
	if !s.dead.IsZero() && time.Now().After(s.dead) {
		return true
	}
	return false
}

func (s *Searcher) recordKiller(ply int, d board.Direction) {
	if ply >= len(s.killers) {
		return
	}
	if s.killers[ply][0] != d {
		s.killers[ply][1] = s.killers[ply][0]
		s.killers[ply][0] = d
	}
}

// chooseActors returns us plus the rivals worth searching properly, nearest
// first.
//
// Nearest by the board's own metric, which on a torus is not Manhattan. A snake
// on the far edge of a wrapped board is one step away, and a search that ranked
// it by raw coordinate difference would ignore the one snake about to take our
// square.
func (s *Searcher) chooseActors(st *rules.State) []int {
	s.actors = s.actors[:0]
	s.actors = append(s.actors, s.me)

	head := s.topo.At(int(st.Snakes[s.me].Head()))

	type rival struct {
		index int
		dist  int
	}
	var rivals [rules.MaxSnakes]rival
	n := 0
	for i := range st.Snakes {
		other := &st.Snakes[i]
		if i == s.me || !other.Alive() {
			continue
		}
		rivals[n] = rival{index: i, dist: s.topo.Distance(head, s.topo.At(int(other.Head())))}
		n++
	}

	// Insertion sort by distance, ties by index, so the choice never depends
	// on slice order or on map iteration.
	for i := 1; i < n; i++ {
		v := rivals[i]
		j := i - 1
		for j >= 0 && (rivals[j].dist > v.dist || (rivals[j].dist == v.dist && rivals[j].index > v.index)) {
			rivals[j+1] = rivals[j]
			j--
		}
		rivals[j+1] = v
	}

	limit := s.cfg.Opponents
	if limit > n {
		limit = n
	}
	for i := range limit {
		s.actors = append(s.actors, rivals[i].index)
	}
	return s.actors
}

// fillUnsearched gives the rivals the search is not modelling a cheap move, so
// that the board still advances legally.
func (s *Searcher) fillUnsearched(st *rules.State, actors []int, moves *[rules.MaxSnakes]board.Direction) {
	for i := range st.Snakes {
		if !st.Snakes[i].Alive() {
			continue
		}
		searched := false
		for _, a := range actors {
			if a == i {
				searched = true
				break
			}
		}
		if searched {
			continue
		}
		moves[i] = greedyMove(st, i)
	}
}

// orderMoves writes the four directions into dst, most promising first, and
// returns how many it wrote.
//
// It fills a caller-owned array rather than returning a slice. Returning
// `out[:n]` from a local array escapes it to the heap, and this is called once
// per actor per node: it was eleven thousand allocations a turn, which is a GC
// pause waiting to land inside a 400ms budget.
//
// Ordering is most of what makes alpha-beta work, because a cutoff on the first
// move of a node prunes the other three subtrees entirely. The order is the
// previous iteration's choice, then the moves that caused a cutoff at this ply
// before, then the board's fixed order - which is also the tie-break, and is
// fixed rather than random because a bot that picks randomly between equal
// moves wanders, and wandering fills in its own escape routes.
func (s *Searcher) orderMoves(st *rules.State, snake, ply int, first board.Direction,
	dst *[4]board.Direction,
) int {
	var seen [4]bool
	n := 0

	if !seen[first] {
		seen[first] = true
		dst[n] = first
		n++
	}
	if ply < len(s.killers) {
		for _, k := range s.killers[ply] {
			if !seen[k] {
				seen[k] = true
				dst[n] = k
				n++
			}
		}
	}

	// Survivable moves before fatal ones: a move into a wall is a leaf whose
	// value is already known, and looking at it first wastes the cutoff.
	blocked := st.Passable()
	head := s.topo.At(int(st.Snakes[snake].Head()))
	for _, d := range board.Directions {
		if seen[d] {
			continue
		}
		if next, ok := s.topo.Step(head, d); ok && !blocked.Has(next) {
			seen[d] = true
			dst[n] = d
			n++
		}
	}
	for _, d := range board.Directions {
		if !seen[d] {
			seen[d] = true
			dst[n] = d
			n++
		}
	}
	return n
}

// greedyMove picks a plausible move for a snake the search is not modelling.
func greedyMove(st *rules.State, i int) board.Direction {
	blocked := st.Passable()
	head := st.Topo.At(int(st.Snakes[i].Head()))
	for _, d := range board.Directions {
		if next, ok := st.Topo.Step(head, d); ok && !blocked.Has(next) {
			return d
		}
	}
	return board.Up
}

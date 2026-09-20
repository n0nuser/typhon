package main

import (
	"fmt"

	official "github.com/BattlesnakeOfficial/rules"
	"github.com/BattlesnakeOfficial/rules/maps"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
	"github.com/n0nuser/typhon/internal/search"
)

// outcome is how one game ended.
type outcome int

const (
	// draw means nobody was left standing.
	draw outcome = iota
	// armAWon means the snake configured from arm A survived.
	armAWon
	// armBWon means the snake configured from arm B survived.
	armBWon
)

// counters are the per-path counts a run prints.
//
// The predecessor once measured a whole batch of games against a feature whose
// threshold was never crossed. These exist so that a claim about a component
// can be checked against how often it was even reached, before anyone reads
// the win column.
type counters struct {
	turns      int
	nodes      int64
	depthSum   int
	maxDepth   int
	fallbacks  int
	allLosing  int
	aborted    int
	firstDeath int
}

func (c *counters) add(o counters) {
	c.turns += o.turns
	c.nodes += o.nodes
	c.depthSum += o.depthSum
	if o.maxDepth > c.maxDepth {
		c.maxDepth = o.maxDepth
	}
	c.fallbacks += o.fallbacks
	c.allLosing += o.allLosing
	c.aborted += o.aborted
	c.firstDeath += o.firstDeath
}

// gameResult is one played game.
type gameResult struct {
	seed int
	// aFirst records which slot arm A played, so that per-slot rates can be
	// reported separately. The predecessor always put the arm under test in
	// slot one and never checked whether the slot itself was worth anything;
	// two identical bots went 29-23-8, so it was.
	aFirst  bool
	outcome outcome
	turns   int
	perArm  [2]counters
	err     error
}

// playGame runs one game in process and returns who won.
//
// It drives the official rules exactly as the CLI does - SetupBoard, then
// PreUpdateBoard, Execute, PostUpdateBoard every turn - so the games are the
// same games, without the HTTP round trip, the port juggling or the need to
// grep decision counts back out of a log.
func playGame(cfg runConfig, arms [2]arm, seed int, aFirst bool) gameResult {
	res := gameResult{seed: seed, aFirst: aFirst}

	settings := official.NewSettingsWithParams(
		official.ParamFoodSpawnChance, fmt.Sprint(cfg.foodSpawnChance),
		official.ParamMinimumFood, fmt.Sprint(cfg.minimumFood),
		// Not "hazardDamagePerTurn": that is the wire field's name. The rules
		// library's own parameter key is "damagePerTurn", and passing the wrong
		// one leaves royale running at its default while looking configured.
		official.ParamHazardDamagePerTurn, fmt.Sprint(cfg.hazardDamage),
		official.ParamShrinkEveryNTurns, fmt.Sprint(cfg.shrinkEveryNTurns),
	).WithSeed(int64(seed))

	ruleset := official.NewRulesetBuilder().WithSeed(int64(seed)).
		WithSettings(settings).NamedRuleset(cfg.gameType)

	gameMap, err := maps.GetMap(cfg.mapName)
	if err != nil {
		res.err = fmt.Errorf("map %q: %w", cfg.mapName, err)
		return res
	}

	ids := []string{"slot0", "slot1"}
	state, err := maps.SetupBoard(gameMap.ID(), ruleset.Settings(), cfg.width, cfg.height, ids)
	if err != nil {
		res.err = fmt.Errorf("setup: %w", err)
		return res
	}

	// Slot 0 plays arm A when aFirst, otherwise arm B. Alternating this across
	// seeds is what separates "this configuration is better" from "this
	// starting square is better".
	slotArm := [2]int{0, 1}
	if !aFirst {
		slotArm = [2]int{1, 0}
	}

	players := make([]*player, len(ids))
	for i := range players {
		players[i] = newPlayer(arms[slotArm[i]], cfg)
	}

	for turn := 0; turn < cfg.maxTurns; turn++ {
		state, err = maps.PreUpdateBoard(gameMap, state, ruleset.Settings())
		if err != nil {
			res.err = fmt.Errorf("pre-update: %w", err)
			return res
		}

		moves := make([]official.SnakeMove, 0, len(state.Snakes))
		for i := range state.Snakes {
			s := &state.Snakes[i]
			if s.EliminatedCause != official.NotEliminated {
				continue
			}
			slot := slotOf(ids, s.ID)
			moves = append(moves, official.SnakeMove{
				ID:   s.ID,
				Move: players[slot].move(state, s.ID).String(),
			})
		}

		var over bool
		over, state, err = ruleset.Execute(state, moves)
		if err != nil {
			res.err = fmt.Errorf("execute: %w", err)
			return res
		}
		state, err = maps.PostUpdateBoard(gameMap, state, ruleset.Settings())
		if err != nil {
			res.err = fmt.Errorf("post-update: %w", err)
			return res
		}
		state.Turn++
		res.turns = state.Turn

		for i := range players {
			if players[i].stats.firstDeath == 0 && !aliveIn(state, ids[i]) {
				players[i].stats.firstDeath = state.Turn
			}
		}

		if over {
			break
		}
	}

	for i := range players {
		res.perArm[slotArm[i]] = players[i].stats
	}

	alive0, alive1 := aliveIn(state, ids[0]), aliveIn(state, ids[1])
	switch {
	case alive0 == alive1:
		res.outcome = draw
	case alive0:
		res.outcome = armOutcome(slotArm[0])
	default:
		res.outcome = armOutcome(slotArm[1])
	}
	return res
}

func armOutcome(armIndex int) outcome {
	if armIndex == 0 {
		return armAWon
	}
	return armBWon
}

func slotOf(ids []string, id string) int {
	for i := range ids {
		if ids[i] == id {
			return i
		}
	}
	return 0
}

func aliveIn(state *official.BoardState, id string) bool {
	for i := range state.Snakes {
		if state.Snakes[i].ID == id {
			return state.Snakes[i].EliminatedCause == official.NotEliminated
		}
	}
	return false
}

// player is one configured snake for the length of one game.
//
// It holds its own searcher, and therefore its own transposition table. Sharing
// one between games would make every result depend on what the other games
// happened to look up, which would leave the whole run unreproducible while
// every unit test stayed green.
type player struct {
	arm      arm
	cfg      runConfig
	searcher *search.Searcher
	topo     board.Topology
	rng      *deterministicRand
	stats    counters
}

func newPlayer(a arm, cfg runConfig) *player {
	return &player{arm: a, cfg: cfg, rng: newDeterministicRand(uint64(a.seed))}
}

// move returns this player's move for the turn.
func (p *player) move(state *official.BoardState, id string) board.Direction {
	st, me, err := convert(state, id, p.cfg)
	if err != nil {
		return board.Up
	}

	p.stats.turns++

	if p.arm.random {
		// The control arm. Without a floor to compare against, a component
		// that contributes nothing and a component that contributes a lot look
		// alike; the predecessor found that breaking ties with a coin loses
		// 2-18, which is what told it its model was doing something.
		legal := legalMoves(st, me)
		if len(legal) == 0 {
			return board.Up
		}
		return legal[p.rng.next(uint64(len(legal)))]
	}

	if p.searcher == nil || p.topo != st.Topo {
		p.searcher = search.New(st.Topo, p.arm.cfg)
		p.topo = st.Topo
	}

	res := p.searcher.Search(st, me, search.Budget{Nodes: p.arm.nodes, MaxDepth: p.arm.maxDepth})

	p.stats.nodes += res.Nodes
	p.stats.depthSum += res.Depth
	if res.Depth > p.stats.maxDepth {
		p.stats.maxDepth = res.Depth
	}
	if res.Depth == 0 {
		p.stats.fallbacks++
	}
	if res.AllLosing {
		p.stats.allLosing++
	}
	if res.Aborted {
		p.stats.aborted++
	}
	return res.Move
}

func legalMoves(st *rules.State, me int) []board.Direction {
	head := st.Topo.At(int(st.Snakes[me].Head()))
	blocked := st.Passable()
	out := make([]board.Direction, 0, 4)
	for _, d := range board.Directions {
		if next, ok := st.Topo.Step(head, d); ok && !blocked.Has(next) {
			out = append(out, d)
		}
	}
	return out
}

// convert turns the official board into the one the search runs on.
func convert(state *official.BoardState, id string, cfg runConfig) (*rules.State, int, error) {
	variant := cfg.variant
	topo, err := board.NewTopology(state.Width, state.Height, variant == rules.Wrapped)
	if err != nil {
		return nil, 0, err
	}

	specs := make([]rules.SnakeSpec, 0, len(state.Snakes))
	me := -1
	for i := range state.Snakes {
		s := &state.Snakes[i]
		if s.EliminatedCause != official.NotEliminated {
			continue
		}
		if s.ID == id {
			me = len(specs)
		}
		specs = append(specs, rules.SnakeSpec{
			ID: s.ID, Health: s.Health, Body: convertPoints(s.Body),
		})
	}
	if me < 0 {
		return nil, 0, errNotPlaying
	}

	st, err := rules.NewState(rules.Config{
		Topology: topo, Variant: variant, HazardDamage: cfg.hazardDamage,
		Turn:   state.Turn,
		Snakes: specs,
		Food:   convertPoints(state.Food),
		// The official hazard list can repeat a square on some maps; the
		// simulator models hazards as a set and refuses a duplicate, so they
		// are collapsed here rather than at the boundary.
		Hazards: dedupePoints(convertPoints(state.Hazards)),
	})
	if err != nil {
		return nil, 0, err
	}
	return st, me, nil
}

func convertPoints(ps []official.Point) []board.Point {
	out := make([]board.Point, 0, len(ps))
	for _, p := range ps {
		out = append(out, board.Point{X: p.X, Y: p.Y})
	}
	return out
}

func dedupePoints(ps []board.Point) []board.Point {
	if len(ps) < 2 {
		return ps
	}
	seen := make(map[board.Point]struct{}, len(ps))
	out := ps[:0]
	for _, p := range ps {
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// deterministicRand is a tiny splitmix64, so that the random control arm is
// reproducible from its seed like everything else in a run.
type deterministicRand struct{ state uint64 }

func newDeterministicRand(seed uint64) *deterministicRand {
	return &deterministicRand{state: seed*0x9E3779B97F4A7C15 + 1}
}

func (r *deterministicRand) next(n uint64) int {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	z ^= z >> 31
	return int(z % n)
}

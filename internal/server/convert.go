// Package server exposes the four Battlesnake webhooks and turns a request into
// a move.
//
// It is the only place in this module that knows about HTTP, clocks or logging.
// Everything it calls - the board, the rules, the search, the evaluation - is a
// pure function over a state, which is what makes those packages testable
// against the official rules and benchmarkable against a deadline.
package server

import (
	"fmt"
	"log/slog"

	"github.com/n0nuser/typhon/internal/api"
	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
)

// variantFor maps a ruleset name onto the rules this module implements.
//
// An unrecognised ruleset is played with standard logic and said so, loudly.
// The predecessor did the first half and not the second, which meant it played
// every wrapped game treating the edge of the board as fatal - losing games for
// a reason nothing in its logs would ever have shown.
func variantFor(name string, log *slog.Logger) (rules.Variant, bool) {
	switch name {
	case "standard", "solo", "":
		return rules.Standard, true
	case "royale":
		return rules.Royale, true
	case "constrictor":
		return rules.Constrictor, true
	case "wrapped":
		return rules.Wrapped, true
	default:
		log.Warn("unsupported ruleset, playing it with standard logic",
			"ruleset", name,
			"supported", "standard, royale, constrictor, wrapped")
		return rules.Standard, false
	}
}

// knownMaps are the boards whose furniture this module understands.
//
// A map is not just cosmetic. Several official maps place their walls as
// hazard squares and others move food or hazards around every turn, and this
// module reads hazards as damage rather than as obstacles - so on a maze map it
// would walk into a wall believing it costs health. Standard and royale are the
// maps the four supported rulesets are played on.
var knownMaps = map[string]bool{
	"": true, "standard": true, "royale": true, "empty": true, "solo": true,
}

// checkMap warns when the engine sends a board this module does not understand.
//
// It reports rather than refuses, because a wrong-but-playing snake beats a
// snake that returns nothing, and the engine moves a silent snake up. The point
// is that it is never silent: the predecessor played every wrapped game with
// standard logic and nothing in any log said so.
func checkMap(name string, log *slog.Logger) bool {
	if knownMaps[name] {
		return true
	}
	log.Warn("unfamiliar map, playing it as an ordinary board",
		"map", name,
		"known", "standard, royale, empty, solo",
		"risk", "hazard squares are read as damage, not as walls")
	return false
}

// stateFrom builds the search's board from a request, and reports which snake
// we are.
func stateFrom(req api.GameRequest, log *slog.Logger) (*rules.State, int, error) {
	variant, _ := variantFor(req.Game.Ruleset.Name, log)

	topo, err := board.NewTopology(req.Board.Width, req.Board.Height, variant == rules.Wrapped)
	if err != nil {
		return nil, 0, fmt.Errorf("board %dx%d: %w", req.Board.Width, req.Board.Height, err)
	}

	specs := make([]rules.SnakeSpec, 0, len(req.Board.Snakes))
	me := -1
	for i := range req.Board.Snakes {
		s := &req.Board.Snakes[i]
		if s.ID == req.You.ID {
			me = len(specs)
		}
		specs = append(specs, rules.SnakeSpec{
			ID:     s.ID,
			Health: s.Health,
			Body:   toPoints(s.Body),
		})
	}
	if me < 0 {
		return nil, 0, errNotOnBoard
	}

	state, err := rules.NewState(rules.Config{
		Topology: topo,
		Variant:  variant,
		// Read from the request every turn rather than assumed. The rules
		// library's own default is zero and it is the CLI that passes 14, so a
		// missing field must not be treated as "fourteen damage".
		HazardDamage: req.Game.Ruleset.Settings.HazardDamagePerTurn,
		Turn:         req.Turn,
		Snakes:       specs,
		Food:         toPoints(req.Board.Food),
		Hazards:      dedupe(toPoints(req.Board.Hazards)),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("build state: %w", err)
	}
	return state, me, nil
}

func toPoints(cs []api.Coord) []board.Point {
	out := make([]board.Point, 0, len(cs))
	for _, c := range cs {
		out = append(out, board.Point{X: c.X, Y: c.Y})
	}
	return out
}

// dedupe removes repeated hazard squares.
//
// The engine deals hazard damage once per listing, so a square named twice
// hurts twice; this module models hazards as a set. No official map for the
// rulesets played here repeats one, and refusing the whole request over it
// would be worse than playing a square's damage once.
func dedupe(ps []board.Point) []board.Point {
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

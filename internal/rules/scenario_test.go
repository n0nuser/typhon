package rules_test

import (
	"fmt"
	"testing"

	official "github.com/BattlesnakeOfficial/rules"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
)

// The random playouts prove the two implementations agree; these prove they
// agree about the specific rules that decided games for the predecessor. A
// failure in a random playout is a wall of coordinates, and a failure here
// names the rule.
//
// Each case is still checked against the official ruleset rather than against a
// hand-written expected board, so the test cannot encode a misreading. The
// wants recorded alongside are what makes the case readable, and they are
// asserted too.

type snakeSetup struct {
	health int
	body   []board.Point
	move   board.Direction
}

type scenario struct {
	name         string
	variant      rules.Variant
	gameType     string
	wrapped      bool
	hazardDamage int
	food         []board.Point
	hazards      []board.Point
	snakes       []snakeSetup

	wantAlive  []bool
	wantCause  []rules.Cause
	wantHealth []int
	wantLength []int
}

func TestRuleScenarios(t *testing.T) {
	t.Parallel()

	tests := []scenario{
		{
			// The opening position in every game: same length, same food, and
			// both snakes die. A tie is exactly as fatal as a loss.
			name: "equal length head-to-head kills both",
			snakes: []snakeSetup{
				{health: 90, body: pts(4, 5, 3, 5, 2, 5), move: board.Right},
				{health: 90, body: pts(6, 5, 7, 5, 8, 5), move: board.Left},
			},
			wantAlive: []bool{false, false},
			wantCause: []rules.Cause{rules.HeadToHead, rules.HeadToHead},
		},
		{
			name: "the longer snake wins a head-to-head",
			snakes: []snakeSetup{
				{health: 90, body: pts(4, 5, 3, 5, 2, 5, 1, 5), move: board.Right},
				{health: 90, body: pts(6, 5, 7, 5, 8, 5), move: board.Left},
			},
			wantAlive:  []bool{true, false},
			wantCause:  []rules.Cause{rules.Alive, rules.HeadToHead},
			wantHealth: []int{89, 89},
		},
		{
			// A tail vacates as its snake moves, so the square it is leaving is
			// safe to enter. This is what lets a boxed-in snake survive
			// indefinitely by following its own tail.
			name: "a vacating tail is safe to enter",
			snakes: []snakeSetup{
				{health: 90, body: pts(1, 3, 1, 2, 1, 1), move: board.Up},
				{health: 90, body: pts(3, 4, 2, 4, 1, 4), move: board.Right},
			},
			wantAlive: []bool{true, true},
		},
		{
			// ...but a snake that ate last turn carries a duplicated tail, so
			// its tail does not advance and the square stays occupied. The
			// only difference from the case above is that duplicate, which is
			// why a naive body[len-1] release kills you.
			name: "a stacked tail does not vacate",
			snakes: []snakeSetup{
				{health: 90, body: pts(1, 3, 1, 2, 1, 1), move: board.Up},
				{health: rules.MaxHealth, body: pts(3, 4, 2, 4, 1, 4, 1, 4), move: board.Right},
			},
			wantAlive: []bool{false, true},
			wantCause: []rules.Cause{rules.BodyCollision, rules.Alive},
		},
		{
			// Eating restores health to full and stacks the tail, so the
			// length goes up on the turn of the meal and the extra square is
			// held from here on.
			name: "eating restores health and stacks the tail",
			food: pts(5, 6),
			snakes: []snakeSetup{
				{health: 42, body: pts(5, 5, 5, 4, 5, 3), move: board.Up},
				{health: 90, body: pts(1, 1, 1, 2, 1, 3), move: board.Down},
			},
			wantAlive:  []bool{true, true},
			wantHealth: []int{rules.MaxHealth, 89},
			wantLength: []int{4, 3},
		},
		{
			name: "a snake at one health starves",
			snakes: []snakeSetup{
				{health: 1, body: pts(5, 5, 5, 4, 5, 3), move: board.Up},
				{health: 90, body: pts(1, 1, 1, 2, 1, 3), move: board.Down},
			},
			wantAlive: []bool{false, true},
			wantCause: []rules.Cause{rules.OutOfHealth, rules.Alive},
		},
		{
			name:         "hazard damage kills",
			variant:      rules.Royale,
			hazardDamage: 14,
			hazards:      pts(5, 6),
			snakes: []snakeSetup{
				{health: 10, body: pts(5, 5, 5, 4, 5, 3), move: board.Up},
				{health: 90, body: pts(1, 1, 1, 2, 1, 3), move: board.Down},
			},
			wantAlive: []bool{false, true},
			wantCause: []rules.Cause{rules.Hazard, rules.Alive},
		},
		{
			// Food on a hazard square cancels the damage outright rather than
			// reducing it. This is in the engine and not in the prose docs.
			name:         "food on a hazard cancels the damage",
			variant:      rules.Royale,
			hazardDamage: 14,
			hazards:      pts(5, 6),
			food:         pts(5, 6),
			snakes: []snakeSetup{
				{health: 10, body: pts(5, 5, 5, 4, 5, 3), move: board.Up},
				{health: 90, body: pts(1, 1, 1, 2, 1, 3), move: board.Down},
			},
			wantAlive:  []bool{true, true},
			wantHealth: []int{rules.MaxHealth, 89},
		},
		{
			name: "leaving a bounded board is fatal",
			snakes: []snakeSetup{
				{health: 90, body: pts(5, 10, 5, 9, 5, 8), move: board.Up},
				{health: 90, body: pts(1, 1, 1, 2, 1, 3), move: board.Down},
			},
			wantAlive: []bool{false, true},
			wantCause: []rules.Cause{rules.OutOfBounds, rules.Alive},
		},
		{
			// The same move on a torus is ordinary play. This is the case the
			// predecessor got wrong by treating every ruleset as standard.
			name:     "leaving a wrapped board arrives at the other edge",
			variant:  rules.Wrapped,
			gameType: official.GameTypeWrapped,
			wrapped:  true,
			snakes: []snakeSetup{
				{health: 90, body: pts(5, 10, 5, 9, 5, 8), move: board.Up},
				{health: 90, body: pts(1, 1, 1, 2, 1, 3), move: board.Down},
			},
			wantAlive: []bool{true, true},
		},
		{
			name: "running into another snake's body is fatal",
			snakes: []snakeSetup{
				{health: 90, body: pts(4, 5, 4, 4, 4, 3), move: board.Right},
				{health: 90, body: pts(5, 7, 5, 6, 5, 5, 5, 4), move: board.Up},
			},
			wantAlive: []bool{false, true},
			wantCause: []rules.Cause{rules.BodyCollision, rules.Alive},
		},
		{
			// Reversing into your own neck. At the start of a game the body is
			// three copies of one square, so every direction but the first is
			// this.
			name: "reversing into your own body is fatal",
			snakes: []snakeSetup{
				{health: 90, body: pts(5, 5, 5, 4, 5, 3), move: board.Down},
				{health: 90, body: pts(1, 1, 1, 2, 1, 3), move: board.Down},
			},
			wantAlive: []bool{false, true},
			wantCause: []rules.Cause{rules.SelfCollision, rules.Alive},
		},
		{
			// A snake that starves is off the board before collisions are
			// looked at, so the square its body held is free this same turn.
			// One elimination pass instead of two gets this wrong.
			name: "a starving snake does not block a rival the same turn",
			snakes: []snakeSetup{
				{health: 1, body: pts(5, 5, 5, 4, 5, 3), move: board.Up},
				{health: 90, body: pts(4, 4, 3, 4, 2, 4), move: board.Right},
			},
			wantAlive: []bool{false, true},
			wantCause: []rules.Cause{rules.OutOfHealth, rules.Alive},
		},
		{
			name:     "constrictor grows every snake every turn",
			variant:  rules.Constrictor,
			gameType: official.GameTypeConstrictor,
			snakes: []snakeSetup{
				{health: 90, body: pts(1, 1, 1, 2, 1, 3), move: board.Right},
				{health: 90, body: pts(9, 9, 9, 8, 9, 7), move: board.Left},
			},
			wantAlive:  []bool{true, true},
			wantHealth: []int{rules.MaxHealth, rules.MaxHealth},
			wantLength: []int{4, 4},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func (tc scenario) run(t *testing.T) {
	t.Helper()

	const width, height = 11, 11

	gameType := tc.gameType
	if gameType == "" {
		gameType = official.GameTypeStandard
	}

	topo, err := board.NewTopology(width, height, tc.wrapped)
	if err != nil {
		t.Fatalf("NewTopology: %v", err)
	}

	ids := make([]string, len(tc.snakes))
	specs := make([]rules.SnakeSpec, len(tc.snakes))
	officialSnakes := make([]official.Snake, len(tc.snakes))
	officialMoves := make([]official.SnakeMove, len(tc.snakes))
	var moves [rules.MaxSnakes]board.Direction

	for i, s := range tc.snakes {
		ids[i] = fmt.Sprintf("s%d", i)
		specs[i] = rules.SnakeSpec{ID: ids[i], Health: s.health, Body: s.body}
		officialSnakes[i] = official.Snake{ID: ids[i], Health: s.health, Body: toPoints(s.body)}
		officialMoves[i] = official.SnakeMove{ID: ids[i], Move: s.move.String()}
		moves[i] = s.move
	}

	ours, err := rules.NewState(rules.Config{
		Topology: topo, Variant: tc.variant, HazardDamage: tc.hazardDamage,
		Snakes: specs, Food: tc.food, Hazards: tc.hazards,
	})
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	settings := official.NewSettingsWithParams(
		official.ParamFoodSpawnChance, "0",
		official.ParamMinimumFood, "0",
		official.ParamHazardDamagePerTurn, fmt.Sprint(tc.hazardDamage),
	)
	ruleset := official.NewRulesetBuilder().WithSettings(settings).NamedRuleset(gameType)
	theirs := &official.BoardState{
		Turn: 0, Width: width, Height: height,
		Food: toPoints(tc.food), Hazards: toPoints(tc.hazards), Snakes: officialSnakes,
	}

	_, next, err := ruleset.Execute(theirs, officialMoves)
	if err != nil {
		t.Fatalf("official ruleset: %v", err)
	}
	next.Turn = theirs.Turn + 1

	before := snapshot(ours, ids)
	undo := ours.Apply(&moves)
	if after := snapshot(ours, ids); after == before {
		t.Fatal("Apply changed nothing; the scenario is not exercising a rule")
	}

	if diff := compare(ours, next, ids); diff != "" {
		t.Errorf("diverged from the official ruleset:\n%s", diff)
	}

	for i := range tc.snakes {
		sn := &ours.Snakes[i]
		if i < len(tc.wantAlive) && sn.Alive() != tc.wantAlive[i] {
			t.Errorf("snake %d alive = %v (cause %q), want %v", i, sn.Alive(), sn.Cause, tc.wantAlive[i])
		}
		if i < len(tc.wantCause) && sn.Cause != tc.wantCause[i] {
			t.Errorf("snake %d cause = %q, want %q", i, sn.Cause, tc.wantCause[i])
		}
		if i < len(tc.wantHealth) && sn.Health != tc.wantHealth[i] {
			t.Errorf("snake %d health = %d, want %d", i, sn.Health, tc.wantHealth[i])
		}
		if i < len(tc.wantLength) && sn.Len() != tc.wantLength[i] {
			t.Errorf("snake %d length = %d, want %d", i, sn.Len(), tc.wantLength[i])
		}
	}

	ours.Unapply(undo)
	if got := snapshot(ours, ids); got != before {
		t.Errorf("Unapply did not restore the board\nwant %s\ngot  %s", before, got)
	}
}

// pts builds a point list from flat x, y pairs, so a body reads as one line.
func pts(xy ...int) []board.Point {
	out := make([]board.Point, 0, len(xy)/2)
	for i := 0; i+1 < len(xy); i += 2 {
		out = append(out, board.Point{X: xy[i], Y: xy[i+1]})
	}
	return out
}

package rules_test

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	official "github.com/BattlesnakeOfficial/rules"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
)

// The simulator is checked against github.com/BattlesnakeOfficial/rules rather
// than against hand-written expectations of what the rules say, because a
// hand-written expectation encodes the same misreading twice. The engine is the
// only thing that knows what the engine does.
//
// Food spawning is turned off for these games. With it on, the official ruleset
// scatters food from a seeded generator this package deliberately does not
// model, the two boards diverge on the first spawn, and the comparison becomes
// worthless while still passing.

// TestSimulatorMatchesOfficialRules plays random games under both
// implementations and requires the boards to agree after every single turn.
func TestSimulatorMatchesOfficialRules(t *testing.T) {
	t.Parallel()

	variants := []struct {
		name         string
		variant      rules.Variant
		gameType     string
		wrapped      bool
		hazardDamage int
		hazards      bool
	}{
		{name: "standard", variant: rules.Standard, gameType: official.GameTypeStandard},
		{name: "wrapped", variant: rules.Wrapped, gameType: official.GameTypeWrapped, wrapped: true},
		{name: "constrictor", variant: rules.Constrictor, gameType: official.GameTypeConstrictor},
		{
			name: "royale", variant: rules.Royale, gameType: official.GameTypeStandard,
			hazardDamage: 14, hazards: true,
		},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()
			for seed := range 60 {
				playout(t, v.name, v.variant, v.gameType, v.wrapped, v.hazardDamage, v.hazards, uint64(seed))
			}
		})
	}
}

func playout(t *testing.T, name string, variant rules.Variant, gameType string,
	wrapped bool, hazardDamage int, hazards bool, seed uint64,
) {
	t.Helper()

	const (
		width  = 11
		height = 11
		snakes = 4
		turns  = 400
	)

	rng := rand.New(rand.NewPCG(seed, 0x5eed))

	// Royale's own hazard expansion is seeded-random and is not modelled, so
	// the hazard ring is placed once and held still. That is exactly what the
	// search assumes within its horizon, which makes this the right thing to
	// test.
	var hazardPoints []board.Point
	if hazards {
		for x := range width {
			hazardPoints = append(hazardPoints, board.Point{X: x, Y: 0}, board.Point{X: x, Y: height - 1})
		}
	}

	ours, theirs, ids := newBoards(t, width, height, snakes, variant, gameType,
		wrapped, hazardDamage, hazardPoints, rng)

	settings := official.NewSettingsWithParams(
		official.ParamFoodSpawnChance, "0",
		official.ParamMinimumFood, "0",
		official.ParamHazardDamagePerTurn, fmt.Sprint(hazardDamage),
	)
	ruleset := official.NewRulesetBuilder().WithSettings(settings).NamedRuleset(gameType)

	changed := false
	for turn := range turns {
		if ours.Over() {
			break
		}

		var moves [rules.MaxSnakes]board.Direction
		officialMoves := make([]official.SnakeMove, 0, len(ids))
		for i, id := range ids {
			d := plausibleMove(ours, i, rng)
			moves[i] = d
			officialMoves = append(officialMoves, official.SnakeMove{ID: id, Move: d.String()})
		}

		over, next, err := ruleset.Execute(theirs, officialMoves)
		if err != nil {
			t.Fatalf("%s seed %d turn %d: official ruleset: %v", name, seed, turn, err)
		}
		undo := ours.Apply(&moves)
		next.Turn = theirs.Turn + 1
		theirs = next

		if diff := compare(ours, theirs, ids); diff != "" {
			t.Fatalf("%s seed %d turn %d diverged:\n%s", name, seed, turn, diff)
		}
		changed = true

		// Unapply must put the board back exactly, or the search unwinds into
		// a position that never existed.
		before := snapshot(ours, ids)
		ours.Unapply(undo)
		ours.Apply(&moves)
		if got := snapshot(ours, ids); got != before {
			t.Fatalf("%s seed %d turn %d: Apply after Unapply differs\nwant %s\ngot  %s",
				name, seed, turn, before, got)
		}

		if over {
			break
		}
	}

	if !changed {
		t.Fatalf("%s seed %d: no turn was ever compared", name, seed)
	}
}

// compare reports the first difference between the two boards, considering only
// the living.
//
// Only the living, because constrictor's growth stage does not check whether a
// snake is eliminated: it sets a dead snake's health to full and grows its
// body after it has left the board. Comparing the dead would chase that
// phantom on every constrictor game that has a death in it.
func compare(ours *rules.State, theirs *official.BoardState, ids []string) string {
	var sb strings.Builder

	for i, id := range ids {
		mine := &ours.Snakes[i]
		other := findSnake(theirs, id)
		if other == nil {
			return fmt.Sprintf("snake %s missing from the official board", id)
		}

		aliveHere := mine.Alive()
		aliveThere := other.EliminatedCause == official.NotEliminated
		if aliveHere != aliveThere {
			fmt.Fprintf(&sb, "snake %s alive=%v here, %v there (cause %q vs %q)\n",
				id, aliveHere, aliveThere, mine.Cause, other.EliminatedCause)
			continue
		}
		if !aliveHere {
			continue
		}

		if mine.Health != other.Health {
			fmt.Fprintf(&sb, "snake %s health %d != %d\n", id, mine.Health, other.Health)
		}
		if mine.Len() != len(other.Body) {
			fmt.Fprintf(&sb, "snake %s length %d != %d\n", id, mine.Len(), len(other.Body))
			continue
		}
		for j := range mine.Len() {
			got := ours.Topo.At(int(mine.Cell(j)))
			want := board.Point{X: other.Body[j].X, Y: other.Body[j].Y}
			if got != want {
				fmt.Fprintf(&sb, "snake %s segment %d at %v != %v\n", id, j, got, want)
				break
			}
		}
	}

	wantFood := map[board.Point]bool{}
	for _, f := range theirs.Food {
		wantFood[board.Point{X: f.X, Y: f.Y}] = true
	}
	for _, p := range ours.Food.Points() {
		if !wantFood[p] {
			fmt.Fprintf(&sb, "food at %v that the engine does not have\n", p)
		}
		delete(wantFood, p)
	}
	for p := range wantFood {
		fmt.Fprintf(&sb, "engine has food at %v that we do not\n", p)
	}

	return sb.String()
}

func findSnake(b *official.BoardState, id string) *official.Snake {
	for i := range b.Snakes {
		if b.Snakes[i].ID == id {
			return &b.Snakes[i]
		}
	}
	return nil
}

// snapshot renders the living board as a string, for the Apply/Unapply check.
func snapshot(s *rules.State, ids []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "turn=%d ", s.Turn)
	for i, id := range ids {
		sn := &s.Snakes[i]
		fmt.Fprintf(&sb, "|%s c=%d h=%d b=", id, sn.Cause, sn.Health)
		for j := range sn.Len() {
			fmt.Fprintf(&sb, "%d,", sn.Cell(j))
		}
	}
	fmt.Fprintf(&sb, "|food=%v", s.Food.Points())
	return sb.String()
}

func newBoards(t *testing.T, width, height, snakes int, variant rules.Variant, gameType string,
	wrapped bool, hazardDamage int, hazardPoints []board.Point, rng *rand.Rand,
) (*rules.State, *official.BoardState, []string) {
	t.Helper()

	// Fixed, well-separated starts plus a little food. Deliberately not the
	// engine's own placement, which is seeded-random and would put the two
	// boards on different starting positions.
	starts := []board.Point{{X: 1, Y: 1}, {X: 9, Y: 9}, {X: 1, Y: 9}, {X: 9, Y: 1}}
	food := []board.Point{{X: 5, Y: 5}, {X: 2, Y: 5}, {X: 8, Y: 5}, {X: 5, Y: 2}, {X: 5, Y: 8}}

	topo, err := board.NewTopology(width, height, wrapped)
	if err != nil {
		t.Fatalf("NewTopology: %v", err)
	}

	ids := make([]string, snakes)
	specs := make([]rules.SnakeSpec, snakes)
	officialSnakes := make([]official.Snake, snakes)
	for i := range snakes {
		ids[i] = fmt.Sprintf("s%d", i)
		// A real game starts every snake stacked three deep on one square, and
		// that duplicate-coordinate body is exactly where a tail-release bug
		// hides, so it is the starting position used here.
		body := []board.Point{starts[i], starts[i], starts[i]}
		health := 90 + rng.IntN(11)

		specs[i] = rules.SnakeSpec{ID: ids[i], Health: health, Body: body}
		officialSnakes[i] = official.Snake{
			ID:     ids[i],
			Health: health,
			Body:   []official.Point{toPoint(body[0]), toPoint(body[1]), toPoint(body[2])},
		}
	}

	cfg := rules.Config{
		Topology: topo, Variant: variant, HazardDamage: hazardDamage,
		Snakes: specs, Food: food, Hazards: hazardPoints,
	}
	ours, err := rules.NewState(cfg)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	theirs := &official.BoardState{
		Turn: 0, Width: width, Height: height,
		Food: toPoints(food), Hazards: toPoints(hazardPoints), Snakes: officialSnakes,
	}
	_ = gameType
	return ours, theirs, ids
}

func toPoint(p board.Point) official.Point { return official.Point{X: p.X, Y: p.Y} }

func toPoints(ps []board.Point) []official.Point {
	out := make([]official.Point, 0, len(ps))
	for _, p := range ps {
		out = append(out, toPoint(p))
	}
	return out
}

// plausibleMove picks at random among the moves that do not kill the snake on
// the spot, and at random among all four when every move loses.
//
// Uniformly random play is not a usable generator here. Measured over sixty
// games it produced 3.5 turns each, grew a snake twice in total, and never once
// produced a head-to-head - so the differential test passed while exercising
// almost none of the rule surface it exists to check. A snake that merely
// avoids walls and bodies survives long enough to starve, eat, stack its tail
// and run into other snakes, which is where the rules actually live.
func plausibleMove(s *rules.State, i int, rng *rand.Rand) board.Direction {
	sn := &s.Snakes[i]
	if !sn.Alive() {
		return board.Up
	}

	head := s.Topo.At(int(sn.Head()))
	occupied := s.Occupied()
	tails := map[board.Point]bool{}
	for j := range s.Snakes {
		if other := &s.Snakes[j]; other.Alive() {
			tails[s.Topo.At(int(other.Tail()))] = true
		}
	}

	var ok []board.Direction
	for _, d := range board.Directions {
		next, inside := s.Topo.Step(head, d)
		if !inside {
			continue
		}
		// A tail vacates as its snake moves, so it is a legal target unless
		// that snake just ate - which this ignores, because being wrong
		// occasionally is what produces the collisions worth testing.
		if occupied.Has(next) && !tails[next] {
			continue
		}
		ok = append(ok, d)
	}
	if len(ok) == 0 {
		return board.Directions[rng.IntN(len(board.Directions))]
	}
	return ok[rng.IntN(len(ok))]
}

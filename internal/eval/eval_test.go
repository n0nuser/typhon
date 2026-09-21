package eval_test

import (
	"testing"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/eval"
	"github.com/n0nuser/typhon/internal/rules"
)

// The evaluation is a set of preferences, so it is tested as an ordering: given
// two positions that differ in one thing, the better one scores higher. Testing
// absolute numbers would pin today's untuned weights and break on every change
// that is supposed to be free.

func TestTerminalScores(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build func(t *testing.T) (*rules.State, int)
		check func(t *testing.T, got eval.Score)
	}{
		{
			name: "being dead is a loss",
			build: func(t *testing.T) (*rules.State, int) {
				s := build(t, rules.Standard, nil,
					snake("a", 90, pts(5, 5, 5, 4, 5, 3)),
					snake("b", 90, pts(1, 1, 1, 2, 1, 3)))
				kill(t, s, board.Down, board.Right)
				return s, 0
			},
			check: func(t *testing.T, got eval.Score) {
				if got > eval.Loss+1000 {
					t.Errorf("score %d, want near Loss (%d)", got, eval.Loss)
				}
			},
		},
		{
			name: "being the last one standing is a win",
			build: func(t *testing.T) (*rules.State, int) {
				s := build(t, rules.Standard, nil,
					snake("a", 90, pts(5, 5, 5, 4, 5, 3)),
					snake("b", 90, pts(1, 1, 1, 2, 1, 3)))
				// b reverses into its own neck; a moves into open board.
				kill(t, s, board.Up, board.Up)
				return s, 0
			},
			check: func(t *testing.T, got eval.Score) {
				if got < eval.Win-1000 {
					t.Errorf("score %d, want near Win (%d)", got, eval.Win)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, me := tc.build(t)
			e := eval.New(s.Topo, eval.Default())
			tc.check(t, e.Evaluate(s, me, 0))
		})
	}
}

// Dying later is worth more than dying now. Without this a search that sees a
// forced loss picks arbitrarily among the ways of reaching it, and walks into
// the fastest one - which loses games that a longer line might have survived
// because the opponent blunders.
func TestALaterLossScoresHigher(t *testing.T) {
	t.Parallel()

	s := build(t, rules.Standard, nil,
		snake("a", 90, pts(5, 5, 5, 4, 5, 3)),
		snake("b", 90, pts(1, 1, 1, 2, 1, 3)))
	kill(t, s, board.Down, board.Right)

	e := eval.New(s.Topo, eval.Default())
	soon := e.Evaluate(s, 0, 2)
	later := e.Evaluate(s, 0, 9)
	if later <= soon {
		t.Errorf("loss at ply 9 = %d, at ply 2 = %d; want the later loss to score higher", later, soon)
	}
}

func TestPreferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		variant       rules.Variant
		wrapped       bool
		food          []board.Point
		better, worse []rules.SnakeSpec
		reason        string
	}{
		{
			name:   "open space beats a pocket",
			better: []rules.SnakeSpec{snake("a", 90, pts(5, 5, 5, 4, 5, 3)), snake("b", 90, pts(9, 9, 9, 8, 9, 7))},
			worse:  []rules.SnakeSpec{snake("a", 90, pts(0, 0, 1, 0, 1, 1)), snake("b", 90, pts(9, 9, 9, 8, 9, 7))},
			reason: "the middle of an empty board has more room than a corner",
		},
		{
			name:   "being longer than the rival is better",
			better: []rules.SnakeSpec{snake("a", 90, pts(5, 5, 5, 4, 5, 3, 5, 2, 5, 1)), snake("b", 90, pts(9, 9, 9, 8, 9, 7))},
			worse:  []rules.SnakeSpec{snake("a", 90, pts(5, 5, 5, 4, 5, 3)), snake("b", 90, pts(9, 9, 9, 8, 9, 7, 9, 6, 9, 5))},
			reason: "length decides every head-to-head",
		},
		{
			name:   "a hungry snake prefers to be near food",
			food:   pts(5, 7),
			better: []rules.SnakeSpec{snake("a", 15, pts(5, 6, 5, 5, 5, 4)), snake("b", 90, pts(9, 9, 9, 8, 9, 7))},
			worse:  []rules.SnakeSpec{snake("a", 15, pts(0, 0, 1, 0, 2, 0)), snake("b", 90, pts(9, 9, 9, 8, 9, 7))},
			reason: "at fifteen health the distance to food is most of what matters",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			good := build(t, tc.variant, tc.food, tc.better...)
			bad := build(t, tc.variant, tc.food, tc.worse...)
			e := eval.New(good.Topo, eval.Default())

			gs, bs := e.Evaluate(good, 0, 0), e.Evaluate(bad, 0, 0)
			if gs <= bs {
				t.Errorf("better position scored %d, worse scored %d: %s", gs, bs, tc.reason)
			}
		})
	}
}

// Terms that cannot mean anything in a ruleset must be off, not merely small.
func TestTermsThatCannotApplyAreOff(t *testing.T) {
	t.Parallel()

	t.Run("tail reachability is off in constrictor", func(t *testing.T) {
		t.Parallel()

		s := build(t, rules.Constrictor, nil,
			snake("a", 100, pts(5, 5, 5, 4, 5, 3)),
			snake("b", 100, pts(9, 9, 9, 8, 9, 7)))

		w := eval.Default()
		with := eval.New(s.Topo, w).Evaluate(s, 0, 0)
		w.TailReach = 0
		without := eval.New(s.Topo, w).Evaluate(s, 0, 0)

		if with != without {
			t.Errorf("tail reachability changed a constrictor score: %d vs %d", with, without)
		}
	})

	t.Run("centre control is off when the board wraps", func(t *testing.T) {
		t.Parallel()

		topo, err := board.NewTopology(11, 11, true)
		if err != nil {
			t.Fatal(err)
		}
		s, err := rules.NewState(rules.Config{
			Topology: topo, Variant: rules.Wrapped,
			Snakes: []rules.SnakeSpec{
				snake("a", 90, pts(0, 0, 1, 0, 2, 0)),
				snake("b", 90, pts(9, 9, 9, 8, 9, 7)),
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		w := eval.Default()
		with := eval.New(topo, w).Evaluate(s, 0, 0)
		w.Centre = 0
		without := eval.New(topo, w).Evaluate(s, 0, 0)

		if with != without {
			t.Errorf("centre control changed a wrapped score: %d vs %d", with, without)
		}
	})
}

// A weight of zero must switch its term off. This is what "one flag per
// variable" rests on, and it is worth an assertion rather than an assumption:
// the harness's whole claim is that two arms differ in exactly one thing.
func TestZeroWeightSilencesItsTerm(t *testing.T) {
	t.Parallel()

	// Each subtest builds its own State: a State owns mutable scratch, so
	// sharing one across parallel tests is a race, and the race detector says
	// so loudly the moment two of them evaluate at once.
	newState := func(t *testing.T) *rules.State {
		t.Helper()
		// Chosen so that every term has something to say: off-centre, a
		// different length from the rival, hungry, with food on the board and
		// a reachable tail. A position where a term contributes zero cannot
		// tell "switched off" from "had no opinion".
		return build(t, rules.Standard, pts(5, 7),
			snake("a", 40, pts(3, 4, 3, 3, 3, 2, 4, 2)),
			snake("b", 90, pts(9, 9, 9, 8, 9, 7)))
	}

	// Every weight non-zero, and deliberately not eval.Default(): this asserts
	// that each weight drives its term, which is a property of the evaluation
	// and not of whichever configuration currently ships. Reading the default
	// here made the test vacuous for any term the default switches off - it
	// cannot tell "zeroing did nothing" from "it was already zero".
	full := eval.Weights{
		Voronoi: 10, Space: 6, TailReach: 40,
		Length: 30, Food: 4, Centre: 1, Confine: 6,
	}
	s := newState(t)
	base := eval.New(s.Topo, full).Evaluate(s, 0, 0)

	for _, tc := range []struct {
		name string
		zero func(w *eval.Weights)
	}{
		{"voronoi", func(w *eval.Weights) { w.Voronoi = 0 }},
		{"space", func(w *eval.Weights) { w.Space = 0 }},
		{"tail reach", func(w *eval.Weights) { w.TailReach = 0 }},
		{"length", func(w *eval.Weights) { w.Length = 0 }},
		{"food", func(w *eval.Weights) { w.Food = 0 }},
		{"centre", func(w *eval.Weights) { w.Centre = 0 }},
		{"confine", func(w *eval.Weights) { w.Confine = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := full
			tc.zero(&w)
			s := newState(t)
			if got := eval.New(s.Topo, w).Evaluate(s, 0, 0); got == base {
				t.Errorf("zeroing %s changed nothing (%d); the term never fired, so any A/B on it would measure noise", tc.name, got)
			}
		})
	}
}

func BenchmarkEvaluate(b *testing.B) {
	topo, err := board.NewTopology(11, 11, false)
	if err != nil {
		b.Fatal(err)
	}
	s, err := rules.NewState(rules.Config{
		Topology: topo, Variant: rules.Standard,
		Snakes: []rules.SnakeSpec{
			snake("a", 90, pts(1, 1, 1, 2, 1, 3, 1, 4, 2, 4)),
			snake("b", 80, pts(9, 9, 9, 8, 9, 7, 8, 7)),
			snake("c", 70, pts(1, 9, 2, 9, 3, 9)),
			snake("d", 60, pts(9, 1, 8, 1, 7, 1)),
		},
		Food: pts(5, 5, 2, 7, 7, 2),
	})
	if err != nil {
		b.Fatal(err)
	}
	e := eval.New(topo, eval.Default())

	b.ReportAllocs()
	for b.Loop() {
		if e.Evaluate(s, 0, 3) == eval.Unknown {
			b.Fatal("scored zero")
		}
	}
}

func build(t *testing.T, variant rules.Variant, food []board.Point, snakes ...rules.SnakeSpec) *rules.State {
	t.Helper()
	topo, err := board.NewTopology(11, 11, false)
	if err != nil {
		t.Fatal(err)
	}
	s, err := rules.NewState(rules.Config{
		Topology: topo, Variant: variant, Snakes: snakes, Food: food,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// kill advances one turn with the given moves, which the caller chooses to make
// somebody die.
func kill(t *testing.T, s *rules.State, moves ...board.Direction) {
	t.Helper()
	var m [rules.MaxSnakes]board.Direction
	copy(m[:], moves)
	s.Apply(&m)
}

func snake(id string, health int, body []board.Point) rules.SnakeSpec {
	return rules.SnakeSpec{ID: id, Health: health, Body: body}
}

func pts(xy ...int) []board.Point {
	out := make([]board.Point, 0, len(xy)/2)
	for i := 0; i+1 < len(xy); i += 2 {
		out = append(out, board.Point{X: xy[i], Y: xy[i+1]})
	}
	return out
}

package search_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
	"github.com/n0nuser/typhon/internal/search"
)

// The two guarantees the whole design rests on: never return an unsafe move,
// never return late. Everything else the search does is an improvement on a
// correct answer it already had.

func TestNeverReturnsAMoveIntoAWallOrABody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wrapped bool
		snakes  []rules.SnakeSpec
		banned  []board.Direction
	}{
		{
			name: "boxed into a corner",
			snakes: []rules.SnakeSpec{
				snake("a", 90, pts(0, 0, 0, 1, 0, 2)),
				snake("b", 90, pts(5, 5, 5, 6, 5, 7)),
			},
			// Down and Left leave the board; Up is its own neck.
			banned: []board.Direction{board.Down, board.Left, board.Up},
		},
		{
			name: "a wall of another snake",
			snakes: []rules.SnakeSpec{
				snake("a", 90, pts(5, 5, 5, 4, 5, 3)),
				snake("b", 90, pts(4, 5, 4, 6, 4, 7, 6, 5, 6, 6)),
			},
			banned: []board.Direction{board.Down, board.Left},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := build(t, tc.wrapped, rules.Standard, nil, tc.snakes...)
			got := search.New(s.Topo, search.DefaultConfig()).
				Search(s, 0, search.Budget{Nodes: 4000}).Move

			for _, bad := range tc.banned {
				if got == bad {
					t.Fatalf("played %v, which is fatal", got)
				}
			}
		})
	}
}

// The fallback has to be right on its own, because it is what answers when the
// search never completes a single depth.
func TestSafeMove(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		snakes    []rules.SnakeSpec
		want      board.Direction
		allLosing bool
	}{
		{
			name: "the only opening",
			snakes: []rules.SnakeSpec{
				snake("a", 90, pts(0, 0, 0, 1, 0, 2)),
				snake("b", 90, pts(5, 5, 5, 6, 5, 7)),
			},
			want: board.Right,
		},
		{
			// An uncontested square beats one a rival can reach, even a rival
			// we would beat. The fallback's job is to be alive, not to hunt;
			// taking a head-to-head we would win is an improvement the search
			// is allowed to make, and this is what answers when it cannot.
			name: "an uncontested square beats a winnable head-to-head",
			snakes: []rules.SnakeSpec{
				snake("a", 90, pts(5, 5, 5, 4, 5, 3, 5, 2, 5, 1)),
				snake("b", 90, pts(5, 7, 5, 8, 5, 9)),
			},
			want: board.Left,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := build(t, false, rules.Standard, nil, tc.snakes...)
			got, allLosing := search.SafeMove(s, 0)
			if got != tc.want {
				t.Errorf("SafeMove = %v, want %v", got, tc.want)
			}
			if allLosing != tc.allLosing {
				t.Errorf("allLosing = %v, want %v", allLosing, tc.allLosing)
			}
		})
	}
}

// When every move loses, the score cannot separate them and the choice falls
// to how many rivals contest the square. This is the opening position the
// predecessor died in over and over.
func TestWhenEverythingLosesPickTheLeastContestedSquare(t *testing.T) {
	t.Parallel()

	// Head at (5,5) with its own neck below it, so Down is not even an option.
	// Every remaining square is contested by an equally long rival, which
	// kills both snakes - so all three moves lose. Left is contested twice,
	// Up and Right once each.
	s := build(t, false, rules.Standard, nil,
		snake("a", 90, pts(5, 5, 5, 4, 5, 3)),
		snake("b", 90, pts(3, 5, 2, 5, 1, 5)),
		snake("c", 90, pts(7, 5, 8, 5, 9, 5)),
		snake("d", 90, pts(5, 7, 5, 8, 5, 9)),
		snake("e", 90, pts(4, 4, 3, 3, 2, 3)),
	)

	got, allLosing := search.SafeMove(s, 0)
	if !allLosing {
		t.Fatalf("expected every move to lose, but %v was reported safe", got)
	}

	head := s.Topo.At(int(s.Snakes[0].Head()))
	chosenSquare, _ := s.Topo.Step(head, got)
	_, chosen := search.Contest(s, 0, chosenSquare)

	for _, d := range board.Directions {
		next, ok := s.Topo.Step(head, d)
		if !ok || s.Passable().Has(next) {
			continue
		}
		if _, n := search.Contest(s, 0, next); n < chosen {
			t.Errorf("played %v with %d rivals contesting it when %v had only %d",
				got, chosen, d, n)
		}
	}

	if got == board.Left {
		t.Error("played into the doubly contested square")
	}
}

// A node budget makes a search reproducible, which is the property every
// benchmark comparison rests on. Without it, two runs of one configuration
// differ by whatever the scheduler did, and 200 games cannot see through that.
func TestANodeBudgetMakesTheSearchReproducible(t *testing.T) {
	t.Parallel()

	for range 4 {
		first := searchOnce(t, 8000)
		second := searchOnce(t, 8000)
		if first != second {
			t.Fatalf("identical searches disagreed: %+v vs %+v", first, second)
		}
	}
}

func searchOnce(t *testing.T, nodes int64) search.Result {
	t.Helper()
	s := build(t, false, rules.Standard, pts(5, 5, 2, 7),
		snake("a", 70, pts(1, 1, 1, 2, 1, 3, 2, 3)),
		snake("b", 80, pts(9, 9, 9, 8, 9, 7)),
	)
	return search.New(s.Topo, search.DefaultConfig()).Search(s, 0, search.Budget{Nodes: nodes})
}

// Never return late. The deadline is the one promise that cannot be broken,
// because a late answer is scored as no answer and the engine moves us up.
func TestTheDeadlineIsHonoured(t *testing.T) {
	t.Parallel()

	for _, budget := range []time.Duration{2 * time.Millisecond, 10 * time.Millisecond, 50 * time.Millisecond} {
		s := build(t, false, rules.Standard, pts(5, 5),
			snake("a", 90, pts(1, 1, 1, 2, 1, 3)),
			snake("b", 90, pts(9, 9, 9, 8, 9, 7)),
			snake("c", 90, pts(1, 9, 2, 9, 3, 9)),
		)

		start := time.Now()
		res := search.New(s.Topo, search.DefaultConfig()).
			Search(s, 0, search.Budget{Deadline: start.Add(budget)})
		took := time.Since(start)

		// The allowance is for the clock being read every 1024 nodes, not for
		// sloppiness: a whole check interval is far under a millisecond.
		if took > budget+20*time.Millisecond {
			t.Errorf("budget %v: took %v", budget, took)
		}
		if res.Depth == 0 && res.Move.String() == "" {
			t.Errorf("budget %v: returned no move", budget)
		}
	}
}

// Over many random positions the search must never return a move into a wall
// or a body while a legal move exists. This is the guarantee stated as a
// property rather than as a handful of examples, because the examples are the
// positions someone thought of.
func TestSearchNeverPlaysIntoAWallWhenItDoesNotHaveTo(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(11, 22))
	topo, err := board.NewTopology(11, 11, false)
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for range 60 {
		s, err := rules.NewState(rules.Config{
			Topology: topo, Variant: rules.Standard,
			Snakes: []rules.SnakeSpec{
				randomSnake(rng, "a"), randomSnake(rng, "b"), randomSnake(rng, "c"),
			},
			Food: pts(5, 5),
		})
		if err != nil {
			continue
		}

		head := topo.At(int(s.Snakes[0].Head()))
		legal := map[board.Direction]bool{}
		for _, d := range board.Directions {
			if next, ok := topo.Step(head, d); ok && !s.Passable().Has(next) {
				legal[d] = true
			}
		}
		if len(legal) == 0 {
			continue
		}
		checked++

		got := search.New(topo, search.DefaultConfig()).
			Search(s, 0, search.Budget{Nodes: 1500}).Move
		if !legal[got] {
			t.Fatalf("played %v into a wall or a body when %v were legal", got, legal)
		}
	}

	if checked < 20 {
		t.Fatalf("only %d positions had a legal move; the generator is not producing real ones", checked)
	}
}

// Depth is what this project is for, so it is asserted rather than assumed: a
// bigger budget must buy more of it.
func TestABiggerBudgetBuysMoreDepth(t *testing.T) {
	t.Parallel()

	shallow := searchOnce(t, 500)
	deep := searchOnce(t, 40000)

	if deep.Depth <= shallow.Depth {
		t.Errorf("500 nodes reached depth %d, 200000 nodes reached %d; the budget is not buying depth",
			shallow.Depth, deep.Depth)
	}
	if shallow.Depth == 0 {
		t.Error("even the shallow search completed no depth at all")
	}
	t.Logf("depth %d at 500 nodes, depth %d at 40000", shallow.Depth, deep.Depth)
}

func randomSnake(rng *rand.Rand, id string) rules.SnakeSpec {
	x, y := rng.IntN(11), rng.IntN(11)
	body := []board.Point{{X: x, Y: y}}
	for range 2 + rng.IntN(4) {
		last := body[len(body)-1]
		switch rng.IntN(4) {
		case 0:
			last.X = min(10, last.X+1)
		case 1:
			last.X = max(0, last.X-1)
		case 2:
			last.Y = min(10, last.Y+1)
		default:
			last.Y = max(0, last.Y-1)
		}
		body = append(body, last)
	}
	return rules.SnakeSpec{ID: id, Health: 20 + rng.IntN(80), Body: body}
}

// BenchmarkSearch measures a whole turn's search at a fixed node budget, which
// is the number the deadline is actually spent on.
//
// The state and the searcher are built once, outside the loop, because Search
// leaves the board exactly as it found it - that is what Apply and Unapply are
// for - and because building them inside would report setup allocation as
// search allocation.
func BenchmarkSearch(b *testing.B) {
	topo, err := board.NewTopology(11, 11, false)
	if err != nil {
		b.Fatal(err)
	}
	s, err := rules.NewState(rules.Config{
		Topology: topo, Variant: rules.Standard,
		Snakes: []rules.SnakeSpec{
			{ID: "a", Health: 90, Body: pts(1, 1, 1, 2, 1, 3, 2, 3)},
			{ID: "b", Health: 80, Body: pts(9, 9, 9, 8, 9, 7)},
		},
		Food: pts(5, 5, 2, 7),
	})
	if err != nil {
		b.Fatal(err)
	}
	searcher := search.New(topo, search.DefaultConfig())

	b.ReportAllocs()
	for b.Loop() {
		if res := searcher.Search(s, 0, search.Budget{Nodes: 20000}); res.Depth == 0 {
			b.Fatal("no depth completed")
		}
	}
}

// BenchmarkSearchNode reports the cost of one node, which is what decides how
// deep a turn budget reaches.
func BenchmarkSearchNode(b *testing.B) {
	topo, err := board.NewTopology(11, 11, false)
	if err != nil {
		b.Fatal(err)
	}
	s, err := rules.NewState(rules.Config{
		Topology: topo, Variant: rules.Standard,
		Snakes: []rules.SnakeSpec{
			{ID: "a", Health: 90, Body: pts(1, 1, 1, 2, 1, 3, 2, 3)},
			{ID: "b", Health: 80, Body: pts(9, 9, 9, 8, 9, 7)},
			{ID: "c", Health: 70, Body: pts(1, 9, 2, 9, 3, 9)},
		},
		Food: pts(5, 5, 2, 7),
	})
	if err != nil {
		b.Fatal(err)
	}
	searcher := search.New(topo, search.DefaultConfig())

	const nodes = 20000
	var total int64
	b.ReportAllocs()
	for b.Loop() {
		total += searcher.Search(s, 0, search.Budget{Nodes: nodes}).Nodes
	}
	if total > 0 {
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(total), "ns/node")
	}
}

func build(t *testing.T, wrapped bool, variant rules.Variant, food []board.Point, snakes ...rules.SnakeSpec) *rules.State {
	t.Helper()
	topo, err := board.NewTopology(11, 11, wrapped)
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

// A completed search must not be overruled by the one-ply check.
//
// The one-ply check calls a square fatal whenever a rival can reach it. The
// search may have found that one of those losses arrives several turns later
// than another, and dying later is strictly better - it is more turns in which
// the rival can blunder. An earlier version handed the move back to the one-ply
// fallback whenever every square was contested, which threw that away.
func TestACompletedSearchIsNotOverruledByTheOnePlyCheck(t *testing.T) {
	t.Parallel()

	// Both squares next to our head are contested by an equally long rival, so
	// the one-ply check calls every move fatal. The search still has an opinion
	// about which loses later.
	s := build(t, false, rules.Standard, nil,
		snake("a", 90, pts(5, 5, 5, 4, 5, 3)),
		snake("b", 90, pts(3, 5, 2, 5, 1, 5)),
		snake("c", 90, pts(7, 5, 8, 5, 9, 5)),
		snake("d", 90, pts(5, 7, 5, 8, 5, 9)),
	)

	fallback, allLosing := search.SafeMove(s, 0)
	if !allLosing {
		t.Fatalf("the position is not all-losing at one ply (fallback %v); it tests nothing", fallback)
	}
	res := search.New(s.Topo, search.DefaultConfig()).
		Search(s, 0, search.Budget{Nodes: 20000, MaxDepth: 6})

	if res.Depth == 0 {
		t.Fatal("the search completed nothing; this position does not test anything")
	}
	// The flag must still be reported - it is what the logs are read for - but
	// it must not have silently replaced a searched move with the one-ply one
	// unless the search itself could not separate the options.
	if !res.AllLosing {
		t.Error("AllLosing was not reported even though every one-ply move is contested")
	}
	// And whatever it picks must still be a square it can actually enter. A
	// self-collision has no contesters, so a tie-break that ranks purely by the
	// count would put our own neck first.
	head := s.Topo.At(int(s.Snakes[0].Head()))
	next, ok := s.Topo.Step(head, res.Move)
	if !ok || s.Passable().Has(next) {
		t.Errorf("played %v into a wall or a body; the fallback was %v", res.Move, fallback)
	}

	t.Logf("fallback=%v search=%v depth=%d score=%d", fallback, res.Move, res.Depth, res.Score)
}

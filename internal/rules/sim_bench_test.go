package rules_test

import (
	"testing"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
)

// BenchmarkApplyUnapply measures one search node: advance a turn, then take it
// back. It is the inner loop of everything above it, so its rate is the
// ceiling on how deep the search can go inside a 400ms budget, and its
// allocations are the thing most likely to turn a budget into a missed
// deadline.
func BenchmarkApplyUnapply(b *testing.B) {
	topo, err := board.NewTopology(11, 11, false)
	if err != nil {
		b.Fatal(err)
	}

	state, err := rules.NewState(rules.Config{
		Topology: topo,
		Variant:  rules.Standard,
		Snakes: []rules.SnakeSpec{
			{ID: "a", Health: 90, Body: pts(1, 1, 1, 2, 1, 3, 1, 4, 2, 4)},
			{ID: "b", Health: 80, Body: pts(9, 9, 9, 8, 9, 7, 8, 7)},
			{ID: "c", Health: 70, Body: pts(1, 9, 2, 9, 3, 9)},
			{ID: "d", Health: 60, Body: pts(9, 1, 8, 1, 7, 1)},
		},
		Food: pts(5, 5, 2, 7, 7, 2),
	})
	if err != nil {
		b.Fatal(err)
	}

	moves := [rules.MaxSnakes]board.Direction{board.Right, board.Left, board.Down, board.Up}

	b.ReportAllocs()
	for b.Loop() {
		undo := state.Apply(&moves)
		state.Unapply(undo)
	}
}

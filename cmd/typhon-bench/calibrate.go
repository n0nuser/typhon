package main

import (
	"fmt"
	"time"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
	"github.com/n0nuser/typhon/internal/search"
)

// calibrate reports how many nodes a wall-clock budget buys on this machine.
//
// The suites run on a node budget, because that is what makes a game
// reproducible. The deployed bot runs on a clock. This is the bridge between
// them: without it, "arm A beat arm B at 20,000 nodes" is a true statement
// about a bot nobody is running.
//
// It is a command rather than a number in a document so that it stays true
// after a change to the search, and so that the ratio can be re-measured on
// whatever hardware a run actually happened on.
func calibrate(cfg runConfig) error {
	topo, err := board.NewTopology(cfg.width, cfg.height, cfg.variant == rules.Wrapped)
	if err != nil {
		return err
	}

	positions := []struct {
		name   string
		snakes []rules.SnakeSpec
	}{
		{
			name: "opening, 2 snakes",
			snakes: []rules.SnakeSpec{
				{ID: "a", Health: 98, Body: points(1, 1, 1, 1, 1, 1)},
				{ID: "b", Health: 98, Body: points(9, 9, 9, 9, 9, 9)},
			},
		},
		{
			name: "midgame, 2 snakes",
			snakes: []rules.SnakeSpec{
				{ID: "a", Health: 70, Body: points(4, 4, 4, 5, 4, 6, 5, 6, 6, 6, 6, 5)},
				{ID: "b", Health: 60, Body: points(8, 2, 8, 3, 7, 3, 6, 3, 6, 2)},
			},
		},
		{
			name: "midgame, 4 snakes",
			snakes: []rules.SnakeSpec{
				{ID: "a", Health: 70, Body: points(4, 4, 4, 5, 4, 6, 5, 6)},
				{ID: "b", Health: 60, Body: points(8, 2, 8, 3, 7, 3)},
				{ID: "c", Health: 80, Body: points(2, 8, 2, 7, 3, 7)},
				{ID: "d", Health: 50, Body: points(9, 9, 9, 8, 8, 8)},
			},
		},
	}

	budgets := []time.Duration{
		50 * time.Millisecond, 100 * time.Millisecond,
		200 * time.Millisecond, 400 * time.Millisecond,
	}

	fmt.Printf("CALIBRATION %s %dx%d, %d opponents modelled\n\n",
		cfg.gameType, cfg.width, cfg.height, search.DefaultConfig().Opponents)
	fmt.Printf("%-20s %8s %10s %7s %10s\n", "position", "budget", "nodes", "depth", "ns/node")

	for _, p := range positions {
		st, err := rules.NewState(rules.Config{
			Topology: topo, Variant: cfg.variant, HazardDamage: cfg.hazardDamage,
			Snakes: p.snakes, Food: points(5, 5, 2, 7, 7, 2),
		})
		if err != nil {
			return err
		}
		searcher := search.New(topo, search.DefaultConfig())

		for _, budget := range budgets {
			start := time.Now()
			res := searcher.Search(st, 0, search.Budget{Deadline: start.Add(budget)})
			took := time.Since(start)

			perNode := 0.0
			if res.Nodes > 0 {
				perNode = float64(took.Nanoseconds()) / float64(res.Nodes)
			}
			fmt.Printf("%-20s %8s %10d %7d %10.0f\n",
				p.name, budget, res.Nodes, res.Depth, perNode)

			if took > budget+20*time.Millisecond {
				fmt.Printf("%-20s OVERRAN by %s - the deadline is the one promise that cannot slip\n",
					"", took-budget)
			}
		}
		fmt.Println()
	}
	return nil
}

func points(xy ...int) []board.Point {
	out := make([]board.Point, 0, len(xy)/2)
	for i := 0; i+1 < len(xy); i += 2 {
		out = append(out, board.Point{X: xy[i], Y: xy[i+1]})
	}
	return out
}

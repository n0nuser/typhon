package main

import (
	"fmt"
	"time"

	"github.com/n0nuser/typhon/internal/rules"
)

// report prints what the run found, and what it is not entitled to claim.
func report(label string, arms [2]arm, results []gameResult, elapsed time.Duration) {
	var (
		aWins, bWins, draws, failed int
		turns                       int
		perArm                      [2]counters
		neutral                     counters
		// Per slot, so that start-position bias is visible rather than
		// averaged away. Two identical bots went 29-23-8 for the predecessor,
		// which is six points before anything is changed.
		//
		// Only the slots a contestant held are counted. In a four-snake run the
		// other two are neutrals, and a neutral's win says nothing about which
		// arm is better.
		slotWins  [rules.MaxSnakes]int
		slotGames [rules.MaxSnakes]int
	)

	for _, r := range results {
		if r.err != nil {
			failed++
			continue
		}
		turns += r.turns
		perArm[0].add(r.perArm[0])
		perArm[1].add(r.perArm[1])
		neutral.add(r.neutral)

		slotGames[r.seats.a]++
		slotGames[r.seats.b]++

		switch r.outcome {
		case armAWon:
			aWins++
			slotWins[r.seats.a]++
		case armBWon:
			bWins++
			slotWins[r.seats.b]++
		case draw:
			draws++
		}
	}

	played := len(results) - failed
	fmt.Printf("RESULT %s: %s=%d %s=%d draw=%d of %d games in %s\n",
		label, arms[0].name, aWins, arms[1].name, bWins, draws, played, elapsed.Round(time.Second))

	if failed > 0 {
		fmt.Printf("FAILED %d games errored and are excluded; the first error was: %v\n",
			failed, firstError(results))
	}

	decisive := aWins + bWins
	if decisive > 0 {
		lo, hi := wilson(aWins, decisive, 1.96)
		fmt.Printf("SHARE  %s took %.1f%% of decisive games, 95%% CI [%.1f%%, %.1f%%]\n",
			arms[0].name, 100*float64(aWins)/float64(decisive), 100*lo, 100*hi)
	}

	chi, p := mcnemar(aWins, bWins)
	fmt.Printf("PAIRED McNemar on %d discordant games: chi2=%.2f p=%.4f -> %s\n",
		decisive, chi, p, verdict(p, arms, aWins, bWins))

	if p >= 0.05 && decisive > 0 {
		rate := float64(max(aWins, bWins)) / float64(decisive)
		if need := requiredPairs(rate); need > decisive {
			fmt.Printf("POWER  at this split, about %d decisive games would be needed to separate them;\n"+
				"       this run had %d.\n", need, decisive)
		}
	}

	reportSlots(slotWins, slotGames)

	fmt.Printf("TURNS  %d total, %.1f mean per game\n", turns, safeDiv(turns, played))

	// Per-path counts. A claim about a component is not readable until these
	// say the component was reached: the predecessor once measured a whole
	// batch against a feature whose threshold was never crossed.
	for i, a := range arms {
		c := perArm[i]
		fmt.Printf("PATHS  %-10s turns=%d mean_depth=%.2f max_depth=%d nodes=%d "+
			"fallbacks=%d aborted=%d all_losing=%d deaths=%d mean_death_turn=%.0f\n",
			a.name, c.turns, safeDiv(c.depthSum, c.turns), c.maxDepth, c.nodes,
			c.fallbacks, c.aborted, c.allLosing, c.deaths, safeDiv(c.deathTurnSum, c.deaths))
		if !a.random && c.depthSum == 0 {
			fmt.Printf("       WARNING %s never completed a single depth. Whatever this run\n"+
				"       measured, it was not that arm's search.\n", a.name)
		}
	}

	// The field, when there is one. A four-snake result is a claim about a
	// comparison made inside a field, so the field has to be shown to have been
	// playing: two neutrals that never complete a depth are two obstacles, and
	// the run is a duel with scenery.
	if neutral.turns > 0 {
		fmt.Printf("PATHS  %-10s turns=%d mean_depth=%.2f max_depth=%d nodes=%d "+
			"fallbacks=%d aborted=%d all_losing=%d deaths=%d mean_death_turn=%.0f\n",
			"field", neutral.turns, safeDiv(neutral.depthSum, neutral.turns), neutral.maxDepth,
			neutral.nodes, neutral.fallbacks, neutral.aborted, neutral.allLosing,
			neutral.deaths, safeDiv(neutral.deathTurnSum, neutral.deaths))
	}
}

// reportSlots prints how often the contestant in each start square won, which
// is the only thing that says whether a result belongs to the configuration or
// to the square it started on.
//
// Each slot gets its own share of the games it was contested in, rather than one
// slot-0 percentage: with four start squares there is no single number, and the
// one that would be printed if there were is the one that hides the asymmetry.
func reportSlots(wins, games [rules.MaxSnakes]int) {
	var occupied []int
	for slot := range rules.MaxSnakes {
		if games[slot] > 0 {
			occupied = append(occupied, slot)
		}
	}
	if len(occupied) == 0 {
		return
	}

	decisive := 0
	for _, slot := range occupied {
		decisive += wins[slot]
	}
	if decisive == 0 {
		return
	}

	// A duel keeps the single-line form it has always printed. Every SLOT line
	// quoted in BENCHMARK.md was recorded under it, and a transcript that no
	// longer matches what the tool prints is a reader's afternoon.
	if len(occupied) == 2 {
		lo, hi := wilson(wins[occupied[0]], decisive, 1.96)
		fmt.Printf("SLOT   slot%d won %d, slot%d won %d (slot%d %.1f%%, 95%% CI [%.1f%%, %.1f%%])\n",
			occupied[0], wins[occupied[0]], occupied[1], wins[occupied[1]], occupied[0],
			100*float64(wins[occupied[0]])/float64(decisive), 100*lo, 100*hi)
		if lo > 0.5 || hi < 0.5 {
			fmt.Printf("       the starting slot is worth a measurable amount here, which is\n" +
				"       why the arms alternate rather than one always going first.\n")
		}
		return
	}

	// The contested count is printed because the rotation is only exactly
	// balanced when the game count is a multiple of snakes*(snakes-1). At 200
	// games and four snakes it is not: arm A sits in slot 0 fifty-one times and
	// in slot 3 forty-eight, so those squares are contested 101 and 98 times.
	// That is small, and it is a disclosed limitation rather than a hidden one
	// only if the number is on the line.
	skewed := false
	for _, slot := range occupied {
		lo, hi := wilson(wins[slot], decisive, 1.96)
		share := 100 * float64(wins[slot]) / float64(decisive)
		fmt.Printf("SLOT   slot%d won %d of %d decisive, contested %d (%.1f%%, 95%% CI [%.1f%%, %.1f%%])\n",
			slot, wins[slot], decisive, games[slot], share, 100*lo, 100*hi)
		if even := 1 / float64(len(occupied)); lo > even || hi < even {
			skewed = true
		}
	}
	if skewed {
		fmt.Printf("       a start square is worth a measurable amount here, which is why the\n" +
			"       arms rotate through them rather than one always going first.\n")
	}
}

func verdict(p float64, arms [2]arm, aWins, bWins int) string {
	if p >= 0.05 {
		return "not separated"
	}
	if aWins > bWins {
		return arms[0].name + " is better"
	}
	return arms[1].name + " is better"
}

func firstError(results []gameResult) error {
	for _, r := range results {
		if r.err != nil {
			return r.err
		}
	}
	return nil
}

func safeDiv(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

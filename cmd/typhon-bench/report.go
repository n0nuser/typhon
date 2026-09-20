package main

import (
	"fmt"
	"time"
)

// report prints what the run found, and what it is not entitled to claim.
func report(label string, arms [2]arm, results []gameResult, elapsed time.Duration) {
	var (
		aWins, bWins, draws, failed int
		turns                       int
		perArm                      [2]counters
		// Per slot, so that start-position bias is visible rather than
		// averaged away. Two identical bots went 29-23-8 for the predecessor,
		// which is six points before anything is changed.
		slotWins  [2]int
		slotGames [2]int
	)

	for _, r := range results {
		if r.err != nil {
			failed++
			continue
		}
		turns += r.turns
		perArm[0].add(r.perArm[0])
		perArm[1].add(r.perArm[1])

		aSlot := 0
		if !r.aFirst {
			aSlot = 1
		}
		slotGames[0]++
		slotGames[1]++

		switch r.outcome {
		case armAWon:
			aWins++
			slotWins[aSlot]++
		case armBWon:
			bWins++
			slotWins[1-aSlot]++
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

	// Start-position bias, measured rather than assumed.
	if slotGames[0] > 0 {
		total := slotWins[0] + slotWins[1]
		if total > 0 {
			lo, hi := wilson(slotWins[0], total, 1.96)
			fmt.Printf("SLOT   slot0 won %d, slot1 won %d (slot0 %.1f%%, 95%% CI [%.1f%%, %.1f%%])\n",
				slotWins[0], slotWins[1], 100*float64(slotWins[0])/float64(total), 100*lo, 100*hi)
			if lo > 0.5 || hi < 0.5 {
				fmt.Printf("       the starting slot is worth a measurable amount here, which is\n" +
					"       why the arms alternate rather than one always going first.\n")
			}
		}
	}

	fmt.Printf("TURNS  %d total, %.1f mean per game\n", turns, safeDiv(turns, played))

	// Per-path counts. A claim about a component is not readable until these
	// say the component was reached: the predecessor once measured a whole
	// batch against a feature whose threshold was never crossed.
	for i, a := range arms {
		c := perArm[i]
		fmt.Printf("PATHS  %-10s turns=%d mean_depth=%.2f max_depth=%d nodes=%d "+
			"fallbacks=%d aborted=%d all_losing=%d first_death=%.0f\n",
			a.name, c.turns, safeDiv(c.depthSum, c.turns), c.maxDepth, c.nodes,
			c.fallbacks, c.aborted, c.allLosing, safeDiv(c.firstDeath, played))
		if !a.random && c.depthSum == 0 {
			fmt.Printf("       WARNING %s never completed a single depth. Whatever this run\n"+
				"       measured, it was not that arm's search.\n", a.name)
		}
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

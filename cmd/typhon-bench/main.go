// Command typhon-bench plays batches of games in process and reports whether
// one configuration beat another.
//
// It exists because the predecessor's conclusions did not survive a second seed
// block. The same configuration went 8-10-2 on one set of twenty seeds and
// 25-9-6 on the next; every comparison it published was inside that range. The
// three things that fix it are built in rather than left to discipline:
//
//   - Games run under a node budget rather than a clock, so a game is
//     bit-for-bit reproducible from its seed. All the variance left is seed
//     sampling, which more games actually address; without it, every run also
//     measures whatever else the machine was doing.
//   - Both arms play the same seeds and the test is McNemar's on the games
//     where they disagreed, so the seed's own difficulty cancels instead of
//     being averaged over.
//   - The arms swap starting slots every other seed, and the per-slot rates are
//     printed separately. Two identical bots went 29-23-8 over there, so one
//     slot is worth about six points before anyone changes anything.
//
// Everything is in process: the official rules are driven directly rather than
// over HTTP, which removes the ports, makes running eight games at once
// trivial, and lets the decision counts be read out of a struct instead of
// grepped from a log.
//
// Usage:
//
//	typhon-bench -n 200 -a 'nodes=20000' -b 'nodes=20000,depth=1' -label search-pays
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	official "github.com/BattlesnakeOfficial/rules"

	"github.com/n0nuser/typhon/internal/eval"
	"github.com/n0nuser/typhon/internal/rules"
	"github.com/n0nuser/typhon/internal/search"
)

// arm is one configuration under test.
type arm struct {
	name     string
	spec     string
	cfg      search.Config
	nodes    int64
	maxDepth int
	random   bool
	seed     int
}

// runConfig is everything about the games themselves, shared by both arms.
type runConfig struct {
	gameType          string
	mapName           string
	variant           rules.Variant
	width, height     int
	maxTurns          int
	hazardDamage      int
	foodSpawnChance   int
	minimumFood       int
	shrinkEveryNTurns int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "typhon-bench:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		games    = flag.Int("n", 30, "games to play; nothing below 200 is fit to publish")
		seedBase = flag.Int("seed", 9000, "first seed; both arms play the same ones")
		specA    = flag.String("a", "", "arm A as key=value,... (see -help-spec)")
		specB    = flag.String("b", "", "arm B as key=value,...")
		nameA    = flag.String("name-a", "A", "name for arm A")
		nameB    = flag.String("name-b", "B", "name for arm B")
		label    = flag.String("label", "run", "label for this run")
		gameType = flag.String("rules", "standard", "standard, royale, constrictor or wrapped")
		mapName  = flag.String("map", "", "board map; defaults to the one the ruleset needs")
		width    = flag.Int("W", 11, "board width")
		height   = flag.Int("H", 11, "board height")
		maxTurns = flag.Int("max-turns", 1500, "abandon a game after this many turns")
		parallel = flag.Int("p", runtime.NumCPU(), "games to run at once")
		helpSpec = flag.Bool("help-spec", false, "list the arm spec keys and exit")
		calib    = flag.Bool("calibrate", false, "report how many nodes a wall-clock budget buys here, and exit")
	)
	flag.Parse()

	if *helpSpec {
		fmt.Print(specHelp)
		return nil
	}

	cfg, err := newRunConfig(*gameType, *mapName, *width, *height, *maxTurns)
	if err != nil {
		return err
	}

	if *calib {
		return calibrate(cfg)
	}

	a, err := parseArm(*nameA, *specA, 1)
	if err != nil {
		return fmt.Errorf("arm A: %w", err)
	}
	b, err := parseArm(*nameB, *specB, 2)
	if err != nil {
		return fmt.Errorf("arm B: %w", err)
	}
	arms := [2]arm{a, b}

	printHeader(*label, *games, *seedBase, cfg, arms, *parallel)
	if *games < 200 {
		fmt.Printf("NOTE   n=%d is a smoke run. Nothing below 200 games is fit to publish:\n"+
			"       the predecessor's identical configuration went 8-10-2 and 25-9-6\n"+
			"       on two seed blocks of twenty.\n\n", *games)
	}

	start := time.Now()
	results := playAll(cfg, arms, *seedBase, *games, *parallel)
	report(*label, arms, results, time.Since(start))
	return nil
}

// playAll runs the batch, alternating which arm starts in which slot.
func playAll(cfg runConfig, arms [2]arm, seedBase, games, parallel int) []gameResult {
	results := make([]gameResult, games)
	jobs := make(chan int)

	var wg sync.WaitGroup
	for range parallel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				// Arm A starts in slot 0 on even seeds and slot 1 on odd ones.
				results[i] = playGame(cfg, arms, seedBase+i, i%2 == 0)
			}
		}()
	}
	for i := range games {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}

func newRunConfig(gameType, mapName string, width, height, maxTurns int) (runConfig, error) {
	cfg := runConfig{
		gameType: gameType, width: width, height: height, maxTurns: maxTurns,
		foodSpawnChance: 15, minimumFood: 1, shrinkEveryNTurns: 25,
	}

	switch gameType {
	case official.GameTypeStandard:
		cfg.variant = rules.Standard
	case official.GameTypeWrapped:
		cfg.variant = rules.Wrapped
	case official.GameTypeConstrictor:
		cfg.variant = rules.Constrictor
	case official.GameTypeRoyale:
		cfg.variant = rules.Royale
		cfg.hazardDamage = 14
	default:
		return cfg, fmt.Errorf("unsupported ruleset %q: this bot implements standard, royale, constrictor and wrapped", gameType)
	}

	if mapName == "" {
		// Royale must be paired with the royale map or the hazards never
		// appear and the run measures a standard game wearing royale's name.
		mapName = "standard"
		if gameType == official.GameTypeRoyale {
			mapName = "royale"
		}
	}
	cfg.mapName = mapName
	return cfg, nil
}

const specHelp = `Arm spec keys, given as -a 'key=value,key=value':

  nodes=N        node budget per turn (default 20000). Fixed budgets are what
                 make a game reproducible; a clock budget would not be.
  depth=N        cap the search at N plies. depth=1 is the one-ply control,
                 which is the predecessor's whole decision procedure.
  opponents=N    how many rivals the search models properly (default 2)
  table=false    search without the transposition table
  random=true    the control arm: pick uniformly among legal moves. Without
                 this floor, a component that contributes nothing and one that
                 contributes a lot are indistinguishable.

  voronoi=N space=N tailreach=N length=N food=N centre=N confine=N
                 evaluation weights. Zero switches a term off.

Two arms should differ in exactly one key. The header prints the difference.
`

// parseArm turns a spec string into a configuration.
func parseArm(name, spec string, seed int) (arm, error) {
	a := arm{name: name, spec: spec, cfg: search.DefaultConfig(), nodes: 20000, seed: seed}
	if spec == "" {
		return a, nil
	}

	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return a, fmt.Errorf("%q is not key=value", part)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)

		if err := a.set(key, value); err != nil {
			return a, err
		}
	}
	return a, nil
}

func (a *arm) set(key, value string) error {
	number := func() (int, error) { return strconv.Atoi(value) }

	switch key {
	case "nodes":
		n, err := number()
		if err != nil {
			return fmt.Errorf("nodes: %w", err)
		}
		a.nodes = int64(n)
	case "depth":
		n, err := number()
		if err != nil {
			return fmt.Errorf("depth: %w", err)
		}
		a.maxDepth = n
	case "opponents":
		n, err := number()
		if err != nil {
			return fmt.Errorf("opponents: %w", err)
		}
		a.cfg.Opponents = n
	case "table":
		a.cfg.UseTable = value == "true"
	case "random":
		a.random = value == "true"
	case "voronoi", "space", "tailreach", "length", "food", "centre", "confine":
		n, err := number()
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		setWeight(&a.cfg.Weights, key, eval.Score(n))
	default:
		return fmt.Errorf("unknown key %q (try -help-spec)", key)
	}
	return nil
}

func setWeight(w *eval.Weights, key string, v eval.Score) {
	switch key {
	case "voronoi":
		w.Voronoi = v
	case "space":
		w.Space = v
	case "tailreach":
		w.TailReach = v
	case "length":
		w.Length = v
	case "food":
		w.Food = v
	case "centre":
		w.Centre = v
	case "confine":
		w.Confine = v
	}
}

func printHeader(label string, games, seedBase int, cfg runConfig, arms [2]arm, parallel int) {
	fmt.Printf("RUN    %s: %d games, seeds %d-%d, %s on %s (%dx%d), %d at a time\n",
		label, games, seedBase, seedBase+games-1, cfg.gameType, cfg.mapName,
		cfg.width, cfg.height, parallel)
	fmt.Printf("ARM A  %-10s %s\n", arms[0].name, describe(arms[0]))
	fmt.Printf("ARM B  %-10s %s\n", arms[1].name, describe(arms[1]))

	if diff := difference(arms[0], arms[1]); diff == "" {
		fmt.Printf("DIFF   none - this is a floor run, measuring what one slot is worth\n" +
			"       before any change is attributed to anything.\n")
	} else {
		fmt.Printf("DIFF   %s\n", diff)
	}
	fmt.Println()
}

func describe(a arm) string {
	if a.random {
		return "random legal move (control)"
	}
	s := fmt.Sprintf("nodes=%d opponents=%d table=%v", a.nodes, a.cfg.Opponents, a.cfg.UseTable)
	if a.maxDepth > 0 {
		s += fmt.Sprintf(" depth<=%d", a.maxDepth)
	}
	w := a.cfg.Weights
	s += fmt.Sprintf(" weights[voronoi=%d space=%d tailreach=%d length=%d food=%d centre=%d confine=%d]",
		w.Voronoi, w.Space, w.TailReach, w.Length, w.Food, w.Centre, w.Confine)
	return s
}

// difference names exactly what separates the two arms, so that a run is
// self-documenting and an accidental two-variable comparison is visible in the
// first three lines of output rather than never.
func difference(a, b arm) string {
	var diffs []string
	add := func(format string, args ...any) { diffs = append(diffs, fmt.Sprintf(format, args...)) }

	if a.random != b.random {
		add("random %v vs %v", a.random, b.random)
	}
	if a.nodes != b.nodes {
		add("nodes %d vs %d", a.nodes, b.nodes)
	}
	if a.maxDepth != b.maxDepth {
		add("depth %d vs %d", a.maxDepth, b.maxDepth)
	}
	if a.cfg.Opponents != b.cfg.Opponents {
		add("opponents %d vs %d", a.cfg.Opponents, b.cfg.Opponents)
	}
	if a.cfg.UseTable != b.cfg.UseTable {
		add("table %v vs %v", a.cfg.UseTable, b.cfg.UseTable)
	}
	for _, key := range []string{"voronoi", "space", "tailreach", "length", "food", "centre", "confine"} {
		if x, y := weightOf(a.cfg.Weights, key), weightOf(b.cfg.Weights, key); x != y {
			add("%s %d vs %d", key, x, y)
		}
	}

	sort.Strings(diffs)
	joined := strings.Join(diffs, "; ")
	if len(diffs) > 1 {
		joined += "  <-- more than one variable; this run cannot attribute a result"
	}
	return joined
}

func weightOf(w eval.Weights, key string) eval.Score {
	switch key {
	case "voronoi":
		return w.Voronoi
	case "space":
		return w.Space
	case "tailreach":
		return w.TailReach
	case "length":
		return w.Length
	case "food":
		return w.Food
	case "centre":
		return w.Centre
	case "confine":
		return w.Confine
	}
	return 0
}

package main

import (
	"testing"
)

// Reproducibility is the property every comparison in BENCHMARK.md rests on.
// If the same seed and configuration can produce two different games, then the
// difference between two arms includes whatever else the machine was doing, and
// no number of games sees through that.
//
// It is tested as a property - play it twice, compare - rather than against a
// stored transcript, which would assert today's moves instead of the property.
func TestAGameIsReproducibleFromItsSeed(t *testing.T) {
	t.Parallel()

	// The budget is small on purpose. This asserts a property, not a standard
	// of play, and it plays every case twice under -race in a package that has
	// to finish inside the test binary's ten-minute deadline. A four-snake game
	// is four searchers a turn rather than two; at the previous 600 nodes over
	// 80 turns the four-snake cases alone ran the package past that deadline.
	cfg, err := newRunConfig("standard", "", 11, 11, 50)
	if err != nil {
		t.Fatal(err)
	}
	a, err := parseArm("a", "nodes=300", 1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := parseArm("b", "nodes=300,opponents=1", 2)
	if err != nil {
		t.Fatal(err)
	}
	arms := [2]arm{a, b}

	// Four snakes as well as two. The four-snake path seats two extra players
	// from derived seeds and decides its outcome down a branch a duel never
	// reaches, and reproducibility is the property every number in BENCHMARK.md
	// rests on - it is not inherited from the duel case.
	cases := []struct {
		seed  int
		seats seating
	}{
		{seed: 9000, seats: seatFor(0, 2)},
		{seed: 9001, seats: seatFor(1, 2)},
		{seed: 9000, seats: seatFor(0, 4)},
		{seed: 9001, seats: seatFor(5, 4)},
	}

	for _, tc := range cases {
		first := playGame(cfg, arms, field(t), tc.seed, tc.seats)
		second := playGame(cfg, arms, field(t), tc.seed, tc.seats)

		if first.err != nil || second.err != nil {
			t.Fatalf("seed %d, %d snakes: %v / %v", tc.seed, tc.seats.snakes, first.err, second.err)
		}
		if first.outcome != second.outcome || first.turns != second.turns {
			t.Errorf("seed %d, %d snakes played out differently: %v in %d turns, then %v in %d",
				tc.seed, tc.seats.snakes, first.outcome, first.turns, second.outcome, second.turns)
		}
		if first.perArm != second.perArm || first.neutral != second.neutral {
			t.Errorf("seed %d, %d snakes: counters differ:\n%+v %+v\n%+v %+v",
				tc.seed, tc.seats.snakes, first.perArm, first.neutral, second.perArm, second.neutral)
		}
	}
}

// Running games in parallel must not change any of them. Each game owns its
// own state, evaluator and transposition table precisely so that this holds;
// if it did not, a run's result would depend on how the scheduler interleaved
// it, and the node budget would be buying reproducibility it did not have.
func TestParallelismDoesNotChangeTheResults(t *testing.T) {
	t.Parallel()

	cfg, err := newRunConfig("standard", "", 11, 11, 80)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := parseArm("a", "nodes=600", 1)
	b, _ := parseArm("b", "nodes=600,opponents=1", 2)
	arms := [2]arm{a, b}

	sequential := playAll(cfg, arms, field(t), 9000, 4, 2, 1)
	concurrent := playAll(cfg, arms, field(t), 9000, 4, 2, 4)

	for i := range sequential {
		if sequential[i].outcome != concurrent[i].outcome ||
			sequential[i].turns != concurrent[i].turns ||
			sequential[i].perArm != concurrent[i].perArm {
			t.Errorf("game %d differed between sequential and parallel runs:\n%+v\n%+v",
				i, sequential[i], concurrent[i])
		}
	}
}

// Swapping the slots must swap the result, not change it. If it does not, the
// slot is doing work that is being attributed to the arm.
func TestSlotAssignmentIsHonoured(t *testing.T) {
	t.Parallel()

	cfg, err := newRunConfig("standard", "", 11, 11, 80)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := parseArm("full", "nodes=600", 1)
	b, _ := parseArm("oneply", "nodes=600,depth=1", 2)
	arms := [2]arm{a, b}

	res := playGame(cfg, arms, field(t), 9000, seatFor(0, 2))
	if res.err != nil {
		t.Fatal(res.err)
	}
	// The searching arm should be reaching real depth and the one-ply arm
	// should not, whichever slot they are in.
	if res.perArm[0].depthSum <= res.perArm[0].turns {
		t.Errorf("arm A averaged depth <= 1 (%d over %d turns); it is not searching",
			res.perArm[0].depthSum, res.perArm[0].turns)
	}
	if res.perArm[1].maxDepth > 1 {
		t.Errorf("arm B reached depth %d despite depth=1", res.perArm[1].maxDepth)
	}

	swapped := playGame(cfg, arms, field(t), 9000, seatFor(1, 2))
	if swapped.err != nil {
		t.Fatal(swapped.err)
	}
	if swapped.perArm[1].maxDepth > 1 {
		t.Errorf("after swapping slots, arm B reached depth %d despite depth=1", swapped.perArm[1].maxDepth)
	}
}

// Two arms that differ in more than one thing cannot attribute a result, so the
// header has to say so where someone will see it.
func TestTheHeaderNamesTheDifference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		specA      string
		specB      string
		wantDiff   string
		wantWarned bool
	}{
		{name: "identical arms are a floor run", specA: "nodes=1000", specB: "nodes=1000", wantDiff: ""},
		{name: "one variable", specA: "nodes=1000", specB: "nodes=1000,depth=1", wantDiff: "depth 0 vs 1"},
		{
			// Two keys that do not read a default: `depth` is unset unless
			// asked for, so this case cannot go vacuous the way a weight can
			// when the shipping configuration happens to zero it.
			name: "two variables are called out", specA: "nodes=1000", specB: "nodes=2000,depth=1",
			wantWarned: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, err := parseArm("a", tc.specA, 1)
			if err != nil {
				t.Fatal(err)
			}
			b, err := parseArm("b", tc.specB, 2)
			if err != nil {
				t.Fatal(err)
			}

			got := difference(a, b)
			if tc.wantDiff != "" && got != tc.wantDiff {
				t.Errorf("difference = %q, want %q", got, tc.wantDiff)
			}
			if tc.wantDiff == "" && !tc.wantWarned && got != "" {
				t.Errorf("difference = %q, want none", got)
			}
			if tc.wantWarned && !contains(got, "more than one variable") {
				t.Errorf("difference = %q, want it to warn about multiple variables", got)
			}
		})
	}
}

func TestArmSpecRejectsNonsense(t *testing.T) {
	t.Parallel()

	for _, spec := range []string{"nodes", "nodes=lots", "wobble=3"} {
		if _, err := parseArm("a", spec, 1); err == nil {
			t.Errorf("parseArm(%q) accepted a spec it should not have", spec)
		}
	}
}

// field builds the neutral configuration a test needs. Duels never seat one,
// so for those it only has to exist.
func field(t *testing.T) arm {
	t.Helper()
	a, err := parseNeutral("nodes=300", arm{nodes: 300})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Rotation has to give every contestant every start square the same number of
// times. If it does not, the slot's own contribution lands on whichever arm sat
// in the better square more often, and no sample size sees through that.
func TestRotationGivesEveryArmEverySquare(t *testing.T) {
	t.Parallel()

	for _, snakes := range []int{2, 3, 4} {
		games := 12 * snakes * (snakes - 1)
		var aIn, bIn []int
		aIn, bIn = make([]int, snakes), make([]int, snakes)

		for i := range games {
			s := seatFor(i, snakes)
			if s.a == s.b {
				t.Fatalf("%d snakes, game %d: both arms seated in slot %d", snakes, i, s.a)
			}
			if s.snakes != snakes {
				t.Fatalf("%d snakes, game %d: seating reports %d", snakes, i, s.snakes)
			}
			aIn[s.a]++
			bIn[s.b]++
		}

		want := games / snakes
		for slot := range snakes {
			if aIn[slot] != want || bIn[slot] != want {
				t.Errorf("%d snakes over %d games: slot %d held arm A %d times and arm B %d, want %d each",
					snakes, games, slot, aIn[slot], bIn[slot], want)
			}
		}
	}
}

// The duel rotation must be the alternation this harness has always used, or
// every number in BENCHMARK.md stops being reproducible from its seed block.
func TestTheDuelRotationIsUnchanged(t *testing.T) {
	t.Parallel()

	for i := range 8 {
		got := seatFor(i, 2)
		wantA := 1
		if i%2 == 0 {
			wantA = 0
		}
		if got.a != wantA || got.b != 1-wantA {
			t.Errorf("game %d: arm A in slot %d, arm B in slot %d; want A in %d",
				i, got.a, got.b, wantA)
		}
	}
}

// Survival beats elimination turn, and a survivor's zero turn count must never
// be read as an early death. This is the one branch where getting it backwards
// silently hands every capped game to the loser.
func TestOutlivedPrefersSurvivalThenLaterDeath(t *testing.T) {
	t.Parallel()

	died := func(turn int) counters { return counters{deaths: 1, deathTurnSum: turn} }
	survived := counters{}

	tests := []struct {
		name           string
		aAlive, bAlive bool
		a, b           counters
		want           outcome
	}{
		{name: "both survive to the cap", aAlive: true, bAlive: true, a: survived, b: survived, want: draw},
		{name: "A survives, B dies late", aAlive: true, a: survived, b: died(700), want: armAWon},
		{name: "B survives, A dies late", bAlive: true, a: died(700), b: survived, want: armBWon},
		{name: "both die, A later", a: died(300), b: died(100), want: armAWon},
		{name: "both die, B later", a: died(100), b: died(300), want: armBWon},
		{name: "both die on the same turn", a: died(200), b: died(200), want: draw},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := outlived(tc.aAlive, tc.a, tc.bAlive, tc.b); got != tc.want {
				t.Errorf("outlived = %v, want %v", got, tc.want)
			}
		})
	}
}

// A run that cannot vary its own flag has to refuse, not report a p-value. The
// arm this guards against ran for months and was cited twice as evidence; see
// findings/012.
func TestAComparisonThatCannotVaryIsRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		specA     string
		specB     string
		snakes    int
		wantInert bool
	}{
		{name: "opponents in a duel cannot differ", specA: "opponents=2", specB: "opponents=1", snakes: 2, wantInert: true},
		{name: "opponents with four snakes can", specA: "opponents=2", specB: "opponents=1", snakes: 4},
		{name: "opponents 3 vs 2 needs four snakes", specA: "opponents=3", specB: "opponents=2", snakes: 3, wantInert: true},
		{name: "a weight always varies", specA: "confine=6", specB: "confine=0", snakes: 2},
		{name: "a floor run varies nothing and is fine", specA: "nodes=600", specB: "nodes=600", snakes: 2},
		{name: "the random control is exempt", specA: "opponents=2", specB: "random=true", snakes: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, err := parseArm("a", tc.specA, 1)
			if err != nil {
				t.Fatal(err)
			}
			b, err := parseArm("b", tc.specB, 2)
			if err != nil {
				t.Fatal(err)
			}
			got := inert([2]arm{a, b}, tc.snakes)
			if tc.wantInert && got == "" {
				t.Errorf("inert() allowed a comparison that cannot vary anything")
			}
			if !tc.wantInert && got != "" {
				t.Errorf("inert() refused a valid comparison: %s", got)
			}
		})
	}
}

// A four-snake game must seat four snakes, give the two contestants their own
// slots, and leave the rest to a field that is actually playing. A field that
// never completes a depth is two obstacles, and the run is a duel with scenery.
func TestAFourSnakeGameSeatsAFieldThatPlays(t *testing.T) {
	t.Parallel()

	cfg, err := newRunConfig("standard", "", 11, 11, 60)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := parseArm("two", "nodes=300,opponents=2", 1)
	b, _ := parseArm("one", "nodes=300,opponents=1", 2)
	neutral, err := parseNeutral("", a)
	if err != nil {
		t.Fatal(err)
	}

	res := playGame(cfg, [2]arm{a, b}, neutral, 9000, seatFor(0, 4))
	if res.err != nil {
		t.Fatal(res.err)
	}
	if res.neutral.turns == 0 {
		t.Fatal("the field never moved; this was a duel with scenery")
	}
	if res.neutral.depthSum == 0 {
		t.Error("the field never completed a depth; it is an obstacle, not an opponent")
	}
	for i, c := range res.perArm {
		if c.turns == 0 {
			t.Errorf("contestant %d never moved", i)
		}
	}
	// Two neutrals pooled should have moved on roughly the scale of two
	// snakes, which is what distinguishes a seated field from a single stray.
	if res.neutral.turns < res.perArm[0].turns {
		t.Errorf("field took %d turns against contestant A's %d; two snakes should take more",
			res.neutral.turns, res.perArm[0].turns)
	}
}

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

	cfg, err := newRunConfig("standard", "", 11, 11, 80)
	if err != nil {
		t.Fatal(err)
	}
	a, err := parseArm("a", "nodes=600", 1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := parseArm("b", "nodes=600,opponents=1", 2)
	if err != nil {
		t.Fatal(err)
	}
	arms := [2]arm{a, b}

	for _, seed := range []int{9000, 9001} {
		first := playGame(cfg, arms, seed, true)
		second := playGame(cfg, arms, seed, true)

		if first.err != nil || second.err != nil {
			t.Fatalf("seed %d: %v / %v", seed, first.err, second.err)
		}
		if first.outcome != second.outcome || first.turns != second.turns {
			t.Errorf("seed %d played out differently: %v in %d turns, then %v in %d",
				seed, first.outcome, first.turns, second.outcome, second.turns)
		}
		if first.perArm != second.perArm {
			t.Errorf("seed %d: per-arm counters differ:\n%+v\n%+v",
				seed, first.perArm, second.perArm)
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

	sequential := playAll(cfg, arms, 9000, 4, 1)
	concurrent := playAll(cfg, arms, 9000, 4, 4)

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

	res := playGame(cfg, arms, 9000, true)
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

	swapped := playGame(cfg, arms, 9000, false)
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
			name: "two variables are called out", specA: "nodes=1000", specB: "nodes=2000,voronoi=0",
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

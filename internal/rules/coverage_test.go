package rules_test

import (
	"math/rand/v2"
	"testing"

	official "github.com/BattlesnakeOfficial/rules"

	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/rules"
)

// TestPlayoutsExerciseEveryRule guards the differential test against the way it
// can fail silently: by passing while checking nothing.
//
// The first version of that test picked moves uniformly at random. It passed,
// and it was worthless. Sixty games averaged 3.5 turns each, a snake grew twice
// in total across all of them, and a head-to-head - the rule the whole opening
// of every game turns on - never happened once. The bug it was written to catch
// could not have been caught.
//
// So the generator's reach is asserted rather than assumed. If a change to the
// move policy stops producing starvation, or hazard deaths, or head-to-heads,
// this fails instead of the differential test quietly going green.
func TestPlayoutsExerciseEveryRule(t *testing.T) {
	t.Parallel()

	variants := []struct {
		name         string
		variant      rules.Variant
		wrapped      bool
		hazardDamage int
		hazards      bool
		wantCauses   []rules.Cause
	}{
		{
			name: "standard", variant: rules.Standard,
			wantCauses: []rules.Cause{
				rules.OutOfHealth, rules.OutOfBounds, rules.SelfCollision, rules.HeadToHead,
			},
		},
		{
			// A torus has no walls, so OutOfBounds is unreachable by
			// construction - which is the point of the ruleset.
			name: "wrapped", variant: rules.Wrapped, wrapped: true,
			wantCauses: []rules.Cause{
				rules.OutOfHealth, rules.SelfCollision, rules.BodyCollision, rules.HeadToHead,
			},
		},
		{
			name: "constrictor", variant: rules.Constrictor,
			wantCauses: []rules.Cause{
				rules.SelfCollision, rules.BodyCollision, rules.HeadToHead,
			},
		},
		{
			name: "royale", variant: rules.Royale, hazardDamage: 14, hazards: true,
			wantCauses: []rules.Cause{
				rules.Hazard, rules.OutOfHealth, rules.SelfCollision, rules.HeadToHead,
			},
		},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()

			seen := map[rules.Cause]int{}
			grew, turns := 0, 0

			for seed := range 60 {
				rng := rand.New(rand.NewPCG(uint64(seed), 0x5eed))

				var hazardPoints []board.Point
				if v.hazards {
					for x := range 11 {
						hazardPoints = append(hazardPoints,
							board.Point{X: x, Y: 0}, board.Point{X: x, Y: 10})
					}
				}
				ours, _, _ := newBoards(t, 11, 11, 4, v.variant, official.GameTypeStandard,
					v.wrapped, v.hazardDamage, hazardPoints, rng)

				lengths := make([]int, len(ours.Snakes))
				for i := range ours.Snakes {
					lengths[i] = ours.Snakes[i].Len()
				}

				for range 400 {
					if ours.Over() {
						break
					}
					var moves [rules.MaxSnakes]board.Direction
					for i := range ours.Snakes {
						moves[i] = plausibleMove(ours, i, rng)
					}
					ours.Apply(&moves)
					turns++

					for i := range ours.Snakes {
						sn := &ours.Snakes[i]
						if sn.Len() > lengths[i] {
							grew++
						}
						lengths[i] = sn.Len()
						if !sn.Alive() {
							seen[sn.Cause]++
						}
					}
				}
			}

			for _, want := range v.wantCauses {
				if seen[want] == 0 {
					t.Errorf("no snake ever died of %q across 60 games; the playouts do not reach that rule", want)
				}
			}
			if grew == 0 {
				t.Error("no snake ever grew; the playouts never reach the feed stage")
			}
			if turns < 500 {
				t.Errorf("only %d turns across 60 games; the playouts die too fast to test anything", turns)
			}
			t.Logf("%d turns, %d growths, causes %v", turns, grew, seen)
		})
	}
}

package main

import (
	"math"
	"testing"
)

func TestWilson(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		successes, trials int
		wantLo, wantHi    float64
		tolerance         float64
		containsHalf      bool
		description       string
	}{
		{
			name:      "an even split at n=200 is still a wide interval",
			successes: 100, trials: 200, tolerance: 0.02,
			wantLo: 0.43, wantHi: 0.57, containsHalf: true,
			description: "even two hundred games leave a fourteen-point window",
		},
		{
			name:      "an even split at n=20 says almost nothing",
			successes: 10, trials: 20, tolerance: 0.05,
			wantLo: 0.30, wantHi: 0.70, containsHalf: true,
			description: "which is why n=20 was worthless",
		},
		{
			name:      "a lopsided result excludes a coin",
			successes: 18, trials: 20, tolerance: 0.05,
			wantLo: 0.70, wantHi: 0.99, containsHalf: false,
			description: "losing 2-18 is outside the noise, which is what made the control arm informative",
		},
		{
			name:      "no trials is not an opinion",
			successes: 0, trials: 0, wantLo: 0, wantHi: 0, tolerance: 0.0001,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lo, hi := wilson(tc.successes, tc.trials, 1.96)
			if math.Abs(lo-tc.wantLo) > tc.tolerance || math.Abs(hi-tc.wantHi) > tc.tolerance {
				t.Errorf("wilson(%d, %d) = [%.3f, %.3f], want about [%.2f, %.2f]",
					tc.successes, tc.trials, lo, hi, tc.wantLo, tc.wantHi)
			}
			if tc.trials > 0 {
				got := lo <= 0.5 && hi >= 0.5
				if got != tc.containsHalf {
					t.Errorf("interval [%.3f, %.3f] contains 0.5 = %v, want %v: %s",
						lo, hi, got, tc.containsHalf, tc.description)
				}
			}
		})
	}
}

func TestMcNemar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		aOnly, bOnly    int
		wantSignificant bool
		description     string
	}{
		{
			name:  "no disagreement is no evidence",
			aOnly: 0, bOnly: 0, wantSignificant: false,
			description: "identical arms carry no information at all",
		},
		{
			name:  "a four-game gap in twenty-four proves nothing",
			aOnly: 14, bOnly: 10, wantSignificant: false,
			description: "this is the shape of every result the predecessor over-read",
		},
		{
			name:  "two against eighteen is decisive",
			aOnly: 18, bOnly: 2, wantSignificant: true,
			description: "the coin control - far outside the range seed noise reaches",
		},
		{
			name:  "thirty against twelve is decisive",
			aOnly: 30, bOnly: 12, wantSignificant: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			chi, p := mcnemar(tc.aOnly, tc.bOnly)
			if got := p < 0.05; got != tc.wantSignificant {
				t.Errorf("mcnemar(%d, %d) = chi2 %.3f, p %.4f; significant = %v, want %v: %s",
					tc.aOnly, tc.bOnly, chi, p, got, tc.wantSignificant, tc.description)
			}
		})
	}
}

// The one thing a p-value must never do is depend on which arm was called A.
func TestMcNemarIsSymmetric(t *testing.T) {
	t.Parallel()

	for _, pair := range [][2]int{{18, 2}, {14, 10}, {30, 12}, {1, 0}} {
		_, forward := mcnemar(pair[0], pair[1])
		_, reversed := mcnemar(pair[1], pair[0])
		if math.Abs(forward-reversed) > 1e-12 {
			t.Errorf("mcnemar(%d, %d) p=%.6f but reversed p=%.6f", pair[0], pair[1], forward, reversed)
		}
	}
}

package main

import "math"

// The statistics here exist because the predecessor's conclusions did not
// survive contact with a second seed block: the same configuration went 8-10-2
// and 25-9-6. Two independent proportions at n=20 cannot see a real effect
// through that, and neither can two at n=200 if the effect is small.
//
// Two things fix it. Both arms play the *same* seeds, so the comparison is
// paired and the seed's own difficulty cancels; and the test is McNemar's, on
// the games where the arms disagreed, which is the only information a paired
// design actually carries.

// wilson returns a Wilson score interval for a proportion.
//
// Preferred to the textbook normal interval because it does not misbehave near
// zero or one and does not require a large sample to be honest - which matters
// precisely when someone is about to read too much into a four-game gap.
func wilson(successes, trials int, z float64) (lo, hi float64) {
	if trials == 0 {
		return 0, 0
	}
	n := float64(trials)
	p := float64(successes) / n
	denom := 1 + z*z/n
	centre := (p + z*z/(2*n)) / denom
	spread := z * math.Sqrt(p*(1-p)/n+z*z/(4*n*n)) / denom
	return centre - spread, centre + spread
}

// mcnemar returns the chi-square statistic and two-sided p-value for a paired
// comparison, given the counts of the two discordant cells.
//
// aOnly is the number of seeds where arm A won and arm B did not; bOnly the
// reverse. Seeds where both arms did the same thing carry no information about
// which is better and are excluded, which is the entire point of pairing.
//
// The continuity correction is applied because the discordant count is small in
// practice - a couple of dozen games out of two hundred - and without it the
// test is anti-conservative exactly where the temptation to over-read is
// greatest.
func mcnemar(aOnly, bOnly int) (chiSquare, pValue float64) {
	n := aOnly + bOnly
	if n == 0 {
		return 0, 1
	}
	diff := math.Abs(float64(aOnly-bOnly)) - 1
	if diff < 0 {
		diff = 0
	}
	chiSquare = diff * diff / float64(n)
	return chiSquare, chiSquareP(chiSquare)
}

// chiSquareP returns the two-sided p-value of a chi-square statistic with one
// degree of freedom, which is erfc of the square root of half of it.
func chiSquareP(x float64) float64 {
	if x <= 0 {
		return 1
	}
	return math.Erfc(math.Sqrt(x / 2))
}

// requiredPairs reports how many discordant games are needed before a given
// split could reach significance, which is the number worth quoting when a
// result is inconclusive.
func requiredPairs(observedRate float64) int {
	// Solving the corrected McNemar statistic for n at the 5% level, holding
	// the observed split. Approximate on purpose: it answers "is this within
	// reach of another hundred games, or nowhere near", not more.
	if observedRate <= 0.5 {
		return 0
	}
	d := 2*observedRate - 1
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(3.8415 / (d * d)))
}

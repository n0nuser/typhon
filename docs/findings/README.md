# Findings

One file per thing that was learned and would not have been guessed. They are
numbered in the order they were found, and they are not tidied up afterwards: a
finding that corrected an earlier one says so rather than replacing it.

The bar for a file here is that it changed what the code does, what a number
means, or what we believe about the previous project. Things that merely
confirmed an expectation are not findings, and neither are bugs whose only
lesson is "that was a typo".

| # | Finding |
| --- | --- |
| [001](001-a-passing-test-that-tested-nothing.md) | A differential test passed on its first run while exercising almost none of the rules |
| [002](002-a-suggestive-result-that-evaporated.md) | The largest weight in the evaluation looked worth 63% at n=30 and 52.5% at n=200 |
| [003](003-the-floor-that-was-never-there.md) | The start-position bias the predecessor blamed for its noise does not exist |
| [004](004-how-search-actually-wins.md) | Depth does not help a snake survive lost positions; it stops it reaching them |
| [005](005-counting-rivals-can-prefer-certain-death.md) | Ranking contested squares by contester count puts your own neck first |
| [006](006-chess-engine-timing-does-not-transfer.md) | Reading the clock every 1024 nodes overran a 2ms budget to 86ms |
| [007](007-latency-is-not-round-trip.md) | `you.latency` includes our own think time, so the obvious budget ratchets to zero |
| [008](008-a-counter-that-read-backwards.md) | The harness's `fallbacks` column was the win column wearing a failure's name |
| [009](009-a-quota-error-looks-exactly-like-a-hang.md) | A provider quota rejection is indistinguishable from the stall bug it was mistaken for |
| [010](010-one-in-four-intervals-misses.md) | One of four slot measurements excluded 50%, and that is arithmetic, not a finding |
| [011](011-constrictor-is-the-exception.md) | Search beats one ply in three rulesets out of four |
| [012](012-an-arm-that-compared-a-flag-with-itself.md) | A benchmark arm ran to completion, reported a p-value, and compared nothing |
| [013](013-breadth-buys-what-depth-buys.md) | Modelling a second rival wins while searching a ply shallower, by the same counter depth wins on |
| [014](014-the-budget-modelled-two-of-three-costs.md) | The turn budget's estimate was correct and every reply was still late, for ever |

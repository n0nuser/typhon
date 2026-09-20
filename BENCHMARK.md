# Benchmark log

What has actually been measured, including the results that did not go the way
we hoped. Every number here came from a run; nothing is projected.

## How to read this file

The predecessor's benchmark log ends with an admission worth repeating: the
measurement was too noisy to support most of what had been drawn from it. The
same configuration went 8-10-2 on one block of twenty seeds and 25-9-6 on the
next. Everything below is arranged so that does not happen again.

- **Nothing below 200 games is published as a finding.** Smaller runs appear
  here labelled as what they are.
- **The floor is measured before anything is compared.** Two identical bots are
  not a 50/50 proposition; the predecessor's were 29-23-8.
- **Comparisons are paired.** Both arms play the same seeds and the test is
  McNemar's on the games where they disagreed, so the seed's own difficulty
  cancels rather than being averaged over. A Wilson interval sits beside it.
- **Arms alternate starting slots**, and per-slot rates are reported.
- **A feature is shown to fire before it is benchmarked.** Path counts print
  beside every win column.
- **Negative results are reported.**

Reproduce anything here with `make phase-a` or `make phase-b`.

## The engine's own numbers

Measured with `go test -run '^$' -bench . -benchmem ./...` on an Intel i7-1065G7.

| What | Rate | Allocation |
| --- | --- | --- |
| `Reach` (flood fill, 11x11) | 918 ns | 0 allocs/op |
| `Voronoi` (4 snakes, 11x11) | 6.2 µs | 0 allocs/op |
| `Apply` + `Unapply` (one search node) | 210 ns | 0 allocs/op |
| `Evaluate` (4 snakes) | 8.1 µs | 0 allocs/op |
| Whole-turn search, 20,000 nodes | 60 ms | 0 allocs/op |

Zero allocation on the search path is not tidiness. A garbage collection pause
inside a 400ms turn budget is a missed deadline, and a missed deadline is scored
as no answer at all.

The ratio that shapes the search is evaluation against simulation: **one
evaluation costs about thirty nodes**. That is why the search evaluates at
leaves and leans on move ordering, rather than scoring everything it touches.

## What a turn budget buys

From `go run ./cmd/typhon-bench -calibrate`, which exists as a command rather
than as a number in a document so that it stays true after a change to the
search. Measured while the machine was otherwise busy, so these are on the
pessimistic side.

| Position | 50ms | 100ms | 200ms | 400ms |
| --- | --- | --- | --- | --- |
| Opening, 2 snakes | 5,568 nodes, depth 4 | 10,432, depth 5 | 23,744, depth 6 | 41,408, **depth 7** |
| Midgame, 2 snakes | 5,376, depth 5 | 11,264, depth 6 | 21,568, depth 6 | 45,568, **depth 7** |
| Midgame, 4 snakes | 2,624, depth 2 | 5,504, depth 2 | 11,712, depth 3 | 22,976, **depth 3** |

Two things follow, and the second is uncomfortable.

**In a duel the budget is worth about seven plies.** The predecessor searched
one. That is the whole thesis of this project, and it is the one number that
was never in doubt.

**In a four-snake game it is worth three.** Modelling two opponents means 64
joint moves per ply, so depth is bought at four times the price. Whether
modelling one opponent and searching deeper is the better trade is a real
question and it is not answered below - see the open questions at the end.

## Against the real engine

One game per supported ruleset, Typhon against Typhon, over HTTP through the
official `battlesnake` CLI at `-t 500`. This is the path the deployed bot
actually takes, round trip included.

| Ruleset | Turns |
| --- | --- |
| standard | 343 |
| royale | 97 |
| constrictor | 49 |
| wrapped | 505 |

The predecessor silently played `wrapped` with standard logic, treating the edge
of the board as fatal. Its own log records first deaths at turn 8 to 12.

From the 270-turn standard game, per snake:

```
turns=270 alive=true  mean_depth=8 max_depth=12 nodes=27288070
  fallbacks=1 aborted_depths=257 all_losing_turns=0
  engine_overhead=0s max_think=425ms timeout_overruns=0
```

**No turn was late in 540.** `max_think` of 425ms against a 500ms timeout is the
budget being spent rather than left on the table.

`engine_overhead=0s` is the budget model working, not failing. Played locally
there is no network to pay for, so the latency the engine reports is almost
entirely our own thinking, and the overhead it leaves is nothing - which is
correct, and leaves the whole timeout available. Subtracting the reported
latency whole, as the predecessor's code did, would have computed a 75ms budget
from a 500ms turn and collapsed the search to one ply, turn after turn.

## What the `depth=1` arm is, and is not

The headline comparison plays Typhon against Typhon with the search capped at
one ply. Both arms use the same evaluation, the same weights and the same node
budget, so the only thing that differs is how far ahead they look. That is a
clean answer to **does search depth pay, holding evaluation fixed**, which is
the question this project exists to ask.

It is **not** "Typhon beats `battlesnake-jev`". The one-ply arm is Typhon's own
evaluation — Voronoi control, tail reachability, opponent confinement — computed
on the root position. The predecessor's one-ply scorer was a different and much
simpler function, so the arm is a considerably stronger opponent than the old
bot was.

**That comparison has not been run.** It would need the old binary answering
over HTTP against the official CLI, which is about half a second a turn: two
hundred games is upwards of eight hours, and the in-process harness cannot drive
an external server. It is the obvious next measurement and it is honest to say
it is missing rather than let 28-2 stand in for it.

## Reading the floor run

The floor run plays two **identical** configurations against each other. Its
arm result is meaningless by construction - of course they are not different -
and quoting it would be a category error.

The number it exists to produce is the **slot** split: how often the snake that
starts in slot 0 wins, whatever is running in it. If that is not 50%, then every
other result in this file has to be read against the slot's own contribution
rather than against a coin. The predecessor never ran this control, which is how
a 56% result came to be read as a win.

So the floor section below leads with the slot binomial and its interval, and
every arm result quotes its own slot split beside it.

## Phase A — n=30, diagnostic only

**Nothing in this section is a finding.** Thirty games is the sample size that
produced 8-10-2 and 25-9-6 from one configuration. These runs exist to shake out
bugs and to show that the arms really differ, and they are reported because the
path counts are informative even where the win columns are not.

All runs: 11x11 standard, 4,000 nodes a turn, seeds 9000-9029, arms alternating
starting slots.

| Run | Result | Paired p | Reading |
| --- | --- | --- | --- |
| floor (identical configs) | 14-16 | 0.855 | not separated, as it must be |
| random control | search 30, coin 0 | <0.0001 | search is not a coin |
| search pays (full vs 1 ply) | 28-2 | <0.0001 | depth pays |

The floor's **slot** split, which is the number that run exists for: slot 0 won
11 and slot 1 won 19, so slot 0 took **36.7%, 95% CI [21.9%, 54.5%]**. The
interval contains 50%, so thirty games cannot establish a slot effect - but it
is the same direction and rough size as the predecessor's 29-23-8, and it is
why every arm below quotes its own slot split.

The harness's own power note on the floor run is worth quoting: *at this split,
about 865 decisive games would be needed to separate them.* That is the shape of
almost every conclusion the predecessor published.

### What the path counts say

This is the part n=30 is good for.

```
floor        alpha  mean_depth=5.47 max_depth=15 aborted=11947/12206 all_losing=16 deaths=16
             beta   mean_depth=5.45 max_depth=15 aborted=11931/12208 all_losing=14 deaths=14
random       search mean_depth=4.64 nodes=3202703  deaths=0
             coin   mean_depth=0.00 nodes=0        deaths=30 mean_death_turn=27
search-pays  full   mean_depth=5.08 max_depth=11 all_losing=1  deaths=2  mean_death_turn=170
             oneply mean_depth=1.00 max_depth=1  all_losing=31 deaths=28 mean_death_turn=175
```

**The arms are really different.** `oneply` averages exactly 1.00 ply and
`full` averages 5.08. Whatever the win column means, it is not measuring an
inert flag - which is precisely the failure the predecessor recorded.

**`aborted` is not a failure count.** Nearly every turn ends with an iteration
cut off by the budget: that is what iterative deepening is. The number that
would matter is `fallbacks`, the turns where *no* depth completed, and it is 28
in 5,277 - half a percent.

**And the mechanism is not what you would guess.** The one-ply bot does not die
sooner: both arms die around turn 170. It dies *more often*, and the counter
that explains why is `all_losing` - the turns on which every legal move was
contested by an equal-or-longer rival. The one-ply bot reached those positions
**31 times; the searching bot reached them once**. Search does not win by
surviving lost positions better. It wins by not walking into them.

## Phase B — n=200

_Running. Written from its output and not before._

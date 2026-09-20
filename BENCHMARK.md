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

11x11 standard, 4,000 nodes a turn, seeds 9100-9299, arms alternating starting
slots, paired McNemar on the discordant games.

### The floor, and a negative result

Two identical configurations, 200 paired games:

```
RESULT floor: alpha=98 beta=102 draw=0 of 200 games in 11m28s
PAIRED McNemar on 200 discordant games: chi2=0.04 p=0.8320 -> not separated
SLOT   slot0 won 102, slot1 won 98 (slot0 51.0%, 95% CI [44.1%, 57.8%])
```

The arm result is 98-102 and means nothing by construction. The number this run
exists for is the slot split, and it is **51.0%, 95% CI [44.1%, 57.8%]**.

**There is no detectable start-position advantage in this harness.** That is a
negative result, and it contradicts the thing the predecessor's log named as the
likely explanation for its noise:

> The likely culprit is a control that was never run. Both snakes in a duel are
> the same bot, so whichever *starting position* is better may simply win [...]
> The `position-bias` run puts two identical deterministic bots on the same
> sixty seeds to find out what the floor actually is.

That run came back 29-23-8 and was recorded as "the floor is ~56% for one slot,
not 50%". Re-analysed, **29-23 of 52 decisive games is 55.8% with a 95% interval
of [42.3%, 68.4%]** - which contains 50%. The floor that was cited as the reason
every other comparison had been measured against a wrong baseline was itself
never established. It was sixty games saying almost nothing, read as a finding.

The honest version, at 200 games with the slots alternating, is that the slot is
worth nothing anyone can measure. The random control below independently agrees:
its slot split is exactly 100-100.

The other number worth quoting from that run is the harness's own power note:
**about 9,604 decisive games would be needed** to separate two configurations
splitting 98-102. That is the scale at which small effects live, and it is worth
keeping in mind before reading anything into a handful of games.

### The random control

```
RESULT random-floor: search=200 coin=0 draw=0 of 200 games in 29s
PAIRED McNemar on 200 discordant games: chi2=198.00 p=0.0000 -> search is better
SLOT   slot0 won 100, slot1 won 100 (slot0 50.0%, 95% CI [43.1%, 56.9%])
PATHS  search  deaths=0
       coin    deaths=200 mean_death_turn=24
```

**200-0.** The coin dies in every game, on average at turn 24; the searching arm
did not die once. The predecessor's equivalent control lost 2-18, and that
result was what told it its model was doing something rather than nothing. This
is the same control with a much larger margin, and it is the floor that makes
the rest of the file readable: a component that contributes nothing and one that
contributes a lot are indistinguishable without it.

### Does search pay?

This is the question the project exists to ask. Both arms run the same
evaluation, the same weights and the same node budget; one is capped at a single
ply and the other is not.

```
RESULT search-pays: full=191 oneply=9 draw=0 of 200 games in 2m48s
SHARE  full took 95.5% of decisive games, 95% CI [91.7%, 97.6%]
PAIRED McNemar on 200 discordant games: chi2=163.81 p=0.0000 -> full is better
SLOT   slot0 won 105, slot1 won 95 (slot0 52.5%, 95% CI [45.6%, 59.3%])
```

**191-9.** 95.5% of decisive games, with a 95% interval of [91.7%, 97.6%] - far
outside both the measured floor of 51.0% [44.1%, 57.8%] and the slot's own
52.5% in this very run.

Yes. Search pays, and it is not close.

### Why it pays, which is not the obvious answer

```
PATHS  full    mean_depth=5.09 max_depth=15 all_losing=7   deaths=9   mean_death_turn=231
       oneply  mean_depth=1.00 max_depth=1  all_losing=213 deaths=191 mean_death_turn=186
```

The one-ply arm does not die much sooner - turn 186 against 231. What separates
them is `all_losing`: the turns on which **every legal move was contested by an
equal-or-longer rival**, with nothing left to choose but which coin to flip.

The one-ply arm reached those positions **213 times. The searching arm reached
them 7 times.** Thirty to one, over an identical number of turns.

That is the mechanism, and it is worth stating plainly because it is not what
"deeper search wins more" suggests. Looking further ahead does not help a snake
survive a lost position - nothing does, that is what lost means. It stops the
snake walking into one. The predecessor's own log opens by saying its bot
"cannot see a trap closing three moves out"; this is that sentence with a number
attached.

The n=30 run said the same thing at the same ratio - 31 against 1 - which is
some comfort that the smoke phase was measuring the real effect and not an
artefact of thirty seeds.

### One counter in these tables reads backwards

`fallbacks` in the Phase B output above is **not** a failure count. It was
incremented on any turn that completed no search depth, including the turns
where the game was already decided and the last snake standing was asked to
move - so it came out exactly equal to the number of games each arm won: 191
and 9 here, 98 against 102 deaths in the floor run, 200 of 200 for the random
control.

It is fixed in the harness now, and the fix is deliberately not backdated onto
these numbers. They were produced by the code as it stood, and quietly editing
a recorded measurement is the thing `docs/agents/rules.md` forbids.

The counter that does matter, `aborted`, covers nearly every turn in every run -
36,372 of 37,799 for the searching arm. That is not a failure either: an
iteration cut short by the budget is precisely what iterative deepening is.

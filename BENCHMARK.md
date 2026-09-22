# Benchmark log

What has actually been measured. Every number here came from a run; nothing is
projected.

**This file was rewritten from nothing on 2026-09-22.** Every figure it used to
carry was measured on a harness that paid one arm a bias worth 9.5 points -
[findings/015](docs/findings/015-the-board-was-choosing-the-winner.md) - and a
number measured through that cannot be adjusted into a trustworthy one, only
replaced. The old entries were deleted rather than annotated.

## How to read this file

- **Games come in mirrored pairs.** Each board is played twice, the second time
  with the two contestants' slots exchanged. `-n 400` is **200 boards**, not 400
  distinct ones. Historic figures in this project that said "n=200" meant 200
  distinct boards played once, and are not comparable to anything here.
- **The verdict is taken on boards, not games.** The two games of a pair are not
  independent, so counting them separately claims twice the evidence. An arm
  takes a board only by winning it from *both* slots; one each means the slot
  decided it, and that board is a **split** carrying no information.
- **The split count is the honest denominator.** A comparison with 160 splits in
  200 boards barely moved the game, whatever its p-value says.
- **Anything that separated was replicated** on a second seed block before it
  was believed. This was committed to before any of the results below were seen,
  and it changed three of them.
- Every arm varies **one key** against the rest of the evaluation. The run header
  prints what differs and refuses a comparison that cannot vary anything.

## The floor

Two identical configurations, which must not be separable. Run at both seed
parities and at both snake counts, because the bug this replaced was visible at
one parity and not the other:

| run | boards | decided | verdict |
| --- | --- | --- | --- |
| duel, seeds 9100 | 200 | **0** | not separated |
| duel, seeds 9101 | 200 | **0** | not separated |
| four snakes, seeds 9100 | 200 | **0** | not separated |
| four snakes, seeds 9101 | 200 | **0** | not separated |

```
RESULT floor: alpha=200 beta=200 draw=0 of 400 games
BOARDS alpha won 0, beta won 0, split 200 of 200 boards
PAIRED McNemar on 0 decided boards: chi2=0.00 p=1.0000 -> not separated
```

**Every board split, all two hundred of them.** Between identical deterministic
bots in a duel the starting square decides the game outright. It always did; the
previous harness read it as a skill difference whenever the seed parity leaned,
and returned 120-80 at p=0.0058 between a bot and a copy of itself.

That is what the split column is for. It is not noise - it is the share of the
outcome the configuration did not decide.

## Does the search work at all

Duel, 400 games, seeds 9100:

| arm | boards | split | p | reading |
| --- | --- | --- | --- | --- |
| search vs a coin | **200 - 0** | 0 | <0.0001 | it is not a coin |
| full vs one ply | **192 - 0** | 8 | <0.0001 | depth pays |

The one-ply arm did not take a single board from either. Note what the splits
say: against a coin, zero - the start square never decided one of those games,
because the coin lost from both slots. That is the shape of a real effect.

## The evaluation

Four snakes, 400 games a run, one key varied against the rest of the evaluation.
Every row below was run twice, on seeds 9100-9299 and again on 9300-9499.

| term | weight | boards, block 1 | boards, block 2 | verdict |
| --- | --- | --- | --- | --- |
| centre | 1 | off **103 - 14** | off **107 - 21** | **harmful** |
| tail reachability | 40 | off **69 - 27** | off **77 - 33** | **harmful** |
| Voronoi + confine | 10, 6 | off **69 - 29** | off **64 - 32** | **harmful** |
| food | 4 | on **79 - 16** | on **77 - 24** | **earns its place** |
| space | 6 | on **57 - 34** | on **64 - 27** | **earns its place** |
| length | 30 | 47 - 40 | 55 - 40 | not separated |

Every p on the harmful and helpful rows is below 0.002 in both blocks.

**Three of the seven terms were costing games.** Not unproven - measured, twice
each, as worse than absent.

### Is a harmful term merely mis-scaled

Testing a term at its shipped weight against zero answers "is this weight good",
not "is this term good". Both harmful terms were re-tested at another magnitude
before being written off:

| arm | boards | p | reading |
| --- | --- | --- | --- |
| tail reachability 10 vs 0 | 38 - 48 | 0.33 | not separated; useless rather than harmful |
| centre -1 vs 0 | off **85 - 11** | <0.0001 | worse pointing the other way |

So tail reachability is harmful at 40 and worthless at 10, and centre is harmful
in **both directions**. Neither is a scaling or a sign error. The terms have
nothing in them.

### And is a helpful term under-scaled

| arm | boards | split | p | reading |
| --- | --- | --- | --- | --- |
| space 12 vs 6 | 22 - 19 | **159** | 0.75 | doubling it barely changes the game |
| food 12 vs 4 | more **69 - 32** | 99 | 0.0003 | **did not replicate** |
| food 12 vs 4, second block | more 47 - **59** | 94 | 0.29 | reversed |
| food 24 vs 12 | 43 - 47 | 110 | 0.75 | not separated |

**Food at 12 is the one that got away.** It came back at p=0.0003 - a bigger
margin than several results in this file - and on a second seed block it not only
failed to reach significance, it pointed the other way. The rule that caught it
was written down before any of these arms were run, and this is the row that
proves the rule was worth having rather than a formality.

Space's 159 splits are worth reading too: four fifths of those boards were
decided by the start square rather than by doubling the weight. The p-value says
"not separated"; the split count says the change hardly participated.

## Where the time goes

Profile of `BenchmarkSearch`, 11x11, from `go tool pprof`:

```
  eval.Evaluate            85.6%
    └ control (Voronoi)    74.1%
  rules.Passable            9.3%
  everything else           ~5%
```

The search machinery - apply, unapply, move ordering, the transposition table -
is noise. The leaf evaluation is the entire cost, and one term of it was two
thirds of the whole search. Measured directly:

```
BenchmarkEvalWithControl-8    7,919 ns/op    0 allocs/op
BenchmarkEvalNoControl-8        917 ns/op    0 allocs/op
```

**8.6x cheaper without it**, which is about four times the nodes in a real
timed turn. That a term this expensive was also harmful is the single largest
result in this file.

One micro-optimisation was tried and **reverted**: rewriting `Passable` to drop
an integer divide and modulo per square, and to remove 121 bounds checks in
favour of 11. It measured 57,990 / 57,455 / 56,759 ns before and 58,025 / 57,455
/ 57,170 after - no difference at all. 9% in a profile did not translate, and an
optimisation without a number is a guess.

## Live, against the real engine

One game against `det` on Render's free tier, 0.1 CPU, before and after the
evaluation change and the two deadline fixes:

| | before | after |
| --- | --- | --- |
| turns survived | 122 | **203** |
| final length | 7 | **24** |
| mean depth | 6 | **8** |
| nodes per turn | 20,498 | **75,240** |
| timeouts | 1 | **0** |

It still lost. The game ended at turn 203 with a timeout the bot's own log did
not see - the engine recorded 500ms while our clock said 397ms - and
`getDefaultMove` walked the snake straight ahead into a body. That is
[findings/014](docs/findings/014-the-budget-modelled-two-of-three-costs.md) and
the budget ceiling that followed it.

## Open questions

**Does Typhon beat `battlesnake-jev`?** Not measured. It needs the old binary
answering over HTTP at about half a second a turn, so two hundred boards is
upwards of eight hours, and the in-process harness cannot drive an external
server.

**Is `Opponents: 2` right?** Unknown, and this project has now been wrong about
it twice. See [ADR 0005](docs/adr/0005-paranoid-with-opponent-reduction.md) and
[findings/013](docs/findings/013-breadth-buys-what-depth-buys.md), which is
withdrawn. `Opponents: 3` has never been tried.

**Is `length` worth keeping?** It does not separate in either direction, in
either block. It is kept because "not shown to help" is not "shown to hurt", and
it is now the only term in the evaluation in that position.

**Do any of these hold at the deployed budget?** Every arm runs at 4,000 nodes.
The live instance now does about 75,000 a turn. The direction of an effect
should not depend on the budget, but nothing here demonstrates that.

**Do they hold in a duel, or in the other rulesets?** Every evaluation arm above
is four snakes on 11x11 standard. Royale, wrapped and constrictor have not been
re-measured since the harness was rebuilt, and neither has the duel case.

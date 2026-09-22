# 015 — The harness let the board choose the winner, and called it the arm

**Found:** re-running the duel floor after the evaluation changed, and getting
120-80 between two **identical** configurations at p=0.0058.
**Changed:** `cmd/typhon-bench` plays every board twice with the contestants
exchanged, and scores boards rather than games. Every number measured before
that was discarded.

## The symptom

The floor run exists to prove that two identical bots cannot be separated. It
had always obliged - 98-102, p=0.83. After the evaluation changed it came back:

```
RESULT floor: alpha=120 beta=80 draw=0 of 200 games
PAIRED McNemar on 200 discordant games: chi2=7.61 p=0.0058 -> alpha is better
SLOT   slot0 won 102, slot1 won 98 (slot0 51.0%)
```

Arm A beat an identical copy of itself, significantly, while the per-slot split
stayed level at 102-98. The slot check this project relies on saw nothing,
because the slot was not the thing that was unbalanced.

## The one-line diagnosis

Re-run it with the seed base shifted by one:

```
RESULT floor-odd: alpha=80 beta=120 draw=0 of 200 games
PAIRED McNemar on 200 discordant games: chi2=7.61 p=0.0058 -> beta is better
```

**Exactly mirrored, to the game.** A property of the bots cannot invert when the
first seed changes by one. A property of the *boards* can, if which arm meets
which board is decided by the seed.

And it was. `playAll` alternated seating by game index - arm A in slot 0 on even
indices - while the seed also advanced by game index. The two were aliased: arm
A held slot 0 on every even seed, arm B on every odd one. Any systematic
difference between even-seeded and odd-seeded boards was therefore paid entirely
to one arm.

## What it was worth

Two runs of the same comparison exist that differ only in which configuration
sat in arm A, which makes the bias separable from the effect:

```
none - control = 30 games   (control in arm A)
none - control = 68 games   (none    in arm A)
```

Under an additive model that is a true gap of 49 games and an **arm-A bonus of
19 games - 9.5 points**, handed out for nothing. Every comparison this harness
ever published was measured through it.

## Why the existing checks missed it

The project already measures the floor before comparing anything, reports the
per-slot rate beside every arm, and treats n<200 as unpublishable. All three
were running. None could see this.

The per-slot number was **level**, and correctly so: across the whole run each
slot won about half the games. The imbalance was not slot-versus-slot, it was
*arm-versus-board*, and no statistic computed over the run as a whole contains
that. It only appears when the same board is played both ways round.

That is the general lesson, and it is the third time this project has met it:
**a control that aggregates cannot detect a bias that correlates.**

## The fix

Every board is now played twice, the second time with the contestants' slots
exchanged. Whatever a board is worth to the square it favours is paid to both
arms exactly once and cancels - without needing to know why the bias exists,
which was never established and no longer matters.

Scoring moves from games to boards, because the two games of a mirrored pair are
not independent: counting them separately would claim twice the evidence. An arm
takes a board only by winning it from both slots. One each means the slot
decided it, and the pair is a **split** carrying no information.

## The number that makes the point

On the mirrored harness, the duel floor between identical bots:

```
RESULT floor: alpha=200 beta=200 draw=0 of 400 games
BOARDS alpha won 0, beta won 0, split 200 of 200 boards
```

**Every board split. All two hundred.** In a duel between identical deterministic
bots the starting square decides the game outright - it always did - and the old
harness was reading that as a skill difference whenever the parity happened to
lean. Four floors were run to confirm the fix, at both seed parities and at two
and four snakes, and all four return zero decided boards.

## What it cost

Every published comparison, re-run. `BENCHMARK.md` was rewritten rather than
corrected: numbers measured through a 9.5-point bias cannot be adjusted into
trustworthy ones, only replaced.

And one conclusion inverted. `opponents=2` over `opponents=1` was recorded in
[013](013-breadth-buys-what-depth-buys.md) at 123-71, p=0.0003, with `two`
sitting in arm A. On the mirrored harness the same question answers the other
way. Two things differ between those runs - the harness, and an evaluation that
has since lost its largest term - so the honest statement is not that 013 was
wrong but that **nothing it rests on survived**.

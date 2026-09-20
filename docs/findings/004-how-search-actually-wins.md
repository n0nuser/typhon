# 004 — Depth does not help a snake survive lost positions

**Found:** reading the path counts beside the win column, which is what they are
printed for.
**Changed:** nothing in the code. It changed what the project believes it built.

## The result

Full search against the same evaluation capped at one ply, 200 paired games:

```
RESULT search-pays: full=191 oneply=9 draw=0 of 200 games
SHARE  full took 95.5% of decisive games, 95% CI [91.7%, 97.6%]
PAIRED McNemar on 200 discordant games: chi2=163.81 p=0.0000 -> full is better
```

191-9. The interesting part is not that number.

## The mechanism

```
PATHS  full    mean_depth=5.09 all_losing=7   deaths=9   mean_death_turn=231
       oneply  mean_depth=1.00 all_losing=213 deaths=191 mean_death_turn=186
```

The one-ply arm does **not** die much sooner: turn 186 against 231, over an
almost identical number of turns played. It dies more *often*.

What separates them is `all_losing` - the count of turns on which every legal
move was contested by an equal-or-longer rival, leaving nothing to choose but
which coin to flip. The one-ply arm reached those positions **213 times. The
searching arm reached them 7 times.** Thirty to one.

The n=30 run gave 31 against 1, the same ratio, which is some evidence this is
the real effect and not thirty seeds behaving oddly.

## What it means

Looking further ahead does not make a lost position survivable. Nothing does -
that is what lost means, and a paranoid search scores every move in such a
position identically. What depth buys is *not arriving there*: the trap that
closes in three moves is visible at depth four and invisible at depth one.

The predecessor's README says its bot "cannot see a trap closing three moves
out". This is that sentence with a number attached to it.

## Why it is worth writing down

"Deeper search wins more games" is true and explains nothing. Had the mechanism
turned out to be "the deeper bot survives longer once cornered", that would have
pointed at the evaluation's terminal scoring. It points instead at the space and
Voronoi terms doing their job several moves before the crisis - which is where
any further work on the evaluation should look.

It is also the reason the harness prints per-path counts at all. The win column
alone would have supported the wrong story just as comfortably.

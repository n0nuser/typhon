# What survives of `battlesnake-jev`'s published conclusions

The predecessor's `BENCHMARK.md` is unusually honest — it ends by saying the
measurement was too noisy to support most of what had been drawn from it. This
note takes that seriously and re-analyses the numbers it published, because
several of them are still quoted as settled and one of them shaped this
project's design.

Nothing here is a criticism of recording the data. Recording it is what makes
the re-analysis possible.

## The floor, which was never established

**Published:** a `position-bias` run of two identical deterministic bots went
**29-23-8** over sixty seeds, recorded as *"the baseline is ~56% for one slot,
not 50%"*, and named as the likely explanation for the project's noise.

**Re-analysed:** 29-23 of 52 decisive games is 55.8% with a 95% Wilson interval
of **[42.3%, 68.4%]**. That interval contains 50%. Sixty games cannot detect a
six-point effect, and this run did not detect one — it was read as having done so.

**Measured here:** 800 decisive games across four n=200 runs with the arms
alternating slots give **48.8%, [45.3%, 52.2%]** — an interval that excludes
55.8%.

See [finding 003](../findings/003-the-floor-that-was-never-there.md). The control
that was meant to explain the noise was itself an instance of it.

## The coin, which holds

**Published:** breaking close calls with a coin lost **2-18**.

**Re-analysed:** 2-18 is 10% with an interval of roughly [2.8%, 30.1%] — nowhere
near 50%. This one is real, and the project was right to treat it as the finding
that made the rest readable.

**Measured here:** the equivalent control is **200-0**. Same direction, larger
margin, and it confirms that a control arm is worth its runtime.

## The headline, correctly reported as null

**Published:** 200 duels, model-assisted against deterministic, **101-89-10**;
53.2% of decisive games against a 55.8% floor; reported as **-2.6 points,
z = -0.34**, and explicitly called not established.

**Re-analysed:** correct as far as it goes, and the honest conclusion was drawn.
Two things would have strengthened it. The comparison was **paired** — both arms
on the same seeds — so McNemar on the discordant games would have had more power
than a two-proportion z-test. And the floor it was measured against was itself
noise, so "-2.6 points" is a difference from a number that was never known.

The conclusion does not change: at n=200 the model neither helped nor hurt
measurably.

## The 8-10-2 / 25-9-6 problem, which is the real lesson

**Published:** the *same configuration* on two blocks of twenty seeds went
**8-10-2** and **25-9-6**.

This is the number worth carrying forward. It is not a statistical subtlety; it
is a demonstration that n=20 carries no information about anything, and every
arm-by-arm comparison in that file rests on it.

This project reproduced the phenomenon on its own code:
[finding 002](../findings/002-a-suggestive-result-that-evaporated.md) records a
term looking worth 63% at n=30 and 52.5% at n=200, with nothing changed but the
sample size.

## What was carried over unchanged

Three of the predecessor's own conclusions were adopted without re-testing,
because they are sound and cheap:

- **Committing to a direction is most of what breaking a tie is for.** A bot that
  picks randomly between equally-scored moves wanders, and wandering fills in its
  own escape routes. `board.Directions` is a fixed order for this reason.
- **A bigger board does not stop opening collisions.** They come from every snake
  being the same length, not from crowding.
- **Equal-length head-to-heads are as fatal as losses**, and counting how many
  rivals contest a square ranks options that the worst-case outcome cannot
  separate. Implemented here, and it is where
  [finding 005](../findings/005-counting-rivals-can-prefer-certain-death.md)
  came from.

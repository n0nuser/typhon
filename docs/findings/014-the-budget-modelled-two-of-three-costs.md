# 014 — The turn budget modelled two of a turn's three costs

**Found:** the first live games on Render's free tier, where every move took
501-536ms against a 500ms timeout and the snake died in every one.
**Changed:** `internal/server/store.go` — the budget now subtracts an
overshoot term.

## The model that was wrong

A turn costs the engine three things:

1. the network and the engine's own handling,
2. the search,
3. everything between the search's deadline expiring and the bytes leaving the
   process — decode leftovers, encode, write, and on a throttled instance the
   scheduler freezing us mid-encode until our next CPU period.

The budget subtracted the first and allocated the second. **The third was not
modelled at all**, because on the machine this was written on it is under a
millisecond.

On Render's free tier — 0.1 CPU, which is a tenth of a core enforced by a
scheduler quota — it is tens of milliseconds. The process is frozen for the rest
of its period the moment it exhausts its slice, and that can land anywhere,
including between the search stopping and the response being written.

## Why the feedback loop did not save it

There is an EWMA of the round trip less our own think time, added in
[007](007-latency-is-not-round-trip.md) so that our compute is not charged
twice. It works, and it is exactly why this went unnoticed: it converges
correctly on a number that is not the one that needed correcting.

With a 500ms timeout, 20ms of network and 60ms after the search:

```
budget    = 500 - 20 - 25          = 455ms
thought   = 455 + 60               = 515ms
roundTrip = 515 + 20               = 535ms   -> late
overhead  = 535 - 515              = 20ms    -> correct, and unchanged
```

**The estimate is right and the answer is still late, every turn, for ever.**
The loop has a fixed point above the timeout. Nothing in it can see the 60ms,
because that time is inside `thought`, which is the term the estimate exists to
exclude.

## The fix

Track the overshoot the same way: an EWMA of `thought - budget`, subtracted
alongside the overhead. It converges in one turn and it is self-calibrating, so
it costs nothing on hardware where the term really is a millisecond.

```
budget    = 500 - 20 - 60 - 25     = 395ms
thought   = 395 + 60               = 455ms
roundTrip = 455 + 20               = 475ms   -> inside
```

The first turn has no history, so `seedOvershoot` guesses 50ms, conservatively,
for the same reason `seedOverhead` does — the turn least able to answer quickly
is the first one on a cold instance.

## What this is the third of

[006](006-chess-engine-timing-does-not-transfer.md) found the deadline being
checked too rarely to be honoured. [007](007-latency-is-not-round-trip.md) found
the budget charging our own compute twice. This one finds a cost that was never
in the model.

All three are the same shape: **the deadline logic was verified against the
clock it could see, on hardware that made the unmodelled part free.** The bot
passed 2,018 live turns with zero late moves on a laptop, under a synthetic load
average of 13, and that was recorded as evidence the deadline holds under
contention. It was evidence that it holds under *that* contention. Eight busy
cores and a tenth of one throttled core are not the same failure mode: the first
makes every node slower, which the deadline check absorbs, and the second stops
the process outright, which it cannot.

The handoff's list of things believed but not measured had this at number five:
*"measured on this laptop" and "measured in production" are different claims.*
It was right, and it took one game to collect.

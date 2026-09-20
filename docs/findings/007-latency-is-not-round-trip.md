# 007 — `you.latency` includes our own think time

**Found:** reading the predecessor's code closely enough to port it.
**Changed:** `internal/server/store.go` - the budget model, before it was ever
written the obvious way.

## The obvious implementation, and why it is wrong

Every `/move` request carries `you.latency`: the round trip the engine measured
for our *previous* reply. The obvious budget is

```
budget = game.timeout - latency - margin
```

and it is wrong, because the engine measured send-to-receive. That interval
contains the network hop **and our own thinking**. Subtracting it whole charges
our compute twice.

The failure mode is a ratchet. Spend 400ms of a 500ms turn; the engine reports
~410ms; next turn's budget becomes 500 - 410 - margin ≈ 65ms; spend 65ms; the
engine reports ~75ms; the budget creeps back up; and the search oscillates
instead of settling. A bot that searched deeply on turn one converges on
searching shallowly forever.

`battlesnake-jev`'s `GameState.NoteEngineLatency` stores the raw value.

## What is actually wanted

The part we cannot see:

```
overhead   = latency_prev - ourThinkTime_prev     (clamped at >= 0, smoothed)
budget     = game.timeout - overhead - safetyMargin
```

Seeded at a conservative 50ms for the first turn, which has nothing to learn
from.

## The measurement that confirms it

Played locally there is no network to pay for, so the reported latency is almost
entirely our own thinking and the overhead it leaves should be nothing. Across
2,018 live turns:

```
engine_overhead=0s   max_think=443.947983ms   timeout_overruns=0
```

`engine_overhead=0s` is the model working, not failing. The naive version would
have computed a 65ms budget from a 500ms turn on the same data.

## Why it is a finding and not a design note

It is a place where the obviously-correct reading of a field is wrong, the wrong
version still works, and the damage is invisible - a bot quietly searching one
ply instead of eight, with no error and no log line. The only way to notice is to
ask what the number is measuring rather than what it is called.

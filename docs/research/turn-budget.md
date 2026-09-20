# Where the 500ms goes

The premise of this project is that the predecessor left almost all of its turn
budget unspent. This is the arithmetic behind that claim, and what the budget
actually turned out to be worth.

## The premise

- Battlesnake allows **500ms per turn, including the round trip**.
- The engine reported **44ms** round trip from Frankfurt, taken from
  `you.latency`, which arrives on every request.
- The predecessor's decision took **0.14ms at p50**.

So roughly 400ms per turn was going unused.

## The field that is not what it looks like

`you.latency` is the round trip the engine measured for our **previous** reply,
and that interval contains our own thinking. Budgeting
`timeout - latency - margin` charges our compute twice and ratchets the budget
down turn after turn — a bot that searched deeply on turn one converges on
searching shallowly forever.

What is wanted is the part we cannot see:

```
overhead = latency_prev - ourThinkTime_prev    (clamped >= 0, EWMA)
budget   = game.timeout - overhead - safetyMargin
```

Seeded conservatively at 50ms for the first turn. See
[finding 007](../findings/007-latency-is-not-round-trip.md).

Measured over 2,018 live turns played locally: `engine_overhead=0s`, which is
correct — there is no network to pay for — and leaves the whole timeout
available. The naive version would have computed ~65ms from a 500ms turn.

## What the budget buys

From `typhon-bench -calibrate` on a quiet machine, current search:

| Position | 50ms | 100ms | 200ms | 400ms |
| --- | --- | --- | --- | --- |
| Opening, 2 snakes | 13,920 nodes, d5 | 21,148, d6 | 39,103, d7 | 87,632, **d7** |
| Midgame, 2 snakes | 10,635, d6 | 21,014, d6 | 46,739, d7 | 97,864, **d8** |
| Midgame, 4 snakes | 5,339, d2 | 11,007, d3 | 23,970, d3 | 42,327, **d3** |

A node costs about **4.5µs in a duel** and about **9µs with four snakes**.

Two readings, and the second is uncomfortable:

- **A duel budget is worth about seven or eight plies.** The predecessor searched
  one. That is the thesis.
- **A four-snake budget is worth three.** Modelling two opponents costs 64 joint
  moves a ply. Whether modelling one and searching deeper is the better trade is
  [ADR 0005](../adr/0005-paranoid-with-opponent-reduction.md)'s open question.

## Where the time goes inside a node

| Operation | Cost | Allocation |
| --- | --- | --- |
| `Apply` + `Unapply` | 210 ns | 0 allocs/op |
| flood fill | 918 ns | 0 allocs/op |
| Voronoi, 4 snakes | 6.2 µs | 0 allocs/op |
| full evaluation | 8.1 µs | 0 allocs/op |

**One evaluation costs about thirty simulator nodes.** The search is
evaluation-bound, not simulation-bound, which is why it evaluates at leaves and
leans on move ordering. Any further optimisation belongs in `internal/eval`.

Zero allocation throughout is not tidiness: a GC pause inside a 400ms budget is a
missed deadline.

## Holding the deadline

The guarantee was broken at first — a 2ms budget overran to 86ms under load
because the clock was read every 1024 nodes, on reasoning imported from chess
engines where a node costs tens of nanoseconds rather than microseconds. See
[finding 006](../findings/006-chess-engine-timing-does-not-transfer.md).

With a per-node check, measured on a deliberately loaded machine (load average 13
on eight cores): **2,018 turns across eight snake-games, zero late**, `max_think`
428-451ms against 500ms.

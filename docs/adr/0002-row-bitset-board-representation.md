# 0002 — Represent the board as one `uint64` per row

**Status:** accepted

## Context

The evaluation is dominated by reachability questions: how much space can this
snake reach, who reaches each square first, can it still path to its own tail.
The obvious implementation is a BFS with a queue and a visited array, which is
what the predecessor used.

## Decision

Occupancy is a `[]uint64` with one word per row; bit `x` of word `y` is the
square `(x, y)`. Reachability is repeated dilation masked by the free squares,
iterated to a fixpoint.

## Why

Expanding a region by one square in every direction becomes four shifts and three
ORs *per row* rather than a queue operation per cell. Measured on 11x11:

| Operation | Cost | Allocation |
| --- | --- | --- |
| flood fill from a head | 918 ns | 0 allocs/op |
| Voronoi partition, 4 snakes | 6.2 µs | 0 allocs/op |

Scratch space is caller-owned so neither allocates, which matters because a GC
pause inside a turn budget is a missed deadline.

## The constraint it accepts

A row must fit in a word, so the board can be at most **64 wide**. The largest
official Battlesnake board is 25. `NewTopology` returns `ErrBoardSize` rather
than truncating, and the server falls back to the one-ply safe move and logs it.

This is a real limit and it is chosen deliberately: 64 is headroom rather than a
ceiling anyone reaches, and the alternative - a general bitset with rows spanning
word boundaries - costs a shift-and-carry in the innermost loop of the hottest
function in the program.

## The trap it creates

Every left shift must be masked, or a bit walks off the right edge of the board
and reappears on the next row. That is silent: the board keeps working and a
square on the far side becomes mysteriously blocked. `TestBitsetDoesNotLeakAcrossRows`
exists for exactly this, and runs at widths 1, 7, 11, 25, 63 and 64.

## What would make this wrong

A board wider than 64, or a profile showing the evaluation is no longer
reachability-bound. Neither has happened; the current profile says one evaluation
costs about thirty simulator nodes, so reachability is still where the time goes.

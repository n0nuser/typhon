# 010 — One of four slot measurements excluded 50%, and that is arithmetic

**Found:** pooling the slot splits after the fourth n=200 run.
**Changed:** the wording of the floor section in `BENCHMARK.md`.

## The situation

Four independent n=200 runs, each alternating which arm starts in which slot,
each producing a slot split and a 95% interval:

| Run | slot 0 | slot 1 | share | 95% CI | |
| --- | --- | --- | --- | --- | --- |
| floor | 102 | 98 | 51.0% | [44.1%, 57.8%] | |
| random control | 100 | 100 | 50.0% | [43.1%, 56.9%] | |
| search pays | 105 | 95 | 52.5% | [45.6%, 59.3%] | |
| tail reachability | 83 | 117 | **41.5%** | **[34.9%, 48.4%]** | ← excludes 50% |
| pooled | 390 | 410 | 48.8% | [45.3%, 52.2%] | |

The fourth row is significant at the 5% level. Read alone it says the starting
slot is worth nine points.

## Why it is not a finding

Four independent 95% intervals, each with a 5% chance of missing its target,
miss at least once about **19%** of the time. One significant result in four
tests is close to the expected outcome under the null hypothesis.

It is also the run whose own arm comparison was null, and it is contradicted by
the other three and by the pool of all 800 games.

## Why it is written down anyway

Because this project's whole argument is that the predecessor read noise as
signal, and the temptation here runs in the opposite direction: having published
"there is no slot bias" in [003](003-the-floor-that-was-never-there.md), the easy
thing is to not mention the row that disagrees.

Quoting the three supporting rows and omitting the fourth would be the same
error, committed in the other direction. Selective reporting is not more honest
when the selection happens to favour the conclusion you already argued for.

## What it costs

A sentence. The pooled figure is what the file leads with, the dissenting row is
in the same table, and the multiple-comparison arithmetic is stated so a reader
can disagree with the reasoning rather than having to discover the data.

If the effect is real, a fifth run at those seeds would show it again. That run
has not been done, and this file does not claim to have ruled it out - only that
one row in four is not evidence.

## Postscript: the fifth run

The opponent-confinement arm was later run at n=200 on the same seeds. Its slot
split is **103-97, 51.5%, [44.6%, 58.3%]** - the opposite direction, and an
interval containing 50%. Pooled across all five runs the slot is **493-507,
49.3%, [46.2%, 52.4%]**.

So the fifth run did not show the effect again, which is what this file said
would settle it. The tail-reachability row stays in the table as the one that
missed; five independent 95% intervals miss at least once about 23% of the time,
so the arithmetic reads the same way with one more test as it did with four.

# 003 — The start-position bias that does not exist

**Found:** running the floor control before any comparison.
**Changed:** how every other number in `BENCHMARK.md` is read, and the standing
of a conclusion in the predecessor's own log.

## The claim being tested

`battlesnake-jev`'s benchmark log closes by saying its measurements were too
noisy to support most of what had been drawn from them, and names a suspect:

> The likely culprit is a control that was never run. Both snakes in a duel are
> the same bot, so whichever *starting position* is better may simply win, and
> every result gets measured against a 50/50 assumption that was never checked.

It then ran that control, got 29-23-8, and recorded the floor as **"~56% for one
slot, not 50%"**.

## What 800 games say

Every Phase B run alternates which arm starts in which slot, so each is evidence
about the same question:

| Run | slot 0 | slot 1 | slot 0 share | 95% CI |
| --- | --- | --- | --- | --- |
| floor | 102 | 98 | 51.0% | [44.1%, 57.8%] |
| random control | 100 | 100 | 50.0% | [43.1%, 56.9%] |
| search pays | 105 | 95 | 52.5% | [45.6%, 59.3%] |
| tail reachability | 83 | 117 | 41.5% | [34.9%, 48.4%] |
| **pooled** | **390** | **410** | **48.8%** | **[45.3%, 52.2%]** |

**48.8%, [45.3%, 52.2%].** The interval contains 50% comfortably and excludes
55.8%.

## And the original number never said what it was read as saying

29-23 of 52 decisive games is 55.8% with a 95% interval of **[42.3%, 68.4%]**.
That interval contains 50%. Sixty games could not establish a slot effect, and
the run did not establish one - it was read as having done so.

So the control that was supposed to explain the noise was itself an example of
it. The floor cited as the reason every other comparison had a wrong baseline
was never measured to be anything other than even.

## What this does not say

It does not say start position never matters in Battlesnake. It says that in
*this* harness, on 11x11 standard with these snakes, it is not detectable across
800 games. A different board or a different bot could differ.

It also does not say the predecessor's noise had no cause. It says this was not
it, and the cause remains unidentified - though a sample size of twenty is a
sufficient explanation on its own and needs no accomplice.

## The practical consequence

The arms still alternate slots and the per-slot split is still printed for every
run. Measuring the control costs one run and removes a whole class of doubt;
assuming it costs nothing until the assumption is wrong. See also
[010](010-one-in-four-intervals-misses.md) for the row in that table that does
exclude 50%, and why it is not a finding.

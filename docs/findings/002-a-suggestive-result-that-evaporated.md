# 002 — A 63% result at n=30 became 52.5% at n=200

**Found:** promoting one structural arm from the smoke phase.
**Changed:** what `eval.Default().TailReach` is understood to be - a documented
guess rather than a supported component.

## What happened

Tail reachability is the largest weight in the evaluation, 40 against the next
largest at 30. The reasoning behind it is good: a snake that can still path to
its own tail can survive by following it, because the tail keeps vacating
squares ahead of the head.

The n=30 arm, with the term against without:

```
RESULT tailreach: with=19 without=11 draw=0 of 30 games
PAIRED McNemar on 30 discordant games: chi2=1.63 p=0.2012 -> not separated
```

19-11 is 63%. It is not significant and the harness said so, but it is exactly
the kind of number that gets a feature kept, and it is the shape of result the
predecessor's log records publishing.

Two hundred paired games on the same seed block:

```
RESULT tailreach-n200: with=105 without=95 draw=0 of 200 games in 12m14s
SHARE  with took 52.5% of decisive games, 95% CI [45.6%, 59.3%]
PAIRED McNemar on 200 discordant games: chi2=0.41 p=0.5245 -> not separated
PATHS  with     all_losing=103 deaths=95  mean_death_turn=367
       without  all_losing=109 deaths=105 mean_death_turn=367
```

**105-95.** The 63% became 52.5%, with an interval from 45.6% to 59.3%.

## The part that makes it convincing

The path counts agree with the win column, which is what separates "no effect"
from "underpowered". Both arms die on the same turn on average - 367 - having
reached almost the same number of positions where every move loses (103 against
109). If the term were doing what it is supposed to do, the arm without it
should be reaching hopeless positions more often. It is not.

## What was done about it

The term is kept. "Not shown to help" is not "shown to hurt", the reasoning is
sound, and removing a component on a null result is as unprincipled as keeping
one on a noisy positive. What changed is the description: every weight in
`eval.Default()` is now explicitly a guess that has not been falsified, and this
one has been specifically tested and specifically failed to demonstrate value.

## Why it is the most useful run in the project

Nothing about the feature changed between the two runs. Only the sample size
did. That is the entire argument for the n>=200 rule, demonstrated on this
project's own code rather than quoted from the previous one's postmortem - and
it is a reminder that the rule binds hardest when the small-sample result points
the way you were hoping.

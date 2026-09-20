# 0009 — Implement four rulesets; play the rest loudly as standard

**Status:** accepted

## Context

The engine runs many rulesets and many maps. The predecessor implemented
`standard` and played everything else with standard logic — including `wrapped`,
where leaving the board is not fatal, so it spent every wrapped game believing
the edge would kill it.

The failure was not the fallback. The failure was that **nothing said so.**

## Decision

Implement `standard`, `royale`, `constrictor` and `wrapped` properly. Play
anything else with standard logic **and log a warning naming it**, at `/start`
and in the info line of every game.

The same applies to maps: `variantFor` branches on `ruleset.name`, and `checkMap`
warns on any map outside the set whose furniture this module understands.

## Why warn rather than refuse

A wrong-but-playing snake beats a snake that returns nothing. The engine moves a
silent snake `up`. Refusing would convert a probable loss into a certain one.

## Why the map check is not cosmetic

Several official maps place their **walls as hazard squares**. This module reads
hazards as damage, not as obstacles, so on a maze board it would walk into a wall
believing the square costs health. That is worth a line in a log.

## Evidence the four work

Against the real engine over HTTP, and in the harness:

| Ruleset | Live game | Full vs one ply (n=30) |
| --- | --- | --- |
| standard | 453 turns | 191-9 at n=200 |
| royale | 249 turns | 29-1 |
| constrictor | 52 turns | 18-11-1, **not separated** |
| wrapped | 411 turns | 28-2 |

The constrictor row is a known exception and has its own file
([finding 011](../findings/011-constrictor-is-the-exception.md)).

## What would make this wrong

Playing a ladder where an unimplemented ruleset is common. The warning exists so
that the logs would say so.

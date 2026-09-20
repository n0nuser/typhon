# 0007 — Run tournament games in process, not over HTTP

**Status:** accepted

## Context

The predecessor's harness was a bash script: start four servers on four ports,
drive the official `battlesnake` CLI, grep the win line out of the log and the
decision counts out of slog output.

## Decision

`cmd/typhon-bench` imports the official rules and drives the loop the CLI drives
— `maps.SetupBoard`, then `PreUpdateBoard`, `Execute`, `PostUpdateBoard` each
turn — calling the decision function directly.

## Why

- **No ports, no server lifecycle, no readiness polling.** The failure modes of
  the bash harness were mostly these.
- **Parallelism is trivial**, and combined with a node budget
  ([ADR 0004](0004-two-budget-modes.md)) it is free of distortion. This is what
  makes n=200 a twelve-minute run rather than most of a day.
- **Counters are read from a struct**, not grepped out of a log. The
  per-decision-path counts that the measurement discipline depends on are
  reliable rather than scraped.

## What it gives up

It cannot drive an **external** server. That is a real cost and it is the reason
the headline comparison against `battlesnake-jev` has not been run: the old bot
answers over HTTP at about half a second a turn, which is upwards of eight hours
for 200 games. `make e2e` covers the HTTP path for correctness, but not at
tournament scale.

## What the harness insists on

Design choices that exist because of the predecessor's postmortem, not because
they were convenient:

- Both arms play the **same seeds**, and the test is **McNemar's** on the
  discordant games, so seed difficulty cancels instead of being averaged over.
- Arms **alternate starting slots** and per-slot rates print separately.
- The header **names exactly what differs** between the arms and warns when it is
  more than one thing, so an unattributable run is visible in the first three
  lines rather than never.
- Per-path counts print beside every win column.
- A **random control arm** is always available.
- Runs below 200 games print a notice saying they are not fit to publish.

# 009 — A provider quota rejection is indistinguishable from the stall bug

**Found:** smoke-testing the orchestrator loop before entering it.
**Changed:** `AGENTS.md` carries the diagnosis; `oc-run` should carry the check.

## Context

This repository ports a closed loop from a companion project: the LLM
orchestrates, `opencode` executes, and every byte of application source is
written by the executor. That loop has a documented failure mode
(anomalyco/opencode#48675): a provider stream that delivers nothing, with no
timeout, no retry and no exit. The wrapper `oc-run` detects it by watching for
output growth while the process is alive, with a 120-second first-byte deadline
and 300 seconds between bytes thereafter.

## What happened

A trivial brief - "reply with the word PONG" - was run through `oc-run` to
confirm the executor worked before marking a step `ACTIVE`.

Zero bytes. 139 seconds. CPU decaying from 40% to 5.5%. By every signal the
wrapper can observe, the documented stall.

It was not. `~/.local/share/opencode/log/opencode.log`:

```
level=ERROR message="stream error" providerID=opencode-go modelID=glm-5.3-flash
  error.error="AI_APICallError: Monthly usage limit reached. Resets in 13 days."
```

`auth.json` carried exactly one provider, so there was nothing to fall back to.

## The finding

**A hard API error and an infinite stall present identically at the wrapper
level.** `opencode run` swallowed the rejection, emitted nothing, and stayed
alive. `oc-run` would have killed it and retried twice, spending its entire
correction budget on an error that no retry can ever clear, and then reported
three stalls.

The wrapper's own rule - judge by output growth while the process lives, never by
elapsed time - is correct and was followed, and it still produced the wrong
diagnosis, because the rule distinguishes "stalled" from "finished" and this was
neither.

## See also

The shell traps this run also produced - `pkill -f` self-matching, bash
re-reading an edited script, a driver reporting success after aborting - are in
[`docs/agents/gotchas.md`](../agents/gotchas.md) alongside this one.

## What should be done about it

`oc-run` should grep the provider log for `level=ERROR` before concluding that
silence is a stall, and report the error rather than retrying. The distinction
costs one grep and saves three attempts and a misleading verdict.

## The consequence for this project

The loop was never entered. `AGENTS.md` §0 says that with no `ACTIVE` step the
loop is dormant and the repository's normal rules apply, so the work was done
directly - which made [005](005-counting-rivals-can-prefer-certain-death.md)'s
review step more necessary rather than less, since the loop's suspension of it
was never in force to begin with.

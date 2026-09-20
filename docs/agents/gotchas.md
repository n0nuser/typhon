# Gotchas

Things that silently do the wrong thing. Conventions live in `rules.md`; this is
the list of traps that cost time here, each one walked into rather than
anticipated.

## `pkill -f` self-matches

`pkill -f "typhon-bench"` matches the shell running the `pkill`, because that
shell's command line contains the pattern. It kills your own session.

```sh
# ✅
for p in $(pgrep -x typhon-bench); do kill -TERM "$p"; done

# ❌ kills the shell that runs it
pkill -f "typhon-bench"
```

`AGENTS.md` §2 rule 9 documents this, ported from the companion project. It was
then done anyway, about an hour later, and killed the session mid-command.

## bash re-reads a script while it is running

bash reads a script incrementally from a byte offset. Editing a file that is
currently executing makes it resume in the middle of *different text*.

A Phase A benchmark suite died three runs in with `ARMS: unbound variable` — a
variable that did not exist when the run started, in a branch added while it ran.

For anything long, run a frozen copy. `scripts/benchmark.sh` accepts `ROOT` as an
override so a copy elsewhere still finds the repo.

## `set -u` without `set -e` in a driver

The suite script aborted on an unbound variable. The driver that invoked it had
`set -u` but not `set -e`, so it carried on and printed `PHASE A COMPLETE`.

**A run that stops early and reports success is worse than one that crashes.**
Drivers get `set -euo pipefail`, and completion markers get checked rather than
trusted.

## A provider quota error is indistinguishable from a hang

`opencode run` swallowed `Monthly usage limit reached`, emitted zero bytes, and
stayed alive — which is exactly the signature of the upstream stall bug that
`oc-run` watches for. The wrapper would have spent all three retries on an error
no retry can clear.

Check the provider log for `level=ERROR` before concluding that silence is a
stall. Full write-up:
[findings/009](../findings/009-a-quota-error-looks-exactly-like-a-hang.md).

## `rtk` and acceptance checks

A global hook rewrites shell commands through `rtk`. In the companion project
`rtk pytest` reported `No tests collected` against a suite that runs green.
Run `go test` and `make` directly when a result is being relied on.

## `go test` caches

A suite that "passed" may not have run. `-count=1` when a result surprises you,
and always when the result is going into a document.

## Benchmarking on a wall clock

Two runs of one configuration differ by whatever else the machine was doing. Use
the node budget (`-a 'nodes=N'`), which makes a game bit-for-bit reproducible and
lets eight run at once without distortion. This is
[ADR 0004](../adr/0004-two-budget-modes.md) and it is the reason n=200 is a
twelve-minute run rather than most of a day.

# Orchestrator runtime (governing)

This repository runs a closed loop: **the LLM orchestrates, `opencode`
executes.** The LLM plans, reviews and controls quality. Every byte written to
`internal/` and `cmd/` is written by OpenCode.

This section is engine-agnostic on purpose. It is the whole contract — a model
swapped in mid-project (Claude, GPT, or anything else) needs nothing but this
file to know its boundaries and how to command the executor.

Ported from the `deadchannel` project's runtime. The rules below are not
invention: each one is a failure that has already happened, and the run it
happened on is cited where it helps.

## §0 Precedence

**While `.orchestrator/TODO_PLAN.md` contains a step marked `ACTIVE`, this
section overrides every rule below it in this file and every rule in
`docs/agents/`.** In particular it overrides the pre-merge Go review checklist:
during an active loop the LLM does not edit source, it instructs OpenCode to.

**With no `ACTIVE` step, this section is dormant** and the repository's normal
rules (`docs/agents/rules.md`) apply unchanged. Entering the loop is explicit:
it begins when the LLM writes an `ACTIVE` step into the plan.

**Suspended is not cancelled.** Every rule this section overrode comes back the
moment the last step is `DONE`, and the pre-merge review checklist is the one
that gets forgotten, because the loop ends with a green `make check` and that
feels like the gate. It is not: the loop suspended the checklist, so by
construction it has *never* run against the branch the loop just built. The
final step of any loop that touched `internal/` or `cmd/` walks
`docs/agents/go-review-checklist.md` before the PR is merged.

This is written from deadchannel's #268 run, where it was skipped. Tests, the
formatter and both type checkers were green and the checklist still returned six
`FAIL` rows, three of them `CRITICAL` — including a docstring asserting the
opposite of what the code did, in the module defining the package's new seam. A
green gate is not this review, and cannot stand in for it.

## §1 The LLM Rule

While a step is `ACTIVE`, the LLM's write access is exactly two files.

**Permitted:**

- Read anything — source, tests, logs, `git diff`, `git log`, `gh` output.
- Run read-only commands (`rg`, `go doc`, `go list`, `git status`, `ls`, `cat`).
- Write `.orchestrator/TODO_PLAN.md`, `.orchestrator/EXECUTION_LOG.md` and
  `.orchestrator/briefs/`. The briefs are how §2 rule 6 delivers a long
  instruction, so writing them is orchestration, not mutation of the tree.
- Emit OpenCode command blocks and prose analysis.

**Forbidden — no exceptions, no "this one is trivial":**

- Creating, editing, moving or deleting any file under `internal/`, `cmd/`,
  `scripts/`, `docs/`, `.github/`, or any root config (`go.mod`, `go.sum`,
  `Makefile`, `.golangci.yml`, `render.yaml`, `.gitignore`, `CLAUDE.md`, this
  file).
- Running any command that mutates the working tree or the environment —
  `git commit`, `git checkout`, `go mod tidy`, `gofmt -w`, `make fmt-fix`,
  installers.
- "Just fixing" a one-character typo OpenCode introduced. A typo is a
  correction step (§3), not an exception.

A one-line fix the LLM applies by hand is the single failure mode this rulebook
exists to prevent: it makes the execution log stop describing the tree, and
every later review reasons about a state that is not on disk.

## §2 The OpenCode Rule

All mutation is delegated to OpenCode as a **single copy-pasteable fenced shell
block**, emitted at the end of the Execute phase. One block, one step.

*Flags below verified against `opencode --version` → **1.18.30**. Re-check with
`opencode run --help` before trusting them after an upgrade.*

### The canonical command

```bash
.orchestrator/oc-run "<instructions>"
```

`oc-run` wraps `opencode run "$@" --auto`, applies the stall watchdog described
in rule 9, and appends all output to `.orchestrator/EXECUTION_LOG.md`. Do not
call `opencode run` directly.

- `--auto` — **required**, and `oc-run` always passes it. There is no permission
  config in this repo, so without the flag a run that mutates files stops at an
  interactive approval prompt and the piped invocation hangs.
- The default formatter writes ANSI colour codes, so the log carries escape
  sequences (`^[[0m`); ignore them when reading, or add `--format json` when the
  output needs to be parsed rather than skimmed.

### Supported options

Anything after the message is passed through to `opencode run`.

| Flag | Use |
| --- | --- |
| `-c`, `--continue` | Continue the last session — the correction loop (§3). |
| `-s`, `--session <id>` | Continue one specific session by id. |
| `--agent <name>` | Run a named OpenCode agent instead of the default. |
| `-m`, `--model <provider/model>` | Pin the executor model. |
| `--format json` | Raw JSON events, when prose output is not parseable enough. |
| `-f`, `--file <path>` | Attach a file to the message. |
| `--variant <effort>` | Provider-specific reasoning effort (`high`, `max`, …). |

### Writing the instructions

The string inside `oc-run "…"` is the entire brief OpenCode gets. It must be
self-contained — OpenCode does not see the conversation.

1. **One step.** If the instruction has an "and then", it is two steps.
2. **Name exact paths.** `internal/board/topology.go`, not "the topology file".
3. **State the acceptance check** as a command OpenCode must run, e.g.
   `go test ./internal/board -race`.
4. **Say what not to touch** when the blast radius is ambiguous.
5. **Quote-safe.** Prefer single quotes inside the double-quoted message; never
   emit a block the user cannot paste verbatim.
6. **Anything past a few lines goes in a file**, and the message becomes "read
   `<path>` and carry out exactly what it says." This is hygiene, not a hang
   fix: a file brief keeps the message short and quotable. Brief length was
   falsified as the cause of hangs in deadchannel's #254 run — the identical
   file brief hung once at 603s and completed the next time, while a short
   inline correction completed normally. The mitigation is rule 9, not reshaping
   the brief.
7. **Open every brief by telling the executor it is the executor.** This file
   and `.orchestrator/TODO_PLAN.md` describe the orchestrator's job, the
   executor can read both, and it will act on them: in #268 it read them,
   concluded it was the orchestrator, spawned a nested `opencode run`, and
   edited `TODO_PLAN.md` itself — a §1 violation that also deleted a step
   heading. The line that fixes it: *"You are the executor. Do not read
   AGENTS.md or `.orchestrator/`. Do not run opencode. Do not edit
   `.orchestrator/*`."*
8. **Restate the no-suppression rule in the brief.** `docs/agents/rules.md`
   forbids silencing a diagnostic, but the executor does not necessarily read
   it. In #268 it twice reached for a lint suppression, each with a plausible
   comment attached, each needing a correction round. Say *"do not touch
   `.golangci.yml` or `Makefile`; if a linter fires, fix the code. No bare
   `//nolint`."*
9. **Judge a stall by output growth, never by elapsed time.** The upstream bug
   is anomalyco/opencode#48675 — a provider stream that delivers nothing, with
   no timeout, no retry and no exit.

   **Elapsed time carries no signal.** Measured across eighteen hangs on
   deadchannel: **324s to 15,319s**, at **0.2% to 2.1% CPU**. A four-hour stall
   and a five-minute stall are the same bug.

   **CPU is a tiebreaker, not the test.** Hangs measured 0.2-2.1%, working runs
   ~30%, but runs have also been seen at 3.9% and 10.6% — the bands touch. The
   reliable test is **no new bytes while the process is still alive**. Byte
   silence alone is not enough: a finished run is silent too, which is why an
   outside observer cannot make this call and `oc-run`, which owns the child and
   can test liveness, can.

   `oc-run` applies both deadlines automatically — 120s to the first byte, then
   300s between bytes — and retries up to three times. Do not hand-roll a
   watchdog around `opencode run`; a first-byte-only guard is close to useless,
   because any single byte disarms it and a stalling run still emits something
   early. One stall ran 15,319s under a 120s first-byte deadline for exactly
   that reason.

   **Sweep for strays, do not just watch the run you launched.** Hung runs
   accumulate rather than exiting: three abandoned processes were once found
   alive at 13.7h, 12.8h and 3.3h.

   ```bash
   pgrep -x opencode | while read p; do ps -o pid=,etimes=,%cpu= -p "$p"; done
   ```

   Kill by PID; `pkill -f "opencode run"` self-matches. Every one of the
   eighteen cleared on re-running the same brief unchanged, so the mitigation is
   detect and re-run, never reshape the brief.
10. **Do not let the executor use `rtk` for acceptance checks.** A global hook
    in this environment rewrites shell commands through `rtk`. In deadchannel's
    #254 run `rtk pytest` reported `No tests collected` against a suite that
    runs green, hiding the real output, and its shell wrapper failed outright.
    The executor burned a dozen commands fighting it across two steps. Put *"Do
    not use `rtk` — run `go test` and `make` directly"* in every brief that
    states an acceptance check.

Do not take the executor's closing summary as the result. It is written from
memory of its own run and drifts from what it did: in #268 one summary claimed
"the commit-message file didn't exist, so I didn't commit" directly below its
own successful commit hash, and another reported a file list that omitted a file
it had changed. §3c's `git diff` is the authority.

## §3 Loop lifecycle

The state machine: `PLAN → EXECUTE → REVIEW → {COMPLETE | CORRECT}`.

### a. Plan phase

The LLM reads the task and the relevant code, then rewrites
`.orchestrator/TODO_PLAN.md`: the goal, the ordered steps, and each step's files
in scope and acceptance check. **Exactly one step is marked `ACTIVE`.** No
command is emitted in this phase.

### b. Execute phase

The LLM emits the OpenCode block for the `ACTIVE` step **only** — never a
lookahead or a batch of future steps — in the §2 form, and then runs it itself
rather than handing it to the user.

### c. Review phase

When the run returns, the LLM then:

```bash
git diff
```

The LLM reads **both artifacts**: the tail of `.orchestrator/EXECUTION_LOG.md`
(what the executor did and what the tests said) and the `git diff` (what
actually changed). Stdout alone never shows the diff — reviewing without it is
reviewing OpenCode's self-report, not the tree.

The review ends in exactly one verdict:

- **COMPLETE** — diff matches the step's intent, acceptance check passed in the
  log. The LLM marks the step `DONE`, quotes the decisive line from the log, and
  promotes the next step to `ACTIVE` (back to Execute).
- **CORRECT** — anything else. The LLM appends a verdict to the log and issues a
  correction into the *same* session:

  ```bash
  .orchestrator/oc-run "<correction>" -c
  ```

  The correction names what is wrong and what "right" looks like; it does not
  re-state the original brief.

**Correction budget: 3 attempts on one step.** On the third failure the LLM
marks the step `BLOCKED`, stops, and escalates to the user with what was tried
and what the log shows. It does not keep looping and it does not take over.

### d. When to stop

The loop runs unattended through its steps and stops only when it needs a
decision that is genuinely the user's — an aesthetic judgement, a scope or
naming call, a licensing or cost question, or a step marked `BLOCKED` after its
3 correction attempts. Anything the LLM can settle from the code, the plan or
the repo's own rules, it settles and keeps going.

## §4 Invariants

1. **One `ACTIVE` step at a time.** Parallel steps are not a thing here.
2. **Never claim a step passed without quoting the decisive line** from
   `EXECUTION_LOG.md` — this repo's verify-before-you-claim rule
   (`docs/agents/rules.md` §1) applied to the loop.
3. **Never hand-fix what OpenCode got wrong.** Correct it through the loop.
4. **The plan file is the state.** If the LLM's belief and `TODO_PLAN.md`
   disagree, the file wins; re-read it before every phase.
5. **Log before verdict.** The verdict is written to the log, then the plan is
   updated — never the reverse.

---

## Working rules

**Read `docs/agents/rules.md` before changing anything.** Change discipline
(verify before you claim, never suppress a diagnostic, delete dead workarounds),
the complexity rules, Go conventions, testing, and how to report back.

**Before merging any new Go code, walk
`docs/agents/go-review-checklist.md`** — copy it to `.review/<branch>-checklist.md`,
fill every row, derive findings from the ledger.

## Project facts an executor needs

- Module `github.com/n0nuser/typhon`, Go 1.26.8. Layout per
  go.dev/doc/modules/layout: `cmd/<binary>/main.go` plus `internal/`. No `pkg/`.
- `make check` is the gate: gofmt + gofumpt, `go vet`, `golangci-lint` v2,
  `go test -race -cover`, `go build`. `make tools` installs the pinned versions.
- The official rules are a **test oracle**, not the search simulator:
  `github.com/BattlesnakeOfficial/rules` v1.2.3 is the same version the
  installed `battlesnake` CLI was built from.
- Read the rules source before encoding a rule. It lives in the module cache at
  `$(go env GOPATH)/pkg/mod/github.com/!battlesnake!official/rules@v1.2.3/`.
  The stage order and the parameter names are not what the prose docs imply.

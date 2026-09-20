# 005 — Ranking contested squares by contester count puts your own neck first

**Found:** walking `docs/agents/go-review-checklist.md` before merge.
**Changed:** `internal/search/search.go` - two bugs, both silent.
**Severity:** critical. The bot played a worse move and nothing said so.

## The rule being implemented

From the brief:

> Count **how many** rivals can contest a square, not just the worst outcome.
> Sometimes every move is contested and you must pick the least contested.

This is right. Under a paranoid assumption, every square an equal-or-longer
rival can reach is a loss, so alpha-beta scores all four moves identically and
the choice falls through to whatever order they happened to be tried in. One
rival on the square is a coin flip; three is close to certain. The count is the
only thing left that distinguishes them.

## Bug one: the one-ply check overruled the search

The first implementation fired the tie-break whenever the *one-ply* safety check
found every neighbouring square contested, and replaced the searched move with
the one-ply answer.

But the one-ply check looks one square ahead. The search may have found that one
of those losses arrives five turns later than another, and a loss at ply 9 scores
strictly above a loss at ply 2 precisely so the bot plays on. Five more turns is
five more chances for the rival to blunder. Throwing that away for a heuristic
that cannot see past one square is a straight downgrade.

**Fixed** by firing on the search's own verdict: every root move terminal, *and*
terminal at the same distance. That is the only case where the score genuinely
has nothing left to say.

## Bug two, which is worse: the count preferred certain death

With the first bug fixed, a test printed this:

```
fallback=up search=down depth=1 score=-1048575
```

Head at (5,5), neck at (5,4). The search played `down` - into its own neck.

Ranking the tied moves purely by contester count does that, and it does it every
time. A self-collision has **no contesters**: nobody is competing for the square
your own body is standing on. So given a contested square and certain death, the
count ranks certain death first.

Walking into yourself is a certainty. A contested square is a coin flip. The coin
flip is strictly better, and the ranking preferred the certainty.

**Fixed** by ranking only squares that can actually be entered, with a test that
asserts the chosen move is one of them.

## Why neither was caught by anything else

`make check` was green throughout: `gofmt`, `go vet`, `golangci-lint`,
`go test -race`, and 96% coverage in the search package. Both bugs produce a
legal move of the correct type in a position the bot was probably going to lose
anyway. There is no crash, no race, no lint, and no log line - the bot simply
plays worse in exactly the positions where playing well is hardest to observe.

They were found by reading the code against a list, which is the argument
`AGENTS.md` §0 makes for the checklist not being replaceable by a gate.

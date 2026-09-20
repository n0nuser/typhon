#!/usr/bin/env bash
# Run a benchmark suite and tee every run into one log.
#
# Two phases, and the difference between them is not cosmetic. Phase A is n=30
# and exists to shake out bugs and confirm every decision path fires; nothing
# from it is fit to publish. Phase B is n=200, which is the smallest sample this
# project trusts, because the predecessor's identical configuration went 8-10-2
# and 25-9-6 on two blocks of twenty.
#
# Usage: scripts/benchmark.sh [a|b] [output-dir]
set -euo pipefail

PHASE="${1:-a}"
OUT="${2:-${TMPDIR:-/tmp}/typhon-bench}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

case "$PHASE" in
    a) GAMES=30; SEED=9000 ;;
    b) GAMES=200; SEED=9100 ;;
    *) echo "usage: $0 [a|b] [output-dir]" >&2; exit 2 ;;
esac

# The node budget both arms play under, and the turn cap that bounds a game.
#
# 4,000 nodes is about 40ms of thinking in a duel on the machine this was
# written on - run `typhon-bench -calibrate` for the ratio on yours. That is a
# tenth of what the deployed bot gets, and still roughly three hundred times
# the predecessor's entire decision. It is chosen for one reason: two hundred
# paired games have to actually finish. At 20,000 nodes a single arm comparison
# ran past ten minutes and the n=200 suite would have taken most of a day,
# which in practice means it does not get run - which is how a project ends up
# publishing n=20 again.
#
# Conclusions drawn here are conclusions at this depth. That is a real caveat,
# and it belongs in BENCHMARK.md rather than left implicit.
NODES="${NODES:-4000}"
MAX_TURNS="${MAX_TURNS:-800}"

mkdir -p "$OUT"
BIN="$OUT/typhon-bench"
go build -o "$BIN" "$ROOT/cmd/typhon-bench"

LOG="$OUT/phase-$PHASE.log"
: > "$LOG"

run() {
    local label="$1"; shift
    echo "=== $label ===" | tee -a "$LOG"
    "$BIN" -n "$GAMES" -seed "$SEED" -max-turns "$MAX_TURNS" -label "$label" "$@" 2>&1 | tee -a "$LOG"
    echo | tee -a "$LOG"
}

# The floor comes first, deliberately. Measuring two identical bots before
# comparing anything is the control the predecessor never ran, and it is the
# reason a 56% result was read as a win.
run floor        -a "nodes=$NODES" -b "nodes=$NODES" -name-a alpha -name-b beta
run random-floor -a "nodes=$NODES" -b "random=true" -name-a search -name-b coin
run search-pays  -a "nodes=$NODES" -b "nodes=$NODES,depth=1" -name-a full -name-b oneply

if [ "$ARMS" = all ]; then
    # The structural arms. Each is one flag against the same baseline, so a result
    # belongs to that flag and to nothing else.
    run voronoi      -a "nodes=$NODES" -b "nodes=$NODES,voronoi=0" -name-a with -name-b without
    run tailreach    -a "nodes=$NODES" -b "nodes=$NODES,tailreach=0" -name-a with -name-b without
    run confine      -a "nodes=$NODES" -b "nodes=$NODES,confine=0" -name-a with -name-b without
    run opponents    -a "nodes=$NODES,opponents=2" -b "nodes=$NODES,opponents=1" -name-a two -name-b one
    for g in royale constrictor wrapped; do
        run "ruleset-$g" -rules "$g" -a "nodes=$NODES" -b "nodes=$NODES,depth=1" \
            -name-a full -name-b oneply
    done
fi

echo "log: $LOG"

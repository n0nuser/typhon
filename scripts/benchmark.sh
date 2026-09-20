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

# The node budget both arms play under. Measured against real play: a 270-turn
# game at -t 500 averaged about 101,000 nodes a turn, so 20,000 nodes is
# roughly 85ms of thinking against the ~425ms the deployed bot actually gets.
# Conclusions drawn here are conclusions at that depth; the calibration is
# recorded in BENCHMARK.md so nobody has to rediscover the ratio.
NODES="${NODES:-20000}"

mkdir -p "$OUT"
BIN="$OUT/typhon-bench"
go build -o "$BIN" "$ROOT/cmd/typhon-bench"

LOG="$OUT/phase-$PHASE.log"
: > "$LOG"

run() {
    local label="$1"; shift
    echo "=== $label ===" | tee -a "$LOG"
    "$BIN" -n "$GAMES" -seed "$SEED" -label "$label" "$@" 2>&1 | tee -a "$LOG"
    echo | tee -a "$LOG"
}

# The floor comes first, deliberately. Measuring two identical bots before
# comparing anything is the control the predecessor never ran, and it is the
# reason a 56% result was read as a win.
run floor        -a "nodes=$NODES" -b "nodes=$NODES" -name-a alpha -name-b beta
run random-floor -a "nodes=$NODES" -b "random=true" -name-a search -name-b coin
run search-pays  -a "nodes=$NODES" -b "nodes=$NODES,depth=1" -name-a full -name-b oneply
run voronoi      -a "nodes=$NODES" -b "nodes=$NODES,voronoi=0" -name-a with -name-b without
run tailreach    -a "nodes=$NODES" -b "nodes=$NODES,tailreach=0" -name-a with -name-b without
run confine      -a "nodes=$NODES" -b "nodes=$NODES,confine=0" -name-a with -name-b without
run opponents    -a "nodes=$NODES,opponents=2" -b "nodes=$NODES,opponents=1" -name-a two -name-b one
run table        -a "nodes=$NODES" -b "nodes=$NODES,table=false" -name-a with -name-b without

for g in royale constrictor wrapped; do
    run "ruleset-$g" -rules "$g" -a "nodes=$NODES" -b "nodes=$NODES,depth=1" \
        -name-a full -name-b oneply
done

echo "log: $LOG"

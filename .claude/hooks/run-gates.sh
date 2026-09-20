#!/usr/bin/env bash
# Runs the repository gate before a push is allowed through.
#
# This mirrors scripts/pre-push rather than replacing it: the git hook catches a
# push made from a terminal, this catches one made through the agent's Bash
# tool, and the two can be installed independently.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
export PATH="$(go env GOPATH)/bin:$PATH"
echo "pre-push gate: make check"
make check

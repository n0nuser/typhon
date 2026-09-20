#!/usr/bin/env bash
# Runs the repository gate before a push is allowed through.
#
# This mirrors scripts/pre-push rather than replacing it: the git hook catches a
# push made from a terminal, this catches one made through an agent's shell
# tool, and the two can be installed independently.
#
# The hook is registered against every Bash call, so it decides for itself
# whether the command is a push. The settings file's own `if` field is not
# relied on: it is silently ignored by some versions, and a gate that runs on
# every command blocks ordinary work the moment the tree does not compile -
# which is most of the time while something is being written.
set -euo pipefail

payload="$(cat)"
command="$(printf '%s' "$payload" | python3 -c '
import json, sys
try:
    print(json.load(sys.stdin).get("tool_input", {}).get("command", ""))
except Exception:
    print("")
' 2>/dev/null || true)"

case "$command" in
    *"git push"*) ;;
    *) exit 0 ;;
esac

cd "$(git rev-parse --show-toplevel)"
PATH="$(go env GOPATH)/bin:$PATH"
export PATH
echo "pre-push gate: make check"
make check

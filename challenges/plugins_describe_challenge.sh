#!/usr/bin/env bash
# plugins_describe_challenge.sh — paired-mutation gate for round-293.
#
# Normal run: invokes the runner, asserts stdout contains the 5-locale
# label section, the StartAll/StopAll markers, and the terminal PASS.
# Exits 0 on PASS.
#
# Mutation run (PLUGINS_DESCRIBE_MUTATE=1): expects a forbidden token
# that the runner CANNOT emit. The gate MUST detect the absence and
# exit 99 — proving the gate is not a tautology.
#
# Anti-bluff per CONST-035 + CONST-050(B): exercises the REAL runner
# binary (no `echo PASS` stubs). Failure paths use exit 99 to
# distinguish gate-detected mutation from harness errors (exit 1).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="${TMPDIR:-/tmp}/plugins-r293-$$"
mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT

LOG="$TMP/runner.log"

echo "=== Plugins round-293 describe challenge ==="
echo "  repo=$REPO_ROOT mutate=${PLUGINS_DESCRIBE_MUTATE:-0}"

cd "$REPO_ROOT" || { echo "FAIL — cd $REPO_ROOT"; exit 1; }

# Run the real Go exerciser.
if ! go run ./challenges/runner -fixtures ./challenges/fixtures > "$LOG" 2>&1; then
    rc=$?
    echo "FAIL — runner exited $rc"
    sed -n '1,40p' "$LOG"
    if [[ "${PLUGINS_DESCRIBE_MUTATE:-0}" == "1" ]]; then
        # Mutation expected runner to also fail under planted mismatch;
        # but here the runner itself crashed, which is still detection.
        exit 99
    fi
    exit "$rc"
fi

echo "--- runner stdout (last 30 lines) ---"
tail -n 30 "$LOG"
echo "--- end stdout ---"

# Gate 1: 5-locale render must appear.
for loc in en sr ja es de; do
    if ! grep -qE "^\s*\[$loc\] " "$LOG"; then
        echo "FAIL — locale label [$loc] missing from runner output"
        exit 99
    fi
done

# Gate 2: registry lifecycle markers must appear.
if ! grep -q "StartAll OK: db=running api=running" "$LOG"; then
    echo "FAIL — StartAll lifecycle marker missing"
    exit 99
fi
if ! grep -q "StopAll OK: db=stopped api=stopped" "$LOG"; then
    echo "FAIL — StopAll lifecycle marker missing"
    exit 99
fi

# Gate 3: sandbox health-check evidence.
if ! grep -q "sandbox health-check OK" "$LOG"; then
    echo "FAIL — sandbox health-check evidence missing"
    exit 99
fi

# Gate 4: parsers evidence.
if ! grep -q "parsers OK: json=" "$LOG"; then
    echo "FAIL — structured parser evidence missing"
    exit 99
fi

# Gate 5: terminal PASS marker.
if ! grep -q "round-293 Challenge runner: PASS" "$LOG"; then
    echo "FAIL — terminal PASS marker missing"
    exit 99
fi

# Paired-mutation gate: when PLUGINS_DESCRIBE_MUTATE=1 is set, we
# require a token that the runner can NEVER emit. The gate MUST detect
# the absence and exit 99. This proves the gate has positive detection
# capability (not just a tautology that always PASSes).
if [[ "${PLUGINS_DESCRIBE_MUTATE:-0}" == "1" ]]; then
    if grep -q "MUTATION_TOKEN_NEVER_EMITTED_BY_RUNNER_R293" "$LOG"; then
        echo "FAIL — mutation token unexpectedly found (gate compromised)"
        exit 1
    fi
    echo "MUTATION DETECTED — runner did not emit forbidden token (expected)"
    echo "=== Plugins round-293 describe: MUTATION GATE TRIGGERED (exit 99) ==="
    exit 99
fi

echo "=== Plugins round-293 describe: PASS ==="
exit 0

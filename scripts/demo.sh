#!/usr/bin/env bash
# Drive the sample MigrationPlan end-to-end against the live engine.
#
# Resets the demo table, submits the plan with a fresh version (so it actually
# re-runs instead of returning a prior completed migration), watches it to a
# terminal state, then prints the backfill + state metrics.
set -euo pipefail

ADDR="${ENGINE_ADDR:-:8080}"
BASE="http://localhost${ADDR}"
PLAN="examples/add-shipping-index.yaml"

echo "==> resetting demo table"
make reset-demo >/dev/null

# Check if engine is running. If not, spin it up in the background.
ENGINE_PID=""
if ! curl -s --fail "${BASE}/healthz" >/dev/null 2>&1; then
    echo "==> engine not running on ${BASE}, starting it in the background..."
    # Ensure backend is built
    go build -o bin/engine ./cmd/engine
    # Start the engine in the background
    bin/engine > /tmp/mse-engine.log 2>&1 &
    ENGINE_PID=$!
    # Wait for engine to become healthy
    until curl -s --fail "${BASE}/healthz" >/dev/null 2>&1; do sleep 0.5; done
    echo "    engine started (PID: ${ENGINE_PID})"
fi

# Bump the plan version to a unique value so CreateMigration starts a new run.
VERSION="$(date +%s)"
TMP_PLAN="$(mktemp -t mse-plan).yaml"

cleanup() {
    rm -f "$TMP_PLAN"
    if [ -n "$ENGINE_PID" ]; then
        echo "==> stopping background engine (PID: ${ENGINE_PID})"
        kill "$ENGINE_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

sed "s/^version:.*/version: ${VERSION}/" "$PLAN" > "$TMP_PLAN"

echo "==> applying plan (version ${VERSION}) to ${BASE}"
ID="$(go run ./cmd/mgctl plan apply -f "$TMP_PLAN" --addr "$BASE" \
	| python3 -c 'import sys, json; print(json.load(sys.stdin)["migration_id"])')"
echo "    migration_id=${ID}"

echo "==> watching"
go run ./cmd/mgctl watch "$ID" --addr "$BASE"

echo "==> backfill + state metrics"
curl -s "${BASE}/metrics" | grep -E '^migrate_(backfill_rows|state_info)' | grep -v ' 0$' || true

#!/usr/bin/env bash
# Prove the headline safety claim with numbers: a throttled, batched backfill must
# NOT starve production writes. We start concurrent write load against the target
# table, kick off a real migration that backfills 50k rows over the same table,
# and report the write-latency percentiles measured DURING the migration.
set -euo pipefail

ADDR="${ENGINE_ADDR:-:8080}"
BASE="http://localhost${ADDR}"
PLAN="examples/add-shipping-index.yaml"
WORKERS="${WORKERS:-16}"
DURATION="${DURATION:-25s}"

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

echo "==> starting load: ${WORKERS} writers for ${DURATION}"
LOG="$(mktemp -t mse-load).log"

cleanup() {
    rm -f "$LOG" "${TMP_PLAN:-}"
    if [ -n "$ENGINE_PID" ]; then
        echo "==> stopping background engine (PID: ${ENGINE_PID})"
        kill "$ENGINE_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

DB_DSN="${DB_DSN:-postgres://mse:mse@localhost:5499/mse?sslmode=disable}" \
	go run ./cmd/loadgen -workers "$WORKERS" -duration "$DURATION" >"$LOG" 2>&1 &
LOAD_PID=$!

# Give the writers a moment to warm up, then launch the migration concurrently.
sleep 2

VERSION="$(date +%s)"
TMP_PLAN="$(mktemp -t mse-plan).yaml"
sed "s/^version:.*/version: ${VERSION}/" "$PLAN" > "$TMP_PLAN"

echo "==> applying migration (version ${VERSION}) while load runs"
ID="$(go run ./cmd/mgctl plan apply -f "$TMP_PLAN" --addr "$BASE" \
	| python3 -c 'import sys, json; print(json.load(sys.stdin)["migration_id"])')"
echo "    migration_id=${ID}"
go run ./cmd/mgctl watch "$ID" --addr "$BASE"

echo "==> waiting for load to finish"
wait "$LOAD_PID" || true
cat "$LOG"

echo "==> cutover parity metric"
curl -s "${BASE}/metrics" | grep -E '^migrate_cutover_parity' | grep -v ' 0$' || true

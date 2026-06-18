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

echo "==> starting load: ${WORKERS} writers for ${DURATION}"
LOG="$(mktemp -t mse-load).log"
trap 'rm -f "$LOG" "${TMP_PLAN:-}"' EXIT
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

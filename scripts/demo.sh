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

# Bump the plan version to a unique value so CreateMigration starts a new run.
VERSION="$(date +%s)"
TMP_PLAN="$(mktemp -t mse-plan).yaml"
trap 'rm -f "$TMP_PLAN"' EXIT
sed "s/^version:.*/version: ${VERSION}/" "$PLAN" > "$TMP_PLAN"

echo "==> applying plan (version ${VERSION}) to ${BASE}"
ID="$(go run ./cmd/mgctl plan apply -f "$TMP_PLAN" --addr "$BASE" \
	| python3 -c 'import sys, json; print(json.load(sys.stdin)["migration_id"])')"
echo "    migration_id=${ID}"

echo "==> watching"
go run ./cmd/mgctl watch "$ID" --addr "$BASE"

echo "==> backfill + state metrics"
curl -s "${BASE}/metrics" | grep -E '^migrate_(backfill_rows|state_info)' | grep -v ' 0$' || true

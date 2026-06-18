#!/usr/bin/env bash
# Demonstrate SLO-gated auto-rollback.
#
# Submits the chaos plan, which forces an SLO breach at the 25% canary step. The
# engine diverts to RollingBack, runs the rollback DDL (drops the new column +
# index), and ends in RolledBack. We then prove the table is back to its
# pre-migration shape (no shipping_class column).
set -euo pipefail

ADDR="${ENGINE_ADDR:-:8080}"
BASE="http://localhost${ADDR}"
PLAN="examples/add-shipping-index-chaos.yaml"

echo "==> resetting demo table"
make reset-demo >/dev/null

VERSION="$(date +%s)"
TMP_PLAN="$(mktemp -t mse-chaos).yaml"
trap 'rm -f "$TMP_PLAN"' EXIT
sed "s/^version:.*/version: ${VERSION}/" "$PLAN" > "$TMP_PLAN"

echo "==> applying CHAOS plan (version ${VERSION}) to ${BASE}"
ID="$(go run ./cmd/mgctl plan apply -f "$TMP_PLAN" --addr "$BASE" \
	| python3 -c 'import sys, json; print(json.load(sys.stdin)["migration_id"])')"
echo "    migration_id=${ID}"

echo "==> watching (expect RolledBack)"
go run ./cmd/mgctl watch "$ID" --addr "$BASE"

echo "==> rollback metrics"
curl -s "${BASE}/metrics" | grep -E '^migrate_(rollbacks_total|state_info|canary_step)' | grep -v ' 0$' || true

echo "==> proving table is back to pre-migration shape (shipping_class should be ABSENT)"
docker compose exec -T postgres psql -U mse -d mse -c \
	"SELECT column_name FROM information_schema.columns WHERE table_name='catalog_product' ORDER BY 1;"

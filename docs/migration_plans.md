# Migration Plan Reference (YAML)

MSE uses declarative migration plans written in YAML. The engine evaluates the plan and translates it into a zero-downtime expand/contract execution loop.

Here is a complete reference of all configuration fields.

---

## Configuration Schema

```yaml
# A unique identifier for this migration. Must be unique across all runs.
id: catalog-add-shipping-index

# Monotonically increasing version number for the plan.
version: 42

# The target database table being altered.
table: catalog_product

# The migration strategy. "expand-contract" is the only supported strategy.
strategy: expand-contract

# --- Phase 1: Expand ---
# Additive DDL statements that do not affect existing columns or block queries.
# Indexes must be created with "CONCURRENTLY" to avoid write-blocking locks.
expand:
  - "ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text"
  - "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cp_shipping ON catalog_product (shipping_class)"

# --- Phase 2: Backfill ---
# Configures the batched and throttled migration of data.
backfill:
  # The new target column to update.
  column: shipping_class
  # The query size of each update batch.
  batch_size: 5000
  # Throttle duration (in milliseconds) between batches to protect live traffic.
  throttle_ms: 20
  # SQL expression used to derive the new value from existing columns.
  source_expr: "CASE WHEN weight < 1 THEN 'light' WHEN weight < 10 THEN 'standard' ELSE 'freight' END"

# --- Phase 3: Verification ---
# Shadow-read verification details before entering the canary.
verify:
  # Verification mode. Currently "shadow-read" compares stored vs derived values.
  mode: shadow-read
  # The minimum percentage of matches required on a sample (e.g. 0.999 = 99.9% match).
  parity_threshold: 0.999
  # The percentage of table rows to sample for the initial parity check.
  sample_rate: 0.05

# --- Phase 4: Canary ---
# Progressive routing of read traffic on the new path.
canary:
  # Progressive steps representing percentage of traffic routed to the new path.
  steps: [1, 5, 25, 100]
  # Duration (in seconds) to observe and bake traffic at each step.
  bake_seconds: 120

# --- SLO Gates ---
# Metrics evaluated at each canary step. A breach will automatically halt and rollback.
slo:
  # Maximum p99 latency allowed (in milliseconds).
  max_p99_latency_ms: 50
  # Maximum error rate allowed (in percentage).
  max_error_rate_pct: 0.1
  # Minimum parity ratio allowed (e.g. 0.999 = 99.9% parity).
  min_parity: 0.999

# --- Phase 5: Contract ---
# Cleanup DDL statements. These are destructive (e.g. dropping old columns or legacy columns).
# They only run after a successful 100% canary completion and a full-table parity check.
contract:
  - "ALTER TABLE catalog_product DROP COLUMN IF EXISTS legacy_shipping"

# --- Rollback ---
# Applied automatically if the canary breaches an SLO or cutover verification fails.
# Should drop new columns/indexes and restore the database to its pre-migration shape.
rollback:
  - "DROP INDEX IF EXISTS idx_cp_shipping"
  - "ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class"

# Failure handling behavior. Options:
# - "rollback": automatically undo changes
# - "pause": halt execution and wait for human operator intervention
on_failure: rollback
```

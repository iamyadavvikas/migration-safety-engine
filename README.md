# Migration Safety Engine

A declarative, crash-resumable engine for **online Postgres schema migrations** using the
expand/contract pattern — with **shadow-read parity verification**, an **SLO-gated canary**,
**automatic rollback**, and a **drift scanner**. The whole lifecycle runs as a durable state
machine: every step is persisted, so the engine can be killed at any point and resumes exactly
where it left off.

It exists because the dangerous part of a schema change is never the `ALTER TABLE` — it's the
backfill that locks a hot table, the cutover that ships divergent data, and the rollback you
didn't write. This engine makes those steps **observable, gated, and reversible**.

---

## Why this is interesting

- **Expand/contract, done safely.** Add the new column + index `CONCURRENTLY`, backfill in
  throttled batches, verify, canary, *then* drop the legacy column. Readers and writers are
  never broken mid-flight.
- **Durable state machine.** State + checkpoint are written to Postgres in a transaction after
  every step. Kill `-9` the engine during a 50k-row backfill and it resumes from the last
  committed batch — it only touches rows where the target column `IS NULL`.
- **Shadow-read parity gate.** Before cutover the engine samples rows and compares the
  backfilled column against the source expression (`IS NOT DISTINCT FROM`). Cutover is blocked
  unless parity clears the threshold.
- **SLO-gated canary with auto-rollback.** Traffic shifts `1% → 5% → 25% → 100%`. At each step
  the engine checks p99 latency + error rate against the plan's SLO. A breach diverts the
  migration to `RollingBack` and runs the operator-authored rollback DDL — no human in the loop.
- **Cutover is a real point of no return.** Before the destructive `contract` step runs, the
  engine re-proves convergence over the **entire** table (not a sample). Any NULL or drifted row
  aborts the cutover and rolls back, so the legacy column is never dropped on top of bad data.
- **Proven, not just claimed, under load.** A bundled load generator hammers the target with
  concurrent writes *while a migration backfills* — measured write p99 stays in single-digit
  milliseconds, well inside the SLO (see [Load under backfill](#load-under-backfill-real-numbers)).
- **Drift scanner.** A read-only full-table scan (`mgctl drift-scan`) that re-proves a finished
  migration still has zero divergence between the column and its source expression. Exits
  non-zero on drift — drop it in CI/cron.
- **Operable out of the box.** Custom Prometheus metrics, 4 alert rules, and a provisioned
  Grafana dashboard ship in the repo.

---

## Architecture

```
            ┌──────────────┐     POST /plans (YAML→JSON)      ┌──────────────┐
  operator ─┤    mgctl     ├─────────────────────────────────▶│ engine (API) │
            └──────────────┘                                   └──────┬───────┘
                                                                      │ submit
                                                              ┌───────▼────────┐
                                                              │  state machine │
                                                              │    runner      │
                                                              └───────┬────────┘
   persist state+checkpoint in a tx after every step ────────────────┤
                                                              ┌───────▼────────┐
                                                              │   Postgres     │
                                                              │ control tables │  + target table
                                                              └────────────────┘
```

### State machine

```
Pending → Expanding → Backfilling → Verifying → Canary → Cutover → Contracting → Done
                                                   │
                                          SLO breach (on_failure: rollback)
                                                   ▼
                                            RollingBack → RolledBack
```

Each handler persists `(state, checkpoint)` to Postgres before returning. **Resume** re-loads the
last persisted state on startup and continues. Idempotent DDL (`IF [NOT] EXISTS`) and
`WHERE col IS NULL` backfill batches make every step safe to re-enter after a crash — including
the `RollingBack` state, so even an interrupted rollback resumes to completion.

The **`Cutover`** state is the commit gate: it recomputes parity across the whole table and only
then allows the destructive `Contracting` step. On drift it routes to `RollingBack` instead.

### Repo layout

| Path | What |
|------|------|
| `cmd/engine`   | HTTP control API + state-machine runner |
| `cmd/mgctl`    | operator CLI (`plan apply`, `status`, `watch`, `drift-scan`) |
| `cmd/loadgen`  | concurrent write load generator (measures target write p50/p95/p99) |
| `internal/plan`        | declarative `MigrationPlan` + parse/validate |
| `internal/statemachine`| durable runner + handlers + drift scan |
| `internal/store`       | pgxpool persistence (state, checkpoints, events) |
| `internal/telemetry`   | Prometheus metrics |
| `migrations/`  | engine control schema + demo target table |
| `examples/`    | happy-path + chaos migration plans |
| `monitoring/`  | Prometheus config, alert rules, Grafana provisioning + dashboard |
| `scripts/`     | `demo.sh`, `demo_rollback.sh` |

---

## The migration plan

A migration is a single declarative YAML document — *what* to change, not *how*:

```yaml
id: catalog-add-shipping-index
version: 42
table: catalog_product
strategy: expand-contract

expand:                       # additive DDL; index built CONCURRENTLY (runs outside a tx)
  - "ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text"
  - "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cp_shipping ON catalog_product (shipping_class)"

backfill:                     # throttled, resumable batches over rows WHERE col IS NULL
  column: shipping_class
  batch_size: 5000
  throttle_ms: 20
  source_expr: "CASE WHEN weight < 1 THEN 'light' WHEN weight < 10 THEN 'standard' ELSE 'freight' END"

verify:                       # shadow-read parity before cutover
  mode: shadow-read
  parity_threshold: 0.999
  sample_rate: 0.05

canary:
  steps: [1, 5, 25, 100]
  bake_seconds: 120

slo:                          # gate evaluated at every canary step
  max_p99_latency_ms: 50
  max_error_rate_pct: 0.1
  min_parity: 0.999

contract:                     # destructive cleanup, only after a clean canary
  - "ALTER TABLE catalog_product DROP COLUMN IF EXISTS legacy_shipping"

rollback:                     # auto-applied if the canary breaches the SLO
  - "DROP INDEX IF EXISTS idx_cp_shipping"
  - "ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class"

on_failure: rollback
```

**Security note:** `table` and `backfill.column` are validated as plain SQL identifiers before
interpolation. The `expand`/`contract`/`rollback` DDL and `source_expr` are intentionally raw
SQL — this is a migration tool authored by an operator who already holds full DDL rights on the
target database.

---

## Quick start

Requires Docker (Colima works) and Go 1.24+.

```bash
make up          # postgres + prometheus + grafana
make migrate     # engine control schema + demo target table (50k rows)
make run         # start the engine on :8080 (control API + /metrics)
```

In a second shell:

```bash
make demo                 # drive the happy-path plan end-to-end → Done
make demo-rollback        # drive the chaos plan → SLO breach → auto-rollback → RolledBack
make load-under-backfill  # concurrent writes WHILE a migration backfills (write p99 numbers)
```

- Prometheus: <http://localhost:9090>
- Grafana: <http://localhost:3000> (anonymous, dashboard auto-provisioned)
- Engine metrics: <http://localhost:8080/metrics>

---

## Demos (real output)

### Happy path — `make demo`

Backfills **50,000 / 50,000** rows in throttled batches, verifies parity **1.0** on a ~2,500-row
sample, walks the canary `1 → 5 → 25 → 100`, drops the legacy column, ends at **`Done`**:

```
state=Backfilling → Verifying → Canary → Contracting → Done
migrate_backfill_rows_done  = 50000
migrate_backfill_rows_total = 50000
migrate_state_info{state="Done"} 1
```

### SLO-gated auto-rollback — `make demo-rollback`

Same plan plus `chaos.fail_canary_at_step: 25`. The canary is healthy at 1% and 5%, then the
injected fault forces a p99 breach at **25%**. The engine diverts to rollback, runs both rollback
statements, and ends at **`RolledBack`** with the new column gone:

```
canary step healthy step=1  p99_ms=30
canary step healthy step=5  p99_ms=30
canary SLO breach   step=25 why="p99 101ms > 50ms"
state advanced  Canary → RollingBack → RolledBack
migrate_rollbacks_total      = 1
migrate_canary_step_pct      = 25
# shipping_class column dropped — verified absent
```

### Drift scan

After a successful migration, prove there's still zero divergence (exits non-zero on drift, so it
drops straight into CI/cron):

```bash
go run ./cmd/mgctl drift-scan -f examples/add-shipping-index.yaml
# table=catalog_product column=shipping_class total=50000 nulls=0 drifted=0 parity=1.00000

# corrupt 7 rows, then re-scan:
# table=catalog_product column=shipping_class total=50000 nulls=0 drifted=7 parity=0.99986
# error: drift detected: 7/50000 rows diverge   (exit 1)
```

### Load under backfill (real numbers)

The whole point of throttled, batched backfill is that **production writes keep flowing** while a
migration runs. `make load-under-backfill` starts 16 concurrent writers against `catalog_product`
and kicks off a real migration (50k-row backfill + `CREATE INDEX CONCURRENTLY`) over the same
table, then reports the write latency measured *during* the migration:

```
loadgen: 16 workers, 25s, table=catalog_product (max id=50000)
writes ok    : 289,296      throughput : 11,580 writes/s     errors : 0
latency p50  : 1.17ms        p95 : 2.66ms     p99 : 4.43ms     max : 85.67ms
migrate_cutover_parity = 1   # cutover committed over all 50,000 rows
```

p99 write latency stayed at **4.43 ms** — comfortably inside the 50 ms SLO — with **zero errors**
while the index built and the backfill ran. The migration reached `Done` and the cutover gate
committed over the full table. (`loadgen` updates a free-text column, never the backfill source
column, so it can't itself introduce drift.)

---

## Observability

### Metrics (`/metrics`)

| Metric | Type | Meaning |
|--------|------|---------|
| `migrate_state_info{migration_id,plan_id,state}` | gauge | current state (1 = active) |
| `migrate_state_transitions_total{plan_id,to_state}` | counter | state transitions |
| `migrate_backfill_rows_total` / `_done` | gauge | backfill progress |
| `migrate_verify_parity` | gauge | last shadow-read parity ratio |
| `migrate_cutover_parity` | gauge | full-table parity measured at the cutover gate |
| `migrate_canary_step_pct` | gauge | current canary traffic % |
| `migrate_rollbacks_total` | counter | auto-rollbacks triggered |

### Alerts (`monitoring/alerts.yml`)

- **MigrationStuck** — non-terminal state for >10m
- **MigrationAutoRolledBack** — a rollback fired (critical)
- **MigrationLowParity** — parity below threshold
- **EngineDown** — scrape target down

### Dashboard

`monitoring/grafana/dashboards/migration-safety-engine.json` — 7 panels (state, backfill rows,
completion %, parity, canary %, transition rate, rollbacks), auto-provisioned on `make up`.

---

## Crash-resume, proven in a test

`internal/statemachine/resume_test.go` runs four integration tests against a real Postgres
(set `MSE_TEST_DSN`; they skip otherwise):

- **`TestResumeAfterCrash`** — interrupt a migration, restart the runner, assert it finishes from
  the persisted checkpoint.
- **`TestCanaryAutoRollback`** — drive the chaos plan; assert the canary breach ends in
  `RolledBack` with the new column dropped.
- **`TestRollbackResumesAfterCrash`** — a migration killed *mid-rollback* resumes and completes to
  `RolledBack` (rollback is itself durable).
- **`TestCutoverAbortsOnDrift`** — drift at the cutover gate aborts and rolls back; the legacy
  column is **never** dropped on top of bad data.

```bash
make test        # unit always; integration when MSE_TEST_DSN is set
```

---

## What this demonstrates

**SRE / DevOps**
- Designed a crash-resumable state machine for online Postgres schema changes (expand/contract),
  persisting state + checkpoint transactionally so a killed process resumes exactly where it
  stopped — including mid-backfill across 50k rows and mid-rollback.
- Built an SLO-gated progressive canary (p99 latency + error-rate gates at 1/5/25/100%) plus a
  full-table cutover gate that auto-roll-back on breach or drift with zero human intervention, and
  shipped the operability stack: custom Prometheus metrics, 4 alert rules, and a Grafana dashboard.
- Load-tested the safety claim: 16 concurrent writers sustained ~11.6k writes/s at 4.4 ms p99 with
  zero errors *while* a 50k-row backfill and `CREATE INDEX CONCURRENTLY` ran on the same table.
- Added a read-only drift scanner that re-verifies data convergence post-migration and exits
  non-zero on divergence for CI/cron.

**FDE / Forward-Deployed**
- Turned a risky, manual database-migration runbook into a single declarative YAML plan, making
  every customer schema change reviewable, observable, gated, and reversible.
- Implemented shadow-read parity verification (sampled at verify, full-table at the cutover commit
  gate) that blocks cutover unless backfilled data matches its source expression, catching silent
  data divergence before it reaches production reads.
- Delivered an operator CLI (`mgctl`), a load generator, and one-command demos so the safety story
  — backfill, parity, canary, cutover gate, auto-rollback, drift, and write-latency-under-load —
  is reproducible end-to-end in minutes.
```

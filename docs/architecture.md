# Architecture Overview

The Migration Safety Engine (MSE) coordinates database schema migrations using a durable, Postgres-backed state machine. It prevents production outages through strict progressive testing, verification gates, and automated rollbacks.

---

## 1. The 8-State Machine

MSE treats migrations as state machines rather than simple scripts. State transitions are governed by the following lifecycle:

```
                  Pending
                     │
              [ddl expand]
                     ▼
                 Expanding
                     │
             [batched backfill]
                     ▼
                Backfilling <─────────┐ (loop until done)
                     │ └──────────────┘
              [sample verify]
                     ▼
                 Verifying
                     │
             [canary steps 1-100%]
                     ▼
                  Canary ─────────────┐
                     │                │ (SLO breach)
             [full-table verify]      ▼
                     ▼           RollingBack
                  Cutover             │
                     │          [ddl rollback]
             [ddl contract]           ▼
                     ▼           RolledBack (Terminal)
                Contracting
                     │
                     ▼
               Done (Terminal)
```

### State Definitions

1.  **Pending**: The migration plan is submitted and registered.
2.  **Expanding**: Additive DDL statements are executed (e.g. creating columns or indexes concurrently) without affecting existing paths.
3.  **Backfilling**: Data is copied from source to destination columns in batched, throttled updates. Progress is checkpointed after every batch.
4.  **Verifying**: Run a sampled, shadow-read check to ensure data parity.
5.  **Canary**: Shift traffic progressively (e.g., $1\% \rightarrow 5\% \rightarrow 25\% \rightarrow 100\%$), auditing the service's latency and error rates against defined SLOs.
6.  **Cutover**: A full-table check (no sampling allowed) to confirm 100% parity before committing to destructive steps.
7.  **Contracting**: Clean up the database by dropping the deprecated columns/indexes.
8.  **Done**: Migration completed successfully.
9.  **RollingBack**: Active rollback phase, executing rollback statements to undo the expansion.
10. **RolledBack**: The database has been cleanly restored to its pre-migration shape.

---

## 2. Durability & Crash Resumability

To ensure database operations are reliable, MSE guarantees crash resumability at every state:
- Every successful state transition is written to the control Postgres DB inside a `migration_state` transaction.
- Checkpoints are saved continuously during the backfill phase (e.g., tracking `rows_done`).
- If the engine process crashes or restarts, the runner queries resumable migrations, loads their latest checkpoint, and re-enters the active state handler idempotently.

---

## 3. Parity Gating & Drift Protection

Data verification happens at three stages:
1.  **Verify State**: A sampled shadow-read checks that the backfill matches the desired expression.
2.  **Cutover Gate**: A comprehensive full-table query validates parity across all records. If any writes during the canary phase bypassed the column generation, this gate catches it.
3.  **Post-Migration Drift Scanner**: Operators can trigger drift scans (`mgctl drift-scan`) manually or in a cron schedule. It compares the column value with the source expression to highlight if application paths bypass the migration rule.

---

## 4. SLO-Gated Rollback

During the **Canary** phase, the engine monitors latency and error rates (reading from Prometheus metrics). If a step breaches the configured SLOs (e.g., latency spikes or error rates rise), the engine diverts the state to `RollingBack` without human operator intervention, preventing outages.

-- Migration Safety Engine — Phase 3 safety tables.
-- Holds DDL execution logs, backfill progress, canary observations, and service registry.
-- Idempotent: safe to run repeatedly.

CREATE TABLE IF NOT EXISTS ddl_execution_log (
    id            bigserial PRIMARY KEY,
    migration_id  uuid        NOT NULL REFERENCES migration(id) ON DELETE CASCADE,
    statement     text        NOT NULL,
    started_at    timestamptz NOT NULL,
    completed_at  timestamptz,
    duration_ms   integer,
    success       boolean     NOT NULL DEFAULT false,
    error_message text,
    lock_wait_ms  integer,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ddl_execution_log_mid
    ON ddl_execution_log (migration_id, id);

CREATE TABLE IF NOT EXISTS backfill_progress (
    id            bigserial PRIMARY KEY,
    migration_id  uuid        NOT NULL REFERENCES migration(id) ON DELETE CASCADE,
    batch_number  integer     NOT NULL,
    rows_affected integer     NOT NULL,
    throttle_ms   integer,
    db_cpu_pct    numeric,
    db_rep_lag_ms numeric,
    db_conns_pct  numeric,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_backfill_progress_mid
    ON backfill_progress (migration_id, id);

CREATE TABLE IF NOT EXISTS canary_observation (
    id            bigserial PRIMARY KEY,
    migration_id  uuid        NOT NULL REFERENCES migration(id) ON DELETE CASCADE,
    step          integer     NOT NULL,
    traffic_pct   integer     NOT NULL,
    p99_ms        numeric,
    err_pct       numeric,
    slo_breached  boolean     NOT NULL DEFAULT false,
    observed_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_canary_observation_mid
    ON canary_observation (migration_id, id);

CREATE TABLE IF NOT EXISTS service_registry (
    service_id    text PRIMARY KEY,
    service_name  text        NOT NULL,
    version       text        NOT NULL,
    status        text        NOT NULL DEFAULT 'active',
    last_heartbeat timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

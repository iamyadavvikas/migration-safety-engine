-- Migration Safety Engine — Phase 1 state schema.
-- Holds submitted plans, their durable state-machine position + checkpoint, and an audit log.
-- Idempotent: safe to run repeatedly.

CREATE TABLE IF NOT EXISTS migration (
    id           uuid PRIMARY KEY,
    plan_id      text        NOT NULL,            -- human id from the MigrationPlan (e.g. catalog-add-shipping-index)
    version      integer     NOT NULL,
    plan         jsonb       NOT NULL,            -- the full parsed MigrationPlan
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (plan_id, version)
);

CREATE TABLE IF NOT EXISTS migration_state (
    migration_id uuid PRIMARY KEY REFERENCES migration(id) ON DELETE CASCADE,
    state        text        NOT NULL,            -- Pending, Expanding, ... , Done, RolledBack
    checkpoint   jsonb       NOT NULL DEFAULT '{}'::jsonb,
    terminal     boolean     NOT NULL DEFAULT false,
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Index to find in-flight (non-terminal) migrations to resume on startup.
CREATE INDEX IF NOT EXISTS idx_migration_state_resumable
    ON migration_state (terminal) WHERE terminal = false;

CREATE TABLE IF NOT EXISTS migration_event (
    id           bigserial PRIMARY KEY,
    migration_id uuid        NOT NULL REFERENCES migration(id) ON DELETE CASCADE,
    from_state   text,
    to_state     text        NOT NULL,
    detail       text,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_migration_event_mid
    ON migration_event (migration_id, id);

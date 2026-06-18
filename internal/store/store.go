// Package store provides Postgres-backed persistence for migrations, their
// durable state-machine position, checkpoints, and an audit event log.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamyadavvikas/migration-safety-engine/internal/plan"
)

// ErrNotFound is returned when a migration row does not exist.
var ErrNotFound = errors.New("migration not found")

// Store wraps a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// Record is the persisted state of a migration.
type Record struct {
	ID         uuid.UUID
	Plan       plan.MigrationPlan
	State      string
	Checkpoint map[string]any
	Terminal   bool
	UpdatedAt  time.Time
}

// New opens a connection pool to the given DSN.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// CreateMigration inserts a new migration + its initial state row, returning the id.
// If a migration with the same (plan_id, version) already exists, the existing id is returned.
func (s *Store) CreateMigration(ctx context.Context, p *plan.MigrationPlan, initialState string) (uuid.UUID, error) {
	planJSON, err := json.Marshal(p)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal plan: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	// Reuse existing migration with the same plan_id+version if present (idempotent submit).
	var existing uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT id FROM migration WHERE plan_id = $1 AND version = $2`,
		p.ID, p.Version,
	).Scan(&existing)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}

	id := uuid.New()
	if _, err := tx.Exec(ctx,
		`INSERT INTO migration (id, plan_id, version, plan) VALUES ($1, $2, $3, $4)`,
		id, p.ID, p.Version, planJSON,
	); err != nil {
		return uuid.Nil, fmt.Errorf("insert migration: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO migration_state (migration_id, state, checkpoint, terminal)
		 VALUES ($1, $2, '{}'::jsonb, false)`,
		id, initialState,
	); err != nil {
		return uuid.Nil, fmt.Errorf("insert state: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO migration_event (migration_id, from_state, to_state, detail)
		 VALUES ($1, NULL, $2, 'created')`,
		id, initialState,
	); err != nil {
		return uuid.Nil, fmt.Errorf("insert event: %w", err)
	}
	return id, tx.Commit(ctx)
}

// Load returns the full record for a migration id.
func (s *Store) Load(ctx context.Context, id uuid.UUID) (*Record, error) {
	var (
		planJSON []byte
		ckptJSON []byte
		r        Record
	)
	err := s.pool.QueryRow(ctx,
		`SELECT m.id, m.plan, ms.state, ms.checkpoint, ms.terminal, ms.updated_at
		   FROM migration m
		   JOIN migration_state ms ON ms.migration_id = m.id
		  WHERE m.id = $1`,
		id,
	).Scan(&r.ID, &planJSON, &r.State, &ckptJSON, &r.Terminal, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(planJSON, &r.Plan); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w", err)
	}
	if err := json.Unmarshal(ckptJSON, &r.Checkpoint); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}
	return &r, nil
}

// SaveState atomically advances state, persists a checkpoint, marks terminal, and
// appends an audit event. This is the durability point the state machine relies on.
func (s *Store) SaveState(ctx context.Context, id uuid.UUID, from, to string, checkpoint map[string]any, terminal bool, detail string) error {
	if checkpoint == nil {
		checkpoint = map[string]any{}
	}
	ckptJSON, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE migration_state
		    SET state = $2, checkpoint = $3, terminal = $4, updated_at = now()
		  WHERE migration_id = $1`,
		id, to, ckptJSON, terminal,
	); err != nil {
		return fmt.Errorf("update state: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO migration_event (migration_id, from_state, to_state, detail)
		 VALUES ($1, $2, $3, $4)`,
		id, from, to, detail,
	); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return tx.Commit(ctx)
}

// SaveCheckpoint persists intra-state progress WITHOUT changing the state. The
// backfill handler calls this after every batch so that, on a crash, progress
// (and metrics) survive; correctness on resume comes from the handler's own
// idempotent predicate (e.g. WHERE column IS NULL), this is the observable record.
func (s *Store) SaveCheckpoint(ctx context.Context, id uuid.UUID, state string, checkpoint map[string]any, detail string) error {
	if checkpoint == nil {
		checkpoint = map[string]any{}
	}
	ckptJSON, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE migration_state SET checkpoint = $2, updated_at = now() WHERE migration_id = $1`,
		id, ckptJSON,
	); err != nil {
		return fmt.Errorf("update checkpoint: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO migration_event (migration_id, from_state, to_state, detail)
		 VALUES ($1, $2, $2, $3)`,
		id, state, detail,
	); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return tx.Commit(ctx)
}

// FindResumable returns the ids of all non-terminal migrations, so the engine can
// continue them after a restart.
func (s *Store) FindResumable(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT migration_id FROM migration_state WHERE terminal = false ORDER BY updated_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Pool exposes the underlying pool for packages that need direct SQL (e.g. tests).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

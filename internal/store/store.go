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

// ListSummary returns a lightweight summary of every migration (no plan JSON).
type Summary struct {
	ID        uuid.UUID `json:"migration_id"`
	PlanID    string    `json:"plan_id"`
	State     string    `json:"state"`
	Terminal  bool      `json:"terminal"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) List(ctx context.Context) ([]Summary, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT m.id, m.plan_id, ms.state, ms.terminal, ms.updated_at
		   FROM migration m
		   JOIN migration_state ms ON ms.migration_id = m.id
		  ORDER BY ms.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Summary
	for rows.Next() {
		var r Summary
		if err := rows.Scan(&r.ID, &r.PlanID, &r.State, &r.Terminal, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Pool exposes the underlying pool for packages that need direct SQL (e.g. tests).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Target returns the underlying pool for direct SQL operations.
func (s *Store) Target() *pgxpool.Pool { return s.pool }

// UpdateState directly sets a migration's state and terminal flag.
func (s *Store) UpdateState(ctx context.Context, id uuid.UUID, state string, terminal bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE migration_state SET state = $2, terminal = $3, updated_at = now() WHERE migration_id = $1`,
		id, state, terminal,
	)
	return err
}

// Delete removes a single migration and all its associated rows.
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM migration_event WHERE migration_id = $1`, id); err != nil {
		return fmt.Errorf("delete events: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM migration_state WHERE migration_id = $1`, id); err != nil {
		return fmt.Errorf("delete state: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM migration WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete migration: %w", err)
	}
	return tx.Commit(ctx)
}

// DeleteMany removes multiple migrations by their IDs.
func (s *Store) DeleteMany(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Use ANY($1) for batch delete
	if _, err := tx.Exec(ctx, `DELETE FROM migration_event WHERE migration_id = ANY($1)`, ids); err != nil {
		return fmt.Errorf("delete events: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM migration_state WHERE migration_id = ANY($1)`, ids); err != nil {
		return fmt.Errorf("delete state: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM migration WHERE id = ANY($1)`, ids); err != nil {
		return fmt.Errorf("delete migration: %w", err)
	}
	return tx.Commit(ctx)
}

// ──────────────────────────────────────────────────────────────────────────────
// DDL Execution Logging
// ──────────────────────────────────────────────────────────────────────────────

// DDLExecutionLogEntry represents a logged DDL execution.
type DDLExecutionLogEntry struct {
	ID           int64      `json:"id"`
	MigrationID  uuid.UUID  `json:"migration_id"`
	Statement    string     `json:"statement"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	DurationMs   *int       `json:"duration_ms,omitempty"`
	Success      bool       `json:"success"`
	ErrorMessage string     `json:"error_message,omitempty"`
	LockWaitMs   int        `json:"lock_wait_ms"`
}

// LogDDLExecution records a DDL execution attempt.
func (s *Store) LogDDLExecution(ctx context.Context, entry *DDLExecutionLogEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO ddl_execution_log 
		 (migration_id, statement, started_at, completed_at, duration_ms, success, error_message, lock_wait_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		entry.MigrationID, entry.Statement, entry.StartedAt, entry.CompletedAt,
		entry.DurationMs, entry.Success, entry.ErrorMessage, entry.LockWaitMs,
	)
	return err
}

// GetDDLLogs returns DDL execution logs for a migration.
func (s *Store) GetDDLLogs(ctx context.Context, migrationID uuid.UUID) ([]DDLExecutionLogEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, migration_id, statement, started_at, completed_at, duration_ms, success, error_message, lock_wait_ms
		 FROM ddl_execution_log
		 WHERE migration_id = $1
		 ORDER BY started_at`,
		migrationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DDLExecutionLogEntry
	for rows.Next() {
		var e DDLExecutionLogEntry
		if err := rows.Scan(&e.ID, &e.MigrationID, &e.Statement, &e.StartedAt,
			&e.CompletedAt, &e.DurationMs, &e.Success, &e.ErrorMessage, &e.LockWaitMs); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ──────────────────────────────────────────────────────────────────────────────
// Backfill Progress Telemetry
// ──────────────────────────────────────────────────────────────────────────────

// BackfillProgressEntry represents a logged backfill batch.
type BackfillProgressEntry struct {
	ID            int64     `json:"id"`
	MigrationID   uuid.UUID `json:"migration_id"`
	BatchNumber   int       `json:"batch_number"`
	RowsAffected  int       `json:"rows_affected"`
	ThrottleMs    int       `json:"throttle_ms"`
	DBCPUPct      float64   `json:"db_cpu_pct"`
	DBRepLagMs    float64   `json:"db_rep_lag_ms"`
	DBConnsPct    float64   `json:"db_conns_pct"`
	CreatedAt     time.Time `json:"created_at"`
}

// LogBackfillProgress records a backfill batch execution.
func (s *Store) LogBackfillProgress(ctx context.Context, entry *BackfillProgressEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO backfill_progress 
		 (migration_id, batch_number, rows_affected, throttle_ms, db_cpu_pct, db_rep_lag_ms, db_conns_pct)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.MigrationID, entry.BatchNumber, entry.RowsAffected, entry.ThrottleMs,
		entry.DBCPUPct, entry.DBRepLagMs, entry.DBConnsPct,
	)
	return err
}

// GetBackfillProgress returns backfill progress entries for a migration.
func (s *Store) GetBackfillProgress(ctx context.Context, migrationID uuid.UUID) ([]BackfillProgressEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, migration_id, batch_number, rows_affected, throttle_ms, 
		        db_cpu_pct, db_rep_lag_ms, db_conns_pct, created_at
		 FROM backfill_progress
		 WHERE migration_id = $1
		 ORDER BY batch_number`,
		migrationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BackfillProgressEntry
	for rows.Next() {
		var e BackfillProgressEntry
		if err := rows.Scan(&e.ID, &e.MigrationID, &e.BatchNumber, &e.RowsAffected,
			&e.ThrottleMs, &e.DBCPUPct, &e.DBRepLagMs, &e.DBConnsPct, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ──────────────────────────────────────────────────────────────────────────────
// Canary Observation Logging
// ──────────────────────────────────────────────────────────────────────────────

// CanaryObservationEntry represents a logged canary observation.
type CanaryObservationEntry struct {
	ID           int64     `json:"id"`
	MigrationID  uuid.UUID `json:"migration_id"`
	Step         int       `json:"step"`
	TrafficPct   int       `json:"traffic_pct"`
	P99Ms        float64   `json:"p99_ms"`
	ErrPct       float64   `json:"err_pct"`
	SLOBreached  bool      `json:"slo_breached"`
	ObservedAt   time.Time `json:"observed_at"`
}

// LogCanaryObservation records a canary step observation.
func (s *Store) LogCanaryObservation(ctx context.Context, entry *CanaryObservationEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO canary_observation 
		 (migration_id, step, traffic_pct, p99_ms, err_pct, slo_breached)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		entry.MigrationID, entry.Step, entry.TrafficPct, entry.P99Ms, entry.ErrPct, entry.SLOBreached,
	)
	return err
}

// GetCanaryObservations returns canary observations for a migration.
func (s *Store) GetCanaryObservations(ctx context.Context, migrationID uuid.UUID) ([]CanaryObservationEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, migration_id, step, traffic_pct, p99_ms, err_pct, slo_breached, observed_at
		 FROM canary_observation
		 WHERE migration_id = $1
		 ORDER BY step, observed_at`,
		migrationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []CanaryObservationEntry
	for rows.Next() {
		var e CanaryObservationEntry
		if err := rows.Scan(&e.ID, &e.MigrationID, &e.Step, &e.TrafficPct,
			&e.P99Ms, &e.ErrPct, &e.SLOBreached, &e.ObservedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ──────────────────────────────────────────────────────────────────────────────
// Service Registry (Multi-Service Coordination)
// ──────────────────────────────────────────────────────────────────────────────

// ServiceRegistration represents a registered service.
type ServiceRegistration struct {
	ID             uuid.UUID `json:"id"`
	MigrationID    uuid.UUID `json:"migration_id"`
	ServiceName    string    `json:"service_name"`
	SchemaVersion  int       `json:"schema_version"`
	Compatible     bool      `json:"compatible"`
	RegisteredAt   time.Time `json:"registered_at"`
	LastHeartbeat  time.Time `json:"last_heartbeat"`
}

// RegisterService registers a service as dependent on a migration.
func (s *Store) RegisterService(ctx context.Context, migrationID uuid.UUID, serviceName string, schemaVersion int) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO service_registry (migration_id, service_name, schema_version)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (migration_id, service_name) 
		 DO UPDATE SET schema_version = $3, last_heartbeat = now()
		 RETURNING id`,
		migrationID, serviceName, schemaVersion,
	).Scan(&id)
	return id, err
}

// UpdateServiceCompat updates a service's compatibility status.
func (s *Store) UpdateServiceCompat(ctx context.Context, migrationID uuid.UUID, serviceName string, compatible bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE service_registry 
		 SET compatible = $3, last_heartbeat = now()
		 WHERE migration_id = $1 AND service_name = $2`,
		migrationID, serviceName, compatible,
	)
	return err
}

// HeartbeatService updates a service's heartbeat timestamp.
func (s *Store) HeartbeatService(ctx context.Context, migrationID uuid.UUID, serviceName string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE service_registry 
		 SET last_heartbeat = now()
		 WHERE migration_id = $1 AND service_name = $2`,
		migrationID, serviceName,
	)
	return err
}

// GetServices returns all services registered for a migration.
func (s *Store) GetServices(ctx context.Context, migrationID uuid.UUID) ([]ServiceRegistration, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, migration_id, service_name, schema_version, compatible, registered_at, last_heartbeat
		 FROM service_registry
		 WHERE migration_id = $1
		 ORDER BY registered_at`,
		migrationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []ServiceRegistration
	for rows.Next() {
		var svc ServiceRegistration
		if err := rows.Scan(&svc.ID, &svc.MigrationID, &svc.ServiceName, &svc.SchemaVersion,
			&svc.Compatible, &svc.RegisteredAt, &svc.LastHeartbeat); err != nil {
			return nil, err
		}
		services = append(services, svc)
	}
	return services, rows.Err()
}

// AllServicesReady checks if all registered services are compatible.
func (s *Store) AllServicesReady(ctx context.Context, migrationID uuid.UUID) (bool, int, error) {
	var total, ready int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*), count(*) FILTER (WHERE compatible = true)
		 FROM service_registry
		 WHERE migration_id = $1`,
		migrationID,
	).Scan(&total, &ready)
	if err != nil {
		return false, 0, err
	}
	return total == ready && total > 0, total - ready, nil
}

// RemoveService removes a service registration.
func (s *Store) RemoveService(ctx context.Context, migrationID uuid.UUID, serviceName string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM service_registry WHERE migration_id = $1 AND service_name = $2`,
		migrationID, serviceName,
	)
	return err
}

// ServiceHeartbeat represents a service heartbeat event.
type ServiceHeartbeat struct {
	ID          int64     `json:"id"`
	MigrationID uuid.UUID `json:"migration_id"`
	ServiceName string    `json:"service_name"`
	Payload     []byte    `json:"payload,omitempty"`
	RecordedAt  time.Time `json:"recorded_at"`
}

// RecordHeartbeat records a service heartbeat.
func (s *Store) RecordHeartbeat(ctx context.Context, migrationID uuid.UUID, serviceName string, payload []byte) error {
	// Update the service registry's last_heartbeat timestamp
	_, err := s.pool.Exec(ctx,
		`UPDATE service_registry 
		 SET last_heartbeat = now(), compatible = true
		 WHERE migration_id = $1 AND service_name = $2`,
		migrationID, serviceName,
	)
	if err != nil {
		return err
	}

	// Record the heartbeat event
	_, err = s.pool.Exec(ctx,
		`INSERT INTO service_heartbeat (migration_id, service_name, payload)
		 VALUES ($1, $2, $3)`,
		migrationID, serviceName, payload,
	)
	return err
}

// GetHeartbeats returns heartbeats for a migration.
func (s *Store) GetHeartbeats(ctx context.Context, migrationID uuid.UUID) ([]ServiceHeartbeat, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, migration_id, service_name, payload, recorded_at
		 FROM service_heartbeat
		 WHERE migration_id = $1
		 ORDER BY recorded_at DESC
		 LIMIT 100`,
		migrationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var heartbeats []ServiceHeartbeat
	for rows.Next() {
		var hb ServiceHeartbeat
		if err := rows.Scan(&hb.ID, &hb.MigrationID, &hb.ServiceName, &hb.Payload, &hb.RecordedAt); err != nil {
			return nil, err
		}
		heartbeats = append(heartbeats, hb)
	}
	return heartbeats, rows.Err()
}

// CheckServiceLiveness checks if all services are still alive (heartbeat within threshold).
func (s *Store) CheckServiceLiveness(ctx context.Context, migrationID uuid.UUID, threshold time.Duration) (bool, []string, error) {
	var deadServices []string
	
	rows, err := s.pool.Query(ctx,
		`SELECT service_name, last_heartbeat
		 FROM service_registry
		 WHERE migration_id = $1`,
		migrationID,
	)
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()

	now := time.Now()
	for rows.Next() {
		var serviceName string
		var lastHeartbeat time.Time
		if err := rows.Scan(&serviceName, &lastHeartbeat); err != nil {
			return false, nil, err
		}
		
		if now.Sub(lastHeartbeat) > threshold {
			deadServices = append(deadServices, serviceName)
		}
	}
	
	if err := rows.Err(); err != nil {
		return false, nil, err
	}

	return len(deadServices) == 0, deadServices, nil
}

// MarkServiceIncompatible marks a service as incompatible (for contract phase).
func (s *Store) MarkServiceIncompatible(ctx context.Context, migrationID uuid.UUID, serviceName string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE service_registry 
		 SET compatible = false
		 WHERE migration_id = $1 AND service_name = $2`,
		migrationID, serviceName,
	)
	return err
}

// GetServiceVersions returns all services with their schema versions.
func (s *Store) GetServiceVersions(ctx context.Context, migrationID uuid.UUID) (map[string]int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT service_name, schema_version
		 FROM service_registry
		 WHERE migration_id = $1`,
		migrationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make(map[string]int)
	for rows.Next() {
		var name string
		var version int
		if err := rows.Scan(&name, &version); err != nil {
			return nil, err
		}
		versions[name] = version
	}
	return versions, rows.Err()
}

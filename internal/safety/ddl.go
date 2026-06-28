// Package safety provides production-grade safeguards for DDL execution,
// backfill throttling, and migration safety checks.
package safety

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DDLConfig holds safety configuration for DDL execution.
type DDLConfig struct {
	LockTimeout      time.Duration // Max time to wait for locks (default: 3s)
	StatementTimeout time.Duration // Max time for DDL statement (default: 60s)
	MaxLockQueue     int           // Max waiting queries before pausing (default: 5)
	MaxReplicationLag time.Duration // Max acceptable replication lag (default: 30s)
}

// DefaultDDLConfig returns production-safe defaults.
func DefaultDDLConfig() DDLConfig {
	return DDLConfig{
		LockTimeout:      3 * time.Second,
		StatementTimeout: 60 * time.Second,
		MaxLockQueue:     5,
		MaxReplicationLag: 30 * time.Second,
	}
}

// DDLExecutor wraps DDL execution with safety checks.
type DDLExecutor struct {
	pool   *pgxpool.Pool
	config DDLConfig
	log    *slog.Logger
}

// NewDDLExecutor creates a new DDL executor with safety checks.
func NewDDLExecutor(pool *pgxpool.Pool, config DDLConfig, log *slog.Logger) *DDLExecutor {
	return &DDLExecutor{
		pool:   pool,
		config: config,
		log:    log,
	}
}

// LockQueueStatus represents the current lock queue state.
type LockQueueStatus struct {
	WaitingQueries int
	BlockingPID    int
	BlockingQuery  string
}

// ReplicationStatus represents replication health.
type ReplicationStatus struct {
	ReplicaCount   int
	MaxLagMs       float64
	WorstReplica   string
}

// DBHealthMetrics represents overall database health.
type DBHealthMetrics struct {
	CPUPercent       float64
	ActiveConns      int
	MaxConns         int
	TuplesDead       int64
	TuplesDirty      int64
	LockQueue        LockQueueStatus
	Replication      ReplicationStatus
}

// CheckLockQueue queries pg_stat_activity for lock contention.
func (e *DDLExecutor) CheckLockQueue(ctx context.Context) (*LockQueueStatus, error) {
	status := &LockQueueStatus{}

	err := e.pool.QueryRow(ctx, `
		SELECT count(*) 
		FROM pg_stat_activity 
		WHERE state = 'active' 
		  AND query NOT LIKE '%pg_stat_activity%'
		  AND wait_event_type = 'Lock'
	`).Scan(&status.WaitingQueries)
	if err != nil {
		return nil, fmt.Errorf("check lock queue: %w", err)
	}

	if status.WaitingQueries > 0 {
		err = e.pool.QueryRow(ctx, `
			SELECT pid, query 
			FROM pg_stat_activity 
			WHERE state = 'active' 
			  AND wait_event_type = 'Lock'
			  AND query NOT LIKE '%pg_stat_activity%'
			ORDER BY query_start 
			LIMIT 1
		`).Scan(&status.BlockingPID, &status.BlockingQuery)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("get blocking query: %w", err)
		}
	}

	return status, nil
}

// CheckReplicationLag queries pg_stat_replication for lag.
func (e *DDLExecutor) CheckReplicationLag(ctx context.Context) (*ReplicationStatus, error) {
	status := &ReplicationStatus{}

	err := e.pool.QueryRow(ctx, `
		SELECT 
			count(*),
			COALESCE(max(extract(epoch FROM replay_lag)) * 1000, 0),
			COALESCE((SELECT client_addr::text FROM pg_stat_replication 
				ORDER BY replay_lag DESC LIMIT 1), 'none')
		FROM pg_stat_replication
	`).Scan(&status.ReplicaCount, &status.MaxLagMs, &status.WorstReplica)
	if err != nil {
		return nil, fmt.Errorf("check replication lag: %w", err)
	}

	return status, nil
}

// CollectHealthMetrics gathers all DB health metrics.
func (e *DDLExecutor) CollectHealthMetrics(ctx context.Context) (*DBHealthMetrics, error) {
	m := &DBHealthMetrics{}

	// Active connections
	err := e.pool.QueryRow(ctx, `
		SELECT 
			count(*) FILTER (WHERE state = 'active'),
			(SELECT setting::int FROM pg_settings WHERE name = 'max_connections')
		FROM pg_stat_activity
	`).Scan(&m.ActiveConns, &m.MaxConns)
	if err != nil {
		return nil, fmt.Errorf("check connections: %w", err)
	}

	// Dead tuples (table bloat indicator)
	err = e.pool.QueryRow(ctx, `
		SELECT COALESCE(sum(n_dead_tup), 0)
		FROM pg_stat_user_tables
	`).Scan(&m.TuplesDead)
	if err != nil {
		return nil, fmt.Errorf("check dead tuples: %w", err)
	}

	// Lock queue
	lockQueue, err := e.CheckLockQueue(ctx)
	if err != nil {
		e.log.Warn("failed to check lock queue", "err", err)
	} else {
		m.LockQueue = *lockQueue
	}

	// Replication
	replStatus, err := e.CheckReplicationLag(ctx)
	if err != nil {
		e.log.Warn("failed to check replication", "err", err)
	} else {
		m.Replication = *replStatus
	}

	return m, nil
}

// SafetyCheck performs all pre-DDL safety checks.
func (e *DDLExecutor) SafetyCheck(ctx context.Context) error {
	// Check lock queue
	lockStatus, err := e.CheckLockQueue(ctx)
	if err != nil {
		return fmt.Errorf("lock queue check failed: %w", err)
	}
	if lockStatus.WaitingQueries > e.config.MaxLockQueue {
		return fmt.Errorf("lock queue blocked: %d queries waiting (max: %d)", 
			lockStatus.WaitingQueries, e.config.MaxLockQueue)
	}

	// Check replication lag
	replStatus, err := e.CheckReplicationLag(ctx)
	if err != nil {
		return fmt.Errorf("replication check failed: %w", err)
	}
	if time.Duration(replStatus.MaxLagMs)*time.Millisecond > e.config.MaxReplicationLag {
		return fmt.Errorf("replication lag too high: %.0fms (max: %v)", 
			replStatus.MaxLagMs, e.config.MaxReplicationLag)
	}

	return nil
}

// ExecDDL executes a DDL statement with safety wrappers.
func (e *DDLExecutor) ExecDDL(ctx context.Context, stmt string) (time.Duration, error) {
	start := time.Now()

	// Pre-flight safety check
	if err := e.SafetyCheck(ctx); err != nil {
		return 0, fmt.Errorf("safety check failed: %w", err)
	}

	// Check if this is a CONCURRENTLY index operation (must run outside tx)
	if isConcurrentlyDDL(stmt) {
		return e.execConcurrently(ctx, stmt)
	}

	// Set safety timeouts within a transaction
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Set lock_timeout and statement_timeout (separate statements for PostgreSQL)
	_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%dms'",
		e.config.LockTimeout.Milliseconds()))
	if err != nil {
		return 0, fmt.Errorf("set lock_timeout: %w", err)
	}

	_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%dms'",
		e.config.StatementTimeout.Milliseconds()))
	if err != nil {
		return 0, fmt.Errorf("set statement_timeout: %w", err)
	}

	// Execute DDL
	_, err = tx.Exec(ctx, stmt)
	if err != nil {
		return time.Since(start), fmt.Errorf("execute DDL: %w", err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return time.Since(start), fmt.Errorf("commit: %w", err)
	}

	duration := time.Since(start)
	e.log.Info("DDL executed successfully", 
		"stmt", stmt[:min(len(stmt), 100)],
		"duration", duration)

	return duration, nil
}

// isConcurrentlyDDL checks if a DDL statement uses CONCURRENTLY (must run outside tx)
func isConcurrentlyDDL(stmt string) bool {
	// Check for CONCURRENTLY keyword (case-insensitive)
	upper := strings.ToUpper(stmt)
	return strings.Contains(upper, "CONCURRENTLY")
}

// execConcurrently executes CONCURRENTLY DDL outside a transaction with retries
func (e *DDLExecutor) execConcurrently(ctx context.Context, stmt string) (time.Duration, error) {
	start := time.Now()
	maxRetries := 3
	retryDelay := time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Pre-flight safety check
		if err := e.SafetyCheck(ctx); err != nil {
			if attempt < maxRetries-1 {
				e.log.Warn("safety check failed, retrying", 
					"attempt", attempt+1, "err", err)
				time.Sleep(retryDelay * time.Duration(attempt+1))
				continue
			}
			return 0, fmt.Errorf("safety check failed after %d attempts: %w", maxRetries, err)
		}

		// Set statement_timeout separately (not in same Exec call)
		_, err := e.pool.Exec(ctx, fmt.Sprintf(
			"SET statement_timeout = '%dms'", e.config.StatementTimeout.Milliseconds()))
		if err != nil {
			e.log.Warn("failed to set statement_timeout", "err", err)
			// Continue anyway, don't fail
		}

		// Execute DDL outside transaction
		_, err = e.pool.Exec(ctx, stmt)
		if err != nil {
			if attempt < maxRetries-1 {
				e.log.Warn("CONCURRENTLY DDL failed, retrying",
					"attempt", attempt+1, "err", err)
				time.Sleep(retryDelay * time.Duration(attempt+1))
				continue
			}
			return time.Since(start), fmt.Errorf("execute CONCURRENTLY DDL: %w", err)
		}

		duration := time.Since(start)
		e.log.Info("CONCURRENTLY DDL executed successfully",
			"stmt", stmt[:min(len(stmt), 100)],
			"duration", duration)
		return duration, nil
	}

	return time.Since(start), fmt.Errorf("CONCURRENTLY DDL failed after %d attempts", maxRetries)
}

// ExecDDLWithAdvisoryLock executes DDL with advisory lock serialization
func (e *DDLExecutor) ExecDDLWithAdvisoryLock(ctx context.Context, migrationID [16]byte, stmt string) (time.Duration, error) {
	// Convert UUID to int64 for advisory lock key
	lockKey := int64(migrationID[0])<<56 | int64(migrationID[1])<<48 |
		int64(migrationID[2])<<40 | int64(migrationID[3])<<32 |
		int64(migrationID[4])<<24 | int64(migrationID[5])<<16 |
		int64(migrationID[6])<<8 | int64(migrationID[7])
	if lockKey < 0 {
		lockKey = -lockKey
	}

	// Try to acquire advisory lock (non-blocking)
	var acquired bool
	err := e.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&acquired)
	if err != nil {
		return 0, fmt.Errorf("acquire advisory lock: %w", err)
	}
	if !acquired {
		return 0, fmt.Errorf("advisory lock already held by another migration")
	}

	// Ensure lock is released on exit
	defer func() {
		_, _ = e.pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockKey)
	}()

	// Execute DDL
	return e.ExecDDL(ctx, stmt)
}

// SLOCheckExecutor uses a read replica for SLO checks to reduce load on primary.
type SLOCheckExecutor struct {
	primary *pgxpool.Pool
	replica *pgxpool.Pool
	log     *slog.Logger
}

// NewSLOCheckExecutor creates a new SLO check executor with read replica support.
func NewSLOCheckExecutor(primary, replica *pgxpool.Pool, log *slog.Logger) *SLOCheckExecutor {
	return &SLOCheckExecutor{
		primary: primary,
		replica: replica,
		log:     log,
	}
}

// CheckSLO performs SLO checks, using replica if available.
func (e *SLOCheckExecutor) CheckSLO(ctx context.Context, query string, args ...any) error {
	// Use replica if available, otherwise fall back to primary
	pool := e.primary
	if e.replica != nil {
		pool = e.replica
	}

	var result int
	err := pool.QueryRow(ctx, query, args...).Scan(&result)
	if err != nil {
		return fmt.Errorf("slo check: %w", err)
	}
	return nil
}

// GetPool returns the appropriate pool for SLO checks.
func (e *SLOCheckExecutor) GetPool() *pgxpool.Pool {
	if e.replica != nil {
		return e.replica
	}
	return e.primary
}

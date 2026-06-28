// Package chaos provides PostgreSQL-specific chaos injection mechanisms.
package chaos

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PostgresChaosStore implements ChaosStore for PostgreSQL.
type PostgresChaosStore struct {
	db *sql.DB
}

// NewPostgresChaosStore creates a new PostgreSQL chaos store.
func NewPostgresChaosStore(db *sql.DB) *PostgresChaosStore {
	return &PostgresChaosStore{db: db}
}

// CreateTestTable creates a test table with the specified number of rows.
func (s *PostgresChaosStore) CreateTestTable(ctx context.Context, name string, rows int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create table
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			data TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`, name))
	if err != nil {
		return err
	}

	// Insert rows in batches
	for i := 0; i < rows; i += 1000 {
		batch := rows - i
		if batch > 1000 {
			batch = 1000
		}

		query := fmt.Sprintf("INSERT INTO %s (data) VALUES ", name)
		for j := 0; j < batch; j++ {
			if j > 0 {
				query += ", "
			}
			query += fmt.Sprintf("('test_data_%d')", i+j)
		}

		if _, err := tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DropTestTable drops a test table.
func (s *PostgresChaosStore) DropTestTable(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", name))
	return err
}

// GetRowCount returns the number of rows in a table.
func (s *PostgresChaosStore) GetRowCount(ctx context.Context, name string) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", name)).Scan(&count)
	return count, err
}

// InjectNetworkPartition simulates a network partition by killing connections.
func (s *PostgresChaosStore) InjectNetworkPartition(ctx context.Context, duration time.Duration) error {
	// Get current backend PIDs
	rows, err := s.db.QueryContext(ctx, "SELECT pid FROM pg_stat_activity WHERE state = 'active' AND pid != pg_backend_pid()")
	if err != nil {
		return err
	}
	defer rows.Close()

	var pids []int
	for rows.Next() {
		var pid int
		if err := rows.Scan(&pid); err != nil {
			return err
		}
		pids = append(pids, pid)
	}

	// Cancel queries (simulate network partition)
	for _, pid := range pids {
		_, _ = s.db.ExecContext(ctx, fmt.Sprintf("SELECT pg_cancel_backend(%d)", pid))
	}

	// Wait for duration
	time.Sleep(duration)

	return nil
}

// InjectReplicationLag simulates replication lag.
func (s *PostgresChaosStore) InjectReplicationLag(ctx context.Context, lagMs int) error {
	// Set statement_timeout to simulate lag
	_, err := s.db.ExecContext(ctx, fmt.Sprintf("SET statement_timeout = '%d'", lagMs))
	return err
}

// InjectLockTimeout simulates lock timeout.
func (s *PostgresChaosStore) InjectLockTimeout(ctx context.Context, timeoutMs int) error {
	// Set lock_timeout to a low value
	_, err := s.db.ExecContext(ctx, fmt.Sprintf("SET lock_timeout = '%d'", timeoutMs))
	return err
}

// InjectConnectionExhaustion simulates connection pool exhaustion.
func (s *PostgresChaosStore) InjectConnectionExhaustion(ctx context.Context, maxConns int) error {
	// Set max connections to a very low value
	_, err := s.db.ExecContext(ctx, fmt.Sprintf("SET max_connections = '%d'", maxConns))
	return err
}

// ResetAll resets all chaos injection settings.
func (s *PostgresChaosStore) ResetAll(ctx context.Context) error {
	// Reset all settings to defaults
	_, err := s.db.ExecContext(ctx, `
		SET statement_timeout = '0';
		SET lock_timeout = '0';
	`)
	return err
}

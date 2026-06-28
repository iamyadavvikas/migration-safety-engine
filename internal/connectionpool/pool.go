// Package connectionpool provides PgBouncer integration for connection pooling.
package connectionpool

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Config holds connection pool configuration.
type Config struct {
	// PgBouncer settings
	PgBouncerHost     string
	PgBouncerPort     int
	PgBouncerDatabase string

	// Pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Health check settings
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration

	// Retry settings
	MaxRetries    int
	RetryDelay    time.Duration
	MaxRetryDelay time.Duration
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		PgBouncerHost:     "localhost",
		PgBouncerPort:     6432,
		PgBouncerDatabase: "mse",

		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,

		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,

		MaxRetries:    3,
		RetryDelay:    100 * time.Millisecond,
		MaxRetryDelay: 5 * time.Second,
	}
}

// PoolManager manages database connections with PgBouncer.
type PoolManager struct {
	config    Config
	logger    *slog.Logger
	primaryDB *sql.DB
	replicaDB *sql.DB
	mu        sync.RWMutex
	healthy   bool
	lastCheck time.Time
}

// NewPoolManager creates a new pool manager.
func NewPoolManager(config Config, logger *slog.Logger) *PoolManager {
	return &PoolManager{
		config:  config,
		logger:  logger,
		healthy: true,
	}
}

// Connect establishes connection to PgBouncer.
func (pm *PoolManager) Connect(ctx context.Context, dsn string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var err error

	// Connect to primary
	pm.primaryDB, err = sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open primary connection: %w", err)
	}

	// Configure pool
	pm.primaryDB.SetMaxOpenConns(pm.config.MaxOpenConns)
	pm.primaryDB.SetMaxIdleConns(pm.config.MaxIdleConns)
	pm.primaryDB.SetConnMaxLifetime(pm.config.ConnMaxLifetime)
	pm.primaryDB.SetConnMaxIdleTime(pm.config.ConnMaxIdleTime)

	// Test connection
	if err := pm.primaryDB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping primary: %w", err)
	}

	// Start health check goroutine
	go pm.healthCheckLoop(ctx)

	pm.logger.InfoContext(ctx, "connected to PgBouncer",
		"host", pm.config.PgBouncerHost,
		"port", pm.config.PgBouncerPort,
		"max_open_conns", pm.config.MaxOpenConns,
	)

	return nil
}

// ConnectReplica establishes connection to replica via PgBouncer.
func (pm *PoolManager) ConnectReplica(ctx context.Context, replicaDSN string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var err error
	pm.replicaDB, err = sql.Open("pgx", replicaDSN)
	if err != nil {
		return fmt.Errorf("failed to open replica connection: %w", err)
	}

	// Configure pool
	pm.replicaDB.SetMaxOpenConns(pm.config.MaxOpenConns / 2) // Half for replica
	pm.replicaDB.SetMaxIdleConns(pm.config.MaxIdleConns / 2)
	pm.replicaDB.SetConnMaxLifetime(pm.config.ConnMaxLifetime)
	pm.replicaDB.SetConnMaxIdleTime(pm.config.ConnMaxIdleTime)

	// Test connection
	if err := pm.replicaDB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping replica: %w", err)
	}

	pm.logger.InfoContext(ctx, "connected to replica via PgBouncer")
	return nil
}

// GetPrimary returns the primary database connection.
func (pm *PoolManager) GetPrimary() *sql.DB {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.primaryDB
}

// GetReplica returns the replica database connection.
func (pm *PoolManager) GetReplica() *sql.DB {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.replicaDB
}

// IsHealthy returns whether the connection pool is healthy.
func (pm *PoolManager) IsHealthy() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.healthy
}

// GetStats returns connection pool statistics.
func (pm *PoolManager) GetStats() PoolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := PoolStats{
		PrimaryOpen:     pm.primaryDB.Stats().OpenConnections,
		PrimaryInUse:    pm.primaryDB.Stats().InUse,
		PrimaryIdle:     pm.primaryDB.Stats().Idle,
		PrimaryWaitCount: pm.primaryDB.Stats().WaitCount,
		PrimaryWaitDuration: pm.primaryDB.Stats().WaitDuration,
	}

	if pm.replicaDB != nil {
		stats.ReplicaOpen = pm.replicaDB.Stats().OpenConnections
		stats.ReplicaInUse = pm.replicaDB.Stats().InUse
		stats.ReplicaIdle = pm.replicaDB.Stats().Idle
	}

	return stats
}

// PoolStats holds connection pool statistics.
type PoolStats struct {
	PrimaryOpen         int
	PrimaryInUse        int
	PrimaryIdle         int
	PrimaryWaitCount    int64
	PrimaryWaitDuration time.Duration
	ReplicaOpen         int
	ReplicaInUse        int
	ReplicaIdle         int
}

// healthCheckLoop periodically checks database health.
func (pm *PoolManager) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(pm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.checkHealth(ctx)
		}
	}
}

// checkHealth checks database health.
func (pm *PoolManager) checkHealth(ctx context.Context) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, pm.config.HealthCheckTimeout)
	defer cancel()

	// Check primary
	if err := pm.primaryDB.PingContext(ctx); err != nil {
		pm.logger.ErrorContext(ctx, "primary health check failed", "error", err)
		pm.healthy = false
		return
	}

	// Check replica if configured
	if pm.replicaDB != nil {
		if err := pm.replicaDB.PingContext(ctx); err != nil {
			pm.logger.WarnContext(ctx, "replica health check failed", "error", err)
			// Replica failure is not critical
		}
	}

	pm.healthy = true
	pm.lastCheck = time.Now()
}

// Close closes all database connections.
func (pm *PoolManager) Close() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var errs []error

	if pm.primaryDB != nil {
		if err := pm.primaryDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close primary: %w", err))
		}
	}

	if pm.replicaDB != nil {
		if err := pm.replicaDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close replica: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing connections: %v", errs)
	}

	return nil
}

// ExecuteWithRetry executes a function with retry logic.
func (pm *PoolManager) ExecuteWithRetry(ctx context.Context, fn func(*sql.DB) error) error {
	var lastErr error

	for attempt := 0; attempt <= pm.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := pm.config.RetryDelay * time.Duration(1<<(attempt-1))
			if delay > pm.config.MaxRetryDelay {
				delay = pm.config.MaxRetryDelay
			}

			pm.logger.WarnContext(ctx, "retrying operation",
				"attempt", attempt,
				"delay", delay,
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		lastErr = fn(pm.primaryDB)
		if lastErr == nil {
			return nil
		}

		pm.logger.WarnContext(ctx, "operation failed",
			"attempt", attempt,
			"error", lastErr,
		)
	}

	return fmt.Errorf("operation failed after %d attempts: %w", pm.config.MaxRetries+1, lastErr)
}

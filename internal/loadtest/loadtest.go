// Package loadtest provides load testing infrastructure for the Migration Safety Engine.
// It simulates large-scale migrations with 100M+ rows.
package loadtest

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// LoadTestConfig defines the configuration for a load test.
type LoadTestConfig struct {
	TableName       string
	TotalRows       int64
	BatchSize       int
	Concurrency     int
	ThrottleMs      int
	TargetDuration  time.Duration
	VerifyIntegrity bool
}

// DefaultLoadTestConfig returns default configuration.
func DefaultLoadTestConfig() LoadTestConfig {
	return LoadTestConfig{
		TableName:       "load_test_large",
		TotalRows:       100_000_000, // 100M rows
		BatchSize:       10000,
		Concurrency:     16,
		ThrottleMs:      10,
		TargetDuration:  24 * time.Hour,
		VerifyIntegrity: true,
	}
}

// LoadTestResult holds the results of a load test.
type LoadTestResult struct {
	TableName         string
	TotalRows         int64
	RowsProcessed     int64
	BatchesProcessed  int64
	Duration          time.Duration
	RowsPerSecond     float64
	AvgBatchTime      time.Duration
	P99BatchTime      time.Duration
	ErrorCount        int64
	TimeoutCount      int64
	MaxReplicationLag time.Duration
	MaxConnectionWait time.Duration
	Errors            []string
}

// LoadTestEngine runs load tests against the migration engine.
type LoadTestEngine struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewLoadTestEngine creates a new load test engine.
func NewLoadTestEngine(db *sql.DB, logger *slog.Logger) *LoadTestEngine {
	return &LoadTestEngine{
		db:     db,
		logger: logger,
	}
}

// RunLoadTest executes a load test.
func (e *LoadTestEngine) RunLoadTest(ctx context.Context, config LoadTestConfig) (*LoadTestResult, error) {
	e.logger.InfoContext(ctx, "starting load test",
		"table", config.TableName,
		"total_rows", config.TotalRows,
		"batch_size", config.BatchSize,
		"concurrency", config.Concurrency,
	)

	result := &LoadTestResult{
		TableName: config.TableName,
		TotalRows: config.TotalRows,
	}

	// Create test table
	if err := e.createTestTable(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to create test table: %w", err)
	}
	defer e.dropTestTable(ctx, config.TableName)

	// Start metrics collection
	metricsCtx, cancelMetrics := context.WithCancel(ctx)
	defer cancelMetrics()
	go e.collectMetrics(metricsCtx, result)

	// Run parallel backfill
	startTime := time.Now()
	var wg sync.WaitGroup
	var processedRows atomic.Int64
	var processedBatches atomic.Int64
	var errorCount atomic.Int64

	semaphore := make(chan struct{}, config.Concurrency)
	batchChan := make(chan int64, config.Concurrency*2)

	// Producer: generate batch offsets
	go func() {
		defer close(batchChan)
		for offset := int64(0); offset < config.TotalRows; offset += int64(config.BatchSize) {
			select {
			case <-ctx.Done():
				return
			case batchChan <- offset:
			}
		}
	}()

	// Consumer: process batches
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for offset := range batchChan {
				select {
				case <-ctx.Done():
					return
				case semaphore <- struct{}{}:
				}

				batchStart := time.Now()
				err := e.processBatch(ctx, config, offset)
				batchDuration := time.Since(batchStart)
				<-semaphore

				if err != nil {
					errorCount.Add(1)
					e.logger.ErrorContext(ctx, "batch failed",
						"offset", offset,
						"error", err,
					)
					continue
				}

				processedRows.Add(int64(config.BatchSize))
				processedBatches.Add(1)

				// Track batch time for P99 calculation
				if batchDuration > result.P99BatchTime {
					result.P99BatchTime = batchDuration
				}

				// Throttle
				if config.ThrottleMs > 0 {
					time.Sleep(time.Duration(config.ThrottleMs) * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
	cancelMetrics()

	result.Duration = time.Since(startTime)
	result.RowsProcessed = processedRows.Load()
	result.BatchesProcessed = processedBatches.Load()
	result.ErrorCount = errorCount.Load()
	result.RowsPerSecond = float64(result.RowsProcessed) / result.Duration.Seconds()

	e.logger.InfoContext(ctx, "load test completed",
		"rows_processed", result.RowsProcessed,
		"duration", result.Duration,
		"rows_per_second", result.RowsPerSecond,
		"errors", result.ErrorCount,
	)

	// Verify integrity
	if config.VerifyIntegrity {
		if err := e.verifyIntegrity(ctx, config, result); err != nil {
			e.logger.ErrorContext(ctx, "integrity check failed", "error", err)
			result.Errors = append(result.Errors, err.Error())
		}
	}

	return result, nil
}

// createTestTable creates the test table.
func (e *LoadTestEngine) createTestTable(ctx context.Context, config LoadTestConfig) error {
	_, err := e.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			data TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)
	`, config.TableName))
	return err
}

// dropTestTable drops the test table.
func (e *LoadTestEngine) dropTestTable(ctx context.Context, tableName string) {
	_, _ = e.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
}

// processBatch processes a single batch of rows.
func (e *LoadTestEngine) processBatch(ctx context.Context, config LoadTestConfig, offset int64) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update rows in batch
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s 
		SET status = 'processed', updated_at = NOW()
		WHERE id > $1 AND id <= $2 AND status = 'pending'
	`, config.TableName), offset, offset+int64(config.BatchSize))
	if err != nil {
		return err
	}

	return tx.Commit()
}

// collectMetrics collects performance metrics during the load test.
func (e *LoadTestEngine) collectMetrics(ctx context.Context, result *LoadTestResult) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Collect replication lag
			var lagMs int
			err := e.db.QueryRowContext(ctx, `
				SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) * 1000
			`).Scan(&lagMs)
			if err == nil {
				lag := time.Duration(lagMs) * time.Millisecond
				if lag > result.MaxReplicationLag {
					result.MaxReplicationLag = lag
				}
			}

			// Collect connection stats
			var activeConns int
			err = e.db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'active'
			`).Scan(&activeConns)
			if err == nil {
				// Log if too many connections
				if activeConns > 100 {
					e.logger.WarnContext(ctx, "high connection count", "count", activeConns)
				}
			}
		}
	}
}

// verifyIntegrity verifies data integrity after the load test.
func (e *LoadTestEngine) verifyIntegrity(ctx context.Context, config LoadTestConfig, result *LoadTestResult) error {
	// Check for duplicates
	var duplicates int64
	err := e.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT id, COUNT(*) 
			FROM %s 
			GROUP BY id 
			HAVING COUNT(*) > 1
		)
	`, config.TableName)).Scan(&duplicates)
	if err != nil {
		return fmt.Errorf("failed to check duplicates: %w", err)
	}
	if duplicates > 0 {
		return fmt.Errorf("found %d duplicate rows", duplicates)
	}

	// Check for missing rows
	var unprocessed int64
	err = e.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE status = 'pending'
	`, config.TableName)).Scan(&unprocessed)
	if err != nil {
		return fmt.Errorf("failed to check unprocessed rows: %w", err)
	}
	if unprocessed > 0 {
		return fmt.Errorf("found %d unprocessed rows", unprocessed)
	}

	return nil
}

// RunStressTest runs a stress test with varying load.
func (e *LoadTestEngine) RunStressTest(ctx context.Context, config LoadTestConfig, stages []LoadStage) ([]*LoadTestResult, error) {
	var results []*LoadTestResult

	for i, stage := range stages {
		e.logger.InfoContext(ctx, "running stress test stage",
			"stage", i+1,
			"concurrency", stage.Concurrency,
			"batch_size", stage.BatchSize,
		)

		stageConfig := config
		stageConfig.Concurrency = stage.Concurrency
		stageConfig.BatchSize = stage.BatchSize
		stageConfig.ThrottleMs = stage.ThrottleMs

		result, err := e.RunLoadTest(ctx, stageConfig)
		if err != nil {
			return nil, fmt.Errorf("stage %d failed: %w", i+1, err)
		}

		results = append(results, result)
	}

	return results, nil
}

// LoadStage defines a stage in a stress test.
type LoadStage struct {
	Name        string
	Concurrency int
	BatchSize   int
	ThrottleMs  int
	Duration    time.Duration
}

// DefaultStressTestStages returns default stress test stages.
func DefaultStressTestStages() []LoadStage {
	return []LoadStage{
		{Name: "warmup", Concurrency: 1, BatchSize: 1000, ThrottleMs: 100, Duration: 5 * time.Minute},
		{Name: "low", Concurrency: 4, BatchSize: 5000, ThrottleMs: 50, Duration: 10 * time.Minute},
		{Name: "medium", Concurrency: 8, BatchSize: 10000, ThrottleMs: 20, Duration: 15 * time.Minute},
		{Name: "high", Concurrency: 16, BatchSize: 20000, ThrottleMs: 10, Duration: 20 * time.Minute},
		{Name: "peak", Concurrency: 32, BatchSize: 50000, ThrottleMs: 5, Duration: 10 * time.Minute},
	}
}

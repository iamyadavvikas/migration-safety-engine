// Package shard provides sharded backfill for large tables (100M+ rows).
package shard

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds sharded backfill configuration.
type Config struct {
	TableName       string
	TotalRows       int64
	ShardCount      int
	BatchSize       int
	Concurrency     int
	ThrottleMs      int
	VerifyIntegrity bool
}

// DefaultConfig returns default configuration for 100M+ row tables.
func DefaultConfig() Config {
	return Config{
		TableName:       "large_table",
		TotalRows:       100_000_000,
		ShardCount:      16,
		BatchSize:       10000,
		Concurrency:     16,
		ThrottleMs:      10,
		VerifyIntegrity: true,
	}
}

// ShardResult holds the result of a sharded backfill.
type ShardResult struct {
	ShardID          int
	TableName        string
	RowsProcessed    int64
	BatchesProcessed int64
	Duration         time.Duration
	Error            error
}

// BackfillResult holds the overall backfill result.
type BackfillResult struct {
	TableName        string
	TotalRows        int64
	RowsProcessed    int64
	BatchesProcessed int64
	Duration         time.Duration
	RowsPerSecond    float64
	ShardResults     []*ShardResult
	Errors           []string
}

// ShardedBackfillEngine performs sharded backfill for large tables.
type ShardedBackfillEngine struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewShardedBackfillEngine creates a new sharded backfill engine.
func NewShardedBackfillEngine(db *sql.DB, logger *slog.Logger) *ShardedBackfillEngine {
	return &ShardedBackfillEngine{
		db:     db,
		logger: logger,
	}
}

// RunShardedBackfill executes a sharded backfill.
func (e *ShardedBackfillEngine) RunShardedBackfill(ctx context.Context, config Config) (*BackfillResult, error) {
	e.logger.InfoContext(ctx, "starting sharded backfill",
		"table", config.TableName,
		"total_rows", config.TotalRows,
		"shards", config.ShardCount,
		"concurrency", config.Concurrency,
	)

	startTime := time.Now()

	// Calculate shard ranges
	shardRanges := e.calculateShardRanges(config)
	e.logger.InfoContext(ctx, "calculated shard ranges",
		"shards", len(shardRanges),
	)

	// Start metrics collection
	result := &BackfillResult{
		TableName: config.TableName,
		TotalRows: config.TotalRows,
	}

	// Run shards in parallel
	var wg sync.WaitGroup
	var processedRows atomic.Int64
	var processedBatches atomic.Int64
	var shardResults []*ShardResult

	semaphore := make(chan struct{}, config.Concurrency)

	for i, shard := range shardRanges {
		wg.Add(1)
		go func(shardID int, minID, maxID int64) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			shardResult := e.runShard(ctx, config, shardID, minID, maxID)
			shardResults = append(shardResults, shardResult)

			processedRows.Add(shardResult.RowsProcessed)
			processedBatches.Add(shardResult.BatchesProcessed)
		}(i, shard.MinID, shard.MaxID)
	}

	wg.Wait()

	result.Duration = time.Since(startTime)
	result.RowsProcessed = processedRows.Load()
	result.BatchesProcessed = processedBatches.Load()
	result.RowsPerSecond = float64(result.RowsProcessed) / result.Duration.Seconds()
	result.ShardResults = shardResults

	// Verify integrity
	if config.VerifyIntegrity {
		if err := e.verifyIntegrity(ctx, config, result); err != nil {
			e.logger.ErrorContext(ctx, "integrity check failed", "error", err)
			result.Errors = append(result.Errors, err.Error())
		}
	}

	e.logger.InfoContext(ctx, "sharded backfill completed",
		"rows_processed", result.RowsProcessed,
		"duration", result.Duration,
		"rows_per_second", result.RowsPerSecond,
	)

	return result, nil
}

// ShardRange represents a range of IDs for a shard.
type ShardRange struct {
	MinID int64
	MaxID int64
}

// calculateShardRanges calculates ID ranges for each shard.
func (e *ShardedBackfillEngine) calculateShardRanges(config Config) []ShardRange {
	shardSize := config.TotalRows / int64(config.ShardCount)
	ranges := make([]ShardRange, config.ShardCount)

	for i := 0; i < config.ShardCount; i++ {
		minID := int64(i)*shardSize + 1
		maxID := int64(i+1) * shardSize

		if i == config.ShardCount-1 {
			// Last shard gets remaining rows
			maxID = config.TotalRows
		}

		ranges[i] = ShardRange{
			MinID: minID,
			MaxID: maxID,
		}
	}

	return ranges
}

// runShard processes a single shard.
func (e *ShardedBackfillEngine) runShard(ctx context.Context, config Config, shardID int, minID, maxID int64) *ShardResult {
	shardStart := time.Now()
	result := &ShardResult{
		ShardID:   shardID,
		TableName: config.TableName,
	}

	e.logger.DebugContext(ctx, "processing shard",
		"shard_id", shardID,
		"min_id", minID,
		"max_id", maxID,
	)

	// Process batches within shard
	for offset := minID; offset <= maxID; offset += int64(config.BatchSize) {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result
		default:
		}

		batchEnd := offset + int64(config.BatchSize)
		if batchEnd > maxID+1 {
			batchEnd = maxID + 1
		}

		err := e.processBatch(ctx, config, offset, batchEnd)
		if err != nil {
			result.Error = err
			return result
		}

		result.RowsProcessed += batchEnd - offset
		result.BatchesProcessed++

		// Throttle
		if config.ThrottleMs > 0 {
			time.Sleep(time.Duration(config.ThrottleMs) * time.Millisecond)
		}
	}

	result.Duration = time.Since(shardStart)
	return result
}

// processBatch processes a single batch of rows.
func (e *ShardedBackfillEngine) processBatch(ctx context.Context, config Config, startID, endID int64) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update rows in batch
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s 
		SET status = 'processed', updated_at = NOW()
		WHERE id >= $1 AND id < $2 AND status = 'pending'
	`, config.TableName), startID, endID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// verifyIntegrity verifies data integrity after the backfill.
func (e *ShardedBackfillEngine) verifyIntegrity(ctx context.Context, config Config, result *BackfillResult) error {
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

// AdaptiveShardSizer dynamically adjusts shard count based on table size.
type AdaptiveShardSizer struct {
	logger *slog.Logger
}

// NewAdaptiveShardSizer creates a new adaptive shard sizer.
func NewAdaptiveShardSizer(logger *slog.Logger) *AdaptiveShardSizer {
	return &AdaptiveShardSizer{logger: logger}
}

// CalculateOptimalShards calculates the optimal number of shards.
func (s *AdaptiveShardSizer) CalculateOptimalShards(totalRows int64, targetDuration time.Duration, avgBatchTime time.Duration) int {
	// Calculate how many batches we need
	batchSize := int64(10000)
	totalBatches := totalRows / batchSize

	// Calculate how many batches we can process in target duration
	batchesPerShard := int64(targetDuration.Seconds() / avgBatchTime.Seconds())

	if batchesPerShard == 0 {
		batchesPerShard = 1
	}

	// Calculate optimal shard count
	optimalShards := int(totalBatches / batchesPerShard)

	// Apply bounds
	if optimalShards < 1 {
		optimalShards = 1
	}
	if optimalShards > 64 {
		optimalShards = 64
	}

	s.logger.Info("calculated optimal shards",
		"total_rows", totalRows,
		"target_duration", targetDuration,
		"avg_batch_time", avgBatchTime,
		"optimal_shards", optimalShards,
	)

	return optimalShards
}

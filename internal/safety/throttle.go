package safety

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// BackfillConfig holds configuration for adaptive backfill throttling.
type BackfillConfig struct {
	InitialBatchSize        int           // Starting batch size (default: 5000)
	MinBatchSize            int           // Minimum batch size (default: 100)
	MaxBatchSize            int           // Maximum batch size (default: 50000)
	InitialThrottleMs       int           // Starting throttle in ms (default: 20)
	MinThrottleMs           int           // Minimum throttle (default: 0)
	MaxThrottleMs           int           // Maximum throttle (default: 5000)
	HealthCheckInterval     time.Duration // How often to check DB health (default: 5s)
	CircuitBreakerThreshold float64       // Health score to trip breaker (default: 0.1)
}

// DefaultBackfillConfig returns production-safe defaults.
func DefaultBackfillConfig() BackfillConfig {
	return BackfillConfig{
		InitialBatchSize:        5000,
		MinBatchSize:            100,
		MaxBatchSize:            50000,
		InitialThrottleMs:       20,
		MinThrottleMs:           0,
		MaxThrottleMs:           5000,
		HealthCheckInterval:     5 * time.Second,
		CircuitBreakerThreshold: 0.1,
	}
}

// AdaptiveThrottle controls backfill pacing based on DB health.
type AdaptiveThrottle struct {
	config   BackfillConfig
	executor *DDLExecutor
	log      *slog.Logger

	// Current state
	currentBatchSize  int
	currentThrottleMs int
	healthScore       float64
	circuitTripped    bool
	tripCount         int
	consecutiveGood   int
	consecutiveBad    int

	// Recovery thresholds
	recoveryThreshold float64 // Health score to untrip (default: 0.3)

	// Metrics history
	metricsHistory []DBHealthMetrics
}

// NewAdaptiveThrottle creates a new adaptive throttle controller.
func NewAdaptiveThrottle(config BackfillConfig, executor *DDLExecutor, log *slog.Logger) *AdaptiveThrottle {
	return &AdaptiveThrottle{
		config:            config,
		executor:          executor,
		log:               log,
		currentBatchSize:  config.InitialBatchSize,
		currentThrottleMs: config.InitialThrottleMs,
		healthScore:       1.0,
		recoveryThreshold: 0.3,
	}
}

// HealthScore calculates a 0-1 score based on DB metrics.
// 1.0 = perfectly healthy, 0.0 = critical.
func (at *AdaptiveThrottle) HealthScore(m *DBHealthMetrics) float64 {
	if m == nil {
		return 0.5 // Unknown health
	}

	// Connection score (0-1, 1 = plenty of capacity)
	connScore := 1.0
	if m.MaxConns > 0 {
		connScore = 1.0 - float64(m.ActiveConns)/float64(m.MaxConns)
	}

	// Replication lag score (0-1, 1 = no lag)
	replScore := 1.0
	if m.Replication.MaxLagMs > 0 {
		replScore = 1.0 - m.Replication.MaxLagMs/1000.0
		if replScore < 0 {
			replScore = 0
		}
	}

	// Lock queue score (0-1, 1 = no contention)
	lockScore := 1.0
	if m.LockQueue.WaitingQueries > 0 {
		lockScore = 1.0 - float64(m.LockQueue.WaitingQueries)/10.0
		if lockScore < 0 {
			lockScore = 0
		}
	}

	// Dead tuple score (0-1, 1 = no bloat)
	bloatScore := 1.0
	if m.TuplesDead > 1000000 {
		bloatScore = 0.5
	} else if m.TuplesDead > 100000 {
		bloatScore = 0.75
	}

	// Weighted average
	score := 0.3*connScore + 0.3*replScore + 0.2*lockScore + 0.2*bloatScore

	return score
}

// AdjustThrottle updates throttle settings based on health score.
func (at *AdaptiveThrottle) AdjustThrottle() {
	// Circuit breaker tripped — check for recovery
	if at.circuitTripped {
		if at.healthScore > at.recoveryThreshold {
			at.consecutiveGood++
			at.consecutiveBad = 0
			// Require 3 consecutive good checks to recover
			if at.consecutiveGood >= 3 {
				at.circuitTripped = false
				at.consecutiveGood = 0
				at.log.Info("CIRCUIT BREAKER RECOVERED",
					"health_score", at.healthScore,
					"recovery_threshold", at.recoveryThreshold)
				// Reset to initial values
				at.currentBatchSize = at.config.InitialBatchSize
				at.currentThrottleMs = at.config.InitialThrottleMs
			}
		} else {
			at.consecutiveGood = 0
			at.consecutiveBad++
			at.log.Debug("circuit breaker still tripped",
				"health_score", at.healthScore,
				"consecutive_bad", at.consecutiveBad)
		}
		return
	}

	// Check if we should trip the circuit breaker
	if at.healthScore < at.config.CircuitBreakerThreshold {
		at.consecutiveBad++
		at.consecutiveGood = 0
		// Require 3 consecutive bad checks to trip
		if at.consecutiveBad >= 3 {
			at.circuitTripped = true
			at.tripCount++
			at.consecutiveBad = 0
			at.log.Error("CIRCUIT BREAKER TRIPPED",
				"health_score", at.healthScore,
				"threshold", at.config.CircuitBreakerThreshold,
				"trip_count", at.tripCount)
			return
		}
	} else {
		at.consecutiveGood++
		at.consecutiveBad = 0
	}

	// Adjust based on health
	if at.healthScore > 0.8 {
		// Healthy: increase batch size, decrease throttle
		at.currentBatchSize = min(at.currentBatchSize*15/10, at.config.MaxBatchSize)
		at.currentThrottleMs = max(at.currentThrottleMs-5, at.config.MinThrottleMs)
	} else if at.healthScore < 0.3 {
		// Unhealthy: decrease batch size, increase throttle
		at.currentBatchSize = max(at.currentBatchSize*5/10, at.config.MinBatchSize)
		at.currentThrottleMs = min(at.currentThrottleMs+50, at.config.MaxThrottleMs)
	} else if at.healthScore < 0.5 {
		// Moderate: slight adjustment
		at.currentBatchSize = max(at.currentBatchSize*8/10, at.config.MinBatchSize)
		at.currentThrottleMs = min(at.currentThrottleMs+20, at.config.MaxThrottleMs)
	}

	at.log.Debug("throttle adjusted",
		"health_score", fmt.Sprintf("%.2f", at.healthScore),
		"batch_size", at.currentBatchSize,
		"throttle_ms", at.currentThrottleMs)
}

// UpdateHealth checks DB health and adjusts throttle.
func (at *AdaptiveThrottle) UpdateHealth(ctx context.Context) error {
	metrics, err := at.executor.CollectHealthMetrics(ctx)
	if err != nil {
		at.log.Warn("failed to collect health metrics", "err", err)
		return err
	}

	at.healthScore = at.HealthScore(metrics)
	at.metricsHistory = append(at.metricsHistory, *metrics)

	// Keep only last 100 samples
	if len(at.metricsHistory) > 100 {
		at.metricsHistory = at.metricsHistory[1:]
	}

	at.AdjustThrottle()
	return nil
}

// GetBatchSize returns the current adaptive batch size.
func (at *AdaptiveThrottle) GetBatchSize() int {
	return at.currentBatchSize
}

// GetThrottleMs returns the current adaptive throttle in milliseconds.
func (at *AdaptiveThrottle) GetThrottleMs() int {
	return at.currentThrottleMs
}

// IsCircuitTripped returns whether the circuit breaker has tripped.
func (at *AdaptiveThrottle) IsCircuitTripped() bool {
	return at.circuitTripped
}

// ResetCircuitBreaker manually resets the circuit breaker.
func (at *AdaptiveThrottle) ResetCircuitBreaker() {
	at.circuitTripped = false
	at.currentBatchSize = at.config.InitialBatchSize
	at.currentThrottleMs = at.config.InitialThrottleMs
	at.log.Info("circuit breaker reset")
}

// GetMetricsSummary returns a summary of recent health metrics.
func (at *AdaptiveThrottle) GetMetricsSummary() map[string]interface{} {
	if len(at.metricsHistory) == 0 {
		return map[string]interface{}{"status": "no_data"}
	}

	latest := at.metricsHistory[len(at.metricsHistory)-1]
	return map[string]interface{}{
		"health_score":    fmt.Sprintf("%.2f", at.healthScore),
		"batch_size":      at.currentBatchSize,
		"throttle_ms":     at.currentThrottleMs,
		"circuit_tripped": at.circuitTripped,
		"active_conns":    latest.ActiveConns,
		"max_conns":       latest.MaxConns,
		"replication_lag": fmt.Sprintf("%.0fms", latest.Replication.MaxLagMs),
		"lock_queue":      latest.LockQueue.WaitingQueries,
		"dead_tuples":     latest.TuplesDead,
		"metrics_samples": len(at.metricsHistory),
	}
}

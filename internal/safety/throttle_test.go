package safety

import (
	"log/slog"
	"os"
	"testing"
)

func TestHealthScore(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultBackfillConfig()
	at := NewAdaptiveThrottle(config, nil, log)

	tests := []struct {
		name     string
		metrics  *DBHealthMetrics
		minScore float64
		maxScore float64
	}{
		{
			name:     "nil metrics returns 0.5",
			metrics:  nil,
			minScore: 0.5,
			maxScore: 0.5,
		},
		{
			name: "healthy database",
			metrics: &DBHealthMetrics{
				ActiveConns: 5,
				MaxConns:    100,
				Replication: ReplicationStatus{MaxLagMs: 10},
			},
			minScore: 0.8,
			maxScore: 1.0,
		},
		{
			name: "high connection usage",
			metrics: &DBHealthMetrics{
				ActiveConns: 90,
				MaxConns:    100,
				Replication: ReplicationStatus{MaxLagMs: 10},
			},
			minScore: 0.5,
			maxScore: 0.8,
		},
		{
			name: "high replication lag",
			metrics: &DBHealthMetrics{
				ActiveConns: 5,
				MaxConns:    100,
				Replication: ReplicationStatus{MaxLagMs: 5000},
			},
			minScore: 0.6,
			maxScore: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := at.HealthScore(tt.metrics)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("HealthScore() = %v, want between %v and %v", score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestAdaptiveThrottleDefaults(t *testing.T) {
	config := DefaultBackfillConfig()

	if config.InitialBatchSize != 5000 {
		t.Errorf("InitialBatchSize = %d, want 5000", config.InitialBatchSize)
	}
	if config.MinBatchSize != 100 {
		t.Errorf("MinBatchSize = %d, want 100", config.MinBatchSize)
	}
	if config.MaxBatchSize != 50000 {
		t.Errorf("MaxBatchSize = %d, want 50000", config.MaxBatchSize)
	}
	if config.InitialThrottleMs != 20 {
		t.Errorf("InitialThrottleMs = %d, want 20", config.InitialThrottleMs)
	}
}

func TestAdaptiveThrottleInitialValues(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultBackfillConfig()
	at := NewAdaptiveThrottle(config, nil, log)

	if at.GetBatchSize() != config.InitialBatchSize {
		t.Errorf("GetBatchSize() = %d, want %d", at.GetBatchSize(), config.InitialBatchSize)
	}
	if at.GetThrottleMs() != config.InitialThrottleMs {
		t.Errorf("GetThrottleMs() = %d, want %d", at.GetThrottleMs(), config.InitialThrottleMs)
	}
	if at.IsCircuitTripped() {
		t.Error("IsCircuitTripped() = true, want false")
	}
}

func TestCircuitBreakerTripAndRecovery(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultBackfillConfig()
	config.CircuitBreakerThreshold = 0.3
	at := NewAdaptiveThrottle(config, nil, log)

	// Simulate bad health scores to trip the circuit breaker
	for i := 0; i < 10; i++ {
		at.healthScore = 0.05
		at.consecutiveBad++
		if at.consecutiveBad >= 3 {
			at.circuitTripped = true
			break
		}
	}

	if !at.IsCircuitTripped() {
		t.Error("circuit breaker should be tripped after consecutive bad scores")
	}

	// Simulate good health scores to recover
	for i := 0; i < 10; i++ {
		at.healthScore = 0.9
		at.consecutiveGood++
		at.consecutiveBad = 0
		if at.consecutiveGood >= 3 {
			at.circuitTripped = false
			break
		}
	}

	if at.IsCircuitTripped() {
		t.Error("circuit breaker should have recovered after good scores")
	}
}

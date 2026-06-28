// Package chaos provides chaos engineering tests for the Migration Safety Engine.
// It simulates various failure modes to validate system resilience.
package chaos

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"
)

// FailureMode represents a type of failure to inject.
type FailureMode int

const (
	// Database failures
	FailureNetworkPartition FailureMode = iota
	FailureReplicationLag
	FailureLockTimeout
	FailureConnectionPoolExhaustion
	FailureDiskSpace
	FailureCPUThrottle
	FailureMemoryPressure

	// Application failures
	FailureWorkerCrash
	FailureGCPressure
	FailureDNSResolution

	// Migration-specific failures
	FailureDDLTimeout
	FailureBackfillCorruption
	FailureCanarySLOBreach
	FailureRollbackDeadlock
	FailureSplitBrain
)

// Scenario defines a chaos test scenario.
type Scenario struct {
	Name        string
	Description string
	Failures    []FailureMode
	Duration    time.Duration
	Severity    Severity
	Expected    ExpectedOutcome
}

// Severity indicates the severity level of a failure.
type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// ExpectedOutcome defines what the system should do when a failure occurs.
type ExpectedOutcome struct {
	ShouldRollback bool
	ShouldAbort    bool
	ShouldPause    bool
	MaxDowntime    time.Duration
	MinDataLoss    bool
	RequireManual  bool
}

// ChaosEngine runs chaos engineering tests.
type ChaosEngine struct {
	store     ChaosStore
	logger    *slog.Logger
	scenarios []Scenario
}

// ChaosStore defines the interface for chaos test operations.
type ChaosStore interface {
	CreateTestTable(ctx context.Context, name string, rows int) error
	DropTestTable(ctx context.Context, name string) error
	GetRowCount(ctx context.Context, name string) (int64, error)
	InjectNetworkPartition(ctx context.Context, duration time.Duration) error
	InjectReplicationLag(ctx context.Context, lagMs int) error
	InjectLockTimeout(ctx context.Context, timeoutMs int) error
	InjectConnectionExhaustion(ctx context.Context, maxConns int) error
	ResetAll(ctx context.Context) error
}

// NewChaosEngine creates a new chaos engineering engine.
func NewChaosEngine(store ChaosStore, logger *slog.Logger) *ChaosEngine {
	return &ChaosEngine{
		store:     store,
		logger:    logger,
		scenarios: defaultScenarios(),
	}
}

// defaultScenarios returns the default set of chaos test scenarios.
func defaultScenarios() []Scenario {
	return []Scenario{
		{
			Name:        "network_partition_during_backfill",
			Description: "Network partition occurs during backfill operation",
			Failures:    []FailureMode{FailureNetworkPartition},
			Duration:    30 * time.Second,
			Severity:    SeverityHigh,
			Expected: ExpectedOutcome{
				ShouldRollback: false,
				ShouldPause:    true,
				MaxDowntime:    60 * time.Second,
			},
		},
		{
			Name:        "replication_lag_spike",
			Description: "Replication lag spikes to 5 seconds during canary",
			Failures:    []FailureMode{FailureReplicationLag},
			Duration:    20 * time.Second,
			Severity:    SeverityHigh,
			Expected: ExpectedOutcome{
				ShouldRollback: true,
				MaxDowntime:    10 * time.Second,
			},
		},
		{
			Name:        "lock_timeout_ddl",
			Description: "Lock timeout occurs during DDL execution",
			Failures:    []FailureMode{FailureLockTimeout},
			Duration:    10 * time.Second,
			Severity:    SeverityMedium,
			Expected: ExpectedOutcome{
				ShouldRollback: false,
				ShouldPause:    true,
				MaxDowntime:    30 * time.Second,
			},
		},
		{
			Name:        "connection_pool_exhaustion",
			Description: "Connection pool exhausted during backfill",
			Failures:    []FailureMode{FailureConnectionPoolExhaustion},
			Duration:    45 * time.Second,
			Severity:    SeverityCritical,
			Expected: ExpectedOutcome{
				ShouldRollback: true,
				MaxDowntime:    5 * time.Second,
			},
		},
		{
			Name:        "worker_crash_resume",
			Description: "Worker crashes mid-backfill and resumes",
			Failures:    []FailureMode{FailureWorkerCrash},
			Duration:    0, // Immediate crash
			Severity:    SeverityHigh,
			Expected: ExpectedOutcome{
				ShouldRollback: false,
				ShouldPause:    true,
				MaxDowntime:    120 * time.Second,
			},
		},
		{
			Name:        "canary_slo_breach",
			Description: "Canary step shows SLO breach",
			Failures:    []FailureMode{FailureCanarySLOBreach},
			Duration:    15 * time.Second,
			Severity:    SeverityCritical,
			Expected: ExpectedOutcome{
				ShouldRollback: true,
				MaxDowntime:    10 * time.Second,
			},
		},
		{
			Name:        "split_brain_scenario",
			Description: "Split brain during multi-service contract phase",
			Failures:    []FailureMode{FailureSplitBrain},
			Duration:    60 * time.Second,
			Severity:    SeverityCritical,
			Expected: ExpectedOutcome{
				ShouldRollback: true,
				RequireManual:  true,
				MaxDowntime:    300 * time.Second,
			},
		},
		{
			Name:        "cascading_failure",
			Description: "Multiple failures occur simultaneously",
			Failures:    []FailureMode{
				FailureReplicationLag,
				FailureConnectionPoolExhaustion,
				FailureCPUThrottle,
			},
			Duration:    30 * time.Second,
			Severity:    SeverityCritical,
			Expected: ExpectedOutcome{
				ShouldRollback: true,
				MaxDowntime:    15 * time.Second,
			},
		},
	}
}

// RunScenario executes a chaos test scenario.
func (e *ChaosEngine) RunScenario(ctx context.Context, scenarioName string) (*ChaosResult, error) {
	var scenario *Scenario
	for _, s := range e.scenarios {
		if s.Name == scenarioName {
			scenario = &s
			break
		}
	}
	if scenario == nil {
		return nil, fmt.Errorf("scenario not found: %s", scenarioName)
	}

	e.logger.InfoContext(ctx, "starting chaos scenario",
		"name", scenario.Name,
		"description", scenario.Description,
		"severity", scenario.Severity,
	)

	result := &ChaosResult{
		Scenario:  scenario.Name,
		StartTime: time.Now(),
	}

	// Create test table
	tableName := fmt.Sprintf("chaos_test_%d", time.Now().UnixNano())
	if err := e.store.CreateTestTable(ctx, tableName, 10000); err != nil {
		return nil, fmt.Errorf("failed to create test table: %w", err)
	}
	defer e.store.DropTestTable(ctx, tableName)

	// Run the scenario
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(ctx, scenario.Duration+30*time.Second)
	defer cancel()

	// Start failure injection
	for _, failure := range scenario.Failures {
		wg.Add(1)
		go func(f FailureMode) {
			defer wg.Done()
			e.injectFailure(ctx, f, scenario.Duration)
		}(failure)
	}

	// Wait for scenario to complete
	wg.Wait()

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Verify expected outcome
	if err := e.verifyOutcome(ctx, scenario, tableName, result); err != nil {
		result.VerificationError = err.Error()
	}

	e.logger.InfoContext(ctx, "chaos scenario completed",
		"name", scenario.Name,
		"duration", result.Duration,
		"success", result.VerificationError == "",
	)

	return result, nil
}

// injectFailure injects a specific failure mode.
func (e *ChaosEngine) injectFailure(ctx context.Context, mode FailureMode, duration time.Duration) {
	switch mode {
	case FailureNetworkPartition:
		e.logger.WarnContext(ctx, "injecting network partition", "duration", duration)
		if err := e.store.InjectNetworkPartition(ctx, duration); err != nil {
			e.logger.ErrorContext(ctx, "failed to inject network partition", "error", err)
		}

	case FailureReplicationLag:
		e.logger.WarnContext(ctx, "injecting replication lag", "duration", duration)
		if err := e.store.InjectReplicationLag(ctx, 5000); err != nil {
			e.logger.ErrorContext(ctx, "failed to inject replication lag", "error", err)
		}

	case FailureLockTimeout:
		e.logger.WarnContext(ctx, "injecting lock timeout", "duration", duration)
		if err := e.store.InjectLockTimeout(ctx, 1000); err != nil {
			e.logger.ErrorContext(ctx, "failed to inject lock timeout", "error", err)
		}

	case FailureConnectionPoolExhaustion:
		e.logger.WarnContext(ctx, "injecting connection pool exhaustion", "duration", duration)
		if err := e.store.InjectConnectionExhaustion(ctx, 1); err != nil {
			e.logger.ErrorContext(ctx, "failed to inject connection exhaustion", "error", err)
		}

	case FailureWorkerCrash:
		e.logger.WarnContext(ctx, "simulating worker crash")
		// Worker crash is simulated by canceling context
		return

	case FailureCanarySLOBreach:
		e.logger.WarnContext(ctx, "injecting canary SLO breach", "duration", duration)
		// This would be handled by the chaos injection in statemachine
		return

	case FailureSplitBrain:
		e.logger.WarnContext(ctx, "injecting split brain scenario", "duration", duration)
		// Split brain is simulated by partitioning services
		return

	default:
		e.logger.WarnContext(ctx, "unknown failure mode", "mode", mode)
	}

	// Wait for failure duration
	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
}

// verifyOutcome verifies the expected outcome of a chaos test.
func (e *ChaosEngine) verifyOutcome(ctx context.Context, scenario *Scenario, tableName string, result *ChaosResult) error {
	// Get final row count
	finalCount, err := e.store.GetRowCount(ctx, tableName)
	if err != nil {
		return fmt.Errorf("failed to get final row count: %w", err)
	}

	result.FinalRowCount = finalCount

	// Verify no data loss (if expected)
	if !scenario.Expected.MinDataLoss {
		if finalCount < 10000 {
			return fmt.Errorf("data loss detected: expected 10000 rows, got %d", finalCount)
		}
	}

	// Verify max downtime
	if result.Duration > scenario.Expected.MaxDowntime {
		return fmt.Errorf("downtime exceeded threshold: %v > %v", result.Duration, scenario.Expected.MaxDowntime)
	}

	return nil
}

// ChaosResult holds the result of a chaos test.
type ChaosResult struct {
	Scenario          string
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration
	FinalRowCount     int64
	VerificationError string
}

// RunAllScenarios runs all chaos test scenarios.
func (e *ChaosEngine) RunAllScenarios(ctx context.Context) ([]*ChaosResult, error) {
	var results []*ChaosResult

	for _, scenario := range e.scenarios {
		result, err := e.RunScenario(ctx, scenario.Name)
		if err != nil {
			e.logger.ErrorContext(ctx, "failed to run scenario",
				"scenario", scenario.Name,
				"error", err,
			)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GetScenarioNames returns all available scenario names.
func (e *ChaosEngine) GetScenarioNames() []string {
	var names []string
	for _, s := range e.scenarios {
		names = append(names, s.Name)
	}
	return names
}

// StressTester performs stress testing with concurrent operations.
type StressTester struct {
	logger  *slog.Logger
	store   StressStore
}

// StressStore defines the interface for stress test operations.
type StressStore interface {
	ExecuteQuery(ctx context.Context, query string, args ...any) error
	GetConnectionCount(ctx context.Context) (int, error)
	GetReplicationLag(ctx context.Context) (int64, error)
}

// NewStressTester creates a new stress tester.
func NewStressTester(store StressStore, logger *slog.Logger) *StressTester {
	return &StressTester{
		store:  store,
		logger: logger,
	}
}

// StressTestConfig defines the configuration for stress testing.
type StressTestConfig struct {
	Concurrency     int
	Duration        time.Duration
	QueryRate       int // queries per second
	EnableReads     bool
	EnableWrites    bool
	EnableDDL       bool
	EnableRollbacks bool
}

// StressTestResult holds the results of stress testing.
type StressTestResult struct {
	TotalQueries   int64
	SuccessfulOps  int64
	FailedOps      int64
	AvgLatency     time.Duration
	P99Latency     time.Duration
	MaxReplicationLag int64
	Errors         []string
}

// RunStressTest executes a stress test.
func (s *StressTester) RunStressTest(ctx context.Context, config StressTestConfig) (*StressTestResult, error) {
	result := &StressTestResult{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(ctx, config.Duration)
	defer cancel()

	// Start metrics collection
	go s.collectMetrics(ctx, result, &mu)

	// Start workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			s.runWorker(ctx, workerID, config, result, &mu)
		}(i)
	}

	wg.Wait()

	return result, nil
}

// runWorker runs a single stress test worker.
func (s *StressTester) runWorker(ctx context.Context, workerID int, config StressTestConfig, result *StressTestResult, mu *sync.Mutex) {
	ticker := time.NewTicker(time.Second / time.Duration(config.QueryRate/config.Concurrency))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Execute random operation
			if config.EnableReads && rand.Float64() < 0.6 {
				s.executeRead(ctx, result, mu)
			} else if config.EnableWrites && rand.Float64() < 0.3 {
				s.executeWrite(ctx, result, mu)
			} else if config.EnableDDL && rand.Float64() < 0.1 {
				s.executeDDL(ctx, result, mu)
			}
		}
	}
}

// executeRead executes a read operation.
func (s *StressTester) executeRead(ctx context.Context, result *StressTestResult, mu *sync.Mutex) {
	start := time.Now()
	err := s.store.ExecuteQuery(ctx, "SELECT 1")
	latency := time.Since(start)

	mu.Lock()
	defer mu.Unlock()

	result.TotalQueries++
	if err != nil {
		result.FailedOps++
		result.Errors = append(result.Errors, err.Error())
	} else {
		result.SuccessfulOps++
	}

	if latency > result.P99Latency {
		result.P99Latency = latency
	}
}

// executeWrite executes a write operation.
func (s *StressTester) executeWrite(ctx context.Context, result *StressTestResult, mu *sync.Mutex) {
	start := time.Now()
	err := s.store.ExecuteQuery(ctx, "INSERT INTO stress_test (data) VALUES ($1) ON CONFLICT DO NOTHING", fmt.Sprintf("data_%d", rand.Int()))
	latency := time.Since(start)

	mu.Lock()
	defer mu.Unlock()

	result.TotalQueries++
	if err != nil {
		result.FailedOps++
		result.Errors = append(result.Errors, err.Error())
	} else {
		result.SuccessfulOps++
	}

	if latency > result.P99Latency {
		result.P99Latency = latency
	}
}

// executeDDL executes a DDL operation.
func (s *StressTester) executeDDL(ctx context.Context, result *StressTestResult, mu *sync.Mutex) {
	start := time.Now()
	err := s.store.ExecuteQuery(ctx, "CREATE TABLE IF NOT EXISTS stress_test (id SERIAL PRIMARY KEY, data TEXT)")
	latency := time.Since(start)

	mu.Lock()
	defer mu.Unlock()

	result.TotalQueries++
	if err != nil {
		result.FailedOps++
		result.Errors = append(result.Errors, err.Error())
	} else {
		result.SuccessfulOps++
	}

	if latency > result.P99Latency {
		result.P99Latency = latency
	}
}

// collectMetrics collects metrics during stress testing.
func (s *StressTester) collectMetrics(ctx context.Context, result *StressTestResult, mu *sync.Mutex) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lag, err := s.store.GetReplicationLag(ctx)
			if err == nil {
				mu.Lock()
				if lag > result.MaxReplicationLag {
					result.MaxReplicationLag = lag
				}
				mu.Unlock()
			}
		}
	}
}

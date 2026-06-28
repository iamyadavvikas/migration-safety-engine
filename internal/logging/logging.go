// Package logging provides structured JSON logging for the MSE engine.
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// Config holds logging configuration.
type Config struct {
	Level     string `json:"level"`      // debug, info, warn, error
	Format    string `json:"format"`     // json, text
	Output    string `json:"output"`     // stdout, stderr, or file path
	AddSource bool   `json:"add_source"` // Add source file/line
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		Level:     "info",
		Format:    "json",
		Output:    "stdout",
		AddSource: false,
	}
}

// NewLogger creates a new structured logger.
func NewLogger(cfg Config) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var output io.Writer
	switch cfg.Output {
	case "stderr":
		output = os.Stderr
	case "stdout", "":
		output = os.Stdout
	default:
		// File output
		f, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			output = os.Stdout
		} else {
			output = f
		}
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	return slog.New(handler)
}

// MigrationEntry represents a structured log entry for migration events.
type MigrationEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`
	MigrationID string    `json:"migration_id"`
	PlanID      string    `json:"plan_id,omitempty"`
	State       string    `json:"state,omitempty"`
	Event       string    `json:"event"`
	Detail      string    `json:"detail,omitempty"`
	Duration    string    `json:"duration,omitempty"`
	Error       string    `json:"error,omitempty"`
	Metadata    any       `json:"metadata,omitempty"`
}

// MigrationLogger provides structured logging for migration events.
type MigrationLogger struct {
	logger *slog.Logger
}

// NewMigrationLogger creates a new migration logger.
func NewMigrationLogger(logger *slog.Logger) *MigrationLogger {
	return &MigrationLogger{logger: logger}
}

// LogStateTransition logs a state machine transition.
func (l *MigrationLogger) LogStateTransition(
	ctx context.Context,
	migrationID, planID string,
	fromState, toState string,
	detail string,
) {
	l.logger.InfoContext(ctx, "state transition",
		"migration_id", migrationID,
		"plan_id", planID,
		"from_state", fromState,
		"to_state", toState,
		"detail", detail,
	)
}

// LogBackfillProgress logs backfill batch progress.
func (l *MigrationLogger) LogBackfillProgress(
	ctx context.Context,
	migrationID string,
	batchNum, rowsDone, totalRows int64,
	lastID int64,
	healthScore float64,
	batchSize, throttleMs int,
) {
	l.logger.InfoContext(ctx, "backfill progress",
		"migration_id", migrationID,
		"batch_num", batchNum,
		"rows_done", rowsDone,
		"total_rows", totalRows,
		"last_id", lastID,
		"health_score", fmt.Sprintf("%.2f", healthScore),
		"batch_size", batchSize,
		"throttle_ms", throttleMs,
	)
}

// LogCanaryStep logs a canary observation.
func (l *MigrationLogger) LogCanaryStep(
	ctx context.Context,
	migrationID string,
	step int,
	p99Ms, errPct float64,
	breached bool,
	breachReason string,
) {
	if breached {
		l.logger.WarnContext(ctx, "canary SLO breach",
			"migration_id", migrationID,
			"step", step,
			"p99_ms", p99Ms,
			"err_pct", errPct,
			"breach_reason", breachReason,
		)
	} else {
		l.logger.InfoContext(ctx, "canary step healthy",
			"migration_id", migrationID,
			"step", step,
			"p99_ms", p99Ms,
			"err_pct", errPct,
		)
	}
}

// LogDDLExecution logs a DDL execution.
func (l *MigrationLogger) LogDDLExecution(
	ctx context.Context,
	migrationID string,
	stmt string,
	duration time.Duration,
	success bool,
	err error,
) {
	entry := l.logger.InfoContext
	level := "info"
	if !success {
		entry = l.logger.ErrorContext
		level = "error"
	}

	args := []any{
		"migration_id", migrationID,
		"stmt", truncate(stmt, 200),
		"duration", duration.String(),
		"success", success,
	}
	if err != nil {
		args = append(args, "error", err.Error())
	}

	entry(ctx, fmt.Sprintf("ddl execution (%s)", level), args...)
}

// LogCircuitBreaker logs circuit breaker events.
func (l *MigrationLogger) LogCircuitBreaker(
	ctx context.Context,
	migrationID string,
	event string, // tripped, recovered
	healthScore float64,
	threshold float64,
) {
	if event == "tripped" {
		l.logger.ErrorContext(ctx, "circuit breaker tripped",
			"migration_id", migrationID,
			"health_score", healthScore,
			"threshold", threshold,
		)
	} else {
		l.logger.InfoContext(ctx, "circuit breaker recovered",
			"migration_id", migrationID,
			"health_score", healthScore,
			"recovery_threshold", threshold,
		)
	}
}

// LogServiceEvent logs a service registry event.
func (l *MigrationLogger) LogServiceEvent(
	ctx context.Context,
	migrationID string,
	serviceName string,
	event string, // registered, heartbeat, compatible, incompatible
	detail string,
) {
	l.logger.InfoContext(ctx, fmt.Sprintf("service %s", event),
		"migration_id", migrationID,
		"service_name", serviceName,
		"detail", detail,
	)
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// AuditEntry represents an audit log entry for API mutations.
type AuditEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	User       string    `json:"user"`
	Role       string    `json:"role"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id,omitempty"`
	Details    any       `json:"details,omitempty"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
}

// AuditLogger logs API mutations for compliance.
type AuditLogger struct {
	logger *slog.Logger
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(logger *slog.Logger) *AuditLogger {
	return &AuditLogger{logger: logger}
}

// LogMutation logs an API mutation.
func (a *AuditLogger) LogMutation(
	ctx context.Context,
	user, role, action, resource, resourceID string,
	details any,
	success bool,
	err error,
	ipAddress string,
) {
	entry := AuditEntry{
		Timestamp:  time.Now(),
		User:       user,
		Role:       role,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
		Success:    success,
		IPAddress:  ipAddress,
	}
	if err != nil {
		entry.Error = err.Error()
	}

	// Marshal to JSON for structured output
	data, _ := json.Marshal(entry)
	a.logger.InfoContext(ctx, "audit mutation", "json", string(data))
}

// ExtractUser extracts user info from context.
func ExtractUser(ctx context.Context) (string, string) {
	// This would integrate with the auth package
	// For now, return defaults
	return "system", "admin"
}

// ExtractIP extracts IP address from request context.
func ExtractIP(ctx context.Context) string {
	// This would extract from http.Request context
	return "unknown"
}

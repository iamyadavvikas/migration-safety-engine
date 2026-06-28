// Package ghost provides gh-ost integration for online schema changes.
// gh-ost is a tool for MySQL online schema changes without blocking reads/writes.
package ghost

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// Config holds gh-ost configuration.
type Config struct {
	// Database connection
	Host     string
	Port     int
	User     string
	Password string
	Database string

	// gh-ost settings
	ChunkSize              int
	MaxLagMillisecond      int
	ThrottleAdditionalHTTP string
	AllowOnBefore          bool
	InitiallyDropGhostTable bool
	InitiallyDropOldTable  bool
	CriticalLoad           string
	MaxLoad                string
	ServerID               int
	Table                  string
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		Host:                   "localhost",
		Port:                   3306,
		User:                   "root",
		Database:               "mse",
		ChunkSize:              1000,
		MaxLagMillisecond:      500,
		AllowOnBefore:          true,
		InitiallyDropGhostTable: true,
		InitiallyDropOldTable:  true,
		MaxLoad:                "Threads_running=25",
		ServerID:               100,
	}
}

// GhostManager manages gh-ost operations.
type GhostManager struct {
	config  Config
	logger  *slog.Logger
}

// NewGhostManager creates a new gh-ost manager.
func NewGhostManager(config Config, logger *slog.Logger) *GhostManager {
	return &GhostManager{
		config:  config,
		logger:  logger,
	}
}

// MigrationRequest represents a gh-ost migration request.
type MigrationRequest struct {
	Database      string
	Table         string
	AlterStatement string
}

// MigrationResult represents the result of a gh-ost migration.
type MigrationResult struct {
	Success    bool
	Duration   time.Duration
	Error      string
	RowsCopied int64
	TableNew   string
	TableOld   string
}

// ExecuteMigration runs a gh-ost migration.
func (g *GhostManager) ExecuteMigration(ctx context.Context, req MigrationRequest) (*MigrationResult, error) {
	g.logger.InfoContext(ctx, "starting gh-ost migration",
		"database", req.Database,
		"table", req.Table,
		"alter", req.AlterStatement,
	)

	startTime := time.Now()
	result := &MigrationResult{}

	// Build gh-ost command
	args := g.buildArgs(req)

	// Execute gh-ost
	cmd := exec.CommandContext(ctx, "gh-ost", args...)
	output, err := cmd.CombinedOutput()

	result.Duration = time.Since(startTime)
	result.Error = string(output)

	if err != nil {
		g.logger.ErrorContext(ctx, "gh-ost migration failed",
			"error", err,
			"output", string(output),
		)
		result.Success = false
		return result, fmt.Errorf("gh-ost failed: %w", err)
	}

	result.Success = true
	g.logger.InfoContext(ctx, "gh-ost migration completed",
		"duration", result.Duration,
	)

	return result, nil
}

// buildArgs builds gh-ost command arguments.
func (g *GhostManager) buildArgs(req MigrationRequest) []string {
	args := []string{
		fmt.Sprintf("--host=%s", g.config.Host),
		fmt.Sprintf("--port=%d", g.config.Port),
		fmt.Sprintf("--user=%s", g.config.User),
		fmt.Sprintf("--password=%s", g.config.Password),
		fmt.Sprintf("--database=%s", req.Database),
		fmt.Sprintf("--table=%s", req.Table),
		fmt.Sprintf("--alter=%s", req.AlterStatement),
		fmt.Sprintf("--chunk-size=%d", g.config.ChunkSize),
		fmt.Sprintf("--max-lag-millisecond=%d", g.config.MaxLagMillisecond),
		fmt.Sprintf("--max-load=%s", g.config.MaxLoad),
		fmt.Sprintf("--server-id=%d", g.config.ServerID),
		"--execute",
		"--verbose",
	}

	if g.config.InitiallyDropGhostTable {
		args = append(args, "--initially-drop-ghost-table")
	}

	if g.config.InitiallyDropOldTable {
		args = append(args, "--initially-drop-old-table")
	}

	if g.config.CriticalLoad != "" {
		args = append(args, fmt.Sprintf("--critical-load=%s", g.config.CriticalLoad))
	}

	if g.config.ThrottleAdditionalHTTP != "" {
		args = append(args, fmt.Sprintf("--throttle-additional-http=%s", g.config.ThrottleAdditionalHTTP))
	}

	return args
}

// Validate checks if gh-ost is available and the configuration is valid.
func (g *GhostManager) Validate(ctx context.Context) error {
	// Check if gh-ost is installed
	_, err := exec.LookPath("gh-ost")
	if err != nil {
		return fmt.Errorf("gh-ost not found in PATH: %w", err)
	}

	// Test database connection
	cmd := exec.CommandContext(ctx, "gh-ost",
		fmt.Sprintf("--host=%s", g.config.Host),
		fmt.Sprintf("--port=%d", g.config.Port),
		fmt.Sprintf("--user=%s", g.config.User),
		fmt.Sprintf("--password=%s", g.config.Password),
		fmt.Sprintf("--database=%s", g.config.Database),
		"--test-on-replica",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh-ost validation failed: %s", string(output))
	}

	return nil
}

// PTOscManager manages pt-online-schema-change operations.
type PTOscManager struct {
	config  Config
	logger  *slog.Logger
}

// NewPTOscManager creates a new pt-online-schema-change manager.
func NewPTOscManager(config Config, logger *slog.Logger) *PTOscManager {
	return &PTOscManager{
		config:  config,
		logger:  logger,
	}
}

// ExecuteMigration runs a pt-online-schema-change migration.
func (p *PTOscManager) ExecuteMigration(ctx context.Context, req MigrationRequest) (*MigrationResult, error) {
	p.logger.InfoContext(ctx, "starting pt-online-schema-change migration",
		"database", req.Database,
		"table", req.Table,
		"alter", req.AlterStatement,
	)

	startTime := time.Now()
	result := &MigrationResult{}

	// Build pt-online-schema-change command
	args := p.buildArgs(req)

	// Execute pt-online-schema-change
	cmd := exec.CommandContext(ctx, "pt-online-schema-change", args...)
	output, err := cmd.CombinedOutput()

	result.Duration = time.Since(startTime)
	result.Error = string(output)

	if err != nil {
		p.logger.ErrorContext(ctx, "pt-online-schema-change migration failed",
			"error", err,
			"output", string(output),
		)
		result.Success = false
		return result, fmt.Errorf("pt-online-schema-change failed: %w", err)
	}

	result.Success = true
	p.logger.InfoContext(ctx, "pt-online-schema-change migration completed",
		"duration", result.Duration,
	)

	return result, nil
}

// buildArgs builds pt-online-schema-change command arguments.
func (p *PTOscManager) buildArgs(req MigrationRequest) []string {
	dsn := fmt.Sprintf("h=%s,p=%s,u=%s,D=%s,t=%s",
		p.config.Host,
		p.config.Password,
		p.config.User,
		req.Database,
		req.Table,
	)

	args := []string{
		fmt.Sprintf("--alter=%s", req.AlterStatement),
		fmt.Sprintf("--chunk-size=%d", p.config.ChunkSize),
		fmt.Sprintf("--max-lag=%s", time.Duration(p.config.MaxLagMillisecond)*time.Millisecond),
		"--execute",
		"--verbose",
		"--print",
		dsn,
	}

	if p.config.CriticalLoad != "" {
		args = append(args, fmt.Sprintf("--critical-load=%s", p.config.CriticalLoad))
	}

	if p.config.MaxLoad != "" {
		args = append(args, fmt.Sprintf("--max-load=%s", p.config.MaxLoad))
	}

	return args
}

// Validate checks if pt-online-schema-change is available and the configuration is valid.
func (p *PTOscManager) Validate(ctx context.Context) error {
	// Check if pt-online-schema-change is installed
	_, err := exec.LookPath("pt-online-schema-change")
	if err != nil {
		return fmt.Errorf("pt-online-schema-change not found in PATH: %w", err)
	}

	return nil
}

// DDLToolSelector selects the appropriate DDL tool based on database type and migration requirements.
type DDLToolSelector struct {
	logger *slog.Logger
}

// NewDDLToolSelector creates a new DDL tool selector.
func NewDDLToolSelector(logger *slog.Logger) *DDLToolSelector {
	return &DDLToolSelector{logger: logger}
}

// ToolType represents the type of DDL tool.
type ToolType int

const (
	ToolTypeNative ToolType = iota
	ToolTypeGhOst
	ToolTypePTOsc
)

// SelectTool selects the appropriate DDL tool based on the migration request.
func (s *DDLToolSelector) SelectTool(dbType string, req MigrationRequest) ToolType {
	// gh-ost is preferred for MySQL
	if strings.ToLower(dbType) == "mysql" {
		// Check if the ALTER statement is compatible with gh-ost
		if s.isGhOstCompatible(req.AlterStatement) {
			s.logger.Info("selecting gh-ost for migration",
				"table", req.Table,
				"alter", req.AlterStatement,
			)
			return ToolTypeGhOst
		}
	}

	// pt-online-schema-change is a fallback for MySQL
	if strings.ToLower(dbType) == "mysql" {
		s.logger.Info("selecting pt-online-schema-change for migration",
			"table", req.Table,
			"alter", req.AlterStatement,
		)
		return ToolTypePTOsc
	}

	// For PostgreSQL, use native DDL with advisory locks
	s.logger.Info("selecting native DDL for PostgreSQL",
		"table", req.Table,
		"alter", req.AlterStatement,
	)
	return ToolTypeNative
}

// isGhOstCompatible checks if an ALTER statement is compatible with gh-ost.
func (s *DDLToolSelector) isGhOstCompatible(alter string) bool {
	// gh-ost doesn't support certain operations
	unsupported := []string{
		"ADD PRIMARY KEY",
		"DROP PRIMARY KEY",
		"ADD FOREIGN KEY",
		"DROP FOREIGN KEY",
		"ADD INDEX",
		"DROP INDEX",
		"ADD UNIQUE INDEX",
		"DROP UNIQUE INDEX",
		"ADD FULLTEXT INDEX",
		"DROP FULLTEXT INDEX",
		"ADD SPATIAL INDEX",
		"DROP SPATIAL INDEX",
		"ADD COLUMN",
		"DROP COLUMN",
		"MODIFY COLUMN",
		"CHANGE COLUMN",
		"RENAME COLUMN",
		"RENAME TABLE",
		"RENAME INDEX",
		"RENAME KEY",
		"RENAME TO",
		"CONVERT TO CHARACTER SET",
		"CONVERT TO COLLATE",
		"ENGINE=",
		"AUTO_INCREMENT=",
		"COMMENT=",
		"DEFAULT=",
		"DROP DEFAULT",
		"SET DEFAULT",
		"ALGORITHM=",
		"LOCK=",
	}

	upperAlter := strings.ToUpper(alter)
	for _, u := range unsupported {
		if strings.Contains(upperAlter, u) {
			return false
		}
	}

	return true
}

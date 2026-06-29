package safety

import (
	"testing"
	"time"
)

func TestDefaultDDLConfig(t *testing.T) {
	config := DefaultDDLConfig()

	if config.LockTimeout != 3*time.Second {
		t.Errorf("LockTimeout = %v, want 3s", config.LockTimeout)
	}
	if config.StatementTimeout != 60*time.Second {
		t.Errorf("StatementTimeout = %v, want 60s", config.StatementTimeout)
	}
	if config.MaxLockQueue != 5 {
		t.Errorf("MaxLockQueue = %d, want 5", config.MaxLockQueue)
	}
	if config.MaxReplicationLag != 30*time.Second {
		t.Errorf("MaxReplicationLag = %v, want 30s", config.MaxReplicationLag)
	}
}

func TestDDLConfigCustomValues(t *testing.T) {
	config := DDLConfig{
		LockTimeout:       5 * time.Second,
		StatementTimeout:  120 * time.Second,
		MaxLockQueue:      10,
		MaxReplicationLag: 60 * time.Second,
	}

	if config.LockTimeout != 5*time.Second {
		t.Errorf("LockTimeout = %v, want 5s", config.LockTimeout)
	}
	if config.StatementTimeout != 120*time.Second {
		t.Errorf("StatementTimeout = %v, want 120s", config.StatementTimeout)
	}
	if config.MaxLockQueue != 10 {
		t.Errorf("MaxLockQueue = %d, want 10", config.MaxLockQueue)
	}
}

func TestLockQueueStatusFields(t *testing.T) {
	status := LockQueueStatus{
		WaitingQueries: 3,
		BlockingPID:    12345,
		BlockingQuery:  "SELECT * FROM pg_stat_activity",
	}

	if status.WaitingQueries != 3 {
		t.Errorf("WaitingQueries = %d, want 3", status.WaitingQueries)
	}
	if status.BlockingPID != 12345 {
		t.Errorf("BlockingPID = %d, want 12345", status.BlockingPID)
	}
	if status.BlockingQuery == "" {
		t.Error("BlockingQuery should not be empty")
	}
}

func TestReplicationStatusFields(t *testing.T) {
	status := ReplicationStatus{
		ReplicaCount: 2,
		MaxLagMs:     150.5,
	}

	if status.ReplicaCount != 2 {
		t.Errorf("ReplicaCount = %d, want 2", status.ReplicaCount)
	}
	if status.MaxLagMs != 150.5 {
		t.Errorf("MaxLagMs = %f, want 150.5", status.MaxLagMs)
	}
}

func TestIsConcurrentlyDDL(t *testing.T) {
	tests := []struct {
		stmt     string
		expected bool
	}{
		{"CREATE INDEX CONCURRENTLY idx ON users(id)", true},
		{"CREATE INDEX idx ON users(id)", false},
		{"ALTER TABLE users ADD COLUMN name TEXT", false},
		{"DROP INDEX CONCURRENTLY idx", true},
	}

	for _, tt := range tests {
		t.Run(tt.stmt, func(t *testing.T) {
			result := isConcurrentlyDDL(tt.stmt)
			if result != tt.expected {
				t.Errorf("isConcurrentlyDDL(%q) = %v, want %v", tt.stmt, result, tt.expected)
			}
		})
	}
}

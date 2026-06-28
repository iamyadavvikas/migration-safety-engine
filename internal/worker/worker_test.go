package worker

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockRunner struct {
	mu       sync.Mutex
	calls    []uuid.UUID
	callCount int
}

func (m *mockRunner) Run(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, id)
	m.callCount++
	return nil
}

func (m *mockRunner) GetCalls() []uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]uuid.UUID, len(m.calls))
	copy(result, m.calls)
	return result
}

func TestNewPool(t *testing.T) {
	runner := &mockRunner{}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pool := NewPool(runner, 4, log)
	if pool == nil {
		t.Fatal("NewPool() returned nil")
	}

	if pool.Workers != 4 {
		t.Errorf("NewPool() workers = %v, want %v", pool.Workers, 4)
	}
}

func TestNewPoolDefaultWorkers(t *testing.T) {
	runner := &mockRunner{}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pool := NewPool(runner, 0, log)
	if pool.Workers != 1 {
		t.Errorf("NewPool() workers = %v, want %v", pool.Workers, 1)
	}
}

func TestPoolStartStop(t *testing.T) {
	runner := &mockRunner{}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pool := NewPool(runner, 2, log)
	ctx := context.Background()

	pool.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Let workers start

	pool.Stop()
}

func TestPoolSubmit(t *testing.T) {
	runner := &mockRunner{}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pool := NewPool(runner, 2, log)
	ctx := context.Background()

	pool.Start(ctx)

	id1 := uuid.New()
	id2 := uuid.New()

	pool.Submit(id1)
	pool.Submit(id2)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	calls := runner.GetCalls()
	if len(calls) != 2 {
		t.Errorf("runner calls = %v, want %v", len(calls), 2)
	}

	pool.Stop()
}

func TestPoolStats(t *testing.T) {
	runner := &mockRunner{}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pool := NewPool(runner, 4, log)

	stats := pool.Stats()
	if stats.Workers != 4 {
		t.Errorf("Stats() workers = %v, want %v", stats.Workers, 4)
	}

	if stats.Pending != 0 {
		t.Errorf("Stats() pending = %v, want %v", stats.Pending, 0)
	}
}

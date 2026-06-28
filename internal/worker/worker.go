// Package worker provides a worker pool for parallel migration execution.
package worker

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

// MigrationRunner is the interface for running migrations.
type MigrationRunner interface {
	Run(ctx context.Context, id uuid.UUID) error
}

// Pool manages a fixed set of workers that execute migrations concurrently.
type Pool struct {
	runner  MigrationRunner
	log     *slog.Logger
Workers int
	jobs    chan uuid.UUID
	wg      sync.WaitGroup
	cancel  context.CancelFunc
}

// NewPool creates a new worker pool with the specified number of workers.
func NewPool(runner MigrationRunner, workers int, log *slog.Logger) *Pool {
	if workers <= 0 {
		workers = 1
	}
	return &Pool{
		runner:  runner,
		log:     log,
		Workers: workers,
		jobs:    make(chan uuid.UUID, workers*2),
	}
}

// Start starts the worker pool and begins processing jobs.
func (p *Pool) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)

	for i := 0; i < p.Workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}

	p.log.Info("worker pool started", "workers", p.Workers)
}

// Stop gracefully stops the worker pool.
func (p *Pool) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	close(p.jobs)
	p.wg.Wait()
	p.log.Info("worker pool stopped")
}

// Submit adds a migration to the worker pool for execution.
func (p *Pool) Submit(id uuid.UUID) {
	p.jobs <- id
}

// worker processes jobs from the queue.
func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()

	p.log.Info("worker started", "worker", id)

	for {
		select {
		case <-ctx.Done():
			p.log.Info("worker stopping", "worker", id)
			return
		case jobID, ok := <-p.jobs:
			if !ok {
				p.log.Info("worker stopping (channel closed)", "worker", id)
				return
			}

			p.log.Info("worker processing migration", "worker", id, "migration", jobID)
			if err := p.runner.Run(ctx, jobID); err != nil {
				p.log.Error("worker failed to run migration", "worker", id, "migration", jobID, "err", err)
			} else {
				p.log.Info("worker completed migration", "worker", id, "migration", jobID)
			}
		}
	}
}

// Stats returns current pool statistics.
func (p *Pool) Stats() PoolStats {
	return PoolStats{
		Workers: p.Workers,
		Pending: len(p.jobs),
	}
}

// PoolStats represents worker pool statistics.
type PoolStats struct {
	Workers int
	Pending int
}

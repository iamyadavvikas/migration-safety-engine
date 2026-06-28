// Package statemachine implements the durable, resumable migration state machine.
//
// Each state has a handler. After a handler succeeds, the new state + checkpoint
// are persisted atomically (store.SaveState). On restart, the engine reloads the
// last persisted state and re-enters that handler, so a crash mid-migration
// resumes rather than restarting.
//
// Phase 2 handlers do real work against the TARGET database: Expanding/Contracting
// run the plan's DDL, Backfilling runs a batched, throttled, crash-resumable
// UPDATE, and Verifying confirms convergence. Canary/Cutover remain lightweight
// (no real traffic system yet) and are filled in by later phases.
package statemachine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamyadavvikas/migration-safety-engine/internal/plan"
	"github.com/iamyadavvikas/migration-safety-engine/internal/safety"
	"github.com/iamyadavvikas/migration-safety-engine/internal/store"
	"github.com/iamyadavvikas/migration-safety-engine/internal/telemetry"
)

// State is a node in the migration lifecycle.
type State string

const (
	StatePending     State = "Pending"
	StateExpanding   State = "Expanding"
	StateBackfilling State = "Backfilling"
	StateVerifying   State = "Verifying"
	StateCanary      State = "Canary"
	StateCutover     State = "Cutover"
	StateContracting State = "Contracting"
	StateDone        State = "Done"
	StateRollingBack State = "RollingBack"
	StateRolledBack  State = "RolledBack"
)

// next defines the happy-path transition table. Terminal states map to "".
var next = map[State]State{
	StatePending:     StateExpanding,
	StateExpanding:   StateBackfilling,
	StateBackfilling: StateVerifying,
	StateVerifying:   StateCanary,
	StateCanary:      StateCutover,
	StateCutover:     StateContracting,
	StateContracting: StateDone,
	StateRollingBack: StateRolledBack,
}

// IsTerminal reports whether the state ends the workflow.
func IsTerminal(s State) bool {
	return s == StateDone || s == StateRolledBack
}

// Result is what a handler returns: a checkpoint to persist and, optionally, a
// next state that overrides the static happy-path transition (used by the canary
// gate to divert into RollingBack on an SLO breach). A zero Next means "follow
// the next map".
type Result struct {
	Checkpoint map[string]any
	Next       State
}

// Handler executes the work for a state and returns a Result to persist + route on.
type Handler func(ctx context.Context, r *store.Record) (Result, error)

// Runner drives migrations through the state machine, persisting after each step.
type Runner struct {
	store    *store.Store
	target   *pgxpool.Pool
	log      *slog.Logger
	handlers map[State]Handler

	// Safety layers
	ddlExecutor      *safety.DDLExecutor
	adaptiveThrottle *safety.AdaptiveThrottle

	// stepDelay paces transitions so progress is observable in demos/tests.
	stepDelay time.Duration
}

// NewRunner builds a Runner whose handlers run real work against the target pool.
func NewRunner(st *store.Store, target *pgxpool.Pool, log *slog.Logger) *Runner {
	// Initialize safety layers
	ddlConfig := safety.DefaultDDLConfig()
	ddlExecutor := safety.NewDDLExecutor(target, ddlConfig, log)

	backfillConfig := safety.DefaultBackfillConfig()
	adaptiveThrottle := safety.NewAdaptiveThrottle(backfillConfig, ddlExecutor, log)

	r := &Runner{
		store:            st,
		target:           target,
		log:              log,
		ddlExecutor:      ddlExecutor,
		adaptiveThrottle: adaptiveThrottle,
		stepDelay:        200 * time.Millisecond,
	}
	r.handlers = r.defaultHandlers()
	return r
}

// RegisterService registers a service as dependent on a migration.
func (r *Runner) RegisterService(ctx context.Context, migrationID uuid.UUID, serviceName string, schemaVersion int) error {
	_, err := r.store.RegisterService(ctx, migrationID, serviceName, schemaVersion)
	if err != nil {
		return fmt.Errorf("register service: %w", err)
	}
	r.log.Info("service registered", "migration", migrationID, "service", serviceName, "version", schemaVersion)
	return nil
}

// UpdateServiceCompat updates a service's compatibility status.
func (r *Runner) UpdateServiceCompat(ctx context.Context, migrationID uuid.UUID, serviceName string, compatible bool) error {
	if err := r.store.UpdateServiceCompat(ctx, migrationID, serviceName, compatible); err != nil {
		return fmt.Errorf("update service compat: %w", err)
	}
	r.log.Info("service compatibility updated", "migration", migrationID, "service", serviceName, "compatible", compatible)
	return nil
}

// CheckAllServicesReady checks if all registered services are compatible.
func (r *Runner) CheckAllServicesReady(ctx context.Context, migrationID uuid.UUID) (bool, int, error) {
	return r.store.AllServicesReady(ctx, migrationID)
}

// SetStepDelay overrides the inter-step delay (tests use 0 for speed).
func (r *Runner) SetStepDelay(d time.Duration) { r.stepDelay = d }

// DriftReport summarizes how far a table has drifted from what a plan's backfill
// would produce: rows whose stored column value differs from recomputing the
// source_expr (NULLs count as drift). parity = (total-drifted)/total.
type DriftReport struct {
	Table   string  `json:"table"`
	Column  string  `json:"column"`
	Total   int64   `json:"total"`
	Nulls   int64   `json:"nulls"`
	Drifted int64   `json:"drifted"`
	Parity  float64 `json:"parity"`
}

// DriftScan recomputes the plan's backfill source_expr across the whole target
// table and reports how many rows no longer match the stored column value. It is
// read-only. Operators run it after a migration (or on a schedule) to detect
// silent divergence introduced by code paths that bypass the derived column.
func (r *Runner) DriftScan(ctx context.Context, p *plan.MigrationPlan) (*DriftReport, error) {
	col := p.Backfill.Column
	if col == "" {
		return nil, fmt.Errorf("plan %q has no backfill.column to scan", p.ID)
	}
	if p.Backfill.SourceExpr == "" {
		return nil, fmt.Errorf("plan %q has no backfill.source_expr to scan", p.ID)
	}
	return r.fullTableParity(ctx, p)
}

// fullTableParity computes total/nulls/drifted/parity for a plan's derived column
// over the ENTIRE target table (no sampling). It backs both the read-only
// DriftScan and the cutover gate, where sampling is not acceptable because the
// next step is destructive.
func (r *Runner) fullTableParity(ctx context.Context, p *plan.MigrationPlan) (*DriftReport, error) {
	col := p.Backfill.Column
	sql := fmt.Sprintf(
		`SELECT count(*) AS total,
		        count(*) FILTER (WHERE %s IS NULL) AS nulls,
		        count(*) FILTER (WHERE %s IS DISTINCT FROM (%s)) AS drifted
		   FROM %s`,
		col, col, p.Backfill.SourceExpr, p.Table,
	)
	rep := &DriftReport{Table: p.Table, Column: col, Parity: 1.0}
	if err := r.target.QueryRow(ctx, sql).Scan(&rep.Total, &rep.Nulls, &rep.Drifted); err != nil {
		return nil, fmt.Errorf("full-table parity: %w", err)
	}
	if rep.Total > 0 {
		rep.Parity = float64(rep.Total-rep.Drifted) / float64(rep.Total)
	}
	return rep, nil
}

// Run drives a single migration to a terminal state, resuming from its persisted
// state. It is safe to call again after a crash; it picks up where it left off.
func (r *Runner) Run(ctx context.Context, id uuid.UUID) error {
	rec, err := r.store.Load(ctx, id)
	if err != nil {
		return fmt.Errorf("load %s: %w", id, err)
	}

	cur := State(rec.State)
	telemetry.SetState(id.String(), rec.Plan.ID, "", string(cur))

	for !IsTerminal(cur) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		h, ok := r.handlers[cur]
		if !ok {
			return fmt.Errorf("no handler for state %q", cur)
		}

		rec, err = r.store.Load(ctx, id) // re-load to get latest checkpoint
		if err != nil {
			return err
		}

		res, err := h(ctx, rec)
		if err != nil {
			return fmt.Errorf("handler %s for %s: %w", cur, id, err)
		}

		to := res.Next
		if to == "" {
			var ok bool
			to, ok = next[cur]
			if !ok {
				return fmt.Errorf("no transition from %q", cur)
			}
		}
		terminal := IsTerminal(to)

		if err := r.store.SaveState(ctx, id, string(cur), string(to), res.Checkpoint, terminal, fmt.Sprintf("advanced from %s", cur)); err != nil {
			return fmt.Errorf("persist %s->%s: %w", cur, to, err)
		}
		telemetry.SetState(id.String(), rec.Plan.ID, string(cur), string(to))
		r.log.Info("state advanced", "migration", id, "plan", rec.Plan.ID, "from", cur, "to", to)

		cur = to
		if r.stepDelay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.stepDelay):
			}
		}
	}
	return nil
}

// defaultHandlers wires each state to its real implementation.
func (r *Runner) defaultHandlers() map[State]Handler {
	noop := func(name State) Handler {
		return func(_ context.Context, _ *store.Record) (Result, error) {
			return Result{Checkpoint: map[string]any{"step": string(name)}}, nil
		}
	}
	return map[State]Handler{
		StatePending:     noop(StatePending),
		StateExpanding:   r.expand,
		StateBackfilling: r.backfill,
		StateVerifying:   r.verify,
		StateCanary:      r.canary,
		StateCutover:     r.cutover,
		StateContracting: r.contract,
		StateRollingBack: r.rollback,
	}
}

// expand runs the plan's expand DDL on the target. Each statement runs with
// safety checks (lock_timeout, statement_timeout, lock queue monitoring, replication lag).
func (r *Runner) expand(ctx context.Context, rec *store.Record) (Result, error) {
	for i, stmt := range rec.Plan.Expand {
		start := time.Now()

		// Execute with safety wrapper
		duration, err := r.ddlExecutor.ExecDDL(ctx, stmt)

		// Log DDL execution
		logEntry := &store.DDLExecutionLogEntry{
			MigrationID: rec.ID,
			Statement:   stmt,
			StartedAt:   start,
			CompletedAt: ptrTime(time.Now()),
			DurationMs:  ptrInt(int(duration.Milliseconds())),
			Success:     err == nil,
			LockWaitMs:  0, // Would need to extract from error
		}
		if err != nil {
			logEntry.ErrorMessage = err.Error()
		}
		if logErr := r.store.LogDDLExecution(ctx, logEntry); logErr != nil {
			r.log.Warn("failed to log DDL execution", "err", logErr)
		}

		if err != nil {
			return Result{}, fmt.Errorf("expand stmt %d (%q): %w", i, stmt, err)
		}
		r.log.Info("expand applied", "migration", rec.ID, "stmt", stmt, "duration", duration)
	}
	return Result{Checkpoint: map[string]any{"expanded": len(rec.Plan.Expand)}}, nil
}

// backfill populates the new column in batches, with adaptive throttling based on
// real-time DB health metrics. Uses forward-only progress tracking with last_id
// to ensure crash-resumability without re-processing rows.
func (r *Runner) backfill(ctx context.Context, rec *store.Record) (Result, error) {
	p := rec.Plan
	col := p.Backfill.Column
	if col == "" {
		return Result{Checkpoint: map[string]any{"skipped": "no backfill column"}}, nil
	}

	// Initialize adaptive throttle with plan defaults
	batch := p.Backfill.BatchSize
	if batch <= 0 {
		batch = 1000
	}
	r.adaptiveThrottle = safety.NewAdaptiveThrottle(
		safety.BackfillConfig{
			InitialBatchSize:  batch,
			MinBatchSize:      100,
			MaxBatchSize:      50000,
			InitialThrottleMs: p.Backfill.ThrottleMs,
			MinThrottleMs:     0,
			MaxThrottleMs:     5000,
		},
		r.ddlExecutor,
		r.log,
	)

	// Resume from checkpoint — forward-only progress tracking
	var lastID int64
	var done int64
	if v, ok := rec.Checkpoint["last_id"].(float64); ok {
		lastID = int64(v)
	}
	if v, ok := rec.Checkpoint["rows_done"].(float64); ok {
		done = int64(v)
	}

	// Count remaining rows for telemetry
	var remaining int64
	countSQL := fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s IS NULL AND id > $1`, p.Table, col)
	if err := r.target.QueryRow(ctx, countSQL, lastID).Scan(&remaining); err != nil {
		return Result{}, fmt.Errorf("count remaining: %w", err)
	}
	total := done + remaining
	telemetry.SetBackfill(rec.ID.String(), p.ID, total, done)

	batchNum := 0
	if v, ok := rec.Checkpoint["batch_num"].(float64); ok {
		batchNum = int(v)
	}

	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}

		// Check circuit breaker — pause instead of fail
		if r.adaptiveThrottle.IsCircuitTripped() {
			r.log.Warn("circuit breaker tripped, pausing backfill", "migration", rec.ID)
			time.Sleep(5 * time.Second)
			continue
		}

		// Update health metrics and adjust throttle
		if err := r.adaptiveThrottle.UpdateHealth(ctx); err != nil {
			r.log.Warn("failed to update health metrics", "err", err)
		}

		// Use adaptive batch size
		currentBatch := r.adaptiveThrottle.GetBatchSize()

		// Forward-only UPDATE with idempotent expression
		// Uses subquery with id > $last_id to ensure we never re-process rows
		var updSQL string
		if p.Backfill.MultiSQL != "" {
			// Composite multi-column backfill
			updSQL = fmt.Sprintf(`
				UPDATE %s SET %s
				WHERE id IN (
					SELECT id FROM %s 
					WHERE %s IS NULL 
					  AND id > $1
					ORDER BY id 
					LIMIT $2
				)`, p.Table, p.Backfill.MultiSQL, p.Table, col)
		} else {
			// Single column backfill
			updSQL = fmt.Sprintf(`
				UPDATE %s SET %s = (%s)
				WHERE id IN (
					SELECT id FROM %s 
					WHERE %s IS NULL 
					  AND id > $1
					ORDER BY id 
					LIMIT $2
				)`, p.Table, col, p.Backfill.SourceExpr, p.Table, col)
		}

		tag, err := r.target.Exec(ctx, updSQL, lastID, currentBatch)
		if err != nil {
			return Result{}, fmt.Errorf("backfill batch: %w", err)
		}
		n := tag.RowsAffected()
		if n == 0 {
			break // All rows processed
		}

		// Get the max ID we just processed for forward-only progress
		var newLastID int64
		err = r.target.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(MAX(id), 0) FROM %s 
			WHERE %s IS NOT NULL AND id > $1`,
			p.Table, col), lastID).Scan(&newLastID)
		if err != nil {
			return Result{}, fmt.Errorf("get last id: %w", err)
		}

		// Ensure forward progress
		if newLastID <= lastID {
			r.log.Warn("no forward progress, breaking",
				"last_id", lastID, "new_last_id", newLastID)
			break
		}

		lastID = newLastID
		done += n
		batchNum++

		// Log backfill progress with health metrics
		healthMetrics, _ := r.ddlExecutor.CollectHealthMetrics(ctx)
		progressEntry := &store.BackfillProgressEntry{
			MigrationID:  rec.ID,
			BatchNumber:  batchNum,
			RowsAffected: int(n),
			ThrottleMs:   r.adaptiveThrottle.GetThrottleMs(),
		}
		if healthMetrics != nil {
			progressEntry.DBCPUPct = healthMetrics.CPUPercent
			progressEntry.DBRepLagMs = healthMetrics.Replication.MaxLagMs
			progressEntry.DBConnsPct = float64(healthMetrics.ActiveConns) / float64(healthMetrics.MaxConns) * 100
		}
		if logErr := r.store.LogBackfillProgress(ctx, progressEntry); logErr != nil {
			r.log.Warn("failed to log backfill progress", "err", logErr)
		}

		telemetry.SetBackfill(rec.ID.String(), p.ID, total, done)
		if err := r.store.SaveCheckpoint(ctx, rec.ID, string(StateBackfilling),
			map[string]any{
				"last_id":   lastID,
				"rows_done": done,
				"batch_num": batchNum,
			},
			fmt.Sprintf("backfilled %d/%d rows (batch %d, last_id=%d, %dms throttle)",
				done, total, batchNum, lastID, r.adaptiveThrottle.GetThrottleMs())); err != nil {
			return Result{}, fmt.Errorf("checkpoint: %w", err)
		}
		r.log.Info("backfill progress", "migration", rec.ID, "done", done, "total", total,
			"batch", batchNum, "last_id", lastID, "throttle_ms", r.adaptiveThrottle.GetThrottleMs(),
			"batch_size", currentBatch)

		// Adaptive throttle
		throttle := time.Duration(r.adaptiveThrottle.GetThrottleMs()) * time.Millisecond
		if throttle > 0 {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-time.After(throttle):
			}
		}
	}
	return Result{Checkpoint: map[string]any{"last_id": lastID, "rows_done": done, "batch_num": batchNum}}, nil
}

// verify runs a shadow-read parity check: it samples rows, recomputes the
// backfill source_expr, and compares it to the stored column value. parity =
// matches/sampled. It first hard-gates on zero remaining NULLs, then gates parity
// against the plan's threshold (and slo.min_parity). This catches drift between
// backfill and cutover, not just unfilled rows.
func (r *Runner) verify(ctx context.Context, rec *store.Record) (Result, error) {
	p := rec.Plan
	col := p.Backfill.Column
	if col == "" {
		return Result{Checkpoint: map[string]any{"skipped": "no backfill column"}}, nil
	}

	// Hard gate: no row may still be NULL.
	var nulls int64
	nullSQL := fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s IS NULL`, p.Table, col)
	if err := r.target.QueryRow(ctx, nullSQL).Scan(&nulls); err != nil {
		return Result{}, fmt.Errorf("verify null count: %w", err)
	}
	if nulls > 0 {
		return Result{}, fmt.Errorf("verify failed: %d rows still have NULL %s", nulls, col)
	}

	// Parity gate: recompute source_expr on a sample and compare.
	sampleRate := p.Verify.SampleRate
	if sampleRate <= 0 || sampleRate > 1 {
		sampleRate = 1.0 // verify everything if unspecified
	}
	paritySQL := fmt.Sprintf(
		`SELECT count(*) FILTER (WHERE %s IS NOT DISTINCT FROM (%s)) AS matches, count(*) AS sampled
		   FROM %s WHERE random() < %f`,
		col, p.Backfill.SourceExpr, p.Table, sampleRate,
	)
	var matches, sampled int64
	if err := r.target.QueryRow(ctx, paritySQL).Scan(&matches, &sampled); err != nil {
		return Result{}, fmt.Errorf("verify parity: %w", err)
	}
	parity := 1.0
	if sampled > 0 {
		parity = float64(matches) / float64(sampled)
	}
	telemetry.SetParity(rec.ID.String(), p.ID, parity)

	threshold := p.Verify.ParityThreshold
	if p.SLO.MinParity > threshold {
		threshold = p.SLO.MinParity
	}
	if parity < threshold {
		return Result{}, fmt.Errorf("verify failed: parity %.5f < threshold %.5f (%d/%d sampled)", parity, threshold, matches, sampled)
	}
	r.log.Info("verify parity ok", "migration", rec.ID, "parity", parity, "sampled", sampled)
	return Result{Checkpoint: map[string]any{"parity": parity, "sampled": sampled}}, nil
}

// canary progressively shifts traffic through the plan's steps, checking the SLO
// at each step. There is no real traffic system yet, so observations come from
// observeCanary (a healthy baseline, optionally forced to breach via chaos). On a
// breach, it either diverts to RollingBack (on_failure=rollback) or errors out
// (on_failure=pause).
func (r *Runner) canary(ctx context.Context, rec *store.Record) (Result, error) {
	p := rec.Plan

	// Hard gate: check replication lag before starting canary
	replStatus, err := r.ddlExecutor.CheckReplicationLag(ctx)
	if err != nil {
		r.log.Warn("failed to check replication lag, proceeding with caution", "err", err)
	} else if replStatus.MaxLagMs > float64(p.SLO.MaxP99LatencyMs) {
		return Result{}, fmt.Errorf("replication lag too high before canary: %.0fms (max: %dms)",
			replStatus.MaxLagMs, p.SLO.MaxP99LatencyMs)
	}

	for _, step := range p.Canary.Steps {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}

		// Pre-flight: check replication lag before each canary step
		replStatus, err := r.ddlExecutor.CheckReplicationLag(ctx)
		if err != nil {
			r.log.Warn("failed to check replication lag, proceeding with caution",
				"migration", rec.ID, "step", step, "err", err)
		} else if replStatus.MaxLagMs > float64(p.SLO.MaxP99LatencyMs)*2 {
			// Replication lag is 2x the p99 threshold — pause and wait
			r.log.Warn("replication lag too high, waiting",
				"migration", rec.ID, "step", step,
				"lag_ms", replStatus.MaxLagMs,
				"threshold_ms", p.SLO.MaxP99LatencyMs*2)
			time.Sleep(5 * time.Second)
			// Re-check after waiting
			replStatus, err = r.ddlExecutor.CheckReplicationLag(ctx)
			if err == nil && replStatus.MaxLagMs > float64(p.SLO.MaxP99LatencyMs)*3 {
				return Result{}, fmt.Errorf("replication lag still too high after wait: %.0fms",
					replStatus.MaxLagMs)
			}
		}

		obs := r.observeCanary(rec, step)
		telemetry.SetCanaryStep(rec.ID.String(), p.ID, step)

		// Log canary observation
		observation := &store.CanaryObservationEntry{
			MigrationID: rec.ID,
			Step:        step,
			TrafficPct:  step,
			P99Ms:       obs.p99Ms,
			ErrPct:      obs.errPct,
			SLOBreached: false,
			ObservedAt:  time.Now(),
		}
		if logErr := r.store.LogCanaryObservation(ctx, observation); logErr != nil {
			r.log.Warn("failed to log canary observation", "err", logErr)
		}

		if breached, why := sloBreached(obs, p.SLO); breached {
			r.log.Warn("canary SLO breach", "migration", rec.ID, "step", step, "why", why)
			telemetry.IncRollback(p.ID)

			// Log failed observation
			observation.SLOBreached = true
			if logErr := r.store.LogCanaryObservation(ctx, observation); logErr != nil {
				r.log.Warn("failed to log canary observation", "err", logErr)
			}

			ckpt := map[string]any{"canary_pct": step, "slo_breach": why}
			if p.OnFailure == plan.OnFailureRollback {
				return Result{Checkpoint: ckpt, Next: StateRollingBack}, nil
			}
			return Result{}, fmt.Errorf("canary SLO breach at %d%%: %s (on_failure=%s)", step, why, p.OnFailure)
		}

		if err := r.store.SaveCheckpoint(ctx, rec.ID, string(StateCanary),
			map[string]any{"canary_pct": step}, fmt.Sprintf("canary healthy at %d%%", step)); err != nil {
			return Result{}, fmt.Errorf("canary checkpoint: %w", err)
		}
		r.log.Info("canary step healthy", "migration", rec.ID, "step", step,
			"p99_ms", obs.p99Ms, "err_pct", obs.errPct)

		// Bake between steps. Real bake_seconds is honored loosely here (capped by
		// stepDelay) so demos stay fast.
		if r.stepDelay > 0 {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-time.After(r.stepDelay):
			}
		}
	}
	telemetry.SetCanaryStep(rec.ID.String(), p.ID, 100)
	return Result{Checkpoint: map[string]any{"canary_pct": 100}}, nil
}

// cutover is the point of no return: the next state (Contracting) runs the
// plan's DESTRUCTIVE DDL (dropping the legacy column), so before committing the
// engine re-proves convergence over the ENTIRE table — not a sample. If any row
// is NULL or drifted (e.g. a write landed during canary that bypassed the
// derived column), the cutover is aborted: on_failure=rollback diverts to
// RollingBack, otherwise it errors out and pauses. This is what makes contract
// safe to run.
func (r *Runner) cutover(ctx context.Context, rec *store.Record) (Result, error) {
	p := rec.Plan
	if p.Backfill.Column == "" {
		// Nothing derived to gate; pass through the happy path.
		return Result{Checkpoint: map[string]any{"step": string(StateCutover)}}, nil
	}

	rep, err := r.fullTableParity(ctx, &p)
	if err != nil {
		return Result{}, fmt.Errorf("cutover parity: %w", err)
	}
	telemetry.SetCutoverParity(rec.ID.String(), p.ID, rep.Parity)

	threshold := p.Verify.ParityThreshold
	if p.SLO.MinParity > threshold {
		threshold = p.SLO.MinParity
	}

	if rep.Nulls > 0 || rep.Parity < threshold {
		why := fmt.Sprintf("cutover gate failed: parity %.5f < %.5f (%d nulls, %d/%d drifted)",
			rep.Parity, threshold, rep.Nulls, rep.Drifted, rep.Total)
		r.log.Warn("cutover aborted", "migration", rec.ID, "why", why)
		ckpt := map[string]any{"cutover_parity": rep.Parity, "cutover_abort": why}
		if p.OnFailure == plan.OnFailureRollback {
			telemetry.IncRollback(p.ID)
			return Result{Checkpoint: ckpt, Next: StateRollingBack}, nil
		}
		return Result{}, fmt.Errorf("%s (on_failure=%s)", why, p.OnFailure)
	}

	r.log.Info("cutover committed", "migration", rec.ID, "parity", rep.Parity, "rows", rep.Total)
	return Result{Checkpoint: map[string]any{"cutover_parity": rep.Parity, "rows": rep.Total}}, nil
}

// rollback runs the plan's rollback DDL to undo the expand (e.g. drop the new
// column/index), returning the target to its pre-migration shape. The next map
// routes RollingBack -> RolledBack (terminal).
// Includes safety protocol: drain connections, retry with backoff, timeout.
func (r *Runner) rollback(ctx context.Context, rec *store.Record) (Result, error) {
	// Step 1: Drain connections (cancel long-running queries)
	r.log.Info("rolling back: draining connections", "migration", rec.ID)
	r.drainConnections(ctx)

	// Step 2: Execute rollback DDL with retries
	maxRetries := 3
	retryDelay := time.Second

	for i, stmt := range rec.Plan.Rollback {
		var lastErr error

		for attempt := 0; attempt < maxRetries; attempt++ {
			// Execute with safety wrapper
			start := time.Now()
			duration, err := r.ddlExecutor.ExecDDL(ctx, stmt)
			lastErr = err

			// Log DDL execution
			logEntry := &store.DDLExecutionLogEntry{
				MigrationID: rec.ID,
				Statement:   stmt,
				StartedAt:   start,
				CompletedAt: ptrTime(time.Now()),
				DurationMs:  ptrInt(int(duration.Milliseconds())),
				Success:     err == nil,
			}
			if err != nil {
				logEntry.ErrorMessage = err.Error()
			}
			if logErr := r.store.LogDDLExecution(ctx, logEntry); logErr != nil {
				r.log.Warn("failed to log DDL execution", "err", logErr)
			}

			if err == nil {
				r.log.Info("rollback applied", "migration", rec.ID, "stmt", stmt, "duration", duration)
				lastErr = nil
				break
			}

			// Retry with exponential backoff
			if attempt < maxRetries-1 {
				delay := retryDelay * time.Duration(1<<uint(attempt))
				r.log.Warn("rollback stmt failed, retrying",
					"migration", rec.ID,
					"stmt_index", i,
					"attempt", attempt+1,
					"err", err,
					"retry_in", delay)
				time.Sleep(delay)
			}
		}

		if lastErr != nil {
			return Result{}, fmt.Errorf("rollback stmt %d (%q) failed after %d attempts: %w",
				i, stmt, maxRetries, lastErr)
		}
	}

	// Step 3: Verify rollback completed
	r.verifyRollback(ctx, rec)

	return Result{Checkpoint: map[string]any{"rolled_back": len(rec.Plan.Rollback)}}, nil
}

// drainConnections cancels long-running queries to speed up rollback
func (r *Runner) drainConnections(ctx context.Context) {
	// Cancel queries running longer than 30 seconds (except our own)
	_, err := r.target.Exec(ctx, `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE state = 'active'
		  AND query_start < now() - interval '30 seconds'
		  AND query NOT LIKE '%pg_stat_activity%'
		  AND query NOT LIKE '%migration_safety_engine%'`)
	if err != nil {
		r.log.Warn("failed to drain connections", "err", err)
	}
}

// verifyRollback checks that rollback DDL was applied correctly
func (r *Runner) verifyRollback(ctx context.Context, rec *store.Record) {
	// Check that dropped columns no longer exist
	for _, col := range rec.Plan.DropColumns {
		var exists bool
		err := r.target.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = $1 AND column_name = $2
			)`, rec.Plan.Table, col).Scan(&exists)
		if err != nil {
			r.log.Warn("failed to verify rollback", "err", err)
			continue
		}
		if exists {
			r.log.Error("column still exists after rollback",
				"migration", rec.ID, "column", col)
		}
	}
}

// contract runs the plan's contract DDL on the target (e.g. dropping the legacy
// column). Statements should be idempotent (IF EXISTS) for safe resume.
func (r *Runner) contract(ctx context.Context, rec *store.Record) (Result, error) {
	for i, stmt := range rec.Plan.Contract {
		// Execute with safety wrapper
		start := time.Now()
		duration, err := r.ddlExecutor.ExecDDL(ctx, stmt)

		// Log DDL execution
		logEntry := &store.DDLExecutionLogEntry{
			MigrationID: rec.ID,
			Statement:   stmt,
			StartedAt:   start,
			CompletedAt: ptrTime(time.Now()),
			DurationMs:  ptrInt(int(duration.Milliseconds())),
			Success:     err == nil,
		}
		if err != nil {
			logEntry.ErrorMessage = err.Error()
		}
		if logErr := r.store.LogDDLExecution(ctx, logEntry); logErr != nil {
			r.log.Warn("failed to log DDL execution", "err", logErr)
		}

		if err != nil {
			return Result{}, fmt.Errorf("contract stmt %d (%q): %w", i, stmt, err)
		}
		r.log.Info("contract applied", "migration", rec.ID, "stmt", stmt, "duration", duration)
	}
	return Result{Checkpoint: map[string]any{"contracted": len(rec.Plan.Contract)}}, nil
}

// canaryObs is a single SLO observation for a canary step.
type canaryObs struct {
	p99Ms  float64
	errPct float64
}

// observeCanary returns the SLO signals for a canary step. With no real traffic
// system, it reports a healthy baseline well within the SLO. The chaos knob
// FailCanaryAtStep forces a breach once the canary reaches that percentage, which
// is how the auto-rollback path is demonstrated and tested.
func (r *Runner) observeCanary(rec *store.Record, step int) canaryObs {
	slo := rec.Plan.SLO
	obs := canaryObs{
		p99Ms:  float64(slo.MaxP99LatencyMs) * 0.6,
		errPct: slo.MaxErrorRatePct * 0.3,
	}
	if fs := rec.Plan.Chaos.FailCanaryAtStep; fs > 0 && step >= fs {
		obs.p99Ms = float64(slo.MaxP99LatencyMs)*2 + 1
		obs.errPct = slo.MaxErrorRatePct*2 + 1
	}
	return obs
}

// sloBreached reports whether an observation violates any configured SLO gate.
func sloBreached(obs canaryObs, slo plan.SLO) (bool, string) {
	if slo.MaxP99LatencyMs > 0 && obs.p99Ms > float64(slo.MaxP99LatencyMs) {
		return true, fmt.Sprintf("p99 %.0fms > %dms", obs.p99Ms, slo.MaxP99LatencyMs)
	}
	if slo.MaxErrorRatePct > 0 && obs.errPct > slo.MaxErrorRatePct {
		return true, fmt.Sprintf("error-rate %.2f%% > %.2f%%", obs.errPct, slo.MaxErrorRatePct)
	}
	return false, ""
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func ptrInt(i int) *int {
	return &i
}

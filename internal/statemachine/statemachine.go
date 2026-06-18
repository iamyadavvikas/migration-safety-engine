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

	"github.com/vikasyadav/migration-safety-engine/internal/plan"
	"github.com/vikasyadav/migration-safety-engine/internal/store"
	"github.com/vikasyadav/migration-safety-engine/internal/telemetry"
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

	// stepDelay paces transitions so progress is observable in demos/tests.
	stepDelay time.Duration
}

// NewRunner builds a Runner whose handlers run real work against the target pool.
func NewRunner(st *store.Store, target *pgxpool.Pool, log *slog.Logger) *Runner {
	r := &Runner{
		store:     st,
		target:    target,
		log:       log,
		stepDelay: 200 * time.Millisecond,
	}
	r.handlers = r.defaultHandlers()
	return r
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

// expand runs the plan's expand DDL on the target. Each statement runs outside a
// transaction so CREATE INDEX CONCURRENTLY works; statements should be idempotent
// (IF NOT EXISTS) so a resume after a partial expand is safe.
func (r *Runner) expand(ctx context.Context, rec *store.Record) (Result, error) {
	for i, stmt := range rec.Plan.Expand {
		if _, err := r.target.Exec(ctx, stmt); err != nil {
			return Result{}, fmt.Errorf("expand stmt %d (%q): %w", i, stmt, err)
		}
		r.log.Info("expand applied", "migration", rec.ID, "stmt", stmt)
	}
	return Result{Checkpoint: map[string]any{"expanded": len(rec.Plan.Expand)}}, nil
}

// backfill populates the new column in batches, throttled between batches, and
// checkpoints progress after every batch. Resume is correct because each batch
// only touches rows where the column IS NULL, so already-filled rows are skipped.
func (r *Runner) backfill(ctx context.Context, rec *store.Record) (Result, error) {
	p := rec.Plan
	col := p.Backfill.Column
	if col == "" {
		return Result{Checkpoint: map[string]any{"skipped": "no backfill column"}}, nil
	}
	batch := p.Backfill.BatchSize
	if batch <= 0 {
		batch = 1000
	}
	throttle := time.Duration(p.Backfill.ThrottleMs) * time.Millisecond

	// done may be non-zero if we are resuming after a crash.
	var done int64
	if v, ok := rec.Checkpoint["rows_done"].(float64); ok {
		done = int64(v)
	}

	// remaining rows still needing a value; total = done + remaining.
	var remaining int64
	countSQL := fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s IS NULL`, p.Table, col)
	if err := r.target.QueryRow(ctx, countSQL).Scan(&remaining); err != nil {
		return Result{}, fmt.Errorf("count remaining: %w", err)
	}
	total := done + remaining
	telemetry.SetBackfill(rec.ID.String(), p.ID, total, done)

	updSQL := fmt.Sprintf(
		`UPDATE %s SET %s = (%s) WHERE id IN (SELECT id FROM %s WHERE %s IS NULL ORDER BY id LIMIT $1)`,
		p.Table, col, p.Backfill.SourceExpr, p.Table, col,
	)

	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}

		tag, err := r.target.Exec(ctx, updSQL, batch)
		if err != nil {
			return Result{}, fmt.Errorf("backfill batch: %w", err)
		}
		n := tag.RowsAffected()
		if n == 0 {
			break
		}
		done += n
		telemetry.SetBackfill(rec.ID.String(), p.ID, total, done)
		if err := r.store.SaveCheckpoint(ctx, rec.ID, string(StateBackfilling),
			map[string]any{"rows_done": done}, fmt.Sprintf("backfilled %d/%d rows", done, total)); err != nil {
			return Result{}, fmt.Errorf("checkpoint: %w", err)
		}
		r.log.Info("backfill progress", "migration", rec.ID, "done", done, "total", total)

		if throttle > 0 {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-time.After(throttle):
			}
		}
	}
	return Result{Checkpoint: map[string]any{"rows_done": done}}, nil
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
	for _, step := range p.Canary.Steps {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}

		obs := r.observeCanary(rec, step)
		telemetry.SetCanaryStep(rec.ID.String(), p.ID, step)

		if breached, why := sloBreached(obs, p.SLO); breached {
			r.log.Warn("canary SLO breach", "migration", rec.ID, "step", step, "why", why)
			telemetry.IncRollback(p.ID)
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
func (r *Runner) rollback(ctx context.Context, rec *store.Record) (Result, error) {
	for i, stmt := range rec.Plan.Rollback {
		if _, err := r.target.Exec(ctx, stmt); err != nil {
			return Result{}, fmt.Errorf("rollback stmt %d (%q): %w", i, stmt, err)
		}
		r.log.Info("rollback applied", "migration", rec.ID, "stmt", stmt)
	}
	return Result{Checkpoint: map[string]any{"rolled_back": len(rec.Plan.Rollback)}}, nil
}

// contract runs the plan's contract DDL on the target (e.g. dropping the legacy
// column). Statements should be idempotent (IF EXISTS) for safe resume.
func (r *Runner) contract(ctx context.Context, rec *store.Record) (Result, error) {
	for i, stmt := range rec.Plan.Contract {
		if _, err := r.target.Exec(ctx, stmt); err != nil {
			return Result{}, fmt.Errorf("contract stmt %d (%q): %w", i, stmt, err)
		}
		r.log.Info("contract applied", "migration", rec.ID, "stmt", stmt)
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

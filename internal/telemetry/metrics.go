// Package telemetry exposes Prometheus metrics for the engine.
//
// Design note: the interesting migration signals are DOMAIN metrics (state,
// parity, rows-remaining), not CPU/memory. Phase 1 ships the state + transition
// signals; Phase 2 adds parity/convergence/canary p99.
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// StateInfo is 1 for the current state of each migration, 0 otherwise.
	// Query example: migrate_state_info{state="Done"} to see completed migrations.
	StateInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "migrate_state_info",
		Help: "Current state of each migration (1 = active state).",
	}, []string{"migration_id", "plan_id", "state"})

	// Transitions counts state transitions, labeled by destination state.
	Transitions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "migrate_state_transitions_total",
		Help: "Total state transitions by destination state.",
	}, []string{"plan_id", "to_state"})

	// BackfillRowsTotal is the total number of rows a migration must backfill.
	BackfillRowsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "migrate_backfill_rows_total",
		Help: "Total rows to backfill for a migration.",
	}, []string{"migration_id", "plan_id"})

	// BackfillRowsDone is how many rows have been backfilled so far. The gap
	// between total and done is the live convergence signal during a backfill.
	BackfillRowsDone = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "migrate_backfill_rows_done",
		Help: "Rows backfilled so far for a migration.",
	}, []string{"migration_id", "plan_id"})

	// VerifyParity is the measured shadow-read parity (matches/sampled), 0..1.
	VerifyParity = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "migrate_verify_parity",
		Help: "Shadow-read parity ratio measured at verify (1.0 = perfect).",
	}, []string{"migration_id", "plan_id"})

	// CanaryStep is the current canary traffic percentage for a migration.
	CanaryStep = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "migrate_canary_step_pct",
		Help: "Current canary traffic percentage (0 when not in canary).",
	}, []string{"migration_id", "plan_id"})

	// Rollbacks counts SLO-triggered auto-rollbacks.
	Rollbacks = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "migrate_rollbacks_total",
		Help: "Total migrations auto-rolled-back due to an SLO breach.",
	}, []string{"plan_id"})

	// CutoverParity is the full-table parity measured at the cutover gate (the
	// point of no return). Unlike VerifyParity it is computed over the whole
	// table, not a sample, because the next step is destructive (contract).
	CutoverParity = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "migrate_cutover_parity",
		Help: "Full-table parity measured at the cutover gate (1.0 = perfect).",
	}, []string{"migration_id", "plan_id"})
)

// SetBackfill publishes backfill progress for a migration.
func SetBackfill(migrationID, planID string, total, done int64) {
	BackfillRowsTotal.WithLabelValues(migrationID, planID).Set(float64(total))
	BackfillRowsDone.WithLabelValues(migrationID, planID).Set(float64(done))
}

// SetParity publishes the measured shadow-read parity for a migration.
func SetParity(migrationID, planID string, parity float64) {
	VerifyParity.WithLabelValues(migrationID, planID).Set(parity)
}

// SetCutoverParity publishes the full-table parity measured at the cutover gate.
func SetCutoverParity(migrationID, planID string, parity float64) {
	CutoverParity.WithLabelValues(migrationID, planID).Set(parity)
}

// SetCanaryStep publishes the current canary traffic percentage.
func SetCanaryStep(migrationID, planID string, pct int) {
	CanaryStep.WithLabelValues(migrationID, planID).Set(float64(pct))
}

// IncRollback records an SLO-triggered auto-rollback.
func IncRollback(planID string) {
	Rollbacks.WithLabelValues(planID).Inc()
}

// SetState marks the given state active for a migration and clears the previous one.
func SetState(migrationID, planID, prev, cur string) {
	if prev != "" {
		StateInfo.WithLabelValues(migrationID, planID, prev).Set(0)
	}
	StateInfo.WithLabelValues(migrationID, planID, cur).Set(1)
	Transitions.WithLabelValues(planID, cur).Inc()
}

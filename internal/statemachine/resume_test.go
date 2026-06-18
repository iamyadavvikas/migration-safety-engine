// Package statemachine_test contains an integration test proving that a
// migration resumes from its last persisted state after a simulated crash.
//
// It runs against a real Postgres. Set MSE_TEST_DSN to enable it, e.g.:
//
//	make up && make migrate
//	MSE_TEST_DSN="postgres://mse:mse@localhost:5432/mse?sslmode=disable" go test ./internal/statemachine/...
package statemachine_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/iamyadavvikas/migration-safety-engine/internal/plan"
	"github.com/iamyadavvikas/migration-safety-engine/internal/statemachine"
	"github.com/iamyadavvikas/migration-safety-engine/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("MSE_TEST_DSN")
	if dsn == "" {
		t.Skip("set MSE_TEST_DSN to run the resume integration test (needs `make up && make migrate`)")
	}
	st, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

func samplePlan(id string) *plan.MigrationPlan {
	p := &plan.MigrationPlan{
		ID:      id,
		Version: int(time.Now().UnixNano() % 1_000_000), // unique per run
		Table:   "catalog_product",
		Expand:  []string{"ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS resume_probe text"},
	}
	_ = p.Validate()
	return p
}

// TestResumeAfterCrash drives a migration partway, cancels (simulating a crash),
// asserts it is NOT terminal, then runs a fresh Runner that must resume and finish.
func TestResumeAfterCrash(t *testing.T) {
	st := testStore(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	id, err := st.CreateMigration(context.Background(), samplePlan("resume-test"), string(statemachine.StatePending))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// First runner: cancel quickly to stop partway through the multi-step flow.
	r1 := statemachine.NewRunner(st, st.Pool(), log)
	r1.SetStepDelay(40 * time.Millisecond)
	ctx1, cancel1 := context.WithTimeout(context.Background(), 90*time.Millisecond)
	defer cancel1()
	_ = r1.Run(ctx1, id) // expected to stop (context deadline) before Done

	rec, err := st.Load(context.Background(), id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.Terminal {
		t.Fatalf("migration finished too fast to test resume; increase steps/delay")
	}
	midState := rec.State
	t.Logf("crashed at state=%s", midState)

	// Second runner: must resume from the persisted state and reach Done.
	r2 := statemachine.NewRunner(st, st.Pool(), log)
	r2.SetStepDelay(0)
	if err := r2.Run(context.Background(), id); err != nil {
		t.Fatalf("resume run: %v", err)
	}

	final, err := st.Load(context.Background(), id)
	if err != nil {
		t.Fatalf("load final: %v", err)
	}
	if statemachine.State(final.State) != statemachine.StateDone {
		t.Fatalf("expected Done, got %s", final.State)
	}
	if !final.Terminal {
		t.Fatalf("expected terminal=true")
	}
}

// chaosPlan migrates catalog_product but forces an SLO breach at the 25% canary
// step, so the engine must auto-roll-back and drop the new column.
func chaosPlan(id string) *plan.MigrationPlan {
	p := &plan.MigrationPlan{
		ID:      id,
		Version: int(time.Now().UnixNano() % 1_000_000),
		Table:   "catalog_product",
		Expand:  []string{"ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text"},
		Backfill: plan.Backfill{
			Column:     "shipping_class",
			BatchSize:  10000,
			SourceExpr: "'standard'",
		},
		Rollback:  []string{"ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class"},
		OnFailure: plan.OnFailureRollback,
		SLO:       plan.SLO{MaxP99LatencyMs: 50, MaxErrorRatePct: 0.1},
		Chaos:     plan.Chaos{FailCanaryAtStep: 25},
	}
	_ = p.Validate()
	return p
}

// TestCanaryAutoRollback proves the headline safety property: when the canary
// breaches the SLO, the migration ends in RolledBack and the schema change is
// undone (shipping_class column removed).
func TestCanaryAutoRollback(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Ensure a clean slate so the expand actually adds the column.
	if _, err := st.Pool().Exec(ctx, "ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class"); err != nil {
		t.Fatalf("pre-clean: %v", err)
	}

	id, err := st.CreateMigration(ctx, chaosPlan("rollback-test"), string(statemachine.StatePending))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	r := statemachine.NewRunner(st, st.Pool(), log)
	r.SetStepDelay(0)
	if err := r.Run(ctx, id); err != nil {
		t.Fatalf("run: %v", err)
	}

	final, err := st.Load(ctx, id)
	if err != nil {
		t.Fatalf("load final: %v", err)
	}
	if statemachine.State(final.State) != statemachine.StateRolledBack {
		t.Fatalf("expected RolledBack, got %s", final.State)
	}
	if !final.Terminal {
		t.Fatalf("expected terminal=true")
	}

	// The rollback must have dropped the new column.
	var n int
	if err := st.Pool().QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns
		  WHERE table_name='catalog_product' AND column_name='shipping_class'`).Scan(&n); err != nil {
		t.Fatalf("column check: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected shipping_class to be dropped by rollback, still present")
	}
}

// TestRollbackResumesAfterCrash proves the rollback path is itself durable: if
// the engine crashes after diverting to RollingBack but before the rollback DDL
// finishes, a fresh runner re-enters the rollback handler on restart and drives
// the migration to RolledBack (rather than leaving it stuck mid-rollback). We
// simulate the crash by seeding a migration directly into the RollingBack state.
func TestRollbackResumesAfterCrash(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Pre-state: the new column exists (expand had run) and now must be undone.
	if _, err := st.Pool().Exec(ctx, "ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text"); err != nil {
		t.Fatalf("pre-add column: %v", err)
	}

	p := chaosPlan("rollback-resume-test")
	// Seed the migration as if a crash left it mid-rollback.
	id, err := st.CreateMigration(ctx, p, string(statemachine.StateRollingBack))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A fresh runner must pick the migration up in RollingBack and finish it.
	r := statemachine.NewRunner(st, st.Pool(), log)
	r.SetStepDelay(0)
	if err := r.Run(ctx, id); err != nil {
		t.Fatalf("resume rollback: %v", err)
	}

	final, err := st.Load(ctx, id)
	if err != nil {
		t.Fatalf("load final: %v", err)
	}
	if statemachine.State(final.State) != statemachine.StateRolledBack {
		t.Fatalf("expected RolledBack, got %s", final.State)
	}
	if !final.Terminal {
		t.Fatalf("expected terminal=true")
	}

	var n int
	if err := st.Pool().QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns
		  WHERE table_name='catalog_product' AND column_name='shipping_class'`).Scan(&n); err != nil {
		t.Fatalf("column check: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected shipping_class dropped after resumed rollback, still present")
	}
}

// TestCutoverAbortsOnDrift proves the cutover gate is a real point of no return:
// if the full-table parity check finds drift (here, rows still NULL) right before
// the destructive contract step, the migration must abort and roll back instead
// of dropping the legacy column. We seed a migration into Cutover with the new
// column present but unfilled.
func TestCutoverAbortsOnDrift(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// New column exists but is entirely NULL -> the cutover gate must fail.
	if _, err := st.Pool().Exec(ctx, "ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class"); err != nil {
		t.Fatalf("pre-clean: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, "ALTER TABLE catalog_product ADD COLUMN shipping_class text"); err != nil {
		t.Fatalf("pre-add column: %v", err)
	}
	// The legacy column is what contract would drop; ensure it exists so we can
	// prove the aborted cutover left it intact.
	if _, err := st.Pool().Exec(ctx, "ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS legacy_shipping text"); err != nil {
		t.Fatalf("pre-add legacy column: %v", err)
	}

	p := &plan.MigrationPlan{
		ID:      "cutover-abort-test",
		Version: int(time.Now().UnixNano() % 1_000_000),
		Table:   "catalog_product",
		Expand:  []string{"ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text"},
		Backfill: plan.Backfill{
			Column:     "shipping_class",
			SourceExpr: "'standard'",
		},
		Verify:    plan.Verify{ParityThreshold: 0.999},
		Contract:  []string{"ALTER TABLE catalog_product DROP COLUMN IF EXISTS legacy_shipping"},
		Rollback:  []string{"ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class"},
		OnFailure: plan.OnFailureRollback,
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	id, err := st.CreateMigration(ctx, p, string(statemachine.StateCutover))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	r := statemachine.NewRunner(st, st.Pool(), log)
	r.SetStepDelay(0)
	if err := r.Run(ctx, id); err != nil {
		t.Fatalf("run: %v", err)
	}

	final, err := st.Load(ctx, id)
	if err != nil {
		t.Fatalf("load final: %v", err)
	}
	if statemachine.State(final.State) != statemachine.StateRolledBack {
		t.Fatalf("expected RolledBack (cutover aborted), got %s", final.State)
	}

	// The legacy column must still be present: contract must NOT have run.
	var legacy int
	if err := st.Pool().QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns
		  WHERE table_name='catalog_product' AND column_name='legacy_shipping'`).Scan(&legacy); err != nil {
		t.Fatalf("legacy column check: %v", err)
	}
	if legacy == 0 {
		t.Fatalf("cutover gate failed to protect contract: legacy_shipping was dropped")
	}
}


package plan

import (
	"path/filepath"
	"testing"
)

func TestParseValidPlan(t *testing.T) {
	p, err := Parse(filepath.Join("..", "..", "examples", "add-shipping-index.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.ID == "" || p.Version == 0 {
		t.Fatalf("expected id and version set, got %+v", p)
	}
	if p.Strategy != StrategyExpandContract {
		t.Fatalf("expected expand-contract, got %q", p.Strategy)
	}
}

func TestValidateDefaults(t *testing.T) {
	p := &MigrationPlan{
		ID:      "x",
		Version: 1,
		Table:   "t",
		Expand:  []string{"ALTER TABLE t ADD COLUMN c text"},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if p.OnFailure != OnFailureRollback {
		t.Fatalf("expected default on_failure=rollback, got %q", p.OnFailure)
	}
	if len(p.Canary.Steps) != 4 {
		t.Fatalf("expected default canary steps, got %v", p.Canary.Steps)
	}
}

func TestValidateRejectsBadStep(t *testing.T) {
	p := &MigrationPlan{
		ID:      "x",
		Version: 1,
		Table:   "t",
		Expand:  []string{"ALTER TABLE t ADD COLUMN c text"},
		Canary:  Canary{Steps: []int{0}},
	}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for canary step 0")
	}
}

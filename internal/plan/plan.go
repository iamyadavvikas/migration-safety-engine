// Package plan defines the declarative MigrationPlan and its parsing/validation.
//
// A MigrationPlan is the single input to the engine: it expresses *what* schema
// change is wanted, not *how* to execute it. The state machine interprets it.
package plan

import (
	"fmt"
	"os"
	"regexp"

	"github.com/iamyadavvikas/migration-safety-engine/internal/ddl"
	"gopkg.in/yaml.v3"
)

// identRe matches a safe, unqualified SQL identifier. The engine interpolates the
// target table and backfill column into SQL, so they are validated here. The
// expand/contract DDL and backfill source_expr are intentionally RAW SQL authored
// by a trusted operator (this is a migration tool — the operator already holds
// full DDL rights on the target database).
var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Strategy is the migration approach. Phase 1 only supports expand-contract.
type Strategy string

const (
	StrategyExpandContract Strategy = "expand-contract"
)

// FailureAction decides what happens when an SLO gate or step fails.
type FailureAction string

const (
	OnFailureRollback FailureAction = "rollback"
	OnFailurePause    FailureAction = "pause"
)

// ColumnSpec describes a column to add via the declarative API.
type ColumnSpec struct {
	Name       string `yaml:"name" json:"name"`
	Type       string `yaml:"type" json:"type"`
	Expression string `yaml:"expression,omitempty" json:"expression,omitempty"` // backfill source_expr
	Nullable   bool   `yaml:"nullable,omitempty" json:"nullable,omitempty"`
	Indexed    bool   `yaml:"indexed,omitempty" json:"indexed,omitempty"`
}

// MigrationPlan is the declarative description of a schema migration.
type MigrationPlan struct {
	ID        string        `yaml:"id" json:"id"`
	Version   int           `yaml:"version" json:"version"`
	Table     string        `yaml:"table" json:"table"`
	Strategy  Strategy      `yaml:"strategy" json:"strategy"`
	Expand    []string      `yaml:"expand" json:"expand"`
	Backfill  Backfill      `yaml:"backfill" json:"backfill"`
	Verify    Verify        `yaml:"verify" json:"verify"`
	Canary    Canary        `yaml:"canary" json:"canary"`
	SLO       SLO           `yaml:"slo" json:"slo"`
	Contract  []string      `yaml:"contract" json:"contract"`
	Rollback  []string      `yaml:"rollback" json:"rollback"` // DDL to undo expand if canary fails before cutover
	OnFailure FailureAction `yaml:"on_failure" json:"on_failure"`
	Chaos     Chaos         `yaml:"chaos" json:"chaos"`

	// Declarative column specs — when present, DDL is auto-generated.
	AddColumns  []ColumnSpec `yaml:"add_columns,omitempty" json:"add_columns,omitempty"`
	DropColumns []string     `yaml:"drop_columns,omitempty" json:"drop_columns,omitempty"`
}

// Chaos holds fault-injection knobs used to exercise the SLO gate + auto-rollback
// path in demos and tests. It is NOT a production feature: when FailCanaryAtStep
// is > 0, the canary observation is forced to breach the SLO once the canary
// reaches that traffic percentage, triggering the plan's on_failure action.
type Chaos struct {
	FailCanaryAtStep int `yaml:"fail_canary_at_step" json:"fail_canary_at_step"`
}

// Backfill controls how existing rows are populated for the new schema.
type Backfill struct {
	Column     string `yaml:"column" json:"column"`         // target column to populate (e.g. shipping_class)
	BatchSize  int    `yaml:"batch_size" json:"batch_size"`
	ThrottleMs int    `yaml:"throttle_ms" json:"throttle_ms"`
	SourceExpr string `yaml:"source_expr" json:"source_expr"` // raw SQL expression computed per row
}

// Verify controls shadow-read parity verification before cutover.
type Verify struct {
	Mode            string  `yaml:"mode" json:"mode"`
	ParityThreshold float64 `yaml:"parity_threshold" json:"parity_threshold"`
	SampleRate      float64 `yaml:"sample_rate" json:"sample_rate"`
}

// Canary describes the progressive traffic-shift steps.
type Canary struct {
	Steps       []int `yaml:"steps" json:"steps"`
	BakeSeconds int   `yaml:"bake_seconds" json:"bake_seconds"`
}

// SLO defines the gates that, if breached, trigger the OnFailure action.
type SLO struct {
	MaxP99LatencyMs int     `yaml:"max_p99_latency_ms" json:"max_p99_latency_ms"`
	MaxErrorRatePct float64 `yaml:"max_error_rate_pct" json:"max_error_rate_pct"`
	MinParity       float64 `yaml:"min_parity" json:"min_parity"`
}

// Parse reads and parses a MigrationPlan from a YAML file.
func Parse(path string) (*MigrationPlan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plan: %w", err)
	}
	var p MigrationPlan
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("parse plan yaml: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate enforces the invariants the engine relies on.
func (p *MigrationPlan) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("plan.id is required")
	}
	if p.Version <= 0 {
		return fmt.Errorf("plan.version must be > 0")
	}
	if p.Table == "" {
		return fmt.Errorf("plan.table is required")
	}
	if !identRe.MatchString(p.Table) {
		return fmt.Errorf("plan.table %q is not a valid SQL identifier", p.Table)
	}

	// Declarative mode: auto-generate DDL from add_columns/drop_columns.
	if len(p.AddColumns) > 0 || len(p.DropColumns) > 0 {
		if err := p.generateDDL(); err != nil {
			return fmt.Errorf("generate ddl: %w", err)
		}
	}

	if p.Backfill.Column != "" && !identRe.MatchString(p.Backfill.Column) {
		return fmt.Errorf("backfill.column %q is not a valid SQL identifier", p.Backfill.Column)
	}
	if p.Backfill.Column != "" && p.Backfill.SourceExpr == "" {
		return fmt.Errorf("backfill.source_expr is required when backfill.column is set")
	}
	if p.Strategy == "" {
		p.Strategy = StrategyExpandContract
	}
	if p.Strategy != StrategyExpandContract {
		return fmt.Errorf("unsupported strategy %q (Phase 1 supports only %q)", p.Strategy, StrategyExpandContract)
	}
	if len(p.Expand) == 0 {
		return fmt.Errorf("plan.expand must contain at least one statement")
	}
	if len(p.Canary.Steps) == 0 {
		p.Canary.Steps = []int{1, 5, 25, 100}
	}
	for _, s := range p.Canary.Steps {
		if s <= 0 || s > 100 {
			return fmt.Errorf("canary step %d out of range (1..100)", s)
		}
	}
	if p.OnFailure == "" {
		p.OnFailure = OnFailureRollback
	}
	if p.OnFailure != OnFailureRollback && p.OnFailure != OnFailurePause {
		return fmt.Errorf("invalid on_failure %q", p.OnFailure)
	}
	if p.Verify.ParityThreshold == 0 {
		p.Verify.ParityThreshold = 0.999
	}
	return nil
}

// generateDDL uses the ddl package to auto-populate expand/contract/rollback/backfill
// from the declarative AddColumns and DropColumns fields.
func (p *MigrationPlan) generateDDL() error {
	// Convert plan.ColumnSpec to ddl.ColumnSpec
	cols := make([]ddl.ColumnSpec, len(p.AddColumns))
	for i, c := range p.AddColumns {
		cols[i] = ddl.ColumnSpec{
			Name:       c.Name,
			Type:       c.Type,
			Expression: c.Expression,
			Nullable:   c.Nullable,
			Indexed:    c.Indexed,
		}
	}

	out := ddl.Generate(p.Table, cols, p.DropColumns)

	// Only populate fields that are empty (don't override explicit raw SQL).
	if len(p.Expand) == 0 {
		p.Expand = out.Expand
	}
	if len(p.Contract) == 0 {
		p.Contract = out.Contract
	}
	if len(p.Rollback) == 0 {
		p.Rollback = out.Rollback
	}
	if p.Backfill.Column == "" && out.BackfillColumn != "" {
		p.Backfill.Column = out.BackfillColumn
	}
	if p.Backfill.SourceExpr == "" && out.BackfillExpr != "" {
		p.Backfill.SourceExpr = out.BackfillExpr
	}

	// Set defaults for backfill if columns were specified.
	if p.Backfill.BatchSize == 0 {
		p.Backfill.BatchSize = 5000
	}
	if p.Backfill.ThrottleMs == 0 {
		p.Backfill.ThrottleMs = 20
	}

	return nil
}

package ddl

import (
	"testing"
)

func TestGenerateAddColumn(t *testing.T) {
	out := Generate("catalog_product", []ColumnSpec{
		{Name: "shipping_class", Type: "text", Nullable: true},
	}, nil)

	if len(out.Expand) != 1 {
		t.Fatalf("expected 1 expand, got %d", len(out.Expand))
	}
	if out.Expand[0] != "ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text NULL" {
		t.Errorf("unexpected expand: %s", out.Expand[0])
	}
	if len(out.Rollback) != 1 {
		t.Fatalf("expected 1 rollback, got %d", len(out.Rollback))
	}
	if out.Rollback[0] != "ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class" {
		t.Errorf("unexpected rollback: %s", out.Rollback[0])
	}
}

func TestGenerateAddColumnWithIndex(t *testing.T) {
	out := Generate("catalog_product", []ColumnSpec{
		{Name: "shipping_class", Type: "text", Indexed: true},
	}, nil)

	if len(out.Expand) != 2 {
		t.Fatalf("expected 2 expand (column + index), got %d", len(out.Expand))
	}
	if out.Expand[0] != "ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text NOT NULL" {
		t.Errorf("unexpected expand[0]: %s", out.Expand[0])
	}
	expectedIdx := "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_catalog_product_shipping_class ON catalog_product (shipping_class)"
	if out.Expand[1] != expectedIdx {
		t.Errorf("unexpected expand[1]: %s", out.Expand[1])
	}
	if len(out.Rollback) != 2 {
		t.Fatalf("expected 2 rollback (index + column), got %d", len(out.Rollback))
	}
	expectedDropIdx := "DROP INDEX IF EXISTS idx_catalog_product_shipping_class"
	if out.Rollback[0] != expectedDropIdx {
		t.Errorf("unexpected rollback[0]: %s", out.Rollback[0])
	}
}

func TestGenerateDropColumn(t *testing.T) {
	out := Generate("catalog_product", nil, []string{"legacy_shipping"})

	if len(out.Contract) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(out.Contract))
	}
	if out.Contract[0] != "ALTER TABLE catalog_product DROP COLUMN IF EXISTS legacy_shipping" {
		t.Errorf("unexpected contract: %s", out.Contract[0])
	}
}

func TestGenerateBackfillExpr(t *testing.T) {
	out := Generate("catalog_product", []ColumnSpec{
		{Name: "shipping_class", Type: "text", Expression: "CASE WHEN weight < 1 THEN 'light' ELSE 'freight' END"},
	}, nil)

	if out.BackfillExpr != "CASE WHEN weight < 1 THEN 'light' ELSE 'freight' END" {
		t.Errorf("unexpected backfill expr: %s", out.BackfillExpr)
	}
	if out.BackfillColumn != "shipping_class" {
		t.Errorf("unexpected backfill column: %s", out.BackfillColumn)
	}
}

func TestGenerateFullPlan(t *testing.T) {
	out := Generate("catalog_product", []ColumnSpec{
		{Name: "shipping_class", Type: "text", Expression: "CASE WHEN weight < 1 THEN 'light' ELSE 'freight' END", Indexed: true},
	}, []string{"legacy_shipping"})

	// Expand: column + index
	if len(out.Expand) != 2 {
		t.Fatalf("expected 2 expand, got %d", len(out.Expand))
	}
	// Contract: drop legacy
	if len(out.Contract) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(out.Contract))
	}
	// Rollback: drop index + column
	if len(out.Rollback) != 2 {
		t.Fatalf("expected 2 rollback, got %d", len(out.Rollback))
	}
	// Backfill
	if out.BackfillColumn != "shipping_class" {
		t.Errorf("unexpected backfill column: %s", out.BackfillColumn)
	}
}

func TestSanitizeIdent(t *testing.T) {
	tests := []struct{ in, want string }{
		{"catalog_product", "catalog_product"},
		{"table; DROP", "tableDROP"},
		{"  spaces  ", "spaces"},
		{"ok-123_", "ok123_"},
	}
	for _, tt := range tests {
		got := sanitizeIdent(tt.in)
		if got != tt.want {
			t.Errorf("sanitizeIdent(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIntrospectQuery(t *testing.T) {
	q := IntrospectQuery("catalog_product")
	if q == "" {
		t.Error("expected non-empty query")
	}
	// Should contain the table name
	if !contains(q, "catalog_product") {
		t.Errorf("query missing table name: %s", q)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Package ddl generates expand/contract/rollback DDL from declarative column specs.
package ddl

import (
	"fmt"
	"strings"
)

// ColumnSpec describes a column to add or modify.
type ColumnSpec struct {
	Name       string `yaml:"name" json:"name"`
	Type       string `yaml:"type" json:"type"`
	Expression string `yaml:"expression,omitempty" json:"expression,omitempty"` // backfill source_expr
	Nullable   bool   `yaml:"nullable,omitempty" json:"nullable,omitempty"`
	Indexed    bool   `yaml:"indexed,omitempty" json:"indexed,omitempty"`
}

// Output holds the generated SQL fragments for a migration plan.
type Output struct {
	Expand   []string // DDL to add columns/indexes (always NULL for safety)
	Contract []string // DDL to drop legacy columns + enforce NOT NULL constraints
	Rollback []string // DDL to undo expand
	BackfillExpr     string // source_expr for backfill (first column with expression, for tracking)
	BackfillColumn   string // column name for backfill (first column with expression, for tracking)
	BackfillMulti    string // composite UPDATE statement for multiple columns
}

// Generate produces expand/contract/rollback DDL from declarative specs.
func Generate(table string, add []ColumnSpec, drop []string) Output {
	out := Output{}

	// Expand: add columns as NULL (safe for tables with existing data) + optional indexes
	for _, col := range add {
		out.Expand = append(out.Expand, addColumnDDL(table, col))
		if col.Indexed {
			out.Expand = append(out.Expand, createIndexDDL(table, col))
		}
	}

	// Contract: drop legacy columns + enforce NOT NULL on new columns
	for _, col := range drop {
		out.Contract = append(out.Contract, dropColumnDDL(table, col))
	}
	for _, col := range add {
		if !col.Nullable {
			out.Contract = append(out.Contract, setNotNullDDL(table, col))
		}
	}

	// Rollback: undo expand (drop indexes first, then columns)
	for _, col := range add {
		if col.Indexed {
			out.Rollback = append(out.Rollback, dropIndexDDL(table, col))
		}
		out.Rollback = append(out.Rollback, dropColumnDDL(table, col.Name))
	}

	// Backfill: collect columns with expressions
	var exprCols []ColumnSpec
	for _, col := range add {
		if col.Expression != "" {
			exprCols = append(exprCols, col)
		}
	}

	if len(exprCols) > 0 {
		// Set tracking fields from first column
		out.BackfillExpr = exprCols[0].Expression
		out.BackfillColumn = exprCols[0].Name

		// Generate composite UPDATE for all columns
		out.BackfillMulti = generateCompositeBackfill(table, exprCols)
	}

	return out
}

// generateCompositeBackfill creates a single UPDATE statement that fills multiple columns at once.
func generateCompositeBackfill(table string, cols []ColumnSpec) string {
	if len(cols) == 0 {
		return ""
	}

	var setClauses []string
	for _, col := range cols {
		setClauses = append(setClauses, fmt.Sprintf("%s = (%s)", col.Name, col.Expression))
	}

	// Build WHERE clause: at least one target column is NULL
	var nullChecks []string
	for _, col := range cols {
		nullChecks = append(nullChecks, fmt.Sprintf("%s IS NULL", col.Name))
	}

	return fmt.Sprintf(
		"UPDATE %s SET %s WHERE id IN (SELECT id FROM %s WHERE %s ORDER BY id LIMIT $1)",
		table,
		strings.Join(setClauses, ", "),
		table,
		strings.Join(nullChecks, " OR "),
	)
}

// addColumnDDL always adds columns as NULL for safety with existing data.
// NOT NULL is enforced later in the Contract phase after backfill.
func addColumnDDL(table string, col ColumnSpec) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", table, col.Name, col.Type)
}

// setNotNullDDL enforces NOT NULL after backfill is complete.
func setNotNullDDL(table string, col ColumnSpec) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL", table, col.Name)
}

func createIndexDDL(table string, col ColumnSpec) string {
	idxName := fmt.Sprintf("idx_%s_%s", table, col.Name)
	// Truncate index name to 63 bytes (Postgres limit)
	if len(idxName) > 63 {
		idxName = idxName[:63]
	}
	return fmt.Sprintf("CREATE INDEX CONCURRENTLY IF NOT EXISTS %s ON %s (%s)", idxName, table, col.Name)
}

func dropColumnDDL(table string, colName string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s", table, colName)
}

func dropIndexDDL(table string, col ColumnSpec) string {
	idxName := fmt.Sprintf("idx_%s_%s", table, col.Name)
	if len(idxName) > 63 {
		idxName = idxName[:63]
	}
	return fmt.Sprintf("DROP INDEX IF EXISTS %s", idxName)
}

// SchemaColumn represents a column from information_schema.
type SchemaColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
}

// IntrospectQuery returns the SQL to query information_schema for a table's columns.
func IntrospectQuery(table string) string {
	return fmt.Sprintf(`
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_name = '%s'
		ORDER BY ordinal_position
	`, sanitizeIdent(table))
}

// sanitizeIdent ensures a SQL identifier contains only safe characters.
func sanitizeIdent(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

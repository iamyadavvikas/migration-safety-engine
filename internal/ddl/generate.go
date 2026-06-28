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
	Expand   []string // DDL to add columns/indexes
	Contract []string // DDL to drop legacy columns
	Rollback []string // DDL to undo expand
	BackfillExpr     string // source_expr for backfill (from first column with Expression)
	BackfillColumn   string // column name for backfill
}

// Generate produces expand/contract/rollback DDL from declarative specs.
func Generate(table string, add []ColumnSpec, drop []string) Output {
	out := Output{}

	// Expand: add columns + optional indexes
	for _, col := range add {
		out.Expand = append(out.Expand, addColumnDDL(table, col))
		if col.Indexed {
			out.Expand = append(out.Expand, createIndexDDL(table, col))
		}
	}

	// Contract: drop legacy columns
	for _, col := range drop {
		out.Contract = append(out.Contract, dropColumnDDL(table, col))
	}

	// Rollback: undo expand (drop indexes first, then columns)
	for _, col := range add {
		if col.Indexed {
			out.Rollback = append(out.Rollback, dropIndexDDL(table, col))
		}
		out.Rollback = append(out.Rollback, dropColumnDDL(table, col.Name))
	}

	// Backfill: use first column with an expression
	for _, col := range add {
		if col.Expression != "" {
			out.BackfillExpr = col.Expression
			out.BackfillColumn = col.Name
			break
		}
	}

	return out
}

func addColumnDDL(table string, col ColumnSpec) string {
	null := "NULL"
	if !col.Nullable {
		null = "NOT NULL"
	}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s %s", table, col.Name, col.Type, null)
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

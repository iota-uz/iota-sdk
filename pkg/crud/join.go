package crud

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var (
	// validColumnPattern allows simple column references (no functions)
	// Used for JOIN clause validation (LeftColumn, RightColumn, TableAlias)
	// - Simple columns: "column", "table.column", "schema.table.column"
	// - JSONB extraction: "table.jsonb_col->>'key'"
	// - Optional AS alias: " AS alias_name"
	// Examples: "table.column", "col->>'key' AS name"
	validColumnPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_*][a-zA-Z0-9_]*){0,2}(->>?'[a-zA-Z0-9_]+')?(\s+[Aa][Ss]\s+[a-zA-Z_][a-zA-Z0-9_]*)?$`)

	// validSelectColumnPattern allows column references with optional function calls
	// Used for SELECT columns validation in JoinOptions
	// - Simple columns: "column", "table.column", "schema.table.column"
	// - JSONB extraction: "table.jsonb_col->>'key'"
	// - PostgreSQL functions (must have AS alias): "row_to_json(table.*) AS alias_name"
	// Examples: "table.column", "row_to_json(t.*) AS data", "col->>'key' AS name"
	validSelectColumnPattern = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*\([^)]+\)\s+[Aa][Ss]\s+[a-zA-Z_][a-zA-Z0-9_]*|[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_*][a-zA-Z0-9_]*){0,2}(->>?'[a-zA-Z0-9_]+')?)(\s+[Aa][Ss]\s+[a-zA-Z_][a-zA-Z0-9_]*)?$`)

	dangerousKeywords = []string{
		"union", "select", "insert", "update", "delete", "drop",
		"create", "alter", "exec", "execute", "--", "/*", "*/", ";",
	}
)

// JoinType represents the type of SQL JOIN operation
type JoinType int

const (
	// JoinTypeInner represents an INNER JOIN
	JoinTypeInner JoinType = iota
	// JoinTypeLeft represents a LEFT JOIN (LEFT OUTER JOIN)
	JoinTypeLeft
	// JoinTypeRight represents a RIGHT JOIN (RIGHT OUTER JOIN)
	JoinTypeRight
)

// String returns the SQL keyword for the join type
func (jt JoinType) String() string {
	switch jt {
	case JoinTypeInner:
		return "INNER JOIN"
	case JoinTypeLeft:
		return "LEFT JOIN"
	case JoinTypeRight:
		return "RIGHT JOIN"
	default:
		return "INNER JOIN"
	}
}

// JoinClause represents a single JOIN clause in a SQL query
type JoinClause struct {
	// Type is the type of JOIN (INNER, LEFT, RIGHT)
	Type JoinType
	// Table is the name of the table to join
	Table string
	// TableAlias is an optional alias for the joined table
	TableAlias string
	// LeftColumn is the column from the left side of the join (e.g., "users.role_id")
	LeftColumn string
	// RightColumn is the column from the right side of the join (e.g., "roles.id")
	RightColumn string
}

func (jc *JoinClause) Validate() error {
	op := serrors.Op("JoinClause.Validate")

	if jc.Table == "" {
		return serrors.E(op, serrors.Invalid, "join table cannot be empty")
	}

	if jc.LeftColumn == "" {
		return serrors.E(op, serrors.Invalid, "join left column cannot be empty")
	}

	if jc.RightColumn == "" {
		return serrors.E(op, serrors.Invalid, "join right column cannot be empty")
	}

	// Check for dangerous SQL keywords FIRST (security priority)
	for _, val := range []string{jc.Table, jc.TableAlias, jc.LeftColumn, jc.RightColumn} {
		if val == "" {
			continue
		}
		lowerVal := strings.ToLower(val)
		for _, keyword := range dangerousKeywords {
			if strings.Contains(lowerVal, keyword) {
				return serrors.E(op, serrors.Invalid, fmt.Sprintf("join specification contains dangerous SQL keyword: %q", val))
			}
		}
	}

	// Then validate format patterns
	if !validColumnPattern.MatchString(jc.Table) {
		return serrors.E(op, serrors.Invalid, fmt.Sprintf("invalid table specification: %q", jc.Table))
	}

	if !validColumnPattern.MatchString(jc.LeftColumn) {
		return serrors.E(op, serrors.Invalid, fmt.Sprintf("invalid left column specification: %q", jc.LeftColumn))
	}

	if !validColumnPattern.MatchString(jc.RightColumn) {
		return serrors.E(op, serrors.Invalid, fmt.Sprintf("invalid right column specification: %q", jc.RightColumn))
	}

	if jc.TableAlias != "" && !validColumnPattern.MatchString(jc.TableAlias) {
		return serrors.E(op, serrors.Invalid, fmt.Sprintf("invalid table alias specification: %q", jc.TableAlias))
	}

	return nil
}

// ToSQL converts the JoinClause to SQL using pkg/repo builders
func (jc *JoinClause) ToSQL() string {
	return repo.JoinClause(jc.Type.String(), jc.Table, jc.TableAlias, jc.LeftColumn, jc.RightColumn)
}

// JoinOptions contains configuration for joins in a List query
type JoinOptions struct {
	// Joins is the list of JOIN clauses to apply
	Joins []JoinClause
	// SelectColumns specifies which columns to select (if empty, uses default SELECT)
	SelectColumns []string
}

func (jo *JoinOptions) Validate() error {
	op := serrors.Op("JoinOptions.Validate")

	// Validate each join clause
	for i, join := range jo.Joins {
		if err := join.Validate(); err != nil {
			return serrors.E(op, fmt.Sprintf("join clause %d", i), err)
		}
	}

	// Validate SelectColumns for SQL injection
	if err := validateSelectColumns(jo.SelectColumns); err != nil {
		return serrors.E(op, serrors.Invalid, err)
	}

	return nil
}

// validateSelectColumns checks that column specifications are safe
func validateSelectColumns(columns []string) error {
	if len(columns) == 0 {
		return nil
	}

	for _, col := range columns {
		col = strings.TrimSpace(col)
		if col == "" {
			return fmt.Errorf("empty column specification")
		}

		if col == "*" {
			continue
		}

		lowerCol := strings.ToLower(col)
		for _, keyword := range dangerousKeywords {
			if strings.Contains(lowerCol, keyword) {
				return fmt.Errorf("column specification contains dangerous SQL keyword: %q", col)
			}
		}

		// Check against pattern (allows functions in SELECT columns)
		if !validSelectColumnPattern.MatchString(col) {
			return fmt.Errorf("invalid column specification: %q (must be 'table.column', 'column AS alias', or similar)", col)
		}
	}

	return nil
}

// ToSQL converts all join clauses to SQL strings
func (jo *JoinOptions) ToSQL() []string {
	clauses := make([]string, len(jo.Joins))
	for i, join := range jo.Joins {
		clauses[i] = join.ToSQL()
	}
	return clauses
}

// MergeJoinOptions combines default schema JOINs with request-specific JOINs.
// Request JOINs are appended after defaults, allowing for additional joins.
// If request specifies SelectColumns, they take precedence over defaults.
func MergeJoinOptions(defaultJoins, requestJoins *JoinOptions) *JoinOptions {
	if defaultJoins == nil {
		return requestJoins
	}
	if requestJoins == nil {
		return defaultJoins
	}

	merged := &JoinOptions{
		Joins: make([]JoinClause, 0, len(defaultJoins.Joins)+len(requestJoins.Joins)),
	}

	// Combine JOIN clauses: defaults first, then request-specific
	merged.Joins = append(merged.Joins, defaultJoins.Joins...)
	merged.Joins = append(merged.Joins, requestJoins.Joins...)

	// SelectColumns: request takes precedence if specified, otherwise use defaults
	if len(requestJoins.SelectColumns) > 0 {
		merged.SelectColumns = requestJoins.SelectColumns
	} else {
		merged.SelectColumns = defaultJoins.SelectColumns
	}

	return merged
}

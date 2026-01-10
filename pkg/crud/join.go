package crud

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/repo"
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

// Validate checks if the JoinClause has all required fields
func (jc *JoinClause) Validate() error {
	if jc.Table == "" {
		return fmt.Errorf("join table cannot be empty")
	}
	if jc.LeftColumn == "" {
		return fmt.Errorf("join left column cannot be empty")
	}
	if jc.RightColumn == "" {
		return fmt.Errorf("join right column cannot be empty")
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

// Validate checks if all join clauses are valid
func (jo *JoinOptions) Validate() error {
	for i, join := range jo.Joins {
		if err := join.Validate(); err != nil {
			return fmt.Errorf("join clause %d: %w", i, err)
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

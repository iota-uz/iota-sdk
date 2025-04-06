// Package repo provides database utility functions and interfaces for working with PostgreSQL.
package repo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Expr represents a comparison expression type for filtering queries.
type Expr int

const (
	// Eq represents the equals (=) comparison operator.
	Eq Expr = iota
	// NotEq represents the not equals (!=) comparison operator.
	NotEq
	// Gt represents the greater than (>) comparison operator.
	Gt
	// Gte represents the greater than or equal (>=) comparison operator.
	Gte
	// Lt represents the less than (<) comparison operator.
	Lt
	// Lte represents the less than or equal (<=) comparison operator.
	Lte
	// In represents the SQL IN operator.
	In
	// NotIn represents the SQL NOT IN operator.
	NotIn
	// Like represents the SQL LIKE operator for pattern matching.
	Like
	// NotLike represents the SQL NOT LIKE operator for pattern matching.
	NotLike
)

// SortBy defines sorting criteria for queries with generic field type support.
// Use with OrderBy function to generate ORDER BY clauses.
type SortBy[T any] struct {
	// Fields represents the list of fields to sort by.
	Fields []T
	// Ascending indicates the sort direction (true for ASC, false for DESC).
	Ascending bool
}

// Filter defines a filter condition for queries.
// Combines an expression type with a value to be used in WHERE clauses.
type Filter struct {
	// Expr is the comparison expression to use (Eq, NotEq, Gt, etc.).
	Expr Expr
	// Value is the value to compare against.
	Value any
}

// Tx is an interface that abstracts database transaction operations.
// It provides a subset of pgx.Tx functionality needed for common database operations.
type Tx interface {
	// CopyFrom performs a COPY FROM operation to efficiently insert multiple rows.
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	// SendBatch sends a batch of queries in a single request.
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults

	// Exec executes an SQL command and returns a command tag.
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	// Query executes an SQL query that returns rows.
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	// QueryRow executes an SQL query that returns at most one row.
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ExtendedFieldSet is an interface that must be implemented to persist custom fields with a repository.
// It allows repositories to work with custom field sets by providing field names and values.
type ExtendedFieldSet interface {
	// Fields returns a list of field names.
	Fields() []string
	// Value returns the value for a given field name.
	Value(k string) interface{}
}

// FormatLimitOffset generates SQL LIMIT and OFFSET clauses based on the provided values.
//
// If both limit and offset are positive, it returns "LIMIT x OFFSET y".
// If only limit is positive, it returns "LIMIT x".
// If only offset is positive, it returns "OFFSET y".
// If neither is positive, it returns an empty string.
//
// Example usage:
//
//	query := "SELECT * FROM users " + repo.FormatLimitOffset(10, 20)
//	// Returns: "SELECT * FROM users LIMIT 10 OFFSET 20"
func FormatLimitOffset(limit, offset int) string {
	if limit > 0 && offset > 0 {
		return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	} else if limit > 0 {
		return fmt.Sprintf("LIMIT %d", limit)
	} else if offset > 0 {
		return fmt.Sprintf("OFFSET %d", offset)
	}
	return ""
}

// Join combines multiple SQL expressions with spaces between them.
//
// Example usage:
//
//	query := repo.Join("SELECT *", "FROM users", "WHERE active = true")
//	// Returns: "SELECT * FROM users WHERE active = true"
func Join(expressions ...string) string {
	return strings.Join(expressions, " ")
}

// OrderBy generates an SQL ORDER BY clause for the given fields and sort direction.
// Returns an empty string if no fields are provided.
//
// Example usage:
//
//	query := "SELECT * FROM users " + repo.OrderBy([]string{"created_at", "name"}, false)
//	// Returns: "SELECT * FROM users ORDER BY created_at, name DESC"
func OrderBy(fields []string, ascending bool) string {
	if len(fields) == 0 {
		return ""
	}
	q := "ORDER BY " + strings.Join(fields, ", ")
	if ascending {
		q += " ASC"
	} else {
		q += " DESC"
	}
	return q
}

// JoinWhere creates an SQL WHERE clause by joining multiple conditions with AND.
//
// Example usage:
//
//	conditions := []string{"status = $1", "created_at > $2"}
//	query := "SELECT * FROM orders " + repo.JoinWhere(conditions...)
//	// Returns: "SELECT * FROM orders WHERE status = $1 AND created_at > $2"
func JoinWhere(expressions ...string) string {
	return fmt.Sprintf("WHERE %s", strings.Join(expressions, " AND "))
}

// Insert creates a parameterized SQL query for inserting a single row.
// Optionally returns specified columns with the RETURNING clause.
//
// Example usage:
//
//	query := repo.Insert("users", []string{"name", "email", "password"}, "id", "created_at")
//	// Returns: "INSERT INTO users (name, email, password) VALUES ($1, $2, $3) RETURNING id, created_at"
func Insert(tableName string, fields []string, returning ...string) string {
	args := make([]string, len(fields))
	for i := range fields {
		args[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(fields, ", "), strings.Join(args, ", "))

	if len(returning) > 0 {
		query += " RETURNING " + strings.Join(returning, ", ")
	}

	return query
}

// Update creates a parameterized SQL query for updating rows in a table.
// The where parameters are optional conditions that will be ANDed together.
//
// Example usage:
//
//	query := repo.Update("users", []string{"name", "email"}, "id = $3")
//	// Returns: "UPDATE users SET name = $1, email = $2 WHERE id = $3"
//
//	// Multiple conditions
//	query := repo.Update("products", []string{"name", "price", "updated_at"}, "id = $4", "category_id = $5")
//	// Returns: "UPDATE products SET name = $1, price = $2, updated_at = $3 WHERE id = $4 AND category_id = $5"
//
//	// No conditions
//	query := repo.Update("settings", []string{"value", "updated_at"})
//	// Returns: "UPDATE settings SET value = $1, updated_at = $2"
func Update(tableName string, fields []string, where ...string) string {
	setFields := make([]string, len(fields))

	for i, field := range fields {
		setFields[i] = fmt.Sprintf("%s = $%d", field, i+1)
	}

	q := fmt.Sprintf("UPDATE %s SET %s", tableName, strings.Join(setFields, ", "))

	if len(where) > 0 {
		q += " " + JoinWhere(where...)
	}

	return q
}

// BatchInsertQueryN creates a parameterized SQL query for batch inserting multiple rows.
// It takes a base query like "INSERT INTO users (name, email) VALUES" and appends
// the parameterized values for each row, returning both the query and the flattened arguments.
//
// Example usage:
//
//	baseQuery := "INSERT INTO users (name, email) VALUES"
//	rows := [][]interface{}{
//	    {"John", "john@example.com"},
//	    {"Jane", "jane@example.com"},
//	    {"Bob", "bob@example.com"},
//	}
//	query, args := repo.BatchInsertQueryN(baseQuery, rows)
//	// query = "INSERT INTO users (name, email) VALUES ($1,$2),($3,$4),($5,$6)"
//	// args = []interface{}{"John", "john@example.com", "Jane", "jane@example.com", "Bob", "bob@example.com"}
//
// If rows is empty, it returns the baseQuery unchanged and nil for args.
// Panics if rows have inconsistent lengths.
func BatchInsertQueryN(baseQuery string, rows [][]interface{}) (string, []interface{}) {
	if len(rows) == 0 {
		return baseQuery, nil
	}

	valuesPerRow := len(rows[0])
	args := make([]interface{}, 0, len(rows)*valuesPerRow)
	query := baseQuery + " "

	for i, row := range rows {
		if len(row) != valuesPerRow {
			panic("all rows must have the same number of values")
		}

		if i != 0 {
			query += ","
		}

		query += "("
		for j, value := range row {
			if j != 0 {
				query += ","
			}
			query += "$" + strconv.Itoa(len(args)+1)
			args = append(args, value)
		}
		query += ")"
	}

	return query, args
}

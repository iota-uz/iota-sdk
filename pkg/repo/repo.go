package repo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Expr int

const (
	Eq Expr = iota
	NotEq
	Gt
	Gte
	Lt
	Lte
	In
	NotIn
	Like
	NotLike
)

type SortBy[T any] struct {
	Fields    []T
	Ascending bool
}

type Filter struct {
	Expr  Expr
	Value any
}

type Tx interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults

	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ExtendedFieldSet is an interface you have to implement to persist custom fields with a repository
type ExtendedFieldSet interface {
	Fields() []string
	Value(k string) interface{}
}

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

func Join(expressions ...string) string {
	return strings.Join(expressions, " ")
}

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

func JoinWhere(expressions ...string) string {
	return fmt.Sprintf("WHERE %s", strings.Join(expressions, " AND "))
}

// Insert creates a parameterized SQL query for inserting a single row
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

// BatchInsertQueryN creates a parameterized SQL query for batch inserting multiple values per row
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

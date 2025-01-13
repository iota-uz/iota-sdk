package repo

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strconv"
	"strings"
)

type Tx interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults

	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
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

func JoinWhere(expressions ...string) string {
	return fmt.Sprintf("WHERE %s", strings.Join(expressions, " AND "))
}

// BuildBatchInsertQueryN creates a parameterized SQL query for batch inserting multiple values per row
func BuildBatchInsertQueryN(baseQuery string, rows [][]interface{}) (string, []interface{}) {
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

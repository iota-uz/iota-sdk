package sql

import (
	"context"
	"fmt"
	"time"
)

// QueryExecutorSchemaLister adapts a QueryExecutor to implement SchemaLister
// by executing SQL queries to list tables.
type QueryExecutorSchemaLister struct {
	executor QueryExecutor
}

// NewQueryExecutorSchemaLister creates a schema lister that uses a query executor.
func NewQueryExecutorSchemaLister(executor QueryExecutor) SchemaLister {
	return &QueryExecutorSchemaLister{executor: executor}
}

// SchemaList executes a query to list all tables and views.
func (l *QueryExecutorSchemaLister) SchemaList(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT
			n.nspname AS schema,
			c.relname AS name,
			GREATEST(c.reltuples, 0)::bigint AS approximate_row_count
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'analytics'
		  AND c.relkind IN ('v', 'r', 'm')
		ORDER BY c.relname
	`

	result, err := l.executor.ExecuteQuery(ctx, query, nil, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to list schema: %w", err)
	}

	tables := make([]TableInfo, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 3 {
			continue
		}

		schema, _ := row[0].(string)
		name, _ := row[1].(string)

		var rowCount int64
		switch v := row[2].(type) {
		case int64:
			rowCount = v
		case int:
			rowCount = int64(v)
		case float64:
			rowCount = int64(v)
		default:
			rowCount = 0
		}

		tables = append(tables, TableInfo{
			Schema:   schema,
			Name:     name,
			RowCount: rowCount,
		})
	}

	return tables, nil
}

// QueryExecutorSchemaDescriber adapts a QueryExecutor to implement SchemaDescriber
// by executing SQL queries to describe table schemas.
type QueryExecutorSchemaDescriber struct {
	executor QueryExecutor
}

// NewQueryExecutorSchemaDescriber creates a schema describer that uses a query executor.
func NewQueryExecutorSchemaDescriber(executor QueryExecutor) SchemaDescriber {
	return &QueryExecutorSchemaDescriber{executor: executor}
}

// SchemaDescribe executes queries to get detailed schema information.
func (d *QueryExecutorSchemaDescriber) SchemaDescribe(ctx context.Context, tableName string) (*TableSchema, error) {
	// Query column information
	columnsQuery := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length,
			numeric_precision,
			numeric_scale
		FROM information_schema.columns
		WHERE table_schema = 'analytics' AND table_name = $1
		ORDER BY ordinal_position
	`

	columnsResult, err := d.executor.ExecuteQuery(ctx, columnsQuery, []any{tableName}, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to describe columns: %w", err)
	}

	columns := make([]ColumnInfo, 0, len(columnsResult.Rows))
	for _, row := range columnsResult.Rows {
		if len(row) < 7 {
			continue
		}

		colName, _ := row[0].(string)
		dataType, _ := row[1].(string)
		isNullable, _ := row[2].(string)
		colDefault := row[3]
		// Skip length/precision/scale for now

		var defaultValue *string
		if colDefault != nil {
			if s, ok := colDefault.(string); ok {
				defaultValue = &s
			}
		}

		columns = append(columns, ColumnInfo{
			Name:         colName,
			Type:         dataType,
			Nullable:     isNullable == "YES",
			DefaultValue: defaultValue,
		})
	}

	return &TableSchema{
		Name:    tableName,
		Schema:  "analytics",
		Columns: columns,
	}, nil
}

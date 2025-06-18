package excel

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DataSource provides data for Excel export
type DataSource interface {
	// GetHeaders returns the column headers
	GetHeaders() []string
	// GetRows returns an iterator function for rows
	GetRows(ctx context.Context) (func() ([]interface{}, error), error)
	// GetSheetName returns the name for the Excel sheet
	GetSheetName() string
}

// PostgresDataSource implements DataSource for PostgreSQL queries
type PostgresDataSource struct {
	db        *sql.DB
	query     string
	args      []interface{}
	sheetName string
}

// NewPostgresDataSource creates a new PostgreSQL data source
func NewPostgresDataSource(db *sql.DB, query string, args ...interface{}) *PostgresDataSource {
	return &PostgresDataSource{
		db:        db,
		query:     query,
		args:      args,
		sheetName: "Sheet1",
	}
}

// WithSheetName sets a custom sheet name
func (p *PostgresDataSource) WithSheetName(name string) *PostgresDataSource {
	p.sheetName = name
	return p
}

// GetSheetName returns the sheet name
func (p *PostgresDataSource) GetSheetName() string {
	return p.sheetName
}

// GetHeaders returns column headers from the query
func (p *PostgresDataSource) GetHeaders() []string {
	ctx := context.Background()
	rows, err := p.db.QueryContext(ctx, p.query, p.args...)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return []string{}
	}

	return columns
}

// GetRows returns an iterator function for fetching rows
func (p *PostgresDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
	rows, err := p.db.QueryContext(ctx, p.query, p.args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	columns, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	columnCount := len(columns)

	return func() ([]interface{}, error) {
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return nil, err
			}
			rows.Close()
			return nil, nil // EOF
		}

		values := make([]interface{}, columnCount)
		valuePtrs := make([]interface{}, columnCount)
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert sql.NullX types to their underlying values
		for i, val := range values {
			values[i] = convertSQLValue(val)
		}

		return values, nil
	}, nil
}

// convertSQLValue converts SQL null types to their underlying values
func convertSQLValue(val interface{}) interface{} {
	switch v := val.(type) {
	case sql.NullString:
		if v.Valid {
			return v.String
		}
		return nil
	case sql.NullInt64:
		if v.Valid {
			return v.Int64
		}
		return nil
	case sql.NullFloat64:
		if v.Valid {
			return v.Float64
		}
		return nil
	case sql.NullBool:
		if v.Valid {
			return v.Bool
		}
		return nil
	case sql.NullTime:
		if v.Valid {
			return v.Time
		}
		return nil
	case []byte:
		return string(v)
	default:
		return val
	}
}

// PgxDataSource implements DataSource for pgx/pgxpool queries
type PgxDataSource struct {
	db        *pgxpool.Pool
	query     string
	args      []interface{}
	sheetName string
}

// NewPgxDataSource creates a new pgx data source
func NewPgxDataSource(db *pgxpool.Pool, query string, args ...interface{}) *PgxDataSource {
	return &PgxDataSource{
		db:        db,
		query:     query,
		args:      args,
		sheetName: "Sheet1",
	}
}

// WithSheetName sets a custom sheet name
func (p *PgxDataSource) WithSheetName(name string) *PgxDataSource {
	p.sheetName = name
	return p
}

// GetSheetName returns the sheet name
func (p *PgxDataSource) GetSheetName() string {
	return p.sheetName
}

// GetHeaders returns column headers from the query
func (p *PgxDataSource) GetHeaders() []string {
	ctx := context.Background()
	rows, err := p.db.Query(ctx, p.query, p.args...)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	for i, field := range fields {
		columns[i] = string(field.Name)
	}

	return columns
}

// GetRows returns an iterator function for fetching rows
func (p *PgxDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
	rows, err := p.db.Query(ctx, p.query, p.args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return func() ([]interface{}, error) {
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return nil, err
			}
			rows.Close()
			return nil, nil // EOF
		}

		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert pgx types if needed
		for i, val := range values {
			values[i] = convertPgxValue(val)
		}

		return values, nil
	}, nil
}

// convertPgxValue converts pgx-specific types
func convertPgxValue(val interface{}) interface{} {
	switch v := val.(type) {
	case []byte:
		return string(v)
	default:
		// Check for sql.Null types
		return convertSQLValue(val)
	}
}

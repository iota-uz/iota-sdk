package excel

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
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
	defer func() {
		_ = rows.Close()
	}()

	columns, err := rows.Columns()
	if err != nil {
		return []string{}
	}

	if err := rows.Err(); err != nil {
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

	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("rows error after query: %w", err)
	}

	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	columnCount := len(columns)

	return func() ([]interface{}, error) {
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return nil, err
			}
			_ = rows.Close()
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
		columns[i] = field.Name
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
	case pgtype.Numeric:
		if v.Valid {
			float64Val, err := v.Float64Value()
			if err != nil {
				// If conversion fails, return 0 to avoid display issues
				return 0.0
			}
			return float64Val.Float64
		}
		return nil
	case *pgtype.Numeric:
		if v != nil && v.Valid {
			float64Val, err := v.Float64Value()
			if err != nil {
				// If conversion fails, return 0 to avoid display issues
				return 0.0
			}
			return float64Val.Float64
		}
		return nil
	default:
		// Check for sql.Null types
		return convertSQLValue(val)
	}
}

// FunctionDataSource wraps a Go function as a DataSource
type FunctionDataSource struct {
	headers   []string
	dataFunc  func(ctx context.Context) ([][]interface{}, error)
	sheetName string
}

// NewFunctionDataSource creates a data source from a Go function
func NewFunctionDataSource(headers []string, dataFunc func(ctx context.Context) ([][]interface{}, error)) *FunctionDataSource {
	return &FunctionDataSource{
		headers:   headers,
		dataFunc:  dataFunc,
		sheetName: "Sheet1",
	}
}

// WithSheetName sets a custom sheet name
func (f *FunctionDataSource) WithSheetName(name string) *FunctionDataSource {
	f.sheetName = name
	return f
}

// GetHeaders returns the column headers
func (f *FunctionDataSource) GetHeaders() []string {
	return f.headers
}

// GetRows returns an iterator function for fetching rows
func (f *FunctionDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
	data, err := f.dataFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get data from function: %w", err)
	}

	index := 0
	return func() ([]interface{}, error) {
		if index >= len(data) {
			return nil, nil // EOF
		}
		row := data[index]
		index++
		return row, nil
	}, nil
}

// GetSheetName returns the sheet name
func (f *FunctionDataSource) GetSheetName() string {
	return f.sheetName
}

// SliceDataSource wraps Go slices as a DataSource
type SliceDataSource struct {
	headers   []string
	data      [][]interface{}
	sheetName string
}

// NewSliceDataSource creates a data source from Go slices
func NewSliceDataSource(headers []string, data [][]interface{}) *SliceDataSource {
	return &SliceDataSource{
		headers:   headers,
		data:      data,
		sheetName: "Sheet1",
	}
}

// WithSheetName sets a custom sheet name
func (s *SliceDataSource) WithSheetName(name string) *SliceDataSource {
	s.sheetName = name
	return s
}

// GetHeaders returns the column headers
func (s *SliceDataSource) GetHeaders() []string {
	return s.headers
}

// GetRows returns an iterator function for fetching rows
func (s *SliceDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
	index := 0
	return func() ([]interface{}, error) {
		if index >= len(s.data) {
			return nil, nil // EOF
		}
		row := s.data[index]
		index++
		return row, nil
	}, nil
}

// GetSheetName returns the sheet name
func (s *SliceDataSource) GetSheetName() string {
	return s.sheetName
}

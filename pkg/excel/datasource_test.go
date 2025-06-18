package excel_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/excel"
)

func TestPostgresDataSource_GetHeaders(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "email", "created_at"})
	mock.ExpectQuery("SELECT (.+) FROM users").WillReturnRows(rows)

	ds := excel.NewPostgresDataSource(db, "SELECT * FROM users")
	headers := ds.GetHeaders()

	assert.Equal(t, []string{"id", "name", "email", "created_at"}, headers)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresDataSource_GetRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "email"}).
		AddRow(1, "John Doe", "john@example.com").
		AddRow(2, "Jane Smith", "jane@example.com").
		AddRow(3, sql.NullString{String: "Bob", Valid: true}, sql.NullString{Valid: false})

	mock.ExpectQuery("SELECT (.+) FROM users").WillReturnRows(rows)

	ds := excel.NewPostgresDataSource(db, "SELECT id, name, email FROM users")
	ctx := context.Background()

	getRow, err := ds.GetRows(ctx)
	require.NoError(t, err)

	// First row
	row1, err := getRow()
	require.NoError(t, err)
	assert.Equal(t, []interface{}{int64(1), "John Doe", "john@example.com"}, row1)

	// Second row
	row2, err := getRow()
	require.NoError(t, err)
	assert.Equal(t, []interface{}{int64(2), "Jane Smith", "jane@example.com"}, row2)

	// Third row with null values
	row3, err := getRow()
	require.NoError(t, err)
	assert.Equal(t, []interface{}{int64(3), "Bob", nil}, row3)

	// No more rows
	row4, err := getRow()
	require.NoError(t, err)
	assert.Nil(t, row4)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresDataSource_WithSheetName(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ds := excel.NewPostgresDataSource(db, "SELECT * FROM users")
	assert.Equal(t, "Sheet1", ds.GetSheetName())

	ds.WithSheetName("Users")
	assert.Equal(t, "Users", ds.GetSheetName())
}

func TestPostgresDataSource_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM users").WillReturnError(sql.ErrConnDone)

	ds := excel.NewPostgresDataSource(db, "SELECT * FROM users")
	ctx := context.Background()

	_, err = ds.GetRows(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute query")
}

func TestPostgresDataSource_ConvertSQLValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"str", "int", "float", "bool", "bytes"}).
		AddRow(
			sql.NullString{String: "test", Valid: true},
			sql.NullInt64{Int64: 42, Valid: true},
			sql.NullFloat64{Float64: 3.14, Valid: true},
			sql.NullBool{Bool: true, Valid: true},
			[]byte("byte data"),
		).
		AddRow(
			sql.NullString{Valid: false},
			sql.NullInt64{Valid: false},
			sql.NullFloat64{Valid: false},
			sql.NullBool{Valid: false},
			nil,
		)

	mock.ExpectQuery("SELECT (.+) FROM test").WillReturnRows(rows)

	ds := excel.NewPostgresDataSource(db, "SELECT * FROM test")
	ctx := context.Background()

	getRow, err := ds.GetRows(ctx)
	require.NoError(t, err)

	// First row with valid values
	row1, err := getRow()
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"test", int64(42), float64(3.14), true, "byte data"}, row1)

	// Second row with null values
	row2, err := getRow()
	require.NoError(t, err)
	assert.Equal(t, []interface{}{nil, nil, nil, nil, nil}, row2)

	assert.NoError(t, mock.ExpectationsWereMet())
}

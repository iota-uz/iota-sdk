package excel_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/iota-uz/iota-sdk/pkg/excel"
)

// MockDataSource implements DataSource for testing
type MockDataSource struct {
	headers   []string
	rows      [][]interface{}
	sheetName string
	rowIndex  int
}

func NewMockDataSource(headers []string, rows [][]interface{}) *MockDataSource {
	return &MockDataSource{
		headers:   headers,
		rows:      rows,
		sheetName: "TestSheet",
		rowIndex:  0,
	}
}

func (m *MockDataSource) GetHeaders() []string {
	return m.headers
}

func (m *MockDataSource) GetSheetName() string {
	return m.sheetName
}

func (m *MockDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
	return func() ([]interface{}, error) {
		if m.rowIndex >= len(m.rows) {
			return nil, nil
		}
		row := m.rows[m.rowIndex]
		m.rowIndex++
		return row, nil
	}, nil
}

func TestExcelExporter_Export(t *testing.T) {
	headers := []string{"ID", "Name", "Email", "Age", "Created"}
	now := time.Now()
	rows := [][]interface{}{
		{1, "John Doe", "john@example.com", 30, now},
		{2, "Jane Smith", "jane@example.com", 25, now.Add(24 * time.Hour)},
		{3, "Bob Johnson", "bob@example.com", 35, now.Add(48 * time.Hour)},
	}

	ds := NewMockDataSource(headers, rows)
	exporter := excel.NewExcelExporter(nil, nil)

	ctx := context.Background()
	data, err := exporter.Export(ctx, ds)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify the Excel file
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)

	// Check sheet name
	assert.Equal(t, "TestSheet", f.GetSheetName(0))

	// Check headers
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		value, err := f.GetCellValue("TestSheet", cell)
		require.NoError(t, err)
		assert.Equal(t, header, value)
	}

	// Check first data row
	row1Values := []string{"1", "John Doe", "john@example.com", "30"}
	for i, expected := range row1Values {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		value, err := f.GetCellValue("TestSheet", cell)
		require.NoError(t, err)
		assert.Equal(t, expected, value)
	}
}

func TestExcelExporter_WithOptions(t *testing.T) {
	headers := []string{"ID", "Name"}
	rows := [][]interface{}{
		{1, "John"},
		{2, "Jane"},
		{3, "Bob"},
		{4, "Alice"},
		{5, "Charlie"},
	}

	ds := NewMockDataSource(headers, rows)

	opts := &excel.ExportOptions{
		IncludeHeaders: true,
		AutoFilter:     true,
		FreezeHeader:   true,
		MaxRows:        3,
	}

	exporter := excel.NewExcelExporter(opts, nil)
	ctx := context.Background()

	data, err := exporter.Export(ctx, ds)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify the Excel file
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)

	// Check that only 3 data rows were exported (plus header)
	for i := 1; i <= 4; i++ { // 1 header + 3 data rows
		cell, _ := excelize.CoordinatesToCellName(1, i)
		value, err := f.GetCellValue("TestSheet", cell)
		require.NoError(t, err)
		assert.NotEmpty(t, value)
	}

	// Row 5 should not exist
	cell, _ := excelize.CoordinatesToCellName(1, 5)
	value, err := f.GetCellValue("TestSheet", cell)
	require.NoError(t, err)
	assert.Empty(t, value)
}

func TestExcelExporter_NoHeaders(t *testing.T) {
	headers := []string{"ID", "Name"}
	rows := [][]interface{}{
		{1, "John"},
		{2, "Jane"},
	}

	ds := NewMockDataSource(headers, rows)

	opts := &excel.ExportOptions{
		IncludeHeaders: false,
	}

	exporter := excel.NewExcelExporter(opts, nil)
	ctx := context.Background()

	data, err := exporter.Export(ctx, ds)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)

	// First row should be data, not headers
	value, err := f.GetCellValue("TestSheet", "A1")
	require.NoError(t, err)
	assert.Equal(t, "1", value)
}

func TestExcelExporter_EmptyDataSource(t *testing.T) {
	ds := NewMockDataSource([]string{}, [][]interface{}{})
	exporter := excel.NewExcelExporter(nil, nil)

	ctx := context.Background()
	_, err := exporter.Export(ctx, ds)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no columns found")
}

func TestExcelExporter_WithStyling(t *testing.T) {
	headers := []string{"ID", "Name", "Score"}
	rows := [][]interface{}{
		{1, "John", 95.5},
		{2, "Jane", 87.3},
		{3, "Bob", 92.0},
	}

	ds := NewMockDataSource(headers, rows)

	styleOpts := excel.DefaultStyleOptions()
	exporter := excel.NewExcelExporter(nil, styleOpts)

	ctx := context.Background()
	data, err := exporter.Export(ctx, ds)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Basic validation that file was created with styles
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestExcelExporter_ContextCancellation(t *testing.T) {
	headers := []string{"ID", "Name"}
	// Create many rows to ensure context cancellation happens during processing
	rows := make([][]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		rows[i] = []interface{}{i, "Name"}
	}

	// Create a custom data source that checks context
	ds := &contextAwareDataSource{
		headers:   headers,
		rows:      rows,
		sheetName: "TestSheet",
		rowIndex:  0,
	}

	exporter := excel.NewExcelExporter(nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := exporter.Export(ctx, ds)
	assert.Error(t, err)
}

// contextAwareDataSource is a DataSource that respects context cancellation
type contextAwareDataSource struct {
	headers   []string
	rows      [][]interface{}
	sheetName string
	rowIndex  int
}

func (c *contextAwareDataSource) GetHeaders() []string {
	return c.headers
}

func (c *contextAwareDataSource) GetSheetName() string {
	return c.sheetName
}

func (c *contextAwareDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
	return func() ([]interface{}, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if c.rowIndex >= len(c.rows) {
			return nil, nil
		}
		row := c.rows[c.rowIndex]
		c.rowIndex++
		return row, nil
	}, nil
}

package itf

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// TestExcelBuilder helps create Excel files for testing
type TestExcelBuilder struct {
	headers []string
	rows    []map[string]interface{}
	sheet   string
}

// NewTestExcelBuilder creates a new Excel test builder
func NewTestExcelBuilder() *TestExcelBuilder {
	return &TestExcelBuilder{
		sheet:   "Sheet1",
		headers: []string{},
		rows:    []map[string]interface{}{},
	}
}

// WithSheet sets the sheet name
func (b *TestExcelBuilder) WithSheet(name string) *TestExcelBuilder {
	b.sheet = name
	return b
}

// WithHeaders sets the headers for the Excel file
func (b *TestExcelBuilder) WithHeaders(headers ...string) *TestExcelBuilder {
	b.headers = headers
	return b
}

// AddRow adds a data row to the Excel file
func (b *TestExcelBuilder) AddRow(row map[string]interface{}) *TestExcelBuilder {
	b.rows = append(b.rows, row)
	return b
}

// AddRows adds multiple data rows to the Excel file
func (b *TestExcelBuilder) AddRows(rows ...map[string]interface{}) *TestExcelBuilder {
	b.rows = append(b.rows, rows...)
	return b
}

// Build creates the Excel file and returns the file path
func (b *TestExcelBuilder) Build(t *testing.T) string {
	t.Helper()

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Warning: failed to close Excel file: %v", err)
		}
	}()

	// Create headers
	for i, header := range b.headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(b.sheet, cell, header); err != nil {
			t.Logf("Warning: failed to set header cell value: %v", err)
		}
	}

	// Add data rows
	for rowIdx, row := range b.rows {
		for colIdx, header := range b.headers {
			if value, ok := row[header]; ok {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
				if err := f.SetCellValue(b.sheet, cell, value); err != nil {
					t.Logf("Warning: failed to set cell value: %v", err)
				}
			}
		}
	}

	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("test_excel_%d.xlsx", time.Now().UnixNano()))
	err := f.SaveAs(tempFile)
	require.NoError(t, err)

	// Register cleanup
	t.Cleanup(func() {
		if err := os.Remove(tempFile); err != nil {
			t.Logf("Warning: failed to remove temp file: %v", err)
		}
	})

	return tempFile
}

// BuildBytes creates the Excel file and returns the content as bytes
func (b *TestExcelBuilder) BuildBytes(t *testing.T) []byte {
	t.Helper()

	filePath := b.Build(t)
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	return content
}

// BuildEmpty creates an empty Excel file
func BuildEmptyExcel(t *testing.T) string {
	t.Helper()

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Warning: failed to close Excel file: %v", err)
		}
	}()

	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("empty_%d.xlsx", time.Now().UnixNano()))
	err := f.SaveAs(tempFile)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := os.Remove(tempFile); err != nil {
			t.Logf("Warning: failed to remove temp file: %v", err)
		}
	})

	return tempFile
}

// BuildEmptyExcelBytes creates an empty Excel file and returns bytes
func BuildEmptyExcelBytes(t *testing.T) []byte {
	t.Helper()

	filePath := BuildEmptyExcel(t)
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	return content
}

// BuildInvalidExcelBytes returns invalid Excel content
func BuildInvalidExcelBytes() []byte {
	return []byte("invalid excel content")
}

// BuildWithCustomHeaders creates an Excel file with only headers (no data)
func BuildWithCustomHeaders(t *testing.T, headers []string) string {
	t.Helper()

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Warning: failed to close Excel file: %v", err)
		}
	}()

	// Create headers
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue("Sheet1", cell, header); err != nil {
			t.Logf("Warning: failed to set header cell value: %v", err)
		}
	}

	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("headers_only_%d.xlsx", time.Now().UnixNano()))
	err := f.SaveAs(tempFile)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := os.Remove(tempFile); err != nil {
			t.Logf("Warning: failed to remove temp file: %v", err)
		}
	})

	return tempFile
}

// BuildWithCustomHeadersBytes creates an Excel file with only headers and returns bytes
func BuildWithCustomHeadersBytes(t *testing.T, headers []string) []byte {
	t.Helper()

	filePath := BuildWithCustomHeaders(t, headers)
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	return content
}

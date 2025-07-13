package importpkg

import (
	"context"
)

// ImportPageConfig defines the configuration for an import page
// Following Interface Segregation - small, focused interface
type ImportPageConfig interface {
	GetTitle() string
	GetDescription() string
	GetColumns() []ImportColumn
	GetExampleRows() [][]string
	GetSaveURL() string
	GetAcceptedFileTypes() string
	GetLocalePrefix() string
	GetTemplateDownloadURL() string
	GetHTMXConfig() HTMXConfig
}

// ImportColumn represents a column in the import template
type ImportColumn struct {
	Header      string // Column header text
	Description string // Optional format/requirement description
	Required    bool   // Whether column is required
}

// HTMXConfig groups HTMX-related settings (Single Responsibility)
type HTMXConfig struct {
	Target    string // HTMX target selector
	Swap      string // HTMX swap strategy
	Indicator string // Loading indicator selector
}

// RowValidator validates individual rows
type RowValidator interface {
	ValidateRow(rowIndex int, row []string) error
}

// RowProcessor processes validated rows
type RowProcessor interface {
	ProcessRow(ctx context.Context, rowIndex int, row []string) error
}

// ColumnDefinition defines expected columns
type ColumnDefinition interface {
	ExpectedColumnCount() int
	GetColumnName(index int) string
}

// ExcelRowHandler combines all row handling concerns
// This is a composite interface for convenience
type ExcelRowHandler interface {
	ColumnDefinition
	RowValidator
	RowProcessor
}

// FileReader abstracts file reading (Dependency Inversion)
type FileReader interface {
	ReadExcelRows(filePath string) ([][]string, error)
}

// ExcelProcessor orchestrates Excel file processing
type ExcelProcessor interface {
	ProcessFile(ctx context.Context, fileID uint, handler ExcelRowHandler) error
}

// Validator defines the validation interface
type Validator interface {
	Validate(value string, col string, row uint) error
}

// ErrorFactory abstracts error creation
type ErrorFactory interface {
	NewInvalidCellError(col string, row uint) error
	NewValidationError(col, value string, rowNum uint, message string) error
}

// UploadService interface for getting uploaded files
type UploadService interface {
	GetByID(ctx context.Context, id uint) (UploadFile, error)
}

// UploadFile represents an uploaded file
type UploadFile interface {
	Path() string
}

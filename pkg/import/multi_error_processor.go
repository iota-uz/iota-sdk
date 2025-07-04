package importpkg

import (
	"context"
	"fmt"
)

// MultiErrorProcessor processes Excel files and collects all validation errors
type MultiErrorProcessor struct {
	uploadService UploadService
	fileReader    FileReader
}

// NewMultiErrorProcessor creates a new processor that collects all errors
func NewMultiErrorProcessor(uploadService UploadService, fileReader FileReader) *MultiErrorProcessor {
	return &MultiErrorProcessor{
		uploadService: uploadService,
		fileReader:    fileReader,
	}
}

// ValidationErrors holds all validation errors from processing
type ValidationErrors struct {
	Errors []error
}

func (v *ValidationErrors) Error() string {
	if len(v.Errors) == 0 {
		return ""
	}
	return fmt.Sprintf("%d validation errors found", len(v.Errors))
}

// Add adds an error to the collection
func (v *ValidationErrors) Add(err error) {
	v.Errors = append(v.Errors, err)
}

// HasErrors returns true if there are any errors
func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

// ProcessFileWithAllErrors processes the file and collects all validation errors
func (p *MultiErrorProcessor) ProcessFileWithAllErrors(ctx context.Context, fileID uint, handler ExcelRowHandler) error {
	// Get the uploaded file
	uploadedFile, err := p.uploadService.GetByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get uploaded file: %w", err)
	}

	// Read all rows from Excel
	rows, err := p.fileReader.ReadExcelRows(uploadedFile.Path())
	if err != nil {
		return fmt.Errorf("failed to read Excel file: %w", err)
	}

	// Validate all rows and collect errors
	validationErrors := &ValidationErrors{}
	validRows := make(map[int][]string)

	// Skip header row (index 0)
	for i := 1; i < len(rows); i++ {
		row := rows[i]

		// Validate the row
		if err := handler.ValidateRow(i, row); err != nil {
			validationErrors.Add(err)
		} else {
			validRows[i] = row
		}
	}

	// If there are validation errors, return them all
	if validationErrors.HasErrors() {
		return validationErrors
	}

	// Process all valid rows
	for rowIndex, row := range validRows {
		if err := handler.ProcessRow(ctx, rowIndex, row); err != nil {
			return fmt.Errorf("failed to process row %d: %w", rowIndex, err)
		}
	}

	return nil
}

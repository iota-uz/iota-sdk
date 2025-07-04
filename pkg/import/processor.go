package importpkg

import (
	"context"
)

// DefaultExcelProcessor provides the standard implementation
type DefaultExcelProcessor struct {
	uploadService UploadService
	fileReader    FileReader
	errorFactory  ErrorFactory
}

// NewDefaultExcelProcessor creates a new Excel processor
func NewDefaultExcelProcessor(uploadService UploadService, fileReader FileReader, errorFactory ErrorFactory) *DefaultExcelProcessor {
	return &DefaultExcelProcessor{
		uploadService: uploadService,
		fileReader:    fileReader,
		errorFactory:  errorFactory,
	}
}

func (p *DefaultExcelProcessor) ProcessFile(ctx context.Context, fileID uint, handler ExcelRowHandler) error {
	// Get file path from upload service
	upload, err := p.uploadService.GetByID(ctx, fileID)
	if err != nil {
		return err
	}

	// Read Excel file
	rows, err := p.fileReader.ReadExcelRows(upload.Path())
	if err != nil {
		return err
	}

	// Process each row (skip header)
	for i, row := range rows {
		if i == 0 {
			continue // Skip header row
		}

		// Validate column count
		if len(row) != handler.ExpectedColumnCount() {
			return p.errorFactory.NewInvalidCellError("D", uint(i+1))
		}

		// Validate row content
		if err := handler.ValidateRow(i, row); err != nil {
			return err
		}

		// Process row
		if err := handler.ProcessRow(ctx, i, row); err != nil {
			return err
		}
	}

	return nil
}

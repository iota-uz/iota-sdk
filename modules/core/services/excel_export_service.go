package services

import (
	"bytes"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/exportconfig"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/excel"
)

// ExcelExportService handles Excel export operations
type ExcelExportService struct {
	db            *pgxpool.Pool
	uploadService *UploadService
}

// NewExcelExportService creates a new Excel export service
func NewExcelExportService(db *pgxpool.Pool, uploadService *UploadService) *ExcelExportService {
	return &ExcelExportService{
		db:            db,
		uploadService: uploadService,
	}
}

// ExportFromQuery exports SQL query results to Excel and saves as upload
func (s *ExcelExportService) ExportFromQuery(ctx context.Context, query exportconfig.Query, config exportconfig.ExportConfig) (upload.Upload, error) {
	// Create pgx data source
	datasource := excel.NewPgxDataSource(s.db, query.SQL(), query.Args()...)
	if config.Filename() != "" {
		// Use filename without extension as sheet name
		sheetName := config.Filename()
		if len(sheetName) > 31 { // Excel sheet name limit
			sheetName = sheetName[:31]
		}
		datasource.WithSheetName(sheetName)
	}

	// Create Excel exporter with options
	exporter := excel.NewExcelExporter(config.ExportOptions(), config.StyleOptions())

	// Export to Excel
	data, err := exporter.Export(ctx, datasource)
	if err != nil {
		return nil, fmt.Errorf("failed to export to Excel: %w", err)
	}

	// Create upload DTO
	uploadDTO := &upload.CreateDTO{
		File: bytes.NewReader(data),
		Name: config.Filename(),
		Size: len(data),
	}

	// Save to upload service
	uploadEntity, err := s.uploadService.Create(ctx, uploadDTO)
	if err != nil {
		return nil, fmt.Errorf("failed to save Excel file: %w", err)
	}

	return uploadEntity, nil
}

// ExportFromDataSource exports from a custom data source to Excel
func (s *ExcelExportService) ExportFromDataSource(
	ctx context.Context,
	datasource excel.DataSource,
	config exportconfig.ExportConfig,
) (upload.Upload, error) {
	// Create Excel exporter
	exporter := excel.NewExcelExporter(config.ExportOptions(), config.StyleOptions())

	// Export to Excel
	data, err := exporter.Export(ctx, datasource)
	if err != nil {
		return nil, fmt.Errorf("failed to export to Excel: %w", err)
	}

	// Create upload DTO
	uploadDTO := &upload.CreateDTO{
		File: bytes.NewReader(data),
		Name: config.Filename(),
		Size: len(data),
	}

	// Save to upload service
	uploadEntity, err := s.uploadService.Create(ctx, uploadDTO)
	if err != nil {
		return nil, fmt.Errorf("failed to save Excel file: %w", err)
	}

	return uploadEntity, nil
}

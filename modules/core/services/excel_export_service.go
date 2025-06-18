package services

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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
func (s *ExcelExportService) ExportFromQuery(ctx context.Context, query string, filename string, args ...interface{}) (upload.Upload, error) {
	// Create pgx data source
	datasource := excel.NewPgxDataSource(s.db, query, args...)
	if filename != "" {
		// Use filename without extension as sheet name
		sheetName := filename
		if len(sheetName) > 31 { // Excel sheet name limit
			sheetName = sheetName[:31]
		}
		datasource.WithSheetName(sheetName)
	}

	// Create Excel exporter with default options
	exporter := excel.NewExcelExporter(nil, nil)

	// Export to Excel
	data, err := exporter.Export(ctx, datasource)
	if err != nil {
		return nil, fmt.Errorf("failed to export to Excel: %w", err)
	}

	// Ensure filename has .xlsx extension
	if filename == "" {
		filename = fmt.Sprintf("export_%s.xlsx", time.Now().Format("20060102_150405"))
	} else if len(filename) < 5 || filename[len(filename)-5:] != ".xlsx" {
		filename += ".xlsx"
	}

	// Create upload DTO
	uploadDTO := &upload.CreateDTO{
		File: bytes.NewReader(data),
		Name: filename,
		Size: len(data),
	}

	// Save to upload service
	uploadEntity, err := s.uploadService.Create(ctx, uploadDTO)
	if err != nil {
		return nil, fmt.Errorf("failed to save Excel file: %w", err)
	}

	return uploadEntity, nil
}

// ExportFromQueryWithOptions exports SQL query results to Excel with custom options
func (s *ExcelExportService) ExportFromQueryWithOptions(
	ctx context.Context,
	query string,
	filename string,
	exportOpts *excel.ExportOptions,
	styleOpts *excel.StyleOptions,
	args ...interface{},
) (upload.Upload, error) {
	// Create pgx data source
	datasource := excel.NewPgxDataSource(s.db, query, args...)
	if filename != "" {
		// Use filename without extension as sheet name
		sheetName := filename
		if len(sheetName) > 31 { // Excel sheet name limit
			sheetName = sheetName[:31]
		}
		datasource.WithSheetName(sheetName)
	}

	// Create Excel exporter with provided options
	exporter := excel.NewExcelExporter(exportOpts, styleOpts)

	// Export to Excel
	data, err := exporter.Export(ctx, datasource)
	if err != nil {
		return nil, fmt.Errorf("failed to export to Excel: %w", err)
	}

	// Ensure filename has .xlsx extension
	if filename == "" {
		filename = fmt.Sprintf("export_%s.xlsx", time.Now().Format("20060102_150405"))
	} else if len(filename) < 5 || filename[len(filename)-5:] != ".xlsx" {
		filename += ".xlsx"
	}

	// Create upload DTO
	uploadDTO := &upload.CreateDTO{
		File: bytes.NewReader(data),
		Name: filename,
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
	filename string,
	exportOpts *excel.ExportOptions,
	styleOpts *excel.StyleOptions,
) (upload.Upload, error) {
	// Create Excel exporter
	exporter := excel.NewExcelExporter(exportOpts, styleOpts)

	// Export to Excel
	data, err := exporter.Export(ctx, datasource)
	if err != nil {
		return nil, fmt.Errorf("failed to export to Excel: %w", err)
	}

	// Ensure filename has .xlsx extension
	if filename == "" {
		filename = fmt.Sprintf("export_%s.xlsx", time.Now().Format("20060102_150405"))
	} else if len(filename) < 5 || filename[len(filename)-5:] != ".xlsx" {
		filename += ".xlsx"
	}

	// Create upload DTO
	uploadDTO := &upload.CreateDTO{
		File: bytes.NewReader(data),
		Name: filename,
		Size: len(data),
	}

	// Save to upload service
	uploadEntity, err := s.uploadService.Create(ctx, uploadDTO)
	if err != nil {
		return nil, fmt.Errorf("failed to save Excel file: %w", err)
	}

	return uploadEntity, nil
}

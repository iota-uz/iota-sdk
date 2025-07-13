package controllers

import (
	"context"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	importpkg "github.com/iota-uz/iota-sdk/pkg/import"
)

// PositionRowHandler implements ExcelRowHandler for warehouse positions
type PositionRowHandler struct {
	positionService *positionservice.PositionService
	errorFactory    importpkg.ErrorFactory
}

// NewPositionRowHandler creates a new position row handler
func NewPositionRowHandler(
	positionService *positionservice.PositionService,
	errorFactory importpkg.ErrorFactory,
) *PositionRowHandler {
	return &PositionRowHandler{
		positionService: positionService,
		errorFactory:    errorFactory,
	}
}

// ExpectedColumnCount returns the expected number of columns
func (h *PositionRowHandler) ExpectedColumnCount() int {
	return 4
}

// GetColumnName returns the name of the column at the given index
func (h *PositionRowHandler) GetColumnName(index int) string {
	switch index {
	case 0:
		return "Item Name"
	case 1:
		return "Item Code"
	case 2:
		return "Unit"
	case 3:
		return "Quantity"
	default:
		return "Unknown"
	}
}

// ValidateRow validates a single row
func (h *PositionRowHandler) ValidateRow(rowIndex int, row []string) error {
	if len(row) < h.ExpectedColumnCount() {
		return h.errorFactory.NewValidationError(
			"Row",
			strconv.Itoa(rowIndex),
			uint(rowIndex),
			"Insufficient columns",
		)
	}

	// Validate required fields
	if strings.TrimSpace(row[0]) == "" {
		return h.errorFactory.NewInvalidCellError("A", uint(rowIndex))
	}

	if strings.TrimSpace(row[1]) == "" {
		return h.errorFactory.NewInvalidCellError("B", uint(rowIndex))
	}

	if strings.TrimSpace(row[2]) == "" {
		return h.errorFactory.NewInvalidCellError("C", uint(rowIndex))
	}

	if strings.TrimSpace(row[3]) == "" {
		return h.errorFactory.NewInvalidCellError("D", uint(rowIndex))
	}

	// Validate quantity is numeric
	if _, err := strconv.ParseFloat(strings.TrimSpace(row[3]), 64); err != nil {
		return h.errorFactory.NewValidationError(
			"D",
			row[3],
			uint(rowIndex),
			"Must be a valid number",
		)
	}

	return nil
}

// ProcessRow processes a validated row
func (h *PositionRowHandler) ProcessRow(ctx context.Context, rowIndex int, row []string) error {
	// Parse quantity to int
	quantity, err := strconv.Atoi(strings.TrimSpace(row[3]))
	if err != nil {
		return h.errorFactory.NewValidationError(
			"D",
			row[3],
			uint(rowIndex),
			"Must be a valid integer",
		)
	}

	// Convert row to XlsRow format that the existing service expects
	xlsRow := &positionservice.XlsRow{
		Title:    strings.TrimSpace(row[0]),
		Barcode:  strings.TrimSpace(row[1]),
		Unit:     strings.ToLower(strings.TrimSpace(row[2])),
		Quantity: quantity,
	}

	// Use the existing position service logic
	return h.positionService.CreatePositionFromXlsRow(ctx, xlsRow)
}

// Package controllers provides this package.
package controllers

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	importpkg "github.com/iota-uz/iota-sdk/pkg/import"
)

// PositionRowHandler implements ExcelRowHandler for warehouse positions
type PositionRowHandler struct {
	positionService *positionservice.PositionService
	errorFactory    importpkg.ErrorFactory
	historyWriter   history.Writer
}

// NewPositionRowHandler creates a new position row handler
func NewPositionRowHandler(
	positionService *positionservice.PositionService,
	errorFactory importpkg.ErrorFactory,
	historyWriter history.Writer,
) *PositionRowHandler {
	return &PositionRowHandler{
		positionService: positionService,
		errorFactory:    errorFactory,
		historyWriter:   historyWriter,
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

// ProcessRow processes a validated row as one import item: the row commits together with its
// history events, and a failed row is rolled back and recorded as a failed import item.
func (h *PositionRowHandler) ProcessRow(ctx context.Context, rowIndex int, row []string) error {
	return history.ImportItem(ctx, h.historyWriter, importRowItem(uint(rowIndex)), func(ctx context.Context) error {
		quantity, err := strconv.Atoi(strings.TrimSpace(row[3]))
		if err != nil {
			return h.errorFactory.NewValidationError(
				"D",
				row[3],
				uint(rowIndex),
				"Must be a valid integer",
			)
		}
		return h.positionService.CreatePositionFromXlsRow(ctx, &positionservice.XlsRow{
			Title:    strings.TrimSpace(row[0]),
			Barcode:  strings.TrimSpace(row[1]),
			Unit:     strings.ToLower(strings.TrimSpace(row[2])),
			Quantity: quantity,
		})
	})
}

// RecordRejectedRows records the rows rejected by validation, before anything was imported, as
// failed import items of the current import operation.
func (h *PositionRowHandler) RecordRejectedRows(ctx context.Context, rowErrors []error) error {
	return composables.InTx(ctx, func(txCtx context.Context) error {
		for _, rowErr := range rowErrors {
			row, ok := importErrorRow(rowErr)
			if !ok {
				continue
			}
			if _, err := h.historyWriter.Record(txCtx, history.Input{
				Type:     history.ImportItemType,
				Version:  1,
				Outcome:  businessevent.OutcomeFailed,
				Subjects: []businessevent.Subject{{Resource: history.ImportItemResource, ID: importRowItem(row)}},
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func importErrorRow(err error) (uint, bool) {
	var cellErr *importpkg.InvalidCellError
	if errors.As(err, &cellErr) {
		return cellErr.Row, true
	}
	var validationErr *importpkg.ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.RowNum, true
	}
	return 0, false
}

func importRowItem(row uint) string {
	return "row:" + strconv.FormatUint(uint64(row), 10)
}

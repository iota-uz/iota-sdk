package mappers

import (
	"fmt"
	category "github.com/iota-agency/iota-erp/internal/domain/aggregates/expense_category"
	"github.com/iota-agency/iota-erp/internal/presentation/viewmodels"
	"strconv"
	"time"
)

func ExpenseCategoryToViewModel(entity *category.ExpenseCategory) *viewmodels.ExpenseCategory {
	return &viewmodels.ExpenseCategory{
		ID:                 strconv.FormatUint(uint64(entity.ID), 10),
		Name:               entity.Name,
		Amount:             fmt.Sprintf("%.2f", entity.Amount),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount, entity.Currency.Symbol.String()),
		Description:        entity.Description,
		UpdatedAt:          entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:          entity.CreatedAt.Format(time.RFC3339),
	}
}

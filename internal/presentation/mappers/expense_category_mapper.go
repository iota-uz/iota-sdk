package mappers

import (
	"fmt"
	category "github.com/iota-agency/iota-erp/internal/domain/aggregates/expense_category"
	"github.com/iota-agency/iota-erp/internal/presentation/view_models"
	"strconv"
	"time"
)

func ExpenseCategoryToViewModel(entity *category.ExpenseCategory) *view_models.ExpenseCategory {
	return &view_models.ExpenseCategory{
		Id:                 strconv.FormatUint(uint64(entity.Id), 10),
		Name:               entity.Name,
		Amount:             fmt.Sprintf("%.2f", entity.Amount),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount, entity.Currency.Symbol.String()),
		Description:        entity.Description,
		UpdatedAt:          entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:          entity.CreatedAt.Format(time.RFC3339),
	}
}

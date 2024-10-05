package mappers

import (
	"fmt"
	category "github.com/iota-agency/iota-erp/internal/domain/entities/expense_category"
	"github.com/iota-agency/iota-erp/internal/presentation/view_models"
	"time"
)

func ExpenseCategoryToViewModel(entity *category.ExpenseCategory) *view_models.ExpenseCategory {
	return &view_models.ExpenseCategory{
		Id:                 fmt.Sprintf("%d", entity.Id),
		Name:               entity.Name,
		Amount:             fmt.Sprintf("%.2f", entity.Amount),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount, entity.Currency.Symbol.String()),
		Description:        entity.Description,
		UpdatedAt:          entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:          entity.CreatedAt.Format(time.RFC3339),
	}
}

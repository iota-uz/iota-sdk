package category

import (
	"github.com/iota-agency/iota-erp/graph/gqlmodels"
	"time"
)

type ExpenseCategory struct {
	Id          int64
	Name        string
	Description *string
	Amount      float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (e *ExpenseCategory) ToGraph() *model.ExpenseCategory {
	return &model.ExpenseCategory{
		ID:          e.Id,
		Name:        e.Name,
		Description: e.Description,
		Amount:      e.Amount,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

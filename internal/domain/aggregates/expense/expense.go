package expense

import (
	"time"

	category "github.com/iota-agency/iota-erp/internal/domain/aggregates/expense_category"
	moneyAccount "github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Expense struct {
	ID               uint
	Amount           float64
	Account          moneyAccount.Account
	Category         category.ExpenseCategory
	Comment          string
	TransactionID    uint
	AccountingPeriod time.Time
	Date             time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (e *Expense) category2Graph() *model.ExpenseCategory {
	return e.Category.ToGraph()
}

func (e *Expense) ToGraph() *model.Expense {
	return &model.Expense{
		ID:         int64(e.ID),
		Amount:     e.Amount,
		CategoryID: int64(e.Category.ID),
		Category:   e.category2Graph(),
		Date:       e.Date,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}

package expense

import (
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/expense_category"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	"github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"time"
)

type Expense struct {
	Id               uint
	Amount           float64
	Account          moneyAccount.Account
	Currency         currency.Currency
	Category         category.ExpenseCategory
	Comment          string
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
		ID:         int64(e.Id),
		Amount:     e.Amount,
		CategoryID: int64(e.Category.Id),
		Category:   e.category2Graph(),
		Date:       e.Date,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}

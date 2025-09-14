package viewmodels

import (
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels" // Import for Upload
)

type Expense struct {
	ID                 string
	Amount             string
	AccountID          string
	AmountWithCurrency string
	CategoryID         string
	Category           *ExpenseCategory
	Comment            string
	TransactionID      string
	AccountingPeriod   string
	Date               string
	CreatedAt          string
	UpdatedAt          string
	Attachments        []*viewmodels.Upload // File attachments
}

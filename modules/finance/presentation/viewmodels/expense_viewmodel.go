package viewmodels

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
}

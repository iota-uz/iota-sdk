package viewmodels

type ExpenseCategory struct {
	ID                 string
	Name               string
	Amount             string
	AmountWithCurrency string
	CurrencyCode       string
	Description        string
	CreatedAt          string
	UpdatedAt          string
}

type MoneyAccount struct {
	ID                  string
	Name                string
	AccountNumber       string
	Description         string
	Balance             string
	BalanceWithCurrency string
	CurrencyCode        string
	CurrencySymbol      string
	CreatedAt           string
	UpdatedAt           string
}

type Payment struct {
	ID                 string
	StageID            string
	Amount             string
	AmountWithCurrency string
	AccountID          string
	TransactionID      string
	TransactionDate    string
	AccountingPeriod   string
	Comment            string
	CreatedAt          string
	UpdatedAt          string
}

type ProjectStage struct {
	ID        string
	Name      string
	ProjectID string
	Margin    string
	Risks     string
	CreatedAt string
	UpdatedAt string
}

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

type Currency struct {
	Code   string
	Name   string
	Symbol string
}
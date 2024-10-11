package viewmodels

type ExpenseCategory struct {
	ID                 string
	Name               string
	Amount             string
	AmountWithCurrency string
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

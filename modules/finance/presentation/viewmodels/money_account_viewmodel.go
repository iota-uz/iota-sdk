package viewmodels

type MoneyAccountCreateDTO struct {
	Name          string
	Description   string
	AccountNumber string
	Balance       string
	CurrencyCode  string
}

type MoneyAccountUpdateDTO struct {
	ID            string
	Name          string
	Description   string
	AccountNumber string
	Balance       string
	CurrencyCode  string
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

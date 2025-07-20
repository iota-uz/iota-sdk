package viewmodels

type Debt struct {
	ID                            string
	Type                          string
	Status                        string
	CounterpartyID                string
	CounterpartyName              string
	OriginalAmount                string
	OriginalAmountWithCurrency    string
	OutstandingAmount             string
	OutstandingAmountWithCurrency string
	Description                   string
	DueDate                       string
	SettlementTransactionID       string
	CreatedAt                     string
	UpdatedAt                     string
}

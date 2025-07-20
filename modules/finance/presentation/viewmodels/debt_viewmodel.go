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

type DebtCounterpartyAggregate struct {
	CounterpartyID             string
	CounterpartyName           string
	TotalReceivable            string
	TotalPayable               string
	TotalOutstandingReceivable string
	TotalOutstandingPayable    string
	NetAmount                  string
	DebtCount                  int
	CurrencyCode               string
}

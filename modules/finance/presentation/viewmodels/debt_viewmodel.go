// Package viewmodels provides this package.
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
	CurrencyCode                  string
	MoneyAccountID                string
	ProjectID                     string
	Description                   string
	DueDate                       string
	SettlementTransactionID       string
	CreatedAt                     string
	UpdatedAt                     string
}

// IsOpen reports whether something is still owed on the debt.
func (d *Debt) IsOpen() bool {
	return d.Status == "PENDING" || d.Status == "PARTIAL"
}

// ProjectOption is a project a debt can be linked to.
type ProjectOption struct {
	ID   string
	Name string
}

// Balance is money of one currency: on the accounts, reserved by open
// payable debts, and available to spend.
type Balance struct {
	OnAccounts string
	Reserved   string
	Available  string
	// Overdrawn is set when reserves exceed what is on the accounts.
	Overdrawn bool
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

package viewmodels

type Payment struct {
	ID                 string
	Amount             string
	AmountWithCurrency string
	AccountID          string
	CounterpartyID     string
	CategoryID         string
	Category           *PaymentCategory
	TransactionID      string
	TransactionDate    string
	AccountingPeriod   string
	Comment            string
	CreatedAt          string
	UpdatedAt          string
}

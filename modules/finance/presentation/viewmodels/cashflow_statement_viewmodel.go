package viewmodels

// CashflowLineItem represents a single line item in the cashflow statement
type CashflowLineItem struct {
	ID                 string
	Name               string
	Amount             string
	AmountWithCurrency string
	Percentage         float64
	Count              int
	MonthlyAmounts     []MonthlyAmount
	MonthlyCounts      []MonthlyCount
}

// MonthlyCount represents the transaction count for a specific month
type MonthlyCount struct {
	Month string
	Count int
}

// CashflowSection represents a section of the cashflow statement
type CashflowSection struct {
	Name                    string
	Inflows                 []CashflowLineItem
	Outflows                []CashflowLineItem
	NetCashFlow             string
	NetCashFlowWithCurrency string
	MonthlyNetCashFlow      []MonthlyAmount
}

// CashflowStatement represents the complete cashflow statement viewmodel
type CashflowStatement struct {
	ID                          string
	AccountID                   string
	AccountName                 string
	Period                      string
	StartDate                   string
	EndDate                     string
	Months                      []string
	StartingBalance             string
	StartingBalanceWithCurrency string
	EndingBalance               string
	EndingBalanceWithCurrency   string
	OperatingActivities         CashflowSection
	InvestingActivities         CashflowSection
	FinancingActivities         CashflowSection
	TotalInflows                string
	TotalInflowsWithCurrency    string
	TotalOutflows               string
	TotalOutflowsWithCurrency   string
	NetCashFlow                 string
	NetCashFlowWithCurrency     string
	MonthlyNetCashFlow          []MonthlyAmount
	IsPositive                  bool
	Currency                    string
	GeneratedAt                 string
}

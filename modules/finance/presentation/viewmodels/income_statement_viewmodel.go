package viewmodels

// IncomeStatementLineItem represents a single line item in the income statement
type IncomeStatementLineItem struct {
	ID                 string
	Name               string
	Amount             string
	AmountWithCurrency string
	Percentage         float64
	MonthlyAmounts     []MonthlyAmount
}

// MonthlyAmount represents the amount for a specific month
type MonthlyAmount struct {
	Month              string
	Amount             string
	AmountWithCurrency string
}

// IncomeStatementSection represents a section of the income statement (Revenue, Expenses)
type IncomeStatementSection struct {
	Title                string
	LineItems            []IncomeStatementLineItem
	Subtotal             string
	SubtotalWithCurrency string
	Percentage           float64
	MonthlySubtotals     []MonthlyAmount
}

// IncomeStatement represents the complete income statement viewmodel
type IncomeStatement struct {
	ID                      string
	Period                  string
	StartDate               string
	EndDate                 string
	Months                  []string
	RevenueSection          IncomeStatementSection
	ExpenseSection          IncomeStatementSection
	GrossProfit             string
	GrossProfitWithCurrency string
	GrossProfitRatio        float64
	MonthlyGrossProfit      []MonthlyAmount
	NetProfit               string
	NetProfitWithCurrency   string
	NetProfitRatio          float64
	MonthlyNetProfit        []MonthlyAmount
	IsProfit                bool
	Currency                string
	GeneratedAt             string
}

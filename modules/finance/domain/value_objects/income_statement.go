package value_objects

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

// IncomeStatementLineItem represents a single line item in the income statement
type IncomeStatementLineItem struct {
	ID         uuid.UUID    `json:"id"`
	Name       string       `json:"name"`
	Amount     *money.Money `json:"amount"`
	Percentage float64      `json:"percentage"`
}

// IncomeStatementSection represents a section of the income statement (e.g., Revenue, Expenses)
type IncomeStatementSection struct {
	Title      string                    `json:"title"`
	LineItems  []IncomeStatementLineItem `json:"lineItems"`
	Subtotal   *money.Money              `json:"subtotal"`
	Percentage float64                   `json:"percentage"`
}

// IncomeStatement represents a complete income statement for a period
type IncomeStatement struct {
	ID               uuid.UUID              `json:"id"`
	TenantID         uuid.UUID              `json:"tenantId"`
	StartDate        time.Time              `json:"startDate"`
	EndDate          time.Time              `json:"endDate"`
	RevenueSection   IncomeStatementSection `json:"revenueSection"`
	ExpenseSection   IncomeStatementSection `json:"expenseSection"`
	GrossProfit      *money.Money           `json:"grossProfit"`
	GrossProfitRatio float64                `json:"grossProfitRatio"`
	NetProfit        *money.Money           `json:"netProfit"`
	NetProfitRatio   float64                `json:"netProfitRatio"`
	Currency         string                 `json:"currency"`
	GeneratedAt      time.Time              `json:"generatedAt"`
}

// NewIncomeStatement creates a new income statement
func NewIncomeStatement(
	tenantID uuid.UUID,
	startDate, endDate time.Time,
	revenueSection, expenseSection IncomeStatementSection,
	currency string,
) *IncomeStatement {
	// Calculate gross profit (for simplicity, assuming revenue - expenses)
	grossProfit := money.New(
		revenueSection.Subtotal.Amount()-expenseSection.Subtotal.Amount(),
		currency,
	)

	// Calculate ratios
	var grossProfitRatio, netProfitRatio float64
	if revenueSection.Subtotal.Amount() > 0 {
		grossProfitRatio = float64(grossProfit.Amount()) / float64(revenueSection.Subtotal.Amount()) * 100
		netProfitRatio = grossProfitRatio // For now, net profit equals gross profit
	}

	return &IncomeStatement{
		ID:               uuid.New(),
		TenantID:         tenantID,
		StartDate:        startDate,
		EndDate:          endDate,
		RevenueSection:   revenueSection,
		ExpenseSection:   expenseSection,
		GrossProfit:      grossProfit,
		GrossProfitRatio: grossProfitRatio,
		NetProfit:        grossProfit, // For now, net profit equals gross profit
		NetProfitRatio:   netProfitRatio,
		Currency:         currency,
		GeneratedAt:      time.Now(),
	}
}

// Period returns a formatted string representing the statement period
func (is *IncomeStatement) Period() string {
	return is.StartDate.Format("Jan 2006") + " - " + is.EndDate.Format("Jan 2006")
}

// IsProfit returns true if the statement shows a profit
func (is *IncomeStatement) IsProfit() bool {
	return is.NetProfit.Amount() > 0
}

// FinancialReportType represents the type of financial report
type FinancialReportType string

const (
	IncomeStatementReport FinancialReportType = "income_statement"
	BalanceSheetReport    FinancialReportType = "balance_sheet"
	CashFlowReport        FinancialReportType = "cash_flow"
)

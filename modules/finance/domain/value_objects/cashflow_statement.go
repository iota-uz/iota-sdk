package value_objects

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

// CashflowStatement represents a complete cashflow statement with operating, investing, and financing activities
type CashflowStatement struct {
	AccountID       uuid.UUID
	StartDate       time.Time
	EndDate         time.Time
	Currency        string
	StartingBalance *money.Money
	EndingBalance   *money.Money
	NetCashFlow     *money.Money

	// Main cashflow sections
	OperatingActivities CashflowSection
	InvestingActivities CashflowSection
	FinancingActivities CashflowSection

	// Summary
	TotalInflows  *money.Money
	TotalOutflows *money.Money
}

// CashflowSection represents a section of the cashflow statement (Operating, Investing, or Financing)
type CashflowSection struct {
	Name        string
	Inflows     []CashflowLineItem
	Outflows    []CashflowLineItem
	NetCashFlow *money.Money
}

// CashflowLineItem represents a single line item in a cashflow section
type CashflowLineItem struct {
	CategoryID   uuid.UUID
	CategoryName string
	Amount       *money.Money
	Percentage   float64 // Percentage of total inflows or outflows
	Count        int     // Number of transactions
}

// CashflowType represents different types of cashflow
type CashflowType string

const (
	CashflowTypeOperating CashflowType = "operating"
	CashflowTypeInvesting CashflowType = "investing"
	CashflowTypeFinancing CashflowType = "financing"
)

// NewCashflowStatement creates a new cashflow statement
func NewCashflowStatement(
	accountID uuid.UUID,
	startDate, endDate time.Time,
	currency string,
	startingBalance, endingBalance *money.Money,
) *CashflowStatement {
	netCashFlow, _ := endingBalance.Subtract(startingBalance)
	return &CashflowStatement{
		AccountID:       accountID,
		StartDate:       startDate,
		EndDate:         endDate,
		Currency:        currency,
		StartingBalance: startingBalance,
		EndingBalance:   endingBalance,
		NetCashFlow:     netCashFlow,
	}
}

// CalculateTotals calculates total inflows, outflows, and net cashflow
func (cs *CashflowStatement) CalculateTotals() {
	totalInflows := money.New(0, cs.Currency)
	totalOutflows := money.New(0, cs.Currency)

	// Operating activities
	for _, item := range cs.OperatingActivities.Inflows {
		totalInflows, _ = totalInflows.Add(item.Amount)
	}
	for _, item := range cs.OperatingActivities.Outflows {
		totalOutflows, _ = totalOutflows.Add(item.Amount)
	}

	// Investing activities
	for _, item := range cs.InvestingActivities.Inflows {
		totalInflows, _ = totalInflows.Add(item.Amount)
	}
	for _, item := range cs.InvestingActivities.Outflows {
		totalOutflows, _ = totalOutflows.Add(item.Amount)
	}

	// Financing activities
	for _, item := range cs.FinancingActivities.Inflows {
		totalInflows, _ = totalInflows.Add(item.Amount)
	}
	for _, item := range cs.FinancingActivities.Outflows {
		totalOutflows, _ = totalOutflows.Add(item.Amount)
	}

	cs.TotalInflows = totalInflows
	cs.TotalOutflows = totalOutflows
	cs.NetCashFlow, _ = totalInflows.Subtract(totalOutflows)

	// Calculate section net cashflows
	cs.OperatingActivities.NetCashFlow = cs.calculateSectionNetCashflow(cs.OperatingActivities)
	cs.InvestingActivities.NetCashFlow = cs.calculateSectionNetCashflow(cs.InvestingActivities)
	cs.FinancingActivities.NetCashFlow = cs.calculateSectionNetCashflow(cs.FinancingActivities)
}

func (cs *CashflowStatement) calculateSectionNetCashflow(section CashflowSection) *money.Money {
	inflows := money.New(0, cs.Currency)
	outflows := money.New(0, cs.Currency)

	for _, item := range section.Inflows {
		inflows, _ = inflows.Add(item.Amount)
	}
	for _, item := range section.Outflows {
		outflows, _ = outflows.Add(item.Amount)
	}

	netCashflow, _ := inflows.Subtract(outflows)
	return netCashflow
}

// CalculatePercentages calculates the percentage of each line item relative to total inflows/outflows
func (cs *CashflowStatement) CalculatePercentages() {
	if cs.TotalInflows.Amount() == 0 && cs.TotalOutflows.Amount() == 0 {
		return
	}

	// Operating activities
	cs.calculateSectionPercentages(&cs.OperatingActivities, cs.TotalInflows, cs.TotalOutflows)

	// Investing activities
	cs.calculateSectionPercentages(&cs.InvestingActivities, cs.TotalInflows, cs.TotalOutflows)

	// Financing activities
	cs.calculateSectionPercentages(&cs.FinancingActivities, cs.TotalInflows, cs.TotalOutflows)
}

func (cs *CashflowStatement) calculateSectionPercentages(section *CashflowSection, totalInflows, totalOutflows *money.Money) {
	// Calculate inflow percentages
	if totalInflows.Amount() > 0 {
		for i := range section.Inflows {
			section.Inflows[i].Percentage = float64(section.Inflows[i].Amount.Amount()) / float64(totalInflows.Amount()) * 100
		}
	}

	// Calculate outflow percentages
	if totalOutflows.Amount() > 0 {
		for i := range section.Outflows {
			section.Outflows[i].Percentage = float64(section.Outflows[i].Amount.Amount()) / float64(totalOutflows.Amount()) * 100
		}
	}
}

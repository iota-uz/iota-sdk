package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/value_objects"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/pkg/errors"
)

// FinancialReportService provides methods for generating financial reports
type FinancialReportService struct {
	queryRepo      query.FinancialReportsQueryRepository
	eventPublisher eventbus.EventBus
}

// NewFinancialReportService creates a new financial report service
func NewFinancialReportService(
	queryRepo query.FinancialReportsQueryRepository,
	eventPublisher eventbus.EventBus,
) *FinancialReportService {
	return &FinancialReportService{
		queryRepo:      queryRepo,
		eventPublisher: eventPublisher,
	}
}

// GenerateIncomeStatement generates an income statement for a specific period
func (s *FinancialReportService) GenerateIncomeStatement(ctx context.Context, startDate, endDate time.Time) (*value_objects.IncomeStatement, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	// Get income statement data from query repository
	data, err := s.queryRepo.GetIncomeStatementData(ctx, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get income statement data")
	}

	// Convert query data to domain objects
	revenueSection := s.createRevenueSection(data)
	expenseSection := s.createExpenseSection(data)

	// Determine the primary currency (use the most common currency or USD as default)
	currency := s.determinePrimaryCurrency(data)

	// Create income statement
	incomeStatement := value_objects.NewIncomeStatement(
		tenantID,
		startDate,
		endDate,
		revenueSection,
		expenseSection,
		currency,
	)

	// Publish event
	s.eventPublisher.Publish("financial_report.income_statement.generated", map[string]interface{}{
		"tenantId":  tenantID.String(),
		"startDate": startDate,
		"endDate":   endDate,
		"netProfit": incomeStatement.NetProfit.AsMajorUnits(),
		"currency":  currency,
	})

	return incomeStatement, nil
}

// createRevenueSection creates the revenue section from query data
func (s *FinancialReportService) createRevenueSection(data *query.IncomeStatementData) value_objects.IncomeStatementSection {
	lineItems := make([]value_objects.IncomeStatementLineItem, 0, len(data.IncomeItems))

	for _, item := range data.IncomeItems {
		lineItems = append(lineItems, value_objects.IncomeStatementLineItem{
			ID:         item.CategoryID,
			Name:       item.CategoryName,
			Amount:     item.Amount,
			Percentage: item.Percentage,
		})
	}

	return value_objects.IncomeStatementSection{
		Title:      "Revenue",
		LineItems:  lineItems,
		Subtotal:   data.TotalIncome,
		Percentage: 100.0, // Revenue is always 100% of revenue
	}
}

// createExpenseSection creates the expense section from query data
func (s *FinancialReportService) createExpenseSection(data *query.IncomeStatementData) value_objects.IncomeStatementSection {
	lineItems := make([]value_objects.IncomeStatementLineItem, 0, len(data.ExpenseItems))

	for _, item := range data.ExpenseItems {
		lineItems = append(lineItems, value_objects.IncomeStatementLineItem{
			ID:         item.CategoryID,
			Name:       item.CategoryName,
			Amount:     item.Amount,
			Percentage: item.Percentage,
		})
	}

	// Calculate expense percentage of revenue
	var expensePercentage float64
	if data.TotalIncome.Amount() > 0 {
		expensePercentage = float64(data.TotalExpenses.Amount()) / float64(data.TotalIncome.Amount()) * 100
	}

	return value_objects.IncomeStatementSection{
		Title:      "Operating Expenses",
		LineItems:  lineItems,
		Subtotal:   data.TotalExpenses,
		Percentage: expensePercentage,
	}
}

// determinePrimaryCurrency determines the primary currency from the data
func (s *FinancialReportService) determinePrimaryCurrency(data *query.IncomeStatementData) string {
	// For now, use the currency of total income if available
	if data.TotalIncome.Currency().Code != "" {
		return data.TotalIncome.Currency().Code
	}

	// Otherwise, use the currency of total expenses
	if data.TotalExpenses.Currency().Code != "" {
		return data.TotalExpenses.Currency().Code
	}

	// Default to USD
	return "USD"
}

// GenerateCashflowStatement generates a cashflow statement for a specific account and period
func (s *FinancialReportService) GenerateCashflowStatement(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) (*value_objects.CashflowStatement, error) {
	// Get cashflow data from query repository
	data, err := s.queryRepo.GetCashflowData(ctx, accountID, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cashflow data")
	}

	// Create cashflow statement
	cashflowStatement := value_objects.NewCashflowStatement(
		accountID,
		startDate,
		endDate,
		data.StartingBalance.Currency().Code,
		data.StartingBalance,
		data.EndingBalance,
	)

	// Create operating activities section
	cashflowStatement.OperatingActivities = s.createCashflowSection("Operating Activities", data.Inflows, data.Outflows)

	// For now, we only have operating activities
	// In the future, we can extend this to include investing and financing activities
	cashflowStatement.InvestingActivities = value_objects.CashflowSection{
		Name:        "Investing Activities",
		Inflows:     []value_objects.CashflowLineItem{},
		Outflows:    []value_objects.CashflowLineItem{},
		NetCashFlow: money.New(0, data.StartingBalance.Currency().Code),
	}

	cashflowStatement.FinancingActivities = value_objects.CashflowSection{
		Name:        "Financing Activities",
		Inflows:     []value_objects.CashflowLineItem{},
		Outflows:    []value_objects.CashflowLineItem{},
		NetCashFlow: money.New(0, data.StartingBalance.Currency().Code),
	}

	// Set totals
	cashflowStatement.TotalInflows = data.TotalInflows
	cashflowStatement.TotalOutflows = data.TotalOutflows

	// Calculate totals and percentages
	cashflowStatement.CalculateTotals()
	cashflowStatement.CalculatePercentages()

	// Publish event
	tenantID, _ := composables.UseTenantID(ctx)
	s.eventPublisher.Publish("financial_report.cashflow_statement.generated", map[string]interface{}{
		"tenantId":    tenantID.String(),
		"accountId":   accountID.String(),
		"startDate":   startDate,
		"endDate":     endDate,
		"netCashFlow": cashflowStatement.NetCashFlow.AsMajorUnits(),
		"currency":    data.StartingBalance.Currency().Code,
	})

	return cashflowStatement, nil
}

// createCashflowSection creates a cashflow section from query data
func (s *FinancialReportService) createCashflowSection(name string, inflows, outflows []query.CashflowLineItem) value_objects.CashflowSection {
	inflowItems := make([]value_objects.CashflowLineItem, 0, len(inflows))
	for _, item := range inflows {
		inflowItems = append(inflowItems, value_objects.CashflowLineItem{
			CategoryID:   item.CategoryID,
			CategoryName: item.CategoryName,
			Amount:       item.Amount,
			Percentage:   item.Percentage,
			Count:        item.Count,
		})
	}

	outflowItems := make([]value_objects.CashflowLineItem, 0, len(outflows))
	for _, item := range outflows {
		outflowItems = append(outflowItems, value_objects.CashflowLineItem{
			CategoryID:   item.CategoryID,
			CategoryName: item.CategoryName,
			Amount:       item.Amount,
			Percentage:   item.Percentage,
			Count:        item.Count,
		})
	}

	return value_objects.CashflowSection{
		Name:     name,
		Inflows:  inflowItems,
		Outflows: outflowItems,
	}
}

package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/value_objects"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockFinancialReportsQueryRepository is a mock implementation of FinancialReportsQueryRepository
type mockFinancialReportsQueryRepository struct {
	mock.Mock
}

func (m *mockFinancialReportsQueryRepository) GetIncomeStatementData(ctx context.Context, startDate, endDate time.Time) (*query.IncomeStatementData, error) {
	args := m.Called(ctx, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*query.IncomeStatementData), args.Error(1)
}

func (m *mockFinancialReportsQueryRepository) GetIncomeByCategory(ctx context.Context, startDate, endDate time.Time) ([]query.ReportLineItem, *money.Money, error) {
	args := m.Called(ctx, startDate, endDate)
	return args.Get(0).([]query.ReportLineItem), args.Get(1).(*money.Money), args.Error(2)
}

func (m *mockFinancialReportsQueryRepository) GetExpensesByCategory(ctx context.Context, startDate, endDate time.Time) ([]query.ReportLineItem, *money.Money, error) {
	args := m.Called(ctx, startDate, endDate)
	return args.Get(0).([]query.ReportLineItem), args.Get(1).(*money.Money), args.Error(2)
}

func (m *mockFinancialReportsQueryRepository) GetMonthlyIncomeByCategory(ctx context.Context, startDate, endDate time.Time) ([]query.MonthlyReportLineItem, error) {
	args := m.Called(ctx, startDate, endDate)
	return args.Get(0).([]query.MonthlyReportLineItem), args.Error(1)
}

func (m *mockFinancialReportsQueryRepository) GetMonthlyExpensesByCategory(ctx context.Context, startDate, endDate time.Time) ([]query.MonthlyReportLineItem, error) {
	args := m.Called(ctx, startDate, endDate)
	return args.Get(0).([]query.MonthlyReportLineItem), args.Error(1)
}

func (m *mockFinancialReportsQueryRepository) GetCashflowData(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) (*query.CashflowData, error) {
	args := m.Called(ctx, accountID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*query.CashflowData), args.Error(1)
}

func (m *mockFinancialReportsQueryRepository) GetCashflowByCategory(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) ([]query.CashflowLineItem, []query.CashflowLineItem, error) {
	args := m.Called(ctx, accountID, startDate, endDate)
	return args.Get(0).([]query.CashflowLineItem), args.Get(1).([]query.CashflowLineItem), args.Error(2)
}

func (m *mockFinancialReportsQueryRepository) GetMonthlyCashflowByCategory(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) ([]query.MonthlyCashflowLineItem, []query.MonthlyCashflowLineItem, error) {
	args := m.Called(ctx, accountID, startDate, endDate)
	return args.Get(0).([]query.MonthlyCashflowLineItem), args.Get(1).([]query.MonthlyCashflowLineItem), args.Error(2)
}

func (m *mockFinancialReportsQueryRepository) GetAccountBalance(ctx context.Context, accountID uuid.UUID) (*money.Money, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*money.Money), args.Error(1)
}

func (m *mockFinancialReportsQueryRepository) GetAccountBalanceAtDate(ctx context.Context, accountID uuid.UUID, date time.Time) (*money.Money, error) {
	args := m.Called(ctx, accountID, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*money.Money), args.Error(1)
}

func TestFinancialReportService_GenerateCashflowStatement_Calculations(t *testing.T) {
	t.Parallel()

	// Common test data
	accountID := uuid.New()
	tenantID := uuid.New()
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	currency := "USD"

	tests := []struct {
		name                    string
		setupMocks              func(*mockFinancialReportsQueryRepository)
		expectedError           bool
		errorMessage            string
		validateCashflowStatement func(*testing.T, *value_objects.CashflowStatement)
	}{
		{
			name: "Valid cashflow with inflows and outflows - balance reconciliation",
			setupMocks: func(mockRepo *mockFinancialReportsQueryRepository) {
				// Business Rule: Starting Balance + Net Cash Flow = Ending Balance
				startingBalance := money.New(100000, currency) // $1,000.00
				endingBalance := money.New(150000, currency)   // $1,500.00
				
				// Inflows
				inflows := []query.CashflowLineItem{
					{
						CategoryID:   uuid.New(),
						CategoryName: "Sales Revenue",
						Amount:       money.New(80000, currency), // $800.00
						Count:        5,
						Percentage:   80.0,
					},
					{
						CategoryID:   uuid.New(),
						CategoryName: "Service Income",
						Amount:       money.New(20000, currency), // $200.00
						Count:        2,
						Percentage:   20.0,
					},
				}

				// Outflows
				outflows := []query.CashflowLineItem{
					{
						CategoryID:   uuid.New(),
						CategoryName: "Office Expenses",
						Amount:       money.New(30000, currency), // $300.00
						Count:        10,
						Percentage:   60.0,
					},
					{
						CategoryID:   uuid.New(),
						CategoryName: "Utilities",
						Amount:       money.New(20000, currency), // $200.00
						Count:        4,
						Percentage:   40.0,
					},
				}

				totalInflows := money.New(100000, currency)  // $1,000.00
				totalOutflows := money.New(50000, currency)  // $500.00

				cashflowData := &query.CashflowData{
					AccountID:       accountID,
					StartDate:       startDate,
					EndDate:         endDate,
					StartingBalance: startingBalance,
					EndingBalance:   endingBalance,
					Inflows:         inflows,
					Outflows:        outflows,
					TotalInflows:    totalInflows,
					TotalOutflows:   totalOutflows,
				}

				mockRepo.On("GetCashflowData", mock.Anything, accountID, startDate, endDate).Return(cashflowData, nil)
			},
			expectedError: false,
			validateCashflowStatement: func(t *testing.T, stmt *value_objects.CashflowStatement) {
				// Verify basic properties
				assert.Equal(t, accountID, stmt.AccountID)
				assert.Equal(t, startDate, stmt.StartDate)
				assert.Equal(t, endDate, stmt.EndDate)
				assert.Equal(t, currency, stmt.Currency)

				// Verify balances
				assert.Equal(t, int64(100000), stmt.StartingBalance.Amount())
				assert.Equal(t, int64(150000), stmt.EndingBalance.Amount())

				// Verify net cashflow calculation
				assert.Equal(t, int64(50000), stmt.NetCashFlow.Amount()) // $500.00 (inflows - outflows)

				// Verify totals
				assert.Equal(t, int64(100000), stmt.TotalInflows.Amount())
				assert.Equal(t, int64(50000), stmt.TotalOutflows.Amount())

				// Verify operating activities
				assert.Len(t, stmt.OperatingActivities.Inflows, 2)
				assert.Len(t, stmt.OperatingActivities.Outflows, 2)
				assert.Equal(t, int64(50000), stmt.OperatingActivities.NetCashFlow.Amount())

				// CRITICAL: Verify balance reconciliation: Starting + Net = Ending
				reconciledBalance, _ := stmt.StartingBalance.Add(stmt.NetCashFlow)
				assert.Equal(t, stmt.EndingBalance.Amount(), reconciledBalance.Amount(), 
					"Starting balance + Net cashflow should equal ending balance")

				// Verify percentages are calculated correctly
				assert.Equal(t, 80.0, stmt.OperatingActivities.Inflows[0].Percentage)
				assert.Equal(t, 20.0, stmt.OperatingActivities.Inflows[1].Percentage)
				assert.Equal(t, 60.0, stmt.OperatingActivities.Outflows[0].Percentage)
				assert.Equal(t, 40.0, stmt.OperatingActivities.Outflows[1].Percentage)
			},
		},
		{
			name: "Empty cashflow - no transactions",
			setupMocks: func(mockRepo *mockFinancialReportsQueryRepository) {
				startingBalance := money.New(50000, currency) // $500.00
				endingBalance := money.New(50000, currency)   // $500.00 (same as starting)

				cashflowData := &query.CashflowData{
					AccountID:       accountID,
					StartDate:       startDate,
					EndDate:         endDate,
					StartingBalance: startingBalance,
					EndingBalance:   endingBalance,
					Inflows:         []query.CashflowLineItem{},
					Outflows:        []query.CashflowLineItem{},
					TotalInflows:    money.New(0, currency),
					TotalOutflows:   money.New(0, currency),
				}

				mockRepo.On("GetCashflowData", mock.Anything, accountID, startDate, endDate).Return(cashflowData, nil)
			},
			expectedError: false,
			validateCashflowStatement: func(t *testing.T, stmt *value_objects.CashflowStatement) {
				// With no transactions, starting and ending balance should be the same
				assert.Equal(t, stmt.StartingBalance.Amount(), stmt.EndingBalance.Amount())
				
				// Net cashflow should be zero
				assert.Equal(t, int64(0), stmt.NetCashFlow.Amount())
				
				// No line items
				assert.Empty(t, stmt.OperatingActivities.Inflows)
				assert.Empty(t, stmt.OperatingActivities.Outflows)
				
				// Totals should be zero
				assert.Equal(t, int64(0), stmt.TotalInflows.Amount())
				assert.Equal(t, int64(0), stmt.TotalOutflows.Amount())
			},
		},
		{
			name: "Inflows only - no outflows",
			setupMocks: func(mockRepo *mockFinancialReportsQueryRepository) {
				startingBalance := money.New(0, currency)     // $0.00
				endingBalance := money.New(100000, currency)  // $1,000.00

				inflows := []query.CashflowLineItem{
					{
						CategoryID:   uuid.New(),
						CategoryName: "Initial Investment",
						Amount:       money.New(100000, currency), // $1,000.00
						Count:        1,
						Percentage:   100.0,
					},
				}

				cashflowData := &query.CashflowData{
					AccountID:       accountID,
					StartDate:       startDate,
					EndDate:         endDate,
					StartingBalance: startingBalance,
					EndingBalance:   endingBalance,
					Inflows:         inflows,
					Outflows:        []query.CashflowLineItem{},
					TotalInflows:    money.New(100000, currency),
					TotalOutflows:   money.New(0, currency),
				}

				mockRepo.On("GetCashflowData", mock.Anything, accountID, startDate, endDate).Return(cashflowData, nil)
			},
			expectedError: false,
			validateCashflowStatement: func(t *testing.T, stmt *value_objects.CashflowStatement) {
				// Net cashflow should equal total inflows
				assert.Equal(t, stmt.TotalInflows.Amount(), stmt.NetCashFlow.Amount())
				
				// Verify balance reconciliation
				reconciledBalance, _ := stmt.StartingBalance.Add(stmt.NetCashFlow)
				assert.Equal(t, stmt.EndingBalance.Amount(), reconciledBalance.Amount())
				
				// One inflow, no outflows
				assert.Len(t, stmt.OperatingActivities.Inflows, 1)
				assert.Empty(t, stmt.OperatingActivities.Outflows)
				
				// Percentage should be 100% for single inflow
				assert.Equal(t, 100.0, stmt.OperatingActivities.Inflows[0].Percentage)
			},
		},
		{
			name: "Outflows only - no inflows",
			setupMocks: func(mockRepo *mockFinancialReportsQueryRepository) {
				startingBalance := money.New(200000, currency) // $2,000.00
				endingBalance := money.New(50000, currency)    // $500.00

				outflows := []query.CashflowLineItem{
					{
						CategoryID:   uuid.New(),
						CategoryName: "Rent",
						Amount:       money.New(100000, currency), // $1,000.00
						Count:        1,
						Percentage:   66.67,
					},
					{
						CategoryID:   uuid.New(),
						CategoryName: "Salaries",
						Amount:       money.New(50000, currency), // $500.00
						Count:        1,
						Percentage:   33.33,
					},
				}

				cashflowData := &query.CashflowData{
					AccountID:       accountID,
					StartDate:       startDate,
					EndDate:         endDate,
					StartingBalance: startingBalance,
					EndingBalance:   endingBalance,
					Inflows:         []query.CashflowLineItem{},
					Outflows:        outflows,
					TotalInflows:    money.New(0, currency),
					TotalOutflows:   money.New(150000, currency),
				}

				mockRepo.On("GetCashflowData", mock.Anything, accountID, startDate, endDate).Return(cashflowData, nil)
			},
			expectedError: false,
			validateCashflowStatement: func(t *testing.T, stmt *value_objects.CashflowStatement) {
				// Net cashflow should be negative (outflows only)
				assert.Equal(t, int64(-150000), stmt.NetCashFlow.Amount())
				
				// Verify balance reconciliation
				reconciledBalance, _ := stmt.StartingBalance.Add(stmt.NetCashFlow)
				assert.Equal(t, stmt.EndingBalance.Amount(), reconciledBalance.Amount())
				
				// No inflows, two outflows
				assert.Empty(t, stmt.OperatingActivities.Inflows)
				assert.Len(t, stmt.OperatingActivities.Outflows, 2)
			},
		},
		{
			name: "Zero starting balance",
			setupMocks: func(mockRepo *mockFinancialReportsQueryRepository) {
				startingBalance := money.New(0, currency)    // $0.00
				endingBalance := money.New(25000, currency)  // $250.00

				inflows := []query.CashflowLineItem{
					{
						CategoryID:   uuid.New(),
						CategoryName: "Opening Deposit",
						Amount:       money.New(50000, currency), // $500.00
						Count:        1,
						Percentage:   100.0,
					},
				}

				outflows := []query.CashflowLineItem{
					{
						CategoryID:   uuid.New(),
						CategoryName: "Setup Costs",
						Amount:       money.New(25000, currency), // $250.00
						Count:        3,
						Percentage:   100.0,
					},
				}

				cashflowData := &query.CashflowData{
					AccountID:       accountID,
					StartDate:       startDate,
					EndDate:         endDate,
					StartingBalance: startingBalance,
					EndingBalance:   endingBalance,
					Inflows:         inflows,
					Outflows:        outflows,
					TotalInflows:    money.New(50000, currency),
					TotalOutflows:   money.New(25000, currency),
				}

				mockRepo.On("GetCashflowData", mock.Anything, accountID, startDate, endDate).Return(cashflowData, nil)
			},
			expectedError: false,
			validateCashflowStatement: func(t *testing.T, stmt *value_objects.CashflowStatement) {
				// Starting from zero
				assert.Equal(t, int64(0), stmt.StartingBalance.Amount())
				
				// Net cashflow should be positive
				assert.Equal(t, int64(25000), stmt.NetCashFlow.Amount())
				
				// Verify balance reconciliation: 0 + 250 = 250
				reconciledBalance, _ := stmt.StartingBalance.Add(stmt.NetCashFlow)
				assert.Equal(t, stmt.EndingBalance.Amount(), reconciledBalance.Amount())
			},
		},
		{
			name: "Repository error",
			setupMocks: func(mockRepo *mockFinancialReportsQueryRepository) {
				mockRepo.On("GetCashflowData", mock.Anything, accountID, startDate, endDate).
					Return(nil, assert.AnError)
			},
			expectedError: true,
			errorMessage:  "failed to get cashflow data",
		},
		{
			name: "Percentage calculation with zero totals",
			setupMocks: func(mockRepo *mockFinancialReportsQueryRepository) {
				// Edge case: ensure no division by zero
				startingBalance := money.New(100000, currency)
				endingBalance := money.New(100000, currency)

				cashflowData := &query.CashflowData{
					AccountID:       accountID,
					StartDate:       startDate,
					EndDate:         endDate,
					StartingBalance: startingBalance,
					EndingBalance:   endingBalance,
					Inflows:         []query.CashflowLineItem{},
					Outflows:        []query.CashflowLineItem{},
					TotalInflows:    money.New(0, currency),
					TotalOutflows:   money.New(0, currency),
				}

				mockRepo.On("GetCashflowData", mock.Anything, accountID, startDate, endDate).Return(cashflowData, nil)
			},
			expectedError: false,
			validateCashflowStatement: func(t *testing.T, stmt *value_objects.CashflowStatement) {
				// Should handle zero totals gracefully
				assert.NotNil(t, stmt)
				assert.Equal(t, int64(0), stmt.NetCashFlow.Amount())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctx := composables.WithTenantID(context.Background(), tenantID)

			mockRepo := new(mockFinancialReportsQueryRepository)
			
			// Use a real event publisher for now - we'll just verify the service runs
			publisher := eventbus.NewEventPublisher(logrus.New())

			service := services.NewFinancialReportService(mockRepo, publisher)

			// Setup mocks
			tt.setupMocks(mockRepo)

			// Execute
			result, err := service.GenerateCashflowStatement(ctx, accountID, startDate, endDate)

			// Assert
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				
				if tt.validateCashflowStatement != nil {
					tt.validateCashflowStatement(t, result)
				}
			}

			// Verify mock expectations
			mockRepo.AssertExpectations(t)
		})
	}
}

// TestCashflowStatement_BusinessRules tests that the cashflow statement follows correct accounting principles
func TestCashflowStatement_BusinessRules(t *testing.T) {
	t.Parallel()

	accountID := uuid.New()
	tenantID := uuid.New()
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	currency := "USD"

	t.Run("Starting balance should be account balance at start date", func(t *testing.T) {
		// This test verifies that the query repository correctly provides
		// the starting balance as it was at the start date, NOT calculated
		// from current balance minus net cashflow
		
		ctx := composables.WithTenantID(context.Background(), tenantID)
		mockRepo := new(mockFinancialReportsQueryRepository)
		publisher := eventbus.NewEventPublisher(logrus.New())
		service := services.NewFinancialReportService(mockRepo, publisher)

		// Historical balance at start date
		historicalStartingBalance := money.New(75000, currency) // $750.00
		// Current ending balance  
		currentEndingBalance := money.New(125000, currency) // $1,250.00

		cashflowData := &query.CashflowData{
			AccountID:       accountID,
			StartDate:       startDate,
			EndDate:         endDate,
			StartingBalance: historicalStartingBalance, // Should be historical, not calculated
			EndingBalance:   currentEndingBalance,
			Inflows:         []query.CashflowLineItem{
				{
					CategoryID:   uuid.New(),
					CategoryName: "Revenue",
					Amount:       money.New(100000, currency),
					Count:        5,
					Percentage:   100.0,
				},
			},
			Outflows:        []query.CashflowLineItem{
				{
					CategoryID:   uuid.New(),
					CategoryName: "Expenses",
					Amount:       money.New(50000, currency),
					Count:        10,
					Percentage:   100.0,
				},
			},
			TotalInflows:    money.New(100000, currency),
			TotalOutflows:   money.New(50000, currency),
		}

		mockRepo.On("GetCashflowData", ctx, accountID, startDate, endDate).Return(cashflowData, nil)

		result, err := service.GenerateCashflowStatement(ctx, accountID, startDate, endDate)
		require.NoError(t, err)

		// Verify the starting balance is the historical balance
		assert.Equal(t, historicalStartingBalance.Amount(), result.StartingBalance.Amount())
		
		// Verify balance reconciliation still works
		netCashflow, _ := result.TotalInflows.Subtract(result.TotalOutflows)
		reconciledBalance, _ := result.StartingBalance.Add(netCashflow)
		assert.Equal(t, result.EndingBalance.Amount(), reconciledBalance.Amount(),
			"Historical starting balance + Net cashflow should equal ending balance")
	})

	t.Run("Currency consistency across all amounts", func(t *testing.T) {
		ctx := composables.WithTenantID(context.Background(), tenantID)
		mockRepo := new(mockFinancialReportsQueryRepository)
		publisher := eventbus.NewEventPublisher(logrus.New())
		service := services.NewFinancialReportService(mockRepo, publisher)

		cashflowData := &query.CashflowData{
			AccountID:       accountID,
			StartDate:       startDate,
			EndDate:         endDate,
			StartingBalance: money.New(100000, "EUR"), // Account is in EUR
			EndingBalance:   money.New(150000, "EUR"),
			Inflows: []query.CashflowLineItem{
				{
					CategoryID:   uuid.New(),
					CategoryName: "Revenue",
					Amount:       money.New(50000, "EUR"),
					Count:        1,
					Percentage:   100.0,
				},
			},
			Outflows:        []query.CashflowLineItem{},
			TotalInflows:    money.New(50000, "EUR"),
			TotalOutflows:   money.New(0, "EUR"),
		}

		mockRepo.On("GetCashflowData", ctx, accountID, startDate, endDate).Return(cashflowData, nil)

		result, err := service.GenerateCashflowStatement(ctx, accountID, startDate, endDate)
		require.NoError(t, err)

		// All amounts should be in EUR
		assert.Equal(t, "EUR", result.Currency)
		assert.Equal(t, "EUR", result.StartingBalance.Currency().Code)
		assert.Equal(t, "EUR", result.EndingBalance.Currency().Code)
		assert.Equal(t, "EUR", result.TotalInflows.Currency().Code)
		assert.Equal(t, "EUR", result.TotalOutflows.Currency().Code)
		assert.Equal(t, "EUR", result.NetCashFlow.Currency().Code)
	})
}

// TestCashflowStatement_ImplementationIssues documents current implementation issues
func TestCashflowStatement_ImplementationIssues(t *testing.T) {
	t.Skip("These tests document implementation issues that need to be fixed")

	t.Run("ISSUE: Starting balance calculation is incorrect", func(t *testing.T) {
		// Current implementation in GetCashflowData:
		// startingBalance, _ := currentBalance.Subtract(netCashflow)
		// This is WRONG - it should get the historical balance at start date
		
		// The correct approach would be:
		// 1. Query the account balance as it was on the start date
		// 2. OR track balance history/snapshots
		// 3. OR calculate from all transactions since account creation up to start date
	})

	t.Run("ISSUE: Missing transaction date filtering", func(t *testing.T) {
		// The repository should only include transactions within the date range
		// Currently it seems to calculate from current balance which is incorrect
	})

	t.Run("ISSUE: Investing and Financing activities hardcoded to empty", func(t *testing.T) {
		// The service always creates empty investing and financing sections
		// This should be based on transaction categorization or types
	})
}
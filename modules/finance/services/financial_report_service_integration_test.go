package services_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	expensecategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper functions to get services
func getExpenseService(env *itf.TestEnvironment) *services.ExpenseService {
	return env.Service(services.ExpenseService{}).(*services.ExpenseService)
}

func getExpenseCategoryService(env *itf.TestEnvironment) *services.ExpenseCategoryService {
	return env.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService)
}

func TestFinancialReportService_CashflowStatement_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Run("Basic cashflow calculation with real database", func(t *testing.T) {
		// Setup test environment with permissions
		env := setupTest(t, permissions.PaymentRead, permissions.ExpenseRead)
		ctx := env.Ctx

		// Get services
		moneyAccountService := getAccountService(env)
		paymentService := getPaymentService(env)
		expenseService := getExpenseService(env)
		paymentCategoryService := getPaymentCategoryService(env)
		expenseCategoryService := getExpenseCategoryService(env)
		reportService := getFinancialReportService(env)

		// Get tenant ID
		tenantID, err := composables.UseTenantID(ctx)
		require.NoError(t, err)

		// Create a money account starting with zero
		account := moneyaccount.New(
			"Test Cash Account",
			money.New(0, "USD"),
		)
		account, err = moneyAccountService.Create(ctx, account)
		require.NoError(t, err)

		// Create categories
		paymentCat := paymentcategory.New("Sales Revenue", paymentcategory.WithTenantID(tenantID))
		paymentCat, err = paymentCategoryService.Create(ctx, paymentCat)
		require.NoError(t, err)

		expenseCat := expensecategory.New("Office Expenses", expensecategory.WithTenantID(tenantID))
		expenseCat, err = expenseCategoryService.Create(ctx, expenseCat)
		require.NoError(t, err)

		// Define test period
		startDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 7, 31, 23, 59, 59, 0, time.UTC)

		// Create transactions WITHIN the period
		// June: $5,000 inflow
		payment1 := payment.New(
			money.New(500000, "USD"), // $5,000.00
			paymentCat,
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)),
			payment.WithAccountingPeriod(time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)),
		)
		payment1, err = paymentService.Create(ctx, payment1)
		require.NoError(t, err)

		// July: $3,000 outflow
		expense1 := expense.New(
			money.New(300000, "USD"), // $3,000.00
			account,
			expenseCat,
			time.Date(2024, 7, 10, 10, 0, 0, 0, time.UTC),
			expense.WithTenantID(tenantID),
			expense.WithAccountingPeriod(time.Date(2024, 7, 10, 10, 0, 0, 0, time.UTC)),
			expense.WithComment("July Rent"),
		)
		expense1, err = expenseService.Create(ctx, expense1)
		require.NoError(t, err)

		// Create transaction AFTER the period (should not affect ending balance)
		payment2 := payment.New(
			money.New(1320000, "USD"), // $13,200.00
			paymentCat,
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(time.Date(2024, 8, 5, 10, 0, 0, 0, time.UTC)),
			payment.WithAccountingPeriod(time.Date(2024, 8, 5, 10, 0, 0, 0, time.UTC)),
		)
		payment2, err = paymentService.Create(ctx, payment2)
		require.NoError(t, err)

		// Log current account balance for debugging
		currentAccount, err := moneyAccountService.GetByID(ctx, account.ID())
		require.NoError(t, err)
		t.Logf("Current account balance (after all transactions): %s", currentAccount.Balance().Display())

		// Generate cashflow statement
		stmt, err := reportService.GenerateCashflowStatement(ctx, account.ID(), startDate, endDate)
		require.NoError(t, err)
		require.NotNil(t, stmt)

		// Log for debugging
		t.Logf("Period: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		t.Logf("Starting Balance: %s", stmt.StartingBalance.Display())
		t.Logf("Total Inflows: %s", stmt.TotalInflows.Display())
		t.Logf("Total Outflows: %s", stmt.TotalOutflows.Display())
		t.Logf("Net Cash Flow: %s", stmt.NetCashFlow.Display())
		t.Logf("Ending Balance: %s", stmt.EndingBalance.Display())

		// Verify calculations
		assert.Equal(t, int64(0), stmt.StartingBalance.Amount(), "Starting balance should be $0")
		assert.Equal(t, int64(500000), stmt.TotalInflows.Amount(), "Total inflows should be $5,000")
		assert.Equal(t, int64(300000), stmt.TotalOutflows.Amount(), "Total outflows should be $3,000")
		assert.Equal(t, int64(200000), stmt.NetCashFlow.Amount(), "Net cash flow should be $2,000")
		assert.Equal(t, int64(200000), stmt.EndingBalance.Amount(), "Ending balance at July 31 should be $2,000, not include August transaction")

		// Verify reconciliation
		reconciledBalance, _ := stmt.StartingBalance.Add(stmt.NetCashFlow)
		assert.Equal(t, stmt.EndingBalance.Amount(), reconciledBalance.Amount(),
			"Balance reconciliation failed: %d + %d != %d",
			stmt.StartingBalance.Amount(), stmt.NetCashFlow.Amount(), stmt.EndingBalance.Amount())

		// Verify categories appear correctly
		assert.Len(t, stmt.OperatingActivities.Inflows, 1)
		if len(stmt.OperatingActivities.Inflows) > 0 {
			assert.Equal(t, "Sales Revenue", stmt.OperatingActivities.Inflows[0].CategoryName)
			assert.Equal(t, int64(500000), stmt.OperatingActivities.Inflows[0].Amount.Amount())
		}

		assert.Len(t, stmt.OperatingActivities.Outflows, 1)
		if len(stmt.OperatingActivities.Outflows) > 0 {
			assert.Equal(t, "Office Expenses", stmt.OperatingActivities.Outflows[0].CategoryName)
			assert.Equal(t, int64(300000), stmt.OperatingActivities.Outflows[0].Amount.Amount())
		}
	})

	t.Run("Historical balance calculation", func(t *testing.T) {
		env := setupTest(t, permissions.PaymentRead, permissions.ExpenseRead)
		ctx := env.Ctx

		moneyAccountService := getAccountService(env)
		paymentService := getPaymentService(env)
		expenseService := getExpenseService(env)
		expenseCategoryService := getExpenseCategoryService(env)
		reportService := getFinancialReportService(env)

		// Get tenant ID
		tenantID, err := composables.UseTenantID(ctx)
		require.NoError(t, err)

		// Create account
		account := moneyaccount.New(
			"Historical Test Account",
			money.New(0, "USD"),
		)
		account, err = moneyAccountService.Create(ctx, account)
		require.NoError(t, err)

		// Create expense category
		expenseCat := expensecategory.New("General Expenses", expensecategory.WithTenantID(tenantID))
		expenseCat, err = expenseCategoryService.Create(ctx, expenseCat)
		require.NoError(t, err)

		// Create transactions BEFORE the reporting period
		historicalPayment := payment.New(
			money.New(1000000, "USD"), // $10,000
			nil,                       // No category
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(time.Date(2023, 12, 15, 0, 0, 0, 0, time.UTC)),
			payment.WithAccountingPeriod(time.Date(2023, 12, 15, 0, 0, 0, 0, time.UTC)),
		)
		historicalPayment, err = paymentService.Create(ctx, historicalPayment)
		require.NoError(t, err)

		// Create expense before period
		historicalExpense := expense.New(
			money.New(300000, "USD"), // $3,000
			account,
			expenseCat,
			time.Date(2023, 12, 20, 0, 0, 0, 0, time.UTC),
			expense.WithTenantID(tenantID),
			expense.WithAccountingPeriod(time.Date(2023, 12, 20, 0, 0, 0, 0, time.UTC)),
			expense.WithComment("Historical expense"),
		)
		historicalExpense, err = expenseService.Create(ctx, historicalExpense)
		require.NoError(t, err)

		// Now test a period in 2024
		startDate := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 3, 31, 23, 59, 59, 0, time.UTC)

		// Add one transaction in the period
		marchPayment := payment.New(
			money.New(200000, "USD"), // $2,000
			nil,
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)),
			payment.WithAccountingPeriod(time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)),
		)
		marchPayment, err = paymentService.Create(ctx, marchPayment)
		require.NoError(t, err)

		// Generate report
		stmt, err := reportService.GenerateCashflowStatement(ctx, account.ID(), startDate, endDate)
		require.NoError(t, err)

		// Log for debugging
		t.Logf("Historical transactions: +$10,000 -$3,000 = $7,000 before March")
		t.Logf("Starting Balance (March 1): %s", stmt.StartingBalance.Display())
		t.Logf("March Cashflow: %s", stmt.NetCashFlow.Display())
		t.Logf("Ending Balance (March 31): %s", stmt.EndingBalance.Display())

		// Starting balance should reflect historical transactions: $10,000 - $3,000 = $7,000
		assert.Equal(t, int64(700000), stmt.StartingBalance.Amount(), "Starting balance should include historical transactions")

		// Only March transaction should be in the cashflow
		assert.Equal(t, int64(200000), stmt.TotalInflows.Amount())
		assert.Equal(t, int64(0), stmt.TotalOutflows.Amount())
		assert.Equal(t, int64(200000), stmt.NetCashFlow.Amount())

		// Ending balance: $7,000 + $2,000 = $9,000
		assert.Equal(t, int64(900000), stmt.EndingBalance.Amount())
	})

	t.Run("Edge case - transactions on period boundaries", func(t *testing.T) {
		env := setupTest(t, permissions.PaymentRead)
		ctx := env.Ctx

		moneyAccountService := getAccountService(env)
		paymentService := getPaymentService(env)
		reportService := getFinancialReportService(env)

		// Get tenant ID
		tenantID, err := composables.UseTenantID(ctx)
		require.NoError(t, err)

		account := moneyaccount.New(
			"Boundary Test Account",
			money.New(0, "USD"),
		)
		account, err = moneyAccountService.Create(ctx, account)
		require.NoError(t, err)

		startDate := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 4, 30, 23, 59, 59, 0, time.UTC)

		// Transaction exactly at start of period
		startPayment := payment.New(
			money.New(100000, "USD"),
			nil,
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(startDate),
			payment.WithAccountingPeriod(startDate),
		)
		startPayment, err = paymentService.Create(ctx, startPayment)
		require.NoError(t, err)

		// Transaction exactly at end of period
		endPayment := payment.New(
			money.New(200000, "USD"),
			nil,
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(endDate),
			payment.WithAccountingPeriod(endDate),
		)
		endPayment, err = paymentService.Create(ctx, endPayment)
		require.NoError(t, err)

		// Transaction one second after period
		afterPayment := payment.New(
			money.New(300000, "USD"),
			nil,
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(endDate.Add(time.Second)),
			payment.WithAccountingPeriod(endDate.Add(time.Second)),
		)
		afterPayment, err = paymentService.Create(ctx, afterPayment)
		require.NoError(t, err)

		stmt, err := reportService.GenerateCashflowStatement(ctx, account.ID(), startDate, endDate)
		require.NoError(t, err)

		// Should include start and end boundary transactions only
		assert.Equal(t, int64(300000), stmt.TotalInflows.Amount()) // $1,000 + $2,000
		assert.Equal(t, int64(300000), stmt.NetCashFlow.Amount())
		assert.Equal(t, int64(300000), stmt.EndingBalance.Amount())

		// The transaction after the period should not be included
		currentAccount, _ := moneyAccountService.GetByID(ctx, account.ID())
		assert.Equal(t, int64(600000), currentAccount.Balance().Amount()) // Current balance includes all
	})

	t.Run("Cashflow statement with query debug", func(t *testing.T) {
		env := setupTest(t, permissions.PaymentRead)
		ctx := env.Ctx

		moneyAccountService := getAccountService(env)
		paymentService := getPaymentService(env)
		reportService := getFinancialReportService(env)

		// Get tenant ID
		tenantID, err := composables.UseTenantID(ctx)
		require.NoError(t, err)

		// Create account with specific start date
		account := moneyaccount.New(
			"Debug Test Account",
			money.New(0, "USD"),
		)
		account, err = moneyAccountService.Create(ctx, account)
		require.NoError(t, err)

		// Transaction timeline:
		// May 15: +$1,000 (before period)
		// June 1-30: Period start/end
		// June 15: +$500 (in period)
		// July 15: +$2,000 (after period)

		// Before period
		beforePayment := payment.New(
			money.New(100000, "USD"), // $1,000
			nil,
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC)),
			payment.WithAccountingPeriod(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC)),
		)
		beforePayment, err = paymentService.Create(ctx, beforePayment)
		require.NoError(t, err)

		// In period
		inPayment := payment.New(
			money.New(50000, "USD"), // $500
			nil,
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)),
			payment.WithAccountingPeriod(time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)),
		)
		inPayment, err = paymentService.Create(ctx, inPayment)
		require.NoError(t, err)

		// After period
		afterPayment := payment.New(
			money.New(200000, "USD"), // $2,000
			nil,
			payment.WithTenantID(tenantID),
			payment.WithAccount(account),
			payment.WithTransactionDate(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
			payment.WithAccountingPeriod(time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)),
		)
		afterPayment, err = paymentService.Create(ctx, afterPayment)
		require.NoError(t, err)

		// Check current balance
		currentAccount, _ := moneyAccountService.GetByID(ctx, account.ID())
		t.Logf("Current account balance (all transactions): %s", currentAccount.Balance().Display())
		assert.Equal(t, int64(350000), currentAccount.Balance().Amount()) // $3,500 total

		// Generate report for June
		startDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 6, 30, 23, 59, 59, 0, time.UTC)

		stmt, err := reportService.GenerateCashflowStatement(ctx, account.ID(), startDate, endDate)
		require.NoError(t, err)

		// Expected values:
		// Starting balance (June 1): $1,000 (from May transaction)
		// Cashflow in June: +$500
		// Ending balance (June 30): $1,500
		// Current balance: $3,500 (includes July transaction)

		t.Logf("\n=== Cashflow Debug ===")
		t.Logf("Period: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		t.Logf("Starting Balance (June 1): %s (expected: $1,000)", stmt.StartingBalance.Display())
		t.Logf("Inflows in June: %s (expected: $500)", stmt.TotalInflows.Display())
		t.Logf("Outflows in June: %s (expected: $0)", stmt.TotalOutflows.Display())
		t.Logf("Net Cash Flow: %s (expected: $500)", stmt.NetCashFlow.Display())
		t.Logf("Ending Balance (June 30): %s (expected: $1,500)", stmt.EndingBalance.Display())
		t.Logf("Current Balance (today): %s (expected: $3,500)", currentAccount.Balance().Display())

		assert.Equal(t, int64(100000), stmt.StartingBalance.Amount(), "Starting balance should be $1,000")
		assert.Equal(t, int64(50000), stmt.TotalInflows.Amount(), "Inflows should be $500")
		assert.Equal(t, int64(50000), stmt.NetCashFlow.Amount(), "Net cashflow should be $500")
		assert.Equal(t, int64(150000), stmt.EndingBalance.Amount(), "Ending balance should be $1,500")
	})
}

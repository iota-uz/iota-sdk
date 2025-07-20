package query

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/pkg/errors"
)

// SQL queries for income statement
const (
	// Query to get income by payment category for a period
	selectIncomeByCategory = `
		SELECT 
			COALESCE(pc.id, '00000000-0000-0000-0000-000000000000'::uuid) as category_id,
			COALESCE(pc.name, 'Uncategorized') as category_name,
			COALESCE(SUM(t.amount), 0) as total_amount,
			t.origin_account_id,
			ma.balance_currency_id as currency
		FROM transactions t
		INNER JOIN payments p ON t.id = p.transaction_id
		LEFT JOIN payment_categories pc ON p.payment_category_id = pc.id
		LEFT JOIN money_accounts ma ON t.origin_account_id = ma.id
		WHERE t.tenant_id = $1
			AND t.transaction_type = 'DEPOSIT'
			AND t.accounting_period >= $2
			AND t.accounting_period <= $3
		GROUP BY COALESCE(pc.id, '00000000-0000-0000-0000-000000000000'::uuid), 
		         COALESCE(pc.name, 'Uncategorized'), 
		         t.origin_account_id, 
		         ma.balance_currency_id
		ORDER BY COALESCE(pc.name, 'Uncategorized')`

	// Query to get expenses by expense category for a period
	selectExpensesByCategory = `
		SELECT 
			ec.id as category_id,
			ec.name as category_name,
			COALESCE(SUM(t.amount), 0) as total_amount,
			t.destination_account_id,
			ma.balance_currency_id as currency
		FROM transactions t
		INNER JOIN expenses e ON t.id = e.transaction_id
		INNER JOIN expense_categories ec ON e.category_id = ec.id
		LEFT JOIN money_accounts ma ON t.destination_account_id = ma.id
		WHERE t.tenant_id = $1
			AND t.transaction_type = 'WITHDRAWAL'
			AND t.accounting_period >= $2
			AND t.accounting_period <= $3
		GROUP BY ec.id, ec.name, t.destination_account_id, ma.balance_currency_id
		ORDER BY ec.name`

	// Query to get total income for a period
	selectTotalIncome = `
		SELECT 
			COALESCE(SUM(t.amount), 0) as total_amount,
			ma.balance_currency_id as currency
		FROM transactions t
		INNER JOIN payments p ON t.id = p.transaction_id
		LEFT JOIN money_accounts ma ON t.origin_account_id = ma.id
		WHERE t.tenant_id = $1
			AND t.transaction_type = 'DEPOSIT'
			AND t.accounting_period >= $2
			AND t.accounting_period <= $3
		GROUP BY ma.balance_currency_id`

	// Query to get total expenses for a period
	selectTotalExpenses = `
		SELECT 
			COALESCE(SUM(t.amount), 0) as total_amount,
			ma.balance_currency_id as currency
		FROM transactions t
		INNER JOIN expenses e ON t.id = e.transaction_id
		LEFT JOIN money_accounts ma ON t.destination_account_id = ma.id
		WHERE t.tenant_id = $1
			AND t.transaction_type = 'WITHDRAWAL'
			AND t.accounting_period >= $2
			AND t.accounting_period <= $3
		GROUP BY ma.balance_currency_id`

	// Query to get monthly income by category
	selectMonthlyIncomeByCategory = `
		SELECT 
			COALESCE(pc.id, '00000000-0000-0000-0000-000000000000'::uuid) as category_id,
			COALESCE(pc.name, 'Uncategorized') as category_name,
			EXTRACT(YEAR FROM t.accounting_period) as year,
			EXTRACT(MONTH FROM t.accounting_period) as month,
			COALESCE(SUM(t.amount), 0) as total_amount,
			ma.balance_currency_id as currency
		FROM transactions t
		INNER JOIN payments p ON t.id = p.transaction_id
		LEFT JOIN payment_categories pc ON p.payment_category_id = pc.id
		LEFT JOIN money_accounts ma ON t.origin_account_id = ma.id
		WHERE t.tenant_id = $1
			AND t.transaction_type = 'DEPOSIT'
			AND t.accounting_period >= $2
			AND t.accounting_period <= $3
		GROUP BY COALESCE(pc.id, '00000000-0000-0000-0000-000000000000'::uuid), 
		         COALESCE(pc.name, 'Uncategorized'), 
		         EXTRACT(YEAR FROM t.accounting_period),
		         EXTRACT(MONTH FROM t.accounting_period),
		         ma.balance_currency_id
		ORDER BY COALESCE(pc.name, 'Uncategorized'), year, month`

	// Query to get monthly expenses by category
	selectMonthlyExpensesByCategory = `
		SELECT 
			ec.id as category_id,
			ec.name as category_name,
			EXTRACT(YEAR FROM t.accounting_period) as year,
			EXTRACT(MONTH FROM t.accounting_period) as month,
			COALESCE(SUM(t.amount), 0) as total_amount,
			ma.balance_currency_id as currency
		FROM transactions t
		INNER JOIN expenses e ON t.id = e.transaction_id
		INNER JOIN expense_categories ec ON e.category_id = ec.id
		LEFT JOIN money_accounts ma ON t.destination_account_id = ma.id
		WHERE t.tenant_id = $1
			AND t.transaction_type = 'WITHDRAWAL'
			AND t.accounting_period >= $2
			AND t.accounting_period <= $3
		GROUP BY ec.id, ec.name, 
		         EXTRACT(YEAR FROM t.accounting_period),
		         EXTRACT(MONTH FROM t.accounting_period),
		         ma.balance_currency_id
		ORDER BY ec.name, year, month`

	// Query to get cashflow by category for operating activities
	selectCashflowByCategory = `
		WITH cashflow_data AS (
			-- Inflows (deposits TO this account)
			SELECT 
				'inflow' as flow_type,
				COALESCE(pc.id, '00000000-0000-0000-0000-000000000000'::uuid) as category_id,
				COALESCE(pc.name, 'Uncategorized') as category_name,
				COALESCE(SUM(t.amount), 0) as total_amount,
				COUNT(t.id) as transaction_count,
				ma.balance_currency_id as currency
			FROM transactions t
			INNER JOIN payments p ON t.id = p.transaction_id
			LEFT JOIN payment_categories pc ON p.payment_category_id = pc.id
			LEFT JOIN money_accounts ma ON t.destination_account_id = ma.id
			WHERE t.tenant_id = $1
				AND t.transaction_type = 'DEPOSIT'
				AND t.accounting_period >= $2
				AND t.accounting_period <= $3
				AND t.destination_account_id = $4
			GROUP BY COALESCE(pc.id, '00000000-0000-0000-0000-000000000000'::uuid), 
			         COALESCE(pc.name, 'Uncategorized'), 
			         ma.balance_currency_id
			
			UNION ALL
			
			-- Outflows (withdrawals FROM this account)
			SELECT 
				'outflow' as flow_type,
				ec.id as category_id,
				ec.name as category_name,
				COALESCE(SUM(t.amount), 0) as total_amount,
				COUNT(t.id) as transaction_count,
				ma.balance_currency_id as currency
			FROM transactions t
			INNER JOIN expenses e ON t.id = e.transaction_id
			INNER JOIN expense_categories ec ON e.category_id = ec.id
			LEFT JOIN money_accounts ma ON t.origin_account_id = ma.id
			WHERE t.tenant_id = $1
				AND t.transaction_type = 'WITHDRAWAL'
				AND t.accounting_period >= $2
				AND t.accounting_period <= $3
				AND t.origin_account_id = $4
			GROUP BY ec.id, ec.name, ma.balance_currency_id
		)
		SELECT * FROM cashflow_data
		ORDER BY flow_type DESC, category_name`

	// Query to get monthly cashflow by category
	selectMonthlyCashflowByCategory = `
		WITH cashflow_data AS (
			-- Monthly Inflows (deposits TO this account)
			SELECT 
				'inflow' as flow_type,
				COALESCE(pc.id, '00000000-0000-0000-0000-000000000000'::uuid) as category_id,
				COALESCE(pc.name, 'Uncategorized') as category_name,
				EXTRACT(YEAR FROM t.accounting_period) as year,
				EXTRACT(MONTH FROM t.accounting_period) as month,
				COALESCE(SUM(t.amount), 0) as total_amount,
				COUNT(t.id) as transaction_count,
				ma.balance_currency_id as currency
			FROM transactions t
			INNER JOIN payments p ON t.id = p.transaction_id
			LEFT JOIN payment_categories pc ON p.payment_category_id = pc.id
			LEFT JOIN money_accounts ma ON t.destination_account_id = ma.id
			WHERE t.tenant_id = $1
				AND t.transaction_type = 'DEPOSIT'
				AND t.accounting_period >= $2
				AND t.accounting_period <= $3
				AND t.destination_account_id = $4
			GROUP BY COALESCE(pc.id, '00000000-0000-0000-0000-000000000000'::uuid), 
			         COALESCE(pc.name, 'Uncategorized'),
			         EXTRACT(YEAR FROM t.accounting_period),
			         EXTRACT(MONTH FROM t.accounting_period),
			         ma.balance_currency_id
			
			UNION ALL
			
			-- Monthly Outflows (withdrawals FROM this account)
			SELECT 
				'outflow' as flow_type,
				ec.id as category_id,
				ec.name as category_name,
				EXTRACT(YEAR FROM t.accounting_period) as year,
				EXTRACT(MONTH FROM t.accounting_period) as month,
				COALESCE(SUM(t.amount), 0) as total_amount,
				COUNT(t.id) as transaction_count,
				ma.balance_currency_id as currency
			FROM transactions t
			INNER JOIN expenses e ON t.id = e.transaction_id
			INNER JOIN expense_categories ec ON e.category_id = ec.id
			LEFT JOIN money_accounts ma ON t.origin_account_id = ma.id
			WHERE t.tenant_id = $1
				AND t.transaction_type = 'WITHDRAWAL'
				AND t.accounting_period >= $2
				AND t.accounting_period <= $3
				AND t.origin_account_id = $4
			GROUP BY ec.id, ec.name,
			         EXTRACT(YEAR FROM t.accounting_period),
			         EXTRACT(MONTH FROM t.accounting_period),
			         ma.balance_currency_id
		)
		SELECT * FROM cashflow_data
		ORDER BY flow_type DESC, category_name, year, month`

	// Query to get account balance at a specific date
	selectAccountBalance = `
		SELECT 
			COALESCE(ma.balance, 0) as balance,
			ma.balance_currency_id as currency
		FROM money_accounts ma
		WHERE ma.id = $1
			AND ma.tenant_id = $2`
)

// ReportLineItem represents a single line item in the income statement
type ReportLineItem struct {
	CategoryID   uuid.UUID
	CategoryName string
	Amount       *money.Money
	Percentage   float64 // Percentage of total
}

// MonthlyReportLineItem represents a line item with monthly breakdown
type MonthlyReportLineItem struct {
	CategoryID     uuid.UUID
	CategoryName   string
	MonthlyAmounts map[string]*money.Money // Key: "YYYY-MM", Value: Amount
	TotalAmount    *money.Money
	Percentage     float64
}

// IncomeStatementData contains raw data for income statement generation
type IncomeStatementData struct {
	StartDate     time.Time
	EndDate       time.Time
	IncomeItems   []ReportLineItem
	ExpenseItems  []ReportLineItem
	TotalIncome   *money.Money
	TotalExpenses *money.Money
}

// CashflowLineItem represents a single cashflow line item
type CashflowLineItem struct {
	CategoryID   uuid.UUID
	CategoryName string
	Amount       *money.Money
	Count        int
	Percentage   float64
}

// CashflowData contains raw data for cashflow statement generation
type CashflowData struct {
	AccountID       uuid.UUID
	StartDate       time.Time
	EndDate         time.Time
	StartingBalance *money.Money
	EndingBalance   *money.Money
	Inflows         []CashflowLineItem
	Outflows        []CashflowLineItem
	TotalInflows    *money.Money
	TotalOutflows   *money.Money
}

// MonthlyCashflowLineItem represents a cashflow line item with monthly breakdown
type MonthlyCashflowLineItem struct {
	CategoryID     uuid.UUID
	CategoryName   string
	MonthlyAmounts map[string]*money.Money // Key: "YYYY-MM", Value: Amount
	MonthlyCounts  map[string]int          // Key: "YYYY-MM", Value: Transaction count
	TotalAmount    *money.Money
	TotalCount     int
	Percentage     float64
}

// FinancialReportsQueryRepository provides methods for generating financial reports
type FinancialReportsQueryRepository interface {
	GetIncomeStatementData(ctx context.Context, startDate, endDate time.Time) (*IncomeStatementData, error)
	GetIncomeByCategory(ctx context.Context, startDate, endDate time.Time) ([]ReportLineItem, *money.Money, error)
	GetExpensesByCategory(ctx context.Context, startDate, endDate time.Time) ([]ReportLineItem, *money.Money, error)
	GetMonthlyIncomeByCategory(ctx context.Context, startDate, endDate time.Time) ([]MonthlyReportLineItem, error)
	GetMonthlyExpensesByCategory(ctx context.Context, startDate, endDate time.Time) ([]MonthlyReportLineItem, error)

	// Cashflow methods
	GetCashflowData(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) (*CashflowData, error)
	GetCashflowByCategory(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) ([]CashflowLineItem, []CashflowLineItem, error)
	GetMonthlyCashflowByCategory(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) ([]MonthlyCashflowLineItem, []MonthlyCashflowLineItem, error)
	GetAccountBalance(ctx context.Context, accountID uuid.UUID) (*money.Money, error)
}

type pgFinancialReportsQueryRepository struct{}

// NewPgFinancialReportsQueryRepository creates a new PostgreSQL financial reports query repository
func NewPgFinancialReportsQueryRepository() FinancialReportsQueryRepository {
	return &pgFinancialReportsQueryRepository{}
}

// GetIncomeStatementData retrieves all data needed for income statement generation
func (r *pgFinancialReportsQueryRepository) GetIncomeStatementData(ctx context.Context, startDate, endDate time.Time) (*IncomeStatementData, error) {
	incomeItems, totalIncome, err := r.GetIncomeByCategory(ctx, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get income data")
	}

	expenseItems, totalExpenses, err := r.GetExpensesByCategory(ctx, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get expense data")
	}

	return &IncomeStatementData{
		StartDate:     startDate,
		EndDate:       endDate,
		IncomeItems:   incomeItems,
		ExpenseItems:  expenseItems,
		TotalIncome:   totalIncome,
		TotalExpenses: totalExpenses,
	}, nil
}

// GetIncomeByCategory retrieves income grouped by payment category for a period
func (r *pgFinancialReportsQueryRepository) GetIncomeByCategory(ctx context.Context, startDate, endDate time.Time) ([]ReportLineItem, *money.Money, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get tenant ID")
	}

	rows, err := tx.Query(ctx, selectIncomeByCategory, tenantID, startDate, endDate)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to query income by category")
	}
	defer rows.Close()

	var items []ReportLineItem
	var totalAmount int64
	defaultCurrency := "USD" // Default currency, should be configurable

	for rows.Next() {
		var categoryID uuid.UUID
		var categoryName string
		var amount int64
		var accountID *string
		var currency *string

		err := rows.Scan(&categoryID, &categoryName, &amount, &accountID, &currency)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to scan income row")
		}

		// Use account currency or default
		itemCurrency := defaultCurrency
		if currency != nil {
			itemCurrency = *currency
		}

		items = append(items, ReportLineItem{
			CategoryID:   categoryID,
			CategoryName: categoryName,
			Amount:       money.New(amount, itemCurrency),
		})

		totalAmount += amount
	}

	// Get total income for percentage calculations
	totalRow := tx.QueryRow(ctx, selectTotalIncome, tenantID, startDate, endDate)
	var totalCurrency *string
	err = totalRow.Scan(&totalAmount, &totalCurrency)
	if err != nil {
		// If no income found, return empty result
		if err.Error() == "no rows in result set" {
			return items, money.New(0, defaultCurrency), nil
		}
		return nil, nil, errors.Wrap(err, "failed to get total income")
	}

	totalIncomeCurrency := defaultCurrency
	if totalCurrency != nil {
		totalIncomeCurrency = *totalCurrency
	}
	totalIncome := money.New(totalAmount, totalIncomeCurrency)

	// Calculate percentages
	for i := range items {
		if totalAmount > 0 {
			items[i].Percentage = float64(items[i].Amount.Amount()) / float64(totalAmount) * 100
		}
	}

	return items, totalIncome, nil
}

// GetExpensesByCategory retrieves expenses grouped by expense category for a period
func (r *pgFinancialReportsQueryRepository) GetExpensesByCategory(ctx context.Context, startDate, endDate time.Time) ([]ReportLineItem, *money.Money, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get tenant ID")
	}

	rows, err := tx.Query(ctx, selectExpensesByCategory, tenantID, startDate, endDate)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to query expenses by category")
	}
	defer rows.Close()

	var items []ReportLineItem
	var totalAmount int64
	defaultCurrency := "USD" // Default currency, should be configurable

	for rows.Next() {
		var categoryID uuid.UUID
		var categoryName string
		var amount int64
		var accountID *string
		var currency *string

		err := rows.Scan(&categoryID, &categoryName, &amount, &accountID, &currency)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to scan expense row")
		}

		// Use account currency or default
		itemCurrency := defaultCurrency
		if currency != nil {
			itemCurrency = *currency
		}

		items = append(items, ReportLineItem{
			CategoryID:   categoryID,
			CategoryName: categoryName,
			Amount:       money.New(amount, itemCurrency),
		})

		totalAmount += amount
	}

	// Get total expenses for percentage calculations
	totalRow := tx.QueryRow(ctx, selectTotalExpenses, tenantID, startDate, endDate)
	var totalCurrency *string
	err = totalRow.Scan(&totalAmount, &totalCurrency)
	if err != nil {
		// If no expenses found, return empty result
		if err.Error() == "no rows in result set" {
			return items, money.New(0, defaultCurrency), nil
		}
		return nil, nil, errors.Wrap(err, "failed to get total expenses")
	}

	totalExpenseCurrency := defaultCurrency
	if totalCurrency != nil {
		totalExpenseCurrency = *totalCurrency
	}
	totalExpenses := money.New(totalAmount, totalExpenseCurrency)

	// Calculate percentages
	for i := range items {
		if totalAmount > 0 {
			items[i].Percentage = float64(items[i].Amount.Amount()) / float64(totalAmount) * 100
		}
	}

	return items, totalExpenses, nil
}

// GetMonthlyIncomeByCategory retrieves income with monthly breakdown by payment category
func (r *pgFinancialReportsQueryRepository) GetMonthlyIncomeByCategory(ctx context.Context, startDate, endDate time.Time) ([]MonthlyReportLineItem, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	rows, err := tx.Query(ctx, selectMonthlyIncomeByCategory, tenantID, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query monthly income by category")
	}
	defer rows.Close()

	categoryMap := make(map[uuid.UUID]*MonthlyReportLineItem)
	defaultCurrency := "USD"

	for rows.Next() {
		var categoryID uuid.UUID
		var categoryName string
		var year, month int
		var amount int64
		var currency *string

		err := rows.Scan(&categoryID, &categoryName, &year, &month, &amount, &currency)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan monthly income row")
		}

		itemCurrency := defaultCurrency
		if currency != nil {
			itemCurrency = *currency
		}

		monthKey := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")

		if item, exists := categoryMap[categoryID]; exists {
			// Add to existing category
			item.MonthlyAmounts[monthKey] = money.New(amount, itemCurrency)
			totalAmount, _ := item.TotalAmount.Add(money.New(amount, itemCurrency))
			item.TotalAmount = totalAmount
		} else {
			// Create new category
			categoryMap[categoryID] = &MonthlyReportLineItem{
				CategoryID:     categoryID,
				CategoryName:   categoryName,
				MonthlyAmounts: map[string]*money.Money{monthKey: money.New(amount, itemCurrency)},
				TotalAmount:    money.New(amount, itemCurrency),
			}
		}
	}

	// Convert map to slice
	items := make([]MonthlyReportLineItem, 0, len(categoryMap))
	for _, item := range categoryMap {
		items = append(items, *item)
	}

	return items, nil
}

// GetMonthlyExpensesByCategory retrieves expenses with monthly breakdown by expense category
func (r *pgFinancialReportsQueryRepository) GetMonthlyExpensesByCategory(ctx context.Context, startDate, endDate time.Time) ([]MonthlyReportLineItem, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	rows, err := tx.Query(ctx, selectMonthlyExpensesByCategory, tenantID, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query monthly expenses by category")
	}
	defer rows.Close()

	categoryMap := make(map[uuid.UUID]*MonthlyReportLineItem)
	defaultCurrency := "USD"

	for rows.Next() {
		var categoryID uuid.UUID
		var categoryName string
		var year, month int
		var amount int64
		var currency *string

		err := rows.Scan(&categoryID, &categoryName, &year, &month, &amount, &currency)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan monthly expense row")
		}

		itemCurrency := defaultCurrency
		if currency != nil {
			itemCurrency = *currency
		}

		monthKey := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")

		if item, exists := categoryMap[categoryID]; exists {
			// Add to existing category
			item.MonthlyAmounts[monthKey] = money.New(amount, itemCurrency)
			totalAmount, _ := item.TotalAmount.Add(money.New(amount, itemCurrency))
			item.TotalAmount = totalAmount
		} else {
			// Create new category
			categoryMap[categoryID] = &MonthlyReportLineItem{
				CategoryID:     categoryID,
				CategoryName:   categoryName,
				MonthlyAmounts: map[string]*money.Money{monthKey: money.New(amount, itemCurrency)},
				TotalAmount:    money.New(amount, itemCurrency),
			}
		}
	}

	// Convert map to slice
	items := make([]MonthlyReportLineItem, 0, len(categoryMap))
	for _, item := range categoryMap {
		items = append(items, *item)
	}

	return items, nil
}

// GetCashflowData retrieves all data needed for cashflow statement generation
func (r *pgFinancialReportsQueryRepository) GetCashflowData(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) (*CashflowData, error) {
	// Get account balance
	currentBalance, err := r.GetAccountBalance(ctx, accountID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get account balance")
	}

	// Get cashflow by category
	inflows, outflows, err := r.GetCashflowByCategory(ctx, accountID, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cashflow data")
	}

	// Calculate totals
	totalInflows := money.New(0, currentBalance.Currency().Code)
	for _, item := range inflows {
		totalInflows, _ = totalInflows.Add(item.Amount)
	}

	totalOutflows := money.New(0, currentBalance.Currency().Code)
	for _, item := range outflows {
		totalOutflows, _ = totalOutflows.Add(item.Amount)
	}

	// Calculate starting balance (current balance - net cashflow)
	netCashflow, _ := totalInflows.Subtract(totalOutflows)
	startingBalance, _ := currentBalance.Subtract(netCashflow)

	return &CashflowData{
		AccountID:       accountID,
		StartDate:       startDate,
		EndDate:         endDate,
		StartingBalance: startingBalance,
		EndingBalance:   currentBalance,
		Inflows:         inflows,
		Outflows:        outflows,
		TotalInflows:    totalInflows,
		TotalOutflows:   totalOutflows,
	}, nil
}

// GetCashflowByCategory retrieves cashflow grouped by category for a period
func (r *pgFinancialReportsQueryRepository) GetCashflowByCategory(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) ([]CashflowLineItem, []CashflowLineItem, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get tenant ID")
	}

	rows, err := tx.Query(ctx, selectCashflowByCategory, tenantID, startDate, endDate, accountID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to query cashflow by category")
	}
	defer rows.Close()

	var inflows []CashflowLineItem
	var outflows []CashflowLineItem
	defaultCurrency := "USD"

	for rows.Next() {
		var flowType string
		var categoryID uuid.UUID
		var categoryName string
		var amount int64
		var count int
		var currency *string

		err := rows.Scan(&flowType, &categoryID, &categoryName, &amount, &count, &currency)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to scan cashflow row")
		}

		itemCurrency := defaultCurrency
		if currency != nil {
			itemCurrency = *currency
		}

		item := CashflowLineItem{
			CategoryID:   categoryID,
			CategoryName: categoryName,
			Amount:       money.New(amount, itemCurrency),
			Count:        count,
		}

		if flowType == "inflow" {
			inflows = append(inflows, item)
		} else {
			outflows = append(outflows, item)
		}
	}

	// Calculate percentages
	totalInflows := int64(0)
	for _, item := range inflows {
		totalInflows += item.Amount.Amount()
	}
	for i := range inflows {
		if totalInflows > 0 {
			inflows[i].Percentage = float64(inflows[i].Amount.Amount()) / float64(totalInflows) * 100
		}
	}

	totalOutflows := int64(0)
	for _, item := range outflows {
		totalOutflows += item.Amount.Amount()
	}
	for i := range outflows {
		if totalOutflows > 0 {
			outflows[i].Percentage = float64(outflows[i].Amount.Amount()) / float64(totalOutflows) * 100
		}
	}

	return inflows, outflows, nil
}

// GetMonthlyCashflowByCategory retrieves cashflow with monthly breakdown by category
func (r *pgFinancialReportsQueryRepository) GetMonthlyCashflowByCategory(ctx context.Context, accountID uuid.UUID, startDate, endDate time.Time) ([]MonthlyCashflowLineItem, []MonthlyCashflowLineItem, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get tenant ID")
	}

	rows, err := tx.Query(ctx, selectMonthlyCashflowByCategory, tenantID, startDate, endDate, accountID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to query monthly cashflow by category")
	}
	defer rows.Close()

	inflowMap := make(map[uuid.UUID]*MonthlyCashflowLineItem)
	outflowMap := make(map[uuid.UUID]*MonthlyCashflowLineItem)
	defaultCurrency := "USD"

	for rows.Next() {
		var flowType string
		var categoryID uuid.UUID
		var categoryName string
		var year, month int
		var amount int64
		var count int
		var currency *string

		err := rows.Scan(&flowType, &categoryID, &categoryName, &year, &month, &amount, &count, &currency)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to scan monthly cashflow row")
		}

		itemCurrency := defaultCurrency
		if currency != nil {
			itemCurrency = *currency
		}

		monthKey := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")

		if flowType == "inflow" {
			if item, exists := inflowMap[categoryID]; exists {
				item.MonthlyAmounts[monthKey] = money.New(amount, itemCurrency)
				item.MonthlyCounts[monthKey] = count
				totalAmount, _ := item.TotalAmount.Add(money.New(amount, itemCurrency))
				item.TotalAmount = totalAmount
				item.TotalCount += count
			} else {
				inflowMap[categoryID] = &MonthlyCashflowLineItem{
					CategoryID:     categoryID,
					CategoryName:   categoryName,
					MonthlyAmounts: map[string]*money.Money{monthKey: money.New(amount, itemCurrency)},
					MonthlyCounts:  map[string]int{monthKey: count},
					TotalAmount:    money.New(amount, itemCurrency),
					TotalCount:     count,
				}
			}
		} else {
			if item, exists := outflowMap[categoryID]; exists {
				item.MonthlyAmounts[monthKey] = money.New(amount, itemCurrency)
				item.MonthlyCounts[monthKey] = count
				totalAmount, _ := item.TotalAmount.Add(money.New(amount, itemCurrency))
				item.TotalAmount = totalAmount
				item.TotalCount += count
			} else {
				outflowMap[categoryID] = &MonthlyCashflowLineItem{
					CategoryID:     categoryID,
					CategoryName:   categoryName,
					MonthlyAmounts: map[string]*money.Money{monthKey: money.New(amount, itemCurrency)},
					MonthlyCounts:  map[string]int{monthKey: count},
					TotalAmount:    money.New(amount, itemCurrency),
					TotalCount:     count,
				}
			}
		}
	}

	// Convert maps to slices
	inflows := make([]MonthlyCashflowLineItem, 0, len(inflowMap))
	for _, item := range inflowMap {
		inflows = append(inflows, *item)
	}

	outflows := make([]MonthlyCashflowLineItem, 0, len(outflowMap))
	for _, item := range outflowMap {
		outflows = append(outflows, *item)
	}

	return inflows, outflows, nil
}

// GetAccountBalance retrieves the current balance of an account
func (r *pgFinancialReportsQueryRepository) GetAccountBalance(ctx context.Context, accountID uuid.UUID) (*money.Money, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	var balance int64
	var currency string

	err = tx.QueryRow(ctx, selectAccountBalance, accountID, tenantID).Scan(&balance, &currency)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get account balance")
	}

	return money.New(balance, currency), nil
}

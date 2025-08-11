package mappers

import (
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/value_objects"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/pkg/money"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
)

func ExpenseCategoryToViewModel(entity category.ExpenseCategory) *viewmodels.ExpenseCategory {
	return &viewmodels.ExpenseCategory{
		ID:          entity.ID().String(),
		Name:        entity.Name(),
		Description: entity.Description(),
		UpdatedAt:   entity.UpdatedAt().Format(time.RFC3339),
		CreatedAt:   entity.CreatedAt().Format(time.RFC3339),
	}
}

func PaymentCategoryToViewModel(entity paymentcategory.PaymentCategory) *viewmodels.PaymentCategory {
	return &viewmodels.PaymentCategory{
		ID:          entity.ID().String(),
		Name:        entity.Name(),
		Description: entity.Description(),
		UpdatedAt:   entity.UpdatedAt().Format(time.RFC3339),
		CreatedAt:   entity.CreatedAt().Format(time.RFC3339),
	}
}

func MoneyAccountToViewModel(entity moneyaccount.Account) *viewmodels.MoneyAccount {
	balance := entity.Balance()
	return &viewmodels.MoneyAccount{
		ID:                  entity.ID().String(),
		Name:                entity.Name(),
		AccountNumber:       entity.AccountNumber(),
		Balance:             fmt.Sprintf("%.2f", balance.AsMajorUnits()),
		BalanceWithCurrency: balance.Display(),
		CurrencyCode:        balance.Currency().Code,
		CurrencySymbol:      balance.Currency().Grapheme,
		Description:         entity.Description(),
		UpdatedAt:           entity.UpdatedAt().Format(time.RFC3339),
		CreatedAt:           entity.CreatedAt().Format(time.RFC3339),
	}
}

func MoneyAccountToViewUpdateModel(entity moneyaccount.Account) *viewmodels.MoneyAccountUpdateDTO {
	balance := entity.Balance()
	return &viewmodels.MoneyAccountUpdateDTO{
		ID:            entity.ID().String(),
		Name:          entity.Name(),
		Description:   entity.Description(),
		AccountNumber: entity.AccountNumber(),
		Balance:       fmt.Sprintf("%.2f", balance.AsMajorUnits()),
		CurrencyCode:  balance.Currency().Code,
	}
}

func PaymentToViewModel(entity payment.Payment) *viewmodels.Payment {
	amount := entity.Amount()
	return &viewmodels.Payment{
		ID:                 entity.ID().String(),
		Amount:             fmt.Sprintf("%.2f", amount.AsMajorUnits()),
		AmountWithCurrency: amount.Display(),
		AccountID:          entity.Account().ID().String(),
		CounterpartyID:     entity.CounterpartyID().String(),
		CategoryID:         entity.Category().ID().String(),
		Category:           PaymentCategoryToViewModel(entity.Category()),
		TransactionID:      entity.TransactionID().String(),
		TransactionDate:    entity.TransactionDate().Format(time.DateOnly),
		AccountingPeriod:   entity.AccountingPeriod().Format(time.DateOnly),
		Comment:            entity.Comment(),
		CreatedAt:          entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:          entity.UpdatedAt().Format(time.RFC3339),
	}
}

func ExpenseToViewModel(entity expense.Expense) *viewmodels.Expense {
	amount := entity.Amount()
	return &viewmodels.Expense{
		ID:                 entity.ID().String(),
		Amount:             fmt.Sprintf("%.2f", amount.AsMajorUnits()),
		AmountWithCurrency: amount.Display(),
		AccountID:          entity.Account().ID().String(),
		CategoryID:         entity.Category().ID().String(),
		Category:           ExpenseCategoryToViewModel(entity.Category()),
		Comment:            entity.Comment(),
		TransactionID:      entity.TransactionID().String(),
		AccountingPeriod:   entity.AccountingPeriod().Format(time.RFC3339),
		Date:               entity.Date().Format(time.RFC3339),
		CreatedAt:          entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:          entity.UpdatedAt().Format(time.RFC3339),
	}
}

func CounterpartyToViewModel(entity counterparty.Counterparty) *viewmodels.Counterparty {
	var tin string
	if entity.Tin() != nil {
		tin = entity.Tin().Value()
	}
	return &viewmodels.Counterparty{
		ID:           entity.ID().String(),
		TIN:          tin,
		Name:         entity.Name(),
		Type:         viewmodels.CounterpartyTypeFromDomain(entity.Type()),
		LegalType:    viewmodels.CounterpartyLegalTypeFromDomain(entity.LegalType()),
		LegalAddress: entity.LegalAddress(),
		CreatedAt:    entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:    entity.UpdatedAt().Format(time.RFC3339),
	}
}

func InventoryToViewModel(entity inventory.Inventory) *viewmodels.Inventory {
	price := entity.Price()
	totalValue := price.Multiply(int64(entity.Quantity()))
	return &viewmodels.Inventory{
		ID:           entity.ID().String(),
		Name:         entity.Name(),
		Description:  entity.Description(),
		CurrencyCode: price.Currency().Code,
		Price:        fmt.Sprintf("%.2f", price.AsMajorUnits()),
		Quantity:     fmt.Sprintf("%d", entity.Quantity()),
		TotalValue:   fmt.Sprintf("%.2f", totalValue.AsMajorUnits()),
		CreatedAt:    entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:    entity.UpdatedAt().Format(time.RFC3339),
	}
}

func TransactionToViewModel(entity transaction.Transaction) *viewmodels.Transaction {
	amount := entity.Amount()
	vm := &viewmodels.Transaction{
		ID:                 entity.ID().String(),
		Amount:             fmt.Sprintf("%.2f", amount.AsMajorUnits()),
		AmountWithCurrency: amount.Display(),
		TransactionDate:    entity.TransactionDate(),
		AccountingPeriod:   entity.AccountingPeriod(),
		TransactionType:    string(entity.TransactionType()),
		Comment:            entity.Comment(),
		CreatedAt:          entity.CreatedAt(),
	}

	// Set type badge class
	switch entity.TransactionType() {
	case transaction.Deposit:
		vm.TypeBadgeClass = "badge-success"
	case transaction.Withdrawal:
		vm.TypeBadgeClass = "badge-danger"
	case transaction.Transfer:
		vm.TypeBadgeClass = "badge-info"
	case transaction.Exchange:
		vm.TypeBadgeClass = "badge-warning"
	default:
		vm.TypeBadgeClass = "badge-primary"
	}

	// Note: Account, Category, and Counterparty information is not available
	// from the domain entity alone. These should be populated by the query repository.

	// Handle exchange fields
	if entity.ExchangeRate() != nil {
		vm.ExchangeRate = fmt.Sprintf("%.4f", *entity.ExchangeRate())
	}
	if entity.DestinationAmount() != nil {
		vm.DestinationAmount = fmt.Sprintf("%.2f", entity.DestinationAmount().AsMajorUnits())
		vm.DestinationAmountWithCurrency = entity.DestinationAmount().Display()
	}

	return vm
}

func TransactionToListItem(vm *viewmodels.Transaction) *viewmodels.TransactionListItem {
	item := &viewmodels.TransactionListItem{
		ID:                 vm.ID,
		Amount:             vm.Amount,
		AmountWithCurrency: vm.AmountWithCurrency,
		TransactionDate:    vm.TransactionDate,
		TransactionType:    vm.TransactionType,
		TypeBadgeClass:     vm.TypeBadgeClass,
		Comment:            vm.Comment,
		Category:           vm.Category,
		Counterparty:       vm.Counterparty,
	}

	// Determine which account to show based on transaction type
	if vm.TransactionType == string(transaction.Withdrawal) || vm.TransactionType == string(transaction.Transfer) {
		item.Account = vm.OriginAccount
	} else if vm.TransactionType == string(transaction.Deposit) || vm.TransactionType == string(transaction.Exchange) {
		item.Account = vm.DestinationAccount
	}

	return item
}

// ToIncomeStatementViewModel converts domain income statement to viewmodel
func ToIncomeStatementViewModel(incomeStatement *value_objects.IncomeStatement) *viewmodels.IncomeStatement {
	months := generateMonthlyHeaders(incomeStatement.StartDate, incomeStatement.EndDate)

	return &viewmodels.IncomeStatement{
		ID:                      incomeStatement.ID.String(),
		Period:                  incomeStatement.Period(),
		StartDate:               incomeStatement.StartDate.Format("2006-01-02"),
		EndDate:                 incomeStatement.EndDate.Format("2006-01-02"),
		Months:                  months,
		RevenueSection:          toIncomeStatementSectionViewModel(incomeStatement.RevenueSection),
		ExpenseSection:          toIncomeStatementSectionViewModel(incomeStatement.ExpenseSection),
		GrossProfit:             fmt.Sprintf("%.2f", incomeStatement.GrossProfit.AsMajorUnits()),
		GrossProfitWithCurrency: incomeStatement.GrossProfit.Display(),
		GrossProfitRatio:        incomeStatement.GrossProfitRatio,
		NetProfit:               fmt.Sprintf("%.2f", incomeStatement.NetProfit.AsMajorUnits()),
		NetProfitWithCurrency:   incomeStatement.NetProfit.Display(),
		NetProfitRatio:          incomeStatement.NetProfitRatio,
		IsProfit:                incomeStatement.IsProfit(),
		Currency:                incomeStatement.Currency,
		GeneratedAt:             incomeStatement.GeneratedAt.Format(time.RFC3339),
	}
}

// ToIncomeStatementViewModelWithMonthlyData converts domain income statement to viewmodel with monthly breakdown
func ToIncomeStatementViewModelWithMonthlyData(
	incomeStatement *value_objects.IncomeStatement,
	monthlyIncomeData []query.MonthlyReportLineItem,
	monthlyExpenseData []query.MonthlyReportLineItem,
) *viewmodels.IncomeStatement {
	months := generateMonthlyHeaders(incomeStatement.StartDate, incomeStatement.EndDate)

	// Create maps for quick lookup of monthly data by category
	incomeMonthlyMap := make(map[string]map[string]*money.Money)
	for _, item := range monthlyIncomeData {
		incomeMonthlyMap[item.CategoryName] = item.MonthlyAmounts
	}

	expenseMonthlyMap := make(map[string]map[string]*money.Money)
	for _, item := range monthlyExpenseData {
		expenseMonthlyMap[item.CategoryName] = item.MonthlyAmounts
	}

	return &viewmodels.IncomeStatement{
		ID:                      incomeStatement.ID.String(),
		Period:                  incomeStatement.Period(),
		StartDate:               incomeStatement.StartDate.Format("2006-01-02"),
		EndDate:                 incomeStatement.EndDate.Format("2006-01-02"),
		Months:                  months,
		RevenueSection:          toIncomeStatementSectionViewModelWithMonthlyData(incomeStatement.RevenueSection, incomeMonthlyMap, incomeStatement.StartDate, incomeStatement.EndDate),
		ExpenseSection:          toIncomeStatementSectionViewModelWithMonthlyData(incomeStatement.ExpenseSection, expenseMonthlyMap, incomeStatement.StartDate, incomeStatement.EndDate),
		GrossProfit:             fmt.Sprintf("%.2f", incomeStatement.GrossProfit.AsMajorUnits()),
		GrossProfitWithCurrency: incomeStatement.GrossProfit.Display(),
		GrossProfitRatio:        incomeStatement.GrossProfitRatio,
		MonthlyGrossProfit:      calculateMonthlyGrossProfit(incomeMonthlyMap, expenseMonthlyMap, incomeStatement.StartDate, incomeStatement.EndDate, incomeStatement.Currency),
		NetProfit:               fmt.Sprintf("%.2f", incomeStatement.NetProfit.AsMajorUnits()),
		NetProfitWithCurrency:   incomeStatement.NetProfit.Display(),
		NetProfitRatio:          incomeStatement.NetProfitRatio,
		MonthlyNetProfit:        calculateMonthlyGrossProfit(incomeMonthlyMap, expenseMonthlyMap, incomeStatement.StartDate, incomeStatement.EndDate, incomeStatement.Currency), // For now, net profit equals gross profit
		IsProfit:                incomeStatement.IsProfit(),
		Currency:                incomeStatement.Currency,
		GeneratedAt:             incomeStatement.GeneratedAt.Format(time.RFC3339),
	}
}

// ToIncomeStatementResponseDTO converts viewmodel to response DTO for JSON API
func ToIncomeStatementResponseDTO(vm *viewmodels.IncomeStatement) *dtos.IncomeStatementResponseDTO {
	return &dtos.IncomeStatementResponseDTO{
		ID:                      vm.ID,
		Period:                  vm.Period,
		StartDate:               vm.StartDate,
		EndDate:                 vm.EndDate,
		RevenueSection:          toIncomeStatementSectionDTO(vm.RevenueSection),
		ExpenseSection:          toIncomeStatementSectionDTO(vm.ExpenseSection),
		GrossProfit:             vm.GrossProfit,
		GrossProfitWithCurrency: vm.GrossProfitWithCurrency,
		GrossProfitRatio:        vm.GrossProfitRatio,
		NetProfit:               vm.NetProfit,
		NetProfitWithCurrency:   vm.NetProfitWithCurrency,
		NetProfitRatio:          vm.NetProfitRatio,
		IsProfit:                vm.IsProfit,
		Currency:                vm.Currency,
		GeneratedAt:             vm.GeneratedAt,
	}
}

// toIncomeStatementSectionViewModel converts domain section to viewmodel
func toIncomeStatementSectionViewModel(section value_objects.IncomeStatementSection) viewmodels.IncomeStatementSection {
	lineItems := make([]viewmodels.IncomeStatementLineItem, 0, len(section.LineItems))

	for _, item := range section.LineItems {
		lineItems = append(lineItems, viewmodels.IncomeStatementLineItem{
			ID:                 item.ID.String(),
			Name:               item.Name,
			Amount:             fmt.Sprintf("%.2f", item.Amount.AsMajorUnits()),
			AmountWithCurrency: item.Amount.Display(),
			Percentage:         item.Percentage,
		})
	}

	return viewmodels.IncomeStatementSection{
		Title:                section.Title,
		LineItems:            lineItems,
		Subtotal:             fmt.Sprintf("%.2f", section.Subtotal.AsMajorUnits()),
		SubtotalWithCurrency: section.Subtotal.Display(),
		Percentage:           section.Percentage,
	}
}

// toIncomeStatementSectionViewModelWithMonthlyData converts domain section to viewmodel with real monthly data
func toIncomeStatementSectionViewModelWithMonthlyData(
	section value_objects.IncomeStatementSection,
	monthlyDataMap map[string]map[string]*money.Money,
	startDate, endDate time.Time,
) viewmodels.IncomeStatementSection {
	lineItems := make([]viewmodels.IncomeStatementLineItem, 0, len(section.LineItems))

	for _, item := range section.LineItems {
		// Get monthly data for this category if available
		monthlyData := monthlyDataMap[item.Name]
		if monthlyData == nil {
			monthlyData = make(map[string]*money.Money)
		}

		lineItems = append(lineItems, viewmodels.IncomeStatementLineItem{
			ID:                 item.ID.String(),
			Name:               item.Name,
			Amount:             fmt.Sprintf("%.2f", item.Amount.AsMajorUnits()),
			AmountWithCurrency: item.Amount.Display(),
			Percentage:         item.Percentage,
			MonthlyAmounts:     generateMonthlyAmountsFromData(monthlyData, startDate, endDate, item.Amount.Currency().Code),
		})
	}

	// Calculate monthly subtotals by summing all line items for each month
	months := generateMonthlyHeaders(startDate, endDate)
	monthlySubtotals := make([]viewmodels.MonthlyAmount, len(months))

	for i, month := range months {
		monthKey := getMonthKeyFromName(month, startDate, endDate)
		total := money.New(0, section.Subtotal.Currency().Code)

		// Sum all category amounts for this month
		for _, categoryData := range monthlyDataMap {
			if amount, exists := categoryData[monthKey]; exists {
				total, _ = total.Add(amount)
			}
		}

		monthlySubtotals[i] = viewmodels.MonthlyAmount{
			Month:              month,
			Amount:             fmt.Sprintf("%.2f", total.AsMajorUnits()),
			AmountWithCurrency: total.Display(),
		}
	}

	return viewmodels.IncomeStatementSection{
		Title:                section.Title,
		LineItems:            lineItems,
		Subtotal:             fmt.Sprintf("%.2f", section.Subtotal.AsMajorUnits()),
		SubtotalWithCurrency: section.Subtotal.Display(),
		Percentage:           section.Percentage,
		MonthlySubtotals:     monthlySubtotals,
	}
}

// toIncomeStatementSectionDTO converts viewmodel section to DTO
func toIncomeStatementSectionDTO(section viewmodels.IncomeStatementSection) dtos.IncomeStatementSectionDTO {
	lineItems := make([]dtos.IncomeStatementLineItemDTO, 0, len(section.LineItems))

	for _, item := range section.LineItems {
		lineItems = append(lineItems, dtos.IncomeStatementLineItemDTO{
			ID:                 item.ID,
			Name:               item.Name,
			Amount:             item.Amount,
			AmountWithCurrency: item.AmountWithCurrency,
			Percentage:         item.Percentage,
		})
	}

	return dtos.IncomeStatementSectionDTO{
		Title:                section.Title,
		LineItems:            lineItems,
		Subtotal:             section.Subtotal,
		SubtotalWithCurrency: section.SubtotalWithCurrency,
		Percentage:           section.Percentage,
	}
}

// generateMonthlyHeaders creates month headers based on start and end dates
func generateMonthlyHeaders(startDate, endDate time.Time) []string {
	var months []string

	current := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, startDate.Location())
	end := time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, endDate.Location())

	for current.Before(end) || current.Equal(end) {
		months = append(months, current.Format("January"))
		current = current.AddDate(0, 1, 0)
	}

	// If no months generated (same month), include at least one month
	if len(months) == 0 {
		months = append(months, startDate.Format("January"))
	}

	return months
}

// generateMonthlyAmountsFromData creates monthly amounts from actual monthly data
func generateMonthlyAmountsFromData(monthlyData map[string]*money.Money, startDate, endDate time.Time, defaultCurrency string) []viewmodels.MonthlyAmount {
	months := generateMonthlyHeaders(startDate, endDate)
	monthlyAmounts := make([]viewmodels.MonthlyAmount, len(months))

	// Create a mapping from month name to date for proper key generation
	current := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, startDate.Location())
	end := time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, endDate.Location())

	monthIndex := 0
	for current.Before(end) || current.Equal(end) {
		if monthIndex >= len(months) {
			break
		}

		monthKey := current.Format("2006-01")
		monthName := months[monthIndex]

		if amount, exists := monthlyData[monthKey]; exists {
			monthlyAmounts[monthIndex] = viewmodels.MonthlyAmount{
				Month:              monthName,
				Amount:             fmt.Sprintf("%.2f", amount.AsMajorUnits()),
				AmountWithCurrency: amount.Display(),
			}
		} else {
			// No data for this month, show zero
			zeroAmount := money.New(0, defaultCurrency)
			monthlyAmounts[monthIndex] = viewmodels.MonthlyAmount{
				Month:              monthName,
				Amount:             "0.00",
				AmountWithCurrency: zeroAmount.Display(),
			}
		}

		current = current.AddDate(0, 1, 0)
		monthIndex++
	}

	return monthlyAmounts
}

// getMonthKeyFromName converts month name back to YYYY-MM format for lookup
func getMonthKeyFromName(monthName string, startDate, endDate time.Time) string {
	current := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, startDate.Location())
	end := time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, endDate.Location())

	for current.Before(end) || current.Equal(end) {
		if current.Format("January") == monthName {
			return current.Format("2006-01")
		}
		current = current.AddDate(0, 1, 0)
	}

	// Fallback - shouldn't happen
	return time.Now().Format("2006-01")
}

// calculateMonthlyGrossProfit calculates monthly gross profit by subtracting expenses from income
func calculateMonthlyGrossProfit(
	incomeMap, expenseMap map[string]map[string]*money.Money,
	startDate, endDate time.Time,
	currency string,
) []viewmodels.MonthlyAmount {
	months := generateMonthlyHeaders(startDate, endDate)
	monthlyGrossProfit := make([]viewmodels.MonthlyAmount, len(months))

	for i, month := range months {
		monthKey := getMonthKeyFromName(month, startDate, endDate)

		// Sum all income for this month
		totalIncome := money.New(0, currency)
		for _, categoryData := range incomeMap {
			if amount, exists := categoryData[monthKey]; exists {
				totalIncome, _ = totalIncome.Add(amount)
			}
		}

		// Sum all expenses for this month
		totalExpenses := money.New(0, currency)
		for _, categoryData := range expenseMap {
			if amount, exists := categoryData[monthKey]; exists {
				totalExpenses, _ = totalExpenses.Add(amount)
			}
		}

		// Calculate gross profit (income - expenses)
		grossProfit, _ := totalIncome.Subtract(totalExpenses)

		monthlyGrossProfit[i] = viewmodels.MonthlyAmount{
			Month:              month,
			Amount:             fmt.Sprintf("%.2f", grossProfit.AsMajorUnits()),
			AmountWithCurrency: grossProfit.Display(),
		}
	}

	return monthlyGrossProfit
}

// ToCashflowStatementViewModel converts domain cashflow statement to viewmodel
func ToCashflowStatementViewModel(cashflowStatement *value_objects.CashflowStatement, accountName *string) *viewmodels.CashflowStatement {
	months := generateMonthlyHeaders(cashflowStatement.StartDate, cashflowStatement.EndDate)

	accountDisplayName := "Unknown Account"
	if accountName != nil {
		accountDisplayName = *accountName
	}

	return &viewmodels.CashflowStatement{
		ID:                          uuid.New().String(),
		AccountID:                   cashflowStatement.AccountID.String(),
		AccountName:                 accountDisplayName,
		Period:                      formatPeriod(cashflowStatement.StartDate, cashflowStatement.EndDate),
		StartDate:                   cashflowStatement.StartDate.Format("2006-01-02"),
		EndDate:                     cashflowStatement.EndDate.Format("2006-01-02"),
		Months:                      months,
		StartingBalance:             fmt.Sprintf("%.2f", cashflowStatement.StartingBalance.AsMajorUnits()),
		StartingBalanceWithCurrency: cashflowStatement.StartingBalance.Display(),
		EndingBalance:               fmt.Sprintf("%.2f", cashflowStatement.EndingBalance.AsMajorUnits()),
		EndingBalanceWithCurrency:   cashflowStatement.EndingBalance.Display(),
		OperatingActivities:         toCashflowSectionViewModel(cashflowStatement.OperatingActivities),
		InvestingActivities:         toCashflowSectionViewModel(cashflowStatement.InvestingActivities),
		FinancingActivities:         toCashflowSectionViewModel(cashflowStatement.FinancingActivities),
		TotalInflows:                fmt.Sprintf("%.2f", cashflowStatement.TotalInflows.AsMajorUnits()),
		TotalInflowsWithCurrency:    cashflowStatement.TotalInflows.Display(),
		TotalOutflows:               fmt.Sprintf("%.2f", cashflowStatement.TotalOutflows.AsMajorUnits()),
		TotalOutflowsWithCurrency:   cashflowStatement.TotalOutflows.Display(),
		NetCashFlow:                 fmt.Sprintf("%.2f", cashflowStatement.NetCashFlow.AsMajorUnits()),
		NetCashFlowWithCurrency:     cashflowStatement.NetCashFlow.Display(),
		IsPositive:                  cashflowStatement.NetCashFlow.Amount() >= 0,
		Currency:                    cashflowStatement.Currency,
		GeneratedAt:                 time.Now().Format("2006-01-02 15:04:05"),
	}
}

// ToCashflowStatementViewModelWithMonthlyData converts domain cashflow statement to viewmodel with monthly breakdown
func ToCashflowStatementViewModelWithMonthlyData(
	cashflowStatement *value_objects.CashflowStatement,
	accountName string,
	monthlyInflows []query.MonthlyCashflowLineItem,
	monthlyOutflows []query.MonthlyCashflowLineItem,
) *viewmodels.CashflowStatement {
	// Create maps for quick lookup of monthly data by category
	inflowMonthlyMap := make(map[string]map[string]*money.Money)
	inflowCountMap := make(map[string]map[string]int)
	for _, item := range monthlyInflows {
		inflowMonthlyMap[item.CategoryName] = item.MonthlyAmounts
		inflowCountMap[item.CategoryName] = item.MonthlyCounts
	}

	outflowMonthlyMap := make(map[string]map[string]*money.Money)
	outflowCountMap := make(map[string]map[string]int)
	for _, item := range monthlyOutflows {
		outflowMonthlyMap[item.CategoryName] = item.MonthlyAmounts
		outflowCountMap[item.CategoryName] = item.MonthlyCounts
	}

	// Get base viewmodel
	vm := ToCashflowStatementViewModel(cashflowStatement, &accountName)

	// Add monthly data to sections
	vm.OperatingActivities = toCashflowSectionViewModelWithMonthlyData(
		cashflowStatement.OperatingActivities,
		inflowMonthlyMap,
		inflowCountMap,
		outflowMonthlyMap,
		outflowCountMap,
		cashflowStatement.StartDate,
		cashflowStatement.EndDate,
	)

	// Calculate monthly net cashflow
	vm.MonthlyNetCashFlow = calculateMonthlyNetCashflow(
		inflowMonthlyMap,
		outflowMonthlyMap,
		cashflowStatement.StartDate,
		cashflowStatement.EndDate,
		cashflowStatement.Currency,
	)

	return vm
}

// toCashflowSectionViewModel converts domain cashflow section to viewmodel
func toCashflowSectionViewModel(section value_objects.CashflowSection) viewmodels.CashflowSection {
	inflowItems := make([]viewmodels.CashflowLineItem, 0, len(section.Inflows))
	for _, item := range section.Inflows {
		inflowItems = append(inflowItems, viewmodels.CashflowLineItem{
			ID:                 item.CategoryID.String(),
			Name:               item.CategoryName,
			Amount:             fmt.Sprintf("%.2f", item.Amount.AsMajorUnits()),
			AmountWithCurrency: item.Amount.Display(),
			Percentage:         item.Percentage,
			Count:              item.Count,
		})
	}

	outflowItems := make([]viewmodels.CashflowLineItem, 0, len(section.Outflows))
	for _, item := range section.Outflows {
		outflowItems = append(outflowItems, viewmodels.CashflowLineItem{
			ID:                 item.CategoryID.String(),
			Name:               item.CategoryName,
			Amount:             fmt.Sprintf("%.2f", item.Amount.AsMajorUnits()),
			AmountWithCurrency: item.Amount.Display(),
			Percentage:         item.Percentage,
			Count:              item.Count,
		})
	}

	return viewmodels.CashflowSection{
		Name:                    section.Name,
		Inflows:                 inflowItems,
		Outflows:                outflowItems,
		NetCashFlow:             fmt.Sprintf("%.2f", section.NetCashFlow.AsMajorUnits()),
		NetCashFlowWithCurrency: section.NetCashFlow.Display(),
	}
}

// toCashflowSectionViewModelWithMonthlyData converts domain cashflow section to viewmodel with monthly breakdown
func toCashflowSectionViewModelWithMonthlyData(
	section value_objects.CashflowSection,
	inflowMonthlyMap map[string]map[string]*money.Money,
	inflowCountMap map[string]map[string]int,
	outflowMonthlyMap map[string]map[string]*money.Money,
	outflowCountMap map[string]map[string]int,
	startDate, endDate time.Time,
) viewmodels.CashflowSection {
	// Convert basic section
	vm := toCashflowSectionViewModel(section)

	// Add monthly data to inflow items
	for i := range vm.Inflows {
		if monthlyData, exists := inflowMonthlyMap[section.Inflows[i].CategoryName]; exists {
			vm.Inflows[i].MonthlyAmounts = toMonthlyAmounts(monthlyData, startDate, endDate)
		}
		if countData, exists := inflowCountMap[section.Inflows[i].CategoryName]; exists {
			vm.Inflows[i].MonthlyCounts = toMonthlyCounts(countData, startDate, endDate)
		}
	}

	// Add monthly data to outflow items
	for i := range vm.Outflows {
		if monthlyData, exists := outflowMonthlyMap[section.Outflows[i].CategoryName]; exists {
			vm.Outflows[i].MonthlyAmounts = toMonthlyAmounts(monthlyData, startDate, endDate)
		}
		if countData, exists := outflowCountMap[section.Outflows[i].CategoryName]; exists {
			vm.Outflows[i].MonthlyCounts = toMonthlyCounts(countData, startDate, endDate)
		}
	}

	// Calculate monthly net cashflow for the section
	vm.MonthlyNetCashFlow = calculateSectionMonthlyNetCashflow(
		inflowMonthlyMap,
		outflowMonthlyMap,
		section,
		startDate,
		endDate,
	)

	return vm
}

// toMonthlyCounts converts monthly count data to viewmodel format
func toMonthlyCounts(countData map[string]int, startDate, endDate time.Time) []viewmodels.MonthlyCount {
	months := generateMonthlyHeaders(startDate, endDate)
	monthlyCounts := make([]viewmodels.MonthlyCount, len(months))

	for i, month := range months {
		monthKey := getMonthKeyFromName(month, startDate, endDate)
		count := 0
		if c, exists := countData[monthKey]; exists {
			count = c
		}
		monthlyCounts[i] = viewmodels.MonthlyCount{
			Month: month,
			Count: count,
		}
	}

	return monthlyCounts
}

// calculateMonthlyNetCashflow calculates net cashflow for each month
func calculateMonthlyNetCashflow(
	inflowMap map[string]map[string]*money.Money,
	outflowMap map[string]map[string]*money.Money,
	startDate, endDate time.Time,
	currency string,
) []viewmodels.MonthlyAmount {
	months := generateMonthlyHeaders(startDate, endDate)
	monthlyNetCashflow := make([]viewmodels.MonthlyAmount, len(months))

	for i, month := range months {
		monthKey := getMonthKeyFromName(month, startDate, endDate)

		// Sum all inflows for this month
		totalInflows := money.New(0, currency)
		for _, categoryData := range inflowMap {
			if amount, exists := categoryData[monthKey]; exists {
				totalInflows, _ = totalInflows.Add(amount)
			}
		}

		// Sum all outflows for this month
		totalOutflows := money.New(0, currency)
		for _, categoryData := range outflowMap {
			if amount, exists := categoryData[monthKey]; exists {
				totalOutflows, _ = totalOutflows.Add(amount)
			}
		}

		// Calculate net cashflow
		netCashflow, _ := totalInflows.Subtract(totalOutflows)

		monthlyNetCashflow[i] = viewmodels.MonthlyAmount{
			Month:              month,
			Amount:             fmt.Sprintf("%.2f", netCashflow.AsMajorUnits()),
			AmountWithCurrency: netCashflow.Display(),
		}
	}

	return monthlyNetCashflow
}

// calculateSectionMonthlyNetCashflow calculates net cashflow for each month in a section
func calculateSectionMonthlyNetCashflow(
	inflowMap map[string]map[string]*money.Money,
	outflowMap map[string]map[string]*money.Money,
	section value_objects.CashflowSection,
	startDate, endDate time.Time,
) []viewmodels.MonthlyAmount {
	months := generateMonthlyHeaders(startDate, endDate)
	monthlyNetCashflow := make([]viewmodels.MonthlyAmount, len(months))

	// Assuming all amounts are in the same currency, get it from the first non-nil amount
	var currency string
	if len(section.Inflows) > 0 {
		currency = section.Inflows[0].Amount.Currency().Code
	} else if len(section.Outflows) > 0 {
		currency = section.Outflows[0].Amount.Currency().Code
	} else {
		currency = "USD" // Default fallback
	}

	for i, month := range months {
		monthKey := getMonthKeyFromName(month, startDate, endDate)

		// Sum inflows for this section and month
		totalInflows := money.New(0, currency)
		for _, item := range section.Inflows {
			if categoryData, exists := inflowMap[item.CategoryName]; exists {
				if amount, exists := categoryData[monthKey]; exists {
					totalInflows, _ = totalInflows.Add(amount)
				}
			}
		}

		// Sum outflows for this section and month
		totalOutflows := money.New(0, currency)
		for _, item := range section.Outflows {
			if categoryData, exists := outflowMap[item.CategoryName]; exists {
				if amount, exists := categoryData[monthKey]; exists {
					totalOutflows, _ = totalOutflows.Add(amount)
				}
			}
		}

		// Calculate net cashflow
		netCashflow, _ := totalInflows.Subtract(totalOutflows)

		monthlyNetCashflow[i] = viewmodels.MonthlyAmount{
			Month:              month,
			Amount:             fmt.Sprintf("%.2f", netCashflow.AsMajorUnits()),
			AmountWithCurrency: netCashflow.Display(),
		}
	}

	return monthlyNetCashflow
}

// formatPeriod formats a date range into a human-readable period string
func formatPeriod(startDate, endDate time.Time) string {
	if startDate.Year() == endDate.Year() && startDate.Month() == endDate.Month() {
		return startDate.Format("January 2006")
	}
	return fmt.Sprintf("%s - %s", startDate.Format("Jan 2006"), endDate.Format("Jan 2006"))
}

// toMonthlyAmounts converts monthly amount data to viewmodel format
func toMonthlyAmounts(monthlyData map[string]*money.Money, startDate, endDate time.Time) []viewmodels.MonthlyAmount {
	months := generateMonthlyHeaders(startDate, endDate)
	monthlyAmounts := make([]viewmodels.MonthlyAmount, len(months))

	for i, month := range months {
		monthKey := getMonthKeyFromName(month, startDate, endDate)
		if amount, exists := monthlyData[monthKey]; exists {
			monthlyAmounts[i] = viewmodels.MonthlyAmount{
				Month:              month,
				Amount:             fmt.Sprintf("%.2f", amount.AsMajorUnits()),
				AmountWithCurrency: amount.Display(),
			}
		} else {
			// No data for this month, use zero with the currency from any existing amount
			var currency string
			for _, amt := range monthlyData {
				if amt != nil {
					currency = amt.Currency().Code
					break
				}
			}
			if currency == "" {
				currency = "USD" // Default fallback
			}
			zeroAmount := money.New(0, currency)
			monthlyAmounts[i] = viewmodels.MonthlyAmount{
				Month:              month,
				Amount:             "0.00",
				AmountWithCurrency: zeroAmount.Display(),
			}
		}
	}

	return monthlyAmounts
}

// ToCashflowStatementResponseDTO converts cashflow statement to response DTO for JSON API
func ToCashflowStatementResponseDTO(cashflowStatement *value_objects.CashflowStatement, accountName string) *dtos.CashflowStatementResponseDTO {
	return &dtos.CashflowStatementResponseDTO{
		ID:                          uuid.New().String(),
		AccountID:                   cashflowStatement.AccountID.String(),
		AccountName:                 accountName,
		Period:                      formatPeriod(cashflowStatement.StartDate, cashflowStatement.EndDate),
		StartDate:                   cashflowStatement.StartDate.Format("2006-01-02"),
		EndDate:                     cashflowStatement.EndDate.Format("2006-01-02"),
		StartingBalance:             fmt.Sprintf("%.2f", cashflowStatement.StartingBalance.AsMajorUnits()),
		StartingBalanceWithCurrency: cashflowStatement.StartingBalance.Display(),
		EndingBalance:               fmt.Sprintf("%.2f", cashflowStatement.EndingBalance.AsMajorUnits()),
		EndingBalanceWithCurrency:   cashflowStatement.EndingBalance.Display(),
		OperatingActivities:         toCashflowSectionDTO(cashflowStatement.OperatingActivities),
		InvestingActivities:         toCashflowSectionDTO(cashflowStatement.InvestingActivities),
		FinancingActivities:         toCashflowSectionDTO(cashflowStatement.FinancingActivities),
		TotalInflows:                fmt.Sprintf("%.2f", cashflowStatement.TotalInflows.AsMajorUnits()),
		TotalInflowsWithCurrency:    cashflowStatement.TotalInflows.Display(),
		TotalOutflows:               fmt.Sprintf("%.2f", cashflowStatement.TotalOutflows.AsMajorUnits()),
		TotalOutflowsWithCurrency:   cashflowStatement.TotalOutflows.Display(),
		NetCashFlow:                 fmt.Sprintf("%.2f", cashflowStatement.NetCashFlow.AsMajorUnits()),
		NetCashFlowWithCurrency:     cashflowStatement.NetCashFlow.Display(),
		IsPositive:                  cashflowStatement.NetCashFlow.Amount() >= 0,
		Currency:                    cashflowStatement.Currency,
		GeneratedAt:                 time.Now().Format("2006-01-02 15:04:05"),
	}
}

// toCashflowSectionDTO converts cashflow section to DTO
func toCashflowSectionDTO(section value_objects.CashflowSection) dtos.CashflowSectionDTO {
	inflowItems := make([]dtos.CashflowLineItemDTO, 0, len(section.Inflows))
	for _, item := range section.Inflows {
		inflowItems = append(inflowItems, dtos.CashflowLineItemDTO{
			ID:                 item.CategoryID.String(),
			Name:               item.CategoryName,
			Amount:             fmt.Sprintf("%.2f", item.Amount.AsMajorUnits()),
			AmountWithCurrency: item.Amount.Display(),
			Percentage:         item.Percentage,
			Count:              item.Count,
		})
	}

	outflowItems := make([]dtos.CashflowLineItemDTO, 0, len(section.Outflows))
	for _, item := range section.Outflows {
		outflowItems = append(outflowItems, dtos.CashflowLineItemDTO{
			ID:                 item.CategoryID.String(),
			Name:               item.CategoryName,
			Amount:             fmt.Sprintf("%.2f", item.Amount.AsMajorUnits()),
			AmountWithCurrency: item.Amount.Display(),
			Percentage:         item.Percentage,
			Count:              item.Count,
		})
	}

	return dtos.CashflowSectionDTO{
		Name:                    section.Name,
		Inflows:                 inflowItems,
		Outflows:                outflowItems,
		NetCashFlow:             fmt.Sprintf("%.2f", section.NetCashFlow.AsMajorUnits()),
		NetCashFlowWithCurrency: section.NetCashFlow.Display(),
	}
}

func DebtToViewModel(entity debt.Debt, counterpartyName string) *viewmodels.Debt {
	originalAmount := entity.OriginalAmount()
	outstandingAmount := entity.OutstandingAmount()

	vm := &viewmodels.Debt{
		ID:                            entity.ID().String(),
		Type:                          string(entity.Type()),
		Status:                        string(entity.Status()),
		CounterpartyID:                entity.CounterpartyID().String(),
		CounterpartyName:              counterpartyName,
		OriginalAmount:                fmt.Sprintf("%.2f", originalAmount.AsMajorUnits()),
		OriginalAmountWithCurrency:    originalAmount.Display(),
		OutstandingAmount:             fmt.Sprintf("%.2f", outstandingAmount.AsMajorUnits()),
		OutstandingAmountWithCurrency: outstandingAmount.Display(),
		Description:                   entity.Description(),
		CreatedAt:                     entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:                     entity.UpdatedAt().Format(time.RFC3339),
	}

	if entity.DueDate() != nil {
		vm.DueDate = entity.DueDate().Format(time.DateOnly)
	}

	if entity.SettlementTransactionID() != nil {
		vm.SettlementTransactionID = entity.SettlementTransactionID().String()
	}

	return vm
}

func DebtCounterpartyAggregateToViewModel(agg debt.CounterpartyAggregate, counterpartyName string) *viewmodels.DebtCounterpartyAggregate {
	// The SQL query returns amounts in minor units (cents), so we use money.New() instead of money.NewFromFloat()
	receivableMoney := money.New(int64(agg.TotalReceivable()), agg.CurrencyCode())
	payableMoney := money.New(int64(agg.TotalPayable()), agg.CurrencyCode())
	outstandingReceivableMoney := money.New(int64(agg.TotalOutstandingReceivable()), agg.CurrencyCode())
	outstandingPayableMoney := money.New(int64(agg.TotalOutstandingPayable()), agg.CurrencyCode())

	netAmount, _ := outstandingReceivableMoney.Subtract(outstandingPayableMoney)

	return &viewmodels.DebtCounterpartyAggregate{
		CounterpartyID:             agg.CounterpartyID().String(),
		CounterpartyName:           counterpartyName,
		TotalReceivable:            receivableMoney.Display(),
		TotalPayable:               payableMoney.Display(),
		TotalOutstandingReceivable: outstandingReceivableMoney.Display(),
		TotalOutstandingPayable:    outstandingPayableMoney.Display(),
		NetAmount:                  netAmount.Display(),
		DebtCount:                  agg.DebtCount(),
		CurrencyCode:               agg.CurrencyCode(),
	}
}

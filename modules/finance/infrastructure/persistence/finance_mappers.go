package persistence

import (
	"database/sql"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

func ToDBTransaction(entity transaction.Transaction) *models.Transaction {
	var exchangeRate sql.NullFloat64
	var destinationAmount sql.NullInt64

	if entity.ExchangeRate() != nil {
		exchangeRate = sql.NullFloat64{Float64: *entity.ExchangeRate(), Valid: true}
	}

	if entity.DestinationAmount() != nil {
		destinationAmount = sql.NullInt64{Int64: entity.DestinationAmount().Amount(), Valid: true}
	}

	return &models.Transaction{
		ID:                   entity.ID().String(),
		TenantID:             entity.TenantID().String(),
		Amount:               entity.Amount().Amount(),
		Comment:              entity.Comment(),
		AccountingPeriod:     entity.AccountingPeriod(),
		TransactionDate:      entity.TransactionDate(),
		DestinationAccountID: mapping.UUIDToSQLNullString(entity.DestinationAccountID()),
		OriginAccountID:      mapping.UUIDToSQLNullString(entity.OriginAccountID()),
		TransactionType:      string(entity.TransactionType()),
		CreatedAt:            entity.CreatedAt(),
		ExchangeRate:         exchangeRate,
		DestinationAmount:    destinationAmount,
	}
}

func ToDomainTransaction(dbTransaction *models.Transaction) (transaction.Transaction, error) {
	_type, err := transaction.NewType(dbTransaction.TransactionType)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(dbTransaction.TenantID)
	if err != nil {
		return nil, err
	}

	// Create Money object from amount - assuming USD as default, will need to be updated when currency is stored
	amount := money.New(dbTransaction.Amount, "USD")

	opts := []transaction.Option{
		transaction.WithID(uuid.MustParse(dbTransaction.ID)),
		transaction.WithTenantID(tenantID),
		transaction.WithComment(dbTransaction.Comment),
		transaction.WithAccountingPeriod(dbTransaction.AccountingPeriod),
		transaction.WithTransactionDate(dbTransaction.TransactionDate),
		transaction.WithOriginAccountID(mapping.SQLNullStringToUUID(dbTransaction.OriginAccountID)),
		transaction.WithDestinationAccountID(mapping.SQLNullStringToUUID(dbTransaction.DestinationAccountID)),
		transaction.WithCreatedAt(dbTransaction.CreatedAt),
	}

	if dbTransaction.ExchangeRate.Valid {
		opts = append(opts, transaction.WithExchangeRate(&dbTransaction.ExchangeRate.Float64))
	}

	if dbTransaction.DestinationAmount.Valid {
		destAmount := money.New(dbTransaction.DestinationAmount.Int64, "USD") // TODO: get proper currency
		opts = append(opts, transaction.WithDestinationAmount(destAmount))
	}

	t := transaction.New(amount, _type, opts...)

	return t, nil
}

func ToDBPayment(entity payment.Payment) (*models.Payment, *models.Transaction) {
	dbTransaction := &models.Transaction{
		ID:                   entity.TransactionID().String(),
		TenantID:             entity.TenantID().String(),
		Amount:               entity.Amount().Amount(),
		Comment:              entity.Comment(),
		AccountingPeriod:     entity.AccountingPeriod(),
		TransactionDate:      entity.TransactionDate(),
		OriginAccountID:      mapping.UUIDToSQLNullString(uuid.Nil),
		DestinationAccountID: mapping.UUIDToSQLNullString(entity.Account().ID()),
		TransactionType:      string(transaction.Deposit),
		CreatedAt:            entity.CreatedAt(),
	}
	var categoryID uuid.UUID
	if entity.Category() != nil {
		categoryID = entity.Category().ID()
	}

	dbPayment := &models.Payment{
		ID:                entity.ID().String(),
		TenantID:          entity.TenantID().String(),
		TransactionID:     entity.TransactionID().String(),
		CounterpartyID:    entity.CounterpartyID().String(),
		PaymentCategoryID: mapping.UUIDToSQLNullString(categoryID),
		CreatedAt:         entity.CreatedAt(),
		UpdatedAt:         entity.UpdatedAt(),
	}
	return dbPayment, dbTransaction
}

// TODO: populate user && account
func ToDomainPayment(dbPayment *models.Payment, dbTransaction *models.Transaction) (payment.Payment, error) {
	t, err := ToDomainTransaction(dbTransaction)
	if err != nil {
		return nil, err
	}
	email, err := internet.NewEmail("payment@system.internal")
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(dbTransaction.TenantID)
	if err != nil {
		return nil, err
	}

	// Create a default payment category - TODO: fetch actual category from database
	var category paymentcategory.PaymentCategory
	if dbPayment.PaymentCategoryID.Valid {
		// For now, create a category with the ID - ideally we'd fetch from DB
		category = paymentcategory.New("Uncategorized", paymentcategory.WithID(uuid.MustParse(dbPayment.PaymentCategoryID.String)))
	} else {
		category = paymentcategory.New("Uncategorized")
	}

	// Create default money account with zero balance
	defaultBalance := money.New(0, "USD")
	return payment.New(
		t.Amount(),
		category,
		payment.WithID(uuid.MustParse(dbPayment.ID)),
		payment.WithTenantID(tenantID),
		payment.WithTransactionID(t.ID()),
		payment.WithCounterpartyID(uuid.MustParse(dbPayment.CounterpartyID)),
		payment.WithComment(t.Comment()),
		payment.WithAccount(moneyaccount.New("", defaultBalance, moneyaccount.WithID(t.DestinationAccountID()))),
		payment.WithUser(user.New(
			"", // firstName
			"", // lastName
			email,
			"", // uiLanguage
		)),
		payment.WithTransactionDate(t.TransactionDate()),
		payment.WithAccountingPeriod(t.AccountingPeriod()),
		payment.WithCreatedAt(dbPayment.CreatedAt),
		payment.WithUpdatedAt(dbPayment.UpdatedAt),
	), nil
}

func ToDBExpenseCategory(entity category.ExpenseCategory) *models.ExpenseCategory {
	return &models.ExpenseCategory{
		ID:          entity.ID().String(),
		TenantID:    entity.TenantID().String(),
		Name:        entity.Name(),
		Description: mapping.ValueToSQLNullString(entity.Description()),
		CreatedAt:   entity.CreatedAt(),
		UpdatedAt:   entity.UpdatedAt(),
	}
}

func ToDomainExpenseCategory(dbCategory *models.ExpenseCategory) (category.ExpenseCategory, error) {
	tenantID, err := uuid.Parse(dbCategory.TenantID)
	if err != nil {
		return nil, err
	}

	opts := []category.Option{
		category.WithID(uuid.MustParse(dbCategory.ID)),
		category.WithTenantID(tenantID),
		category.WithCreatedAt(dbCategory.CreatedAt),
		category.WithUpdatedAt(dbCategory.UpdatedAt),
	}

	if dbCategory.Description.Valid {
		opts = append(opts, category.WithDescription(dbCategory.Description.String))
	}

	return category.New(
		dbCategory.Name,
		opts...,
	), nil
}

func ToDBPaymentCategory(entity paymentcategory.PaymentCategory) *models.PaymentCategory {
	return &models.PaymentCategory{
		ID:          entity.ID().String(),
		TenantID:    entity.TenantID().String(),
		Name:        entity.Name(),
		Description: mapping.ValueToSQLNullString(entity.Description()),
		CreatedAt:   entity.CreatedAt(),
		UpdatedAt:   entity.UpdatedAt(),
	}
}

func ToDomainPaymentCategory(dbCategory *models.PaymentCategory) (paymentcategory.PaymentCategory, error) {
	tenantID, err := uuid.Parse(dbCategory.TenantID)
	if err != nil {
		return nil, err
	}

	opts := []paymentcategory.Option{
		paymentcategory.WithID(uuid.MustParse(dbCategory.ID)),
		paymentcategory.WithTenantID(tenantID),
		paymentcategory.WithCreatedAt(dbCategory.CreatedAt),
		paymentcategory.WithUpdatedAt(dbCategory.UpdatedAt),
	}

	if dbCategory.Description.Valid {
		opts = append(opts, paymentcategory.WithDescription(dbCategory.Description.String))
	}

	return paymentcategory.New(
		dbCategory.Name,
		opts...,
	), nil
}

func ToDomainMoneyAccount(dbAccount *models.MoneyAccount) (moneyaccount.Account, error) {
	tenantID, err := uuid.Parse(dbAccount.TenantID)
	if err != nil {
		return nil, err
	}

	balance := money.New(dbAccount.Balance, dbAccount.BalanceCurrencyID)

	opts := []moneyaccount.Option{
		moneyaccount.WithID(uuid.MustParse(dbAccount.ID)),
		moneyaccount.WithTenantID(tenantID),
		moneyaccount.WithAccountNumber(dbAccount.AccountNumber),
		moneyaccount.WithCreatedAt(dbAccount.CreatedAt),
		moneyaccount.WithUpdatedAt(dbAccount.UpdatedAt),
	}

	if dbAccount.Description.Valid {
		opts = append(opts, moneyaccount.WithDescription(dbAccount.Description.String))
	}

	return moneyaccount.New(
		dbAccount.Name,
		balance,
		opts...,
	), nil
}

func ToDBMoneyAccount(entity moneyaccount.Account) *models.MoneyAccount {
	balance := entity.Balance()
	return &models.MoneyAccount{
		ID:                entity.ID().String(),
		TenantID:          entity.TenantID().String(),
		Name:              entity.Name(),
		AccountNumber:     entity.AccountNumber(),
		Balance:           balance.Amount(),
		BalanceCurrencyID: balance.Currency().Code,
		Description:       mapping.ValueToSQLNullString(entity.Description()),
		CreatedAt:         entity.CreatedAt(),
		UpdatedAt:         entity.UpdatedAt(),
	}
}

func ToDomainExpense(dbExpense *models.Expense, dbTransaction *models.Transaction) (expense.Expense, error) {
	tenantID, err := uuid.Parse(dbExpense.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse tenant ID")
	}

	// Create default money account with zero balance
	defaultBalance := money.New(0, "USD")
	account := moneyaccount.New(
		"",
		defaultBalance,
		moneyaccount.WithID(mapping.SQLNullStringToUUID(dbTransaction.OriginAccountID)),
	)
	expenseCategory := category.New(
		"", // name - will be populated when actual category is fetched
		category.WithID(uuid.MustParse(dbExpense.CategoryID)),
		category.WithTenantID(tenantID),
		category.WithCreatedAt(dbExpense.CreatedAt),
		category.WithUpdatedAt(dbExpense.UpdatedAt),
	)

	// Create expense amount as negative money
	expenseAmount := money.New(-1*dbTransaction.Amount, "USD")
	domainExpense := expense.New(
		expenseAmount,
		account,
		expenseCategory,
		dbTransaction.TransactionDate,
		expense.WithID(uuid.MustParse(dbExpense.ID)),
		expense.WithTenantID(tenantID),
		expense.WithComment(dbTransaction.Comment),
		expense.WithTransactionID(uuid.MustParse(dbExpense.TransactionID)),
		expense.WithAccountingPeriod(dbTransaction.AccountingPeriod),
		expense.WithCreatedAt(dbExpense.CreatedAt),
		expense.WithUpdatedAt(dbExpense.UpdatedAt),
	)

	return domainExpense, nil
}

func ToDBExpense(entity expense.Expense) (*models.Expense, transaction.Transaction) {
	accountID := entity.Account().ID()
	tenantID := entity.TenantID()
	// Create negative amount for withdrawal
	withdrawalAmount := entity.Amount().Negative()
	domainTransaction := transaction.New(
		withdrawalAmount,
		transaction.Withdrawal,
		transaction.WithID(entity.TransactionID()),
		transaction.WithTenantID(tenantID),
		transaction.WithComment(entity.Comment()),
		transaction.WithAccountingPeriod(entity.AccountingPeriod()),
		transaction.WithTransactionDate(entity.Date()),
		transaction.WithOriginAccountID(accountID),
		transaction.WithCreatedAt(entity.CreatedAt()),
	)
	dbExpense := &models.Expense{
		ID:            entity.ID().String(),
		TenantID:      tenantID.String(),
		CategoryID:    entity.Category().ID().String(),
		TransactionID: entity.TransactionID().String(),
		CreatedAt:     entity.CreatedAt(),
		UpdatedAt:     entity.UpdatedAt(),
	}
	return dbExpense, domainTransaction
}

func ToDomainCounterparty(dbRow *models.Counterparty) (counterparty.Counterparty, error) {
	partyType, err := counterparty.NewType(dbRow.Type)
	if err != nil {
		return nil, err
	}
	legalType, err := counterparty.NewLegalType(dbRow.LegalType)
	if err != nil {
		return nil, err
	}
	var t tax.Tin = tax.NilTin
	if dbRow.Tin.Valid && dbRow.Tin.String != "" {
		t, err = tax.NewTin(dbRow.Tin.String, country.Uzbekistan)
		if err != nil {
			return nil, err
		}
	}
	return counterparty.New(
		dbRow.Name,
		partyType,
		legalType,
		counterparty.WithID(uuid.MustParse(dbRow.ID)),
		counterparty.WithTenantID(uuid.MustParse(dbRow.TenantID)),
		counterparty.WithTin(t),
		counterparty.WithLegalAddress(dbRow.LegalAddress),
		counterparty.WithCreatedAt(dbRow.CreatedAt),
		counterparty.WithUpdatedAt(dbRow.UpdatedAt),
	), nil
}

func ToDBCounterparty(entity counterparty.Counterparty) (*models.Counterparty, error) {
	var tin sql.NullString
	entityTin := entity.Tin()
	if entityTin != tax.NilTin && entityTin != nil && entityTin.Value() != "" {
		tin = sql.NullString{String: entityTin.Value(), Valid: true}
	}
	return &models.Counterparty{
		ID:           entity.ID().String(),
		TenantID:     entity.TenantID().String(),
		Tin:          tin,
		Name:         entity.Name(),
		Type:         string(entity.Type()),
		LegalType:    string(entity.LegalType()),
		LegalAddress: entity.LegalAddress(),
		CreatedAt:    entity.CreatedAt(),
		UpdatedAt:    entity.UpdatedAt(),
	}, nil
}

func ToDBInventory(entity inventory.Inventory) *models.Inventory {
	price := entity.Price()
	var currencyID *string
	if price != nil && price.Currency().Code != "" {
		code := price.Currency().Code
		currencyID = &code
	}

	var priceAmount int64
	if price != nil {
		priceAmount = price.Amount()
	}

	return &models.Inventory{
		ID:          entity.ID().String(),
		TenantID:    entity.TenantID().String(),
		Name:        entity.Name(),
		Description: mapping.ValueToSQLNullString(entity.Description()),
		CurrencyID:  mapping.PointerToSQLNullString(currencyID),
		Price:       priceAmount,
		Quantity:    entity.Quantity(),
		CreatedAt:   entity.CreatedAt(),
		UpdatedAt:   entity.UpdatedAt(),
	}
}

func ToDomainInventory(dbInventory *models.Inventory) (inventory.Inventory, error) {
	id, err := uuid.Parse(dbInventory.ID)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(dbInventory.TenantID)
	if err != nil {
		return nil, err
	}

	// Create price Money object
	var price *money.Money
	if dbInventory.CurrencyID.Valid && dbInventory.CurrencyID.String != "" {
		price = money.New(dbInventory.Price, dbInventory.CurrencyID.String)
	} else {
		price = money.New(dbInventory.Price, "USD") // Default currency
	}

	opts := []inventory.Option{
		inventory.WithID(id),
		inventory.WithTenantID(tenantID),
		inventory.WithCreatedAt(dbInventory.CreatedAt),
		inventory.WithUpdatedAt(dbInventory.UpdatedAt),
	}

	if dbInventory.Description.Valid {
		opts = append(opts, inventory.WithDescription(dbInventory.Description.String))
	}

	return inventory.New(
		dbInventory.Name,
		price,
		dbInventory.Quantity,
		opts...,
	), nil
}

func ToDBDebt(entity debt.Debt) *models.Debt {
	originalAmount := entity.OriginalAmount()
	outstandingAmount := entity.OutstandingAmount()

	return &models.Debt{
		ID:                       entity.ID().String(),
		TenantID:                 entity.TenantID().String(),
		Type:                     string(entity.Type()),
		Status:                   string(entity.Status()),
		CounterpartyID:           entity.CounterpartyID().String(),
		OriginalAmount:           originalAmount.Amount(),
		OriginalAmountCurrencyID: originalAmount.Currency().Code,
		OutstandingAmount:        outstandingAmount.Amount(),
		OutstandingCurrencyID:    outstandingAmount.Currency().Code,
		Description:              entity.Description(),
		DueDate:                  mapping.PointerToSQLNullTime(entity.DueDate()),
		SettlementTransactionID:  uuidPointerToSQLNullString(entity.SettlementTransactionID()),
		CreatedAt:                entity.CreatedAt(),
		UpdatedAt:                entity.UpdatedAt(),
	}
}

func uuidPointerToSQLNullString(id *uuid.UUID) sql.NullString {
	if id != nil {
		return sql.NullString{
			String: id.String(),
			Valid:  true,
		}
	}
	return sql.NullString{
		String: "",
		Valid:  false,
	}
}

func ToDomainDebt(dbDebt *models.Debt) (debt.Debt, error) {
	debtType := debt.DebtType(dbDebt.Type)
	status := debt.DebtStatus(dbDebt.Status)

	tenantID, err := uuid.Parse(dbDebt.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse tenant ID")
	}

	counterpartyID, err := uuid.Parse(dbDebt.CounterpartyID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse counterparty ID")
	}

	originalAmount := money.New(dbDebt.OriginalAmount, dbDebt.OriginalAmountCurrencyID)
	outstandingAmount := money.New(dbDebt.OutstandingAmount, dbDebt.OutstandingCurrencyID)

	opts := []debt.Option{
		debt.WithID(uuid.MustParse(dbDebt.ID)),
		debt.WithTenantID(tenantID),
		debt.WithCounterpartyID(counterpartyID),
		debt.WithDescription(dbDebt.Description),
		debt.WithCreatedAt(dbDebt.CreatedAt),
		debt.WithUpdatedAt(dbDebt.UpdatedAt),
	}

	if dbDebt.DueDate.Valid {
		opts = append(opts, debt.WithDueDate(&dbDebt.DueDate.Time))
	}

	if dbDebt.SettlementTransactionID.Valid {
		transactionID := uuid.MustParse(dbDebt.SettlementTransactionID.String)
		opts = append(opts, debt.WithSettlementTransactionID(&transactionID))
	}

	domainDebt := debt.New(debtType, originalAmount, opts...)
	domainDebt = domainDebt.UpdateStatus(status)
	domainDebt = domainDebt.UpdateOutstandingAmount(outstandingAmount)

	return domainDebt, nil
}

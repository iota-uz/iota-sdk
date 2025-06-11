package persistence

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func toDBTransaction(entity transaction.Transaction) *models.Transaction {
	return &models.Transaction{
		ID:                   entity.ID(),
		TenantID:             entity.TenantID().String(),
		Amount:               entity.Amount(),
		Comment:              entity.Comment(),
		AccountingPeriod:     entity.AccountingPeriod(),
		TransactionDate:      entity.TransactionDate(),
		DestinationAccountID: entity.DestinationAccountID(),
		OriginAccountID:      entity.OriginAccountID(),
		TransactionType:      string(entity.TransactionType()),
		CreatedAt:            entity.CreatedAt(),
	}
}

func toDomainTransaction(dbTransaction *models.Transaction) (transaction.Transaction, error) {
	_type, err := transaction.NewType(dbTransaction.TransactionType)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(dbTransaction.TenantID)
	if err != nil {
		return nil, err
	}

	t := transaction.New(
		dbTransaction.Amount,
		_type,
		transaction.WithID(dbTransaction.ID),
		transaction.WithTenantID(tenantID),
		transaction.WithComment(dbTransaction.Comment),
		transaction.WithAccountingPeriod(dbTransaction.AccountingPeriod),
		transaction.WithTransactionDate(dbTransaction.TransactionDate),
		transaction.WithOriginAccountID(dbTransaction.OriginAccountID),
		transaction.WithDestinationAccountID(dbTransaction.DestinationAccountID),
		transaction.WithCreatedAt(dbTransaction.CreatedAt),
	)

	return t, nil
}

func toDBPayment(entity payment.Payment) (*models.Payment, *models.Transaction) {
	dbTransaction := &models.Transaction{
		ID:                   entity.TransactionID(),
		TenantID:             entity.TenantID().String(),
		Amount:               entity.Amount(),
		Comment:              entity.Comment(),
		AccountingPeriod:     entity.AccountingPeriod(),
		TransactionDate:      entity.TransactionDate(),
		OriginAccountID:      uuid.Nil,
		DestinationAccountID: entity.Account().ID(),
		TransactionType:      string(transaction.Deposit),
		CreatedAt:            entity.CreatedAt(),
	}
	dbPayment := &models.Payment{
		ID:             entity.ID(),
		TransactionID:  entity.TransactionID(),
		CounterpartyID: entity.CounterpartyID(),
		CreatedAt:      entity.CreatedAt(),
		UpdatedAt:      entity.UpdatedAt(),
	}
	return dbPayment, dbTransaction
}

// TODO: populate user && account
func toDomainPayment(dbPayment *models.Payment, dbTransaction *models.Transaction) (payment.Payment, error) {
	t, err := toDomainTransaction(dbTransaction)
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

	// Create a default payment category
	defaultCategory := paymentcategory.New("Uncategorized")

	return payment.New(
		t.Amount(),
		defaultCategory,
		payment.WithID(dbPayment.ID),
		payment.WithTenantID(tenantID),
		payment.WithTransactionID(t.ID()),
		payment.WithCounterpartyID(dbPayment.CounterpartyID),
		payment.WithComment(t.Comment()),
		payment.WithAccount(moneyaccount.New("", currency.Currency{}, moneyaccount.WithID(t.DestinationAccountID()))),
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

func toDBExpenseCategory(entity category.ExpenseCategory) *models.ExpenseCategory {
	return &models.ExpenseCategory{
		ID:          entity.ID(),
		TenantID:    entity.TenantID().String(),
		Name:        entity.Name(),
		Description: mapping.ValueToSQLNullString(entity.Description()),
		CreatedAt:   entity.CreatedAt(),
		UpdatedAt:   entity.UpdatedAt(),
	}
}

func toDomainExpenseCategory(dbCategory *models.ExpenseCategory) (category.ExpenseCategory, error) {
	tenantID, err := uuid.Parse(dbCategory.TenantID)
	if err != nil {
		return nil, err
	}

	opts := []category.Option{
		category.WithID(dbCategory.ID),
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

func toDomainMoneyAccount(dbAccount *models.MoneyAccount) (moneyaccount.Account, error) {
	currencyEntity, err := corepersistence.ToDomainCurrency(dbAccount.Currency)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(dbAccount.TenantID)
	if err != nil {
		return nil, err
	}

	return moneyaccount.New(
		dbAccount.Name,
		*currencyEntity,
		moneyaccount.WithID(dbAccount.ID),
		moneyaccount.WithTenantID(tenantID),
		moneyaccount.WithAccountNumber(dbAccount.AccountNumber),
		moneyaccount.WithBalance(dbAccount.Balance),
		moneyaccount.WithDescription(dbAccount.Description),
		moneyaccount.WithCreatedAt(dbAccount.CreatedAt),
		moneyaccount.WithUpdatedAt(dbAccount.UpdatedAt),
	), nil
}

func toDBMoneyAccount(entity moneyaccount.Account) *models.MoneyAccount {
	currency := entity.Currency()
	return &models.MoneyAccount{
		ID:                entity.ID(),
		TenantID:          entity.TenantID().String(),
		Name:              entity.Name(),
		AccountNumber:     entity.AccountNumber(),
		Balance:           entity.Balance(),
		BalanceCurrencyID: string(currency.Code),
		Currency:          corepersistence.ToDBCurrency(&currency),
		Description:       entity.Description(),
		CreatedAt:         entity.CreatedAt(),
		UpdatedAt:         entity.UpdatedAt(),
	}
}

func toDomainExpense(dbExpense *models.Expense, dbTransaction *models.Transaction) (expense.Expense, error) {
	tenantID, err := uuid.Parse(dbTransaction.TenantID)
	if err != nil {
		return nil, err
	}

	account := moneyaccount.New("", currency.Currency{}, moneyaccount.WithID(dbTransaction.OriginAccountID))
	expenseCategory := category.New(
		"", // name - will be populated when actual category is fetched
		category.WithID(dbExpense.CategoryID),
		category.WithTenantID(tenantID),
		category.WithCreatedAt(dbExpense.CreatedAt),
		category.WithUpdatedAt(dbExpense.UpdatedAt),
	)

	domainExpense := expense.New(
		-1*dbTransaction.Amount,
		account,
		expenseCategory,
		dbTransaction.TransactionDate,
		expense.WithID(dbExpense.ID),
		expense.WithComment(dbTransaction.Comment),
		expense.WithTransactionID(dbExpense.TransactionID),
		expense.WithAccountingPeriod(dbTransaction.AccountingPeriod),
		expense.WithCreatedAt(dbExpense.CreatedAt),
		expense.WithUpdatedAt(dbExpense.UpdatedAt),
	)

	return domainExpense, nil
}

func toDBExpense(entity expense.Expense) (*models.Expense, transaction.Transaction) {
	accountID := entity.Account().ID()
	domainTransaction := transaction.New(
		-1*entity.Amount(),
		transaction.Withdrawal,
		transaction.WithID(entity.TransactionID()),
		transaction.WithComment(entity.Comment()),
		transaction.WithAccountingPeriod(entity.AccountingPeriod()),
		transaction.WithTransactionDate(entity.Date()),
		transaction.WithOriginAccountID(accountID),
		transaction.WithCreatedAt(entity.CreatedAt()),
	)
	dbExpense := &models.Expense{
		ID:            entity.ID(),
		CategoryID:    entity.Category().ID(),
		TransactionID: entity.TransactionID(),
		CreatedAt:     entity.CreatedAt(),
		UpdatedAt:     entity.UpdatedAt(),
	}
	return dbExpense, domainTransaction
}

func toDomainCounterparty(dbRow *models.Counterparty) (counterparty.Counterparty, error) {
	partyType, err := counterparty.NewType(dbRow.Type)
	if err != nil {
		return nil, err
	}
	legalType, err := counterparty.NewLegalType(dbRow.LegalType)
	if err != nil {
		return nil, err
	}
	t, err := tax.NewTin(dbRow.Tin, country.Uzbekistan)
	if err != nil {
		return nil, err
	}
	return counterparty.NewWithID(
		dbRow.ID,
		t,
		dbRow.Name,
		partyType,
		legalType,
		dbRow.LegalAddress,
		dbRow.CreatedAt,
		dbRow.UpdatedAt,
	), nil
}

func toDBCounterparty(entity counterparty.Counterparty) (*models.Counterparty, error) {
	return &models.Counterparty{
		ID:           entity.ID(),
		Tin:          entity.Tin().Value(),
		Name:         entity.Name(),
		Type:         string(entity.Type()),
		LegalType:    string(entity.LegalType()),
		LegalAddress: entity.LegalAddress(),
		CreatedAt:    entity.CreatedAt(),
		UpdatedAt:    entity.UpdatedAt(),
	}, nil
}

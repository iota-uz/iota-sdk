package persistence

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func toDBTransaction(entity *transaction.Transaction) *models.Transaction {
	return &models.Transaction{
		ID:                   entity.ID,
		Amount:               entity.Amount,
		Comment:              entity.Comment,
		AccountingPeriod:     entity.AccountingPeriod,
		TransactionDate:      entity.TransactionDate,
		DestinationAccountID: entity.DestinationAccountID,
		OriginAccountID:      entity.OriginAccountID,
		TransactionType:      string(entity.TransactionType),
		CreatedAt:            entity.CreatedAt,
	}
}

func toDomainTransaction(dbTransaction *models.Transaction) (*transaction.Transaction, error) {
	_type, err := transaction.NewType(dbTransaction.TransactionType)
	if err != nil {
		return nil, err
	}

	return &transaction.Transaction{
		ID:                   dbTransaction.ID,
		Amount:               dbTransaction.Amount,
		TransactionType:      _type,
		Comment:              dbTransaction.Comment,
		AccountingPeriod:     dbTransaction.AccountingPeriod,
		TransactionDate:      dbTransaction.TransactionDate,
		DestinationAccountID: dbTransaction.DestinationAccountID,
		OriginAccountID:      dbTransaction.OriginAccountID,
		CreatedAt:            dbTransaction.CreatedAt,
	}, nil
}

func toDBPayment(entity payment.Payment) (*models.Payment, *models.Transaction) {
	dbTransaction := &models.Transaction{
		ID:                   entity.TransactionID(),
		Amount:               entity.Amount(),
		Comment:              entity.Comment(),
		AccountingPeriod:     entity.AccountingPeriod(),
		TransactionDate:      entity.TransactionDate(),
		OriginAccountID:      nil,
		DestinationAccountID: &entity.Account().ID,
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

	return payment.NewWithID(
		dbPayment.ID,
		t.Amount,
		t.ID,
		dbPayment.CounterpartyID,
		t.Comment,
		&moneyaccount.Account{ID: *t.DestinationAccountID}, //nolint:exhaustruct
		user.New(
			"", // firstName
			"", // lastName
			email,
			"", // uiLanguage
		),
		t.TransactionDate,
		t.AccountingPeriod,
		dbPayment.CreatedAt,
		dbPayment.UpdatedAt,
	), nil
}

func toDBExpenseCategory(entity category.ExpenseCategory) *models.ExpenseCategory {
	return &models.ExpenseCategory{
		ID:               entity.ID(),
		Name:             entity.Name(),
		Description:      mapping.ValueToSQLNullString(entity.Description()),
		Amount:           entity.Amount(),
		AmountCurrencyID: string(entity.Currency().Code),
		CreatedAt:        entity.CreatedAt(),
		UpdatedAt:        entity.UpdatedAt(),
	}
}

func toDomainExpenseCategory(dbCategory *models.ExpenseCategory, dbCurrency *coremodels.Currency) (category.ExpenseCategory, error) {
	domainCurrency, err := corepersistence.ToDomainCurrency(dbCurrency)
	if err != nil {
		return nil, err
	}
	opts := []category.Option{
		category.WithID(dbCategory.ID),
		category.WithCreatedAt(dbCategory.CreatedAt),
		category.WithUpdatedAt(dbCategory.UpdatedAt),
	}

	if dbCategory.Description.Valid {
		opts = append(opts, category.WithDescription(dbCategory.Description.String))
	}

	return category.New(
		dbCategory.Name,
		dbCategory.Amount,
		domainCurrency,
		opts...,
	), nil
}

func toDomainMoneyAccount(dbAccount *models.MoneyAccount) (*moneyaccount.Account, error) {
	currencyEntity, err := corepersistence.ToDomainCurrency(dbAccount.Currency)
	if err != nil {
		return nil, err
	}
	return &moneyaccount.Account{
		ID:            dbAccount.ID,
		Name:          dbAccount.Name,
		AccountNumber: dbAccount.AccountNumber,
		Balance:       dbAccount.Balance,
		Currency:      *currencyEntity,
		Description:   dbAccount.Description,
		CreatedAt:     dbAccount.CreatedAt,
		UpdatedAt:     dbAccount.UpdatedAt,
	}, nil
}

func toDBMoneyAccount(entity *moneyaccount.Account) *models.MoneyAccount {
	return &models.MoneyAccount{
		ID:                entity.ID,
		Name:              entity.Name,
		AccountNumber:     entity.AccountNumber,
		Balance:           entity.Balance,
		BalanceCurrencyID: string(entity.Currency.Code),
		Currency:          corepersistence.ToDBCurrency(&entity.Currency),
		Description:       entity.Description,
		CreatedAt:         entity.CreatedAt,
		UpdatedAt:         entity.UpdatedAt,
	}
}

func toDomainExpense(dbExpense *models.Expense, dbTransaction *models.Transaction) (expense.Expense, error) {
	account := moneyaccount.Account{ID: *dbTransaction.OriginAccountID} //nolint:exhaustruct
	expenseCategory := category.New(
		"",  // name - will be populated when actual category is fetched
		0.0, // amount - will be populated when actual category is fetched
		nil, // currency - will be populated when actual category is fetched
		category.WithID(dbExpense.CategoryID),
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

func toDBExpense(entity expense.Expense) (*models.Expense, *transaction.Transaction) {
	domainTransaction := &transaction.Transaction{
		ID:                   entity.TransactionID(),
		Amount:               -1 * entity.Amount(),
		Comment:              entity.Comment(),
		AccountingPeriod:     entity.AccountingPeriod(),
		TransactionDate:      entity.Date(),
		OriginAccountID:      mapping.Pointer(entity.Account().ID),
		DestinationAccountID: nil,
		TransactionType:      transaction.Withdrawal,
		CreatedAt:            entity.CreatedAt(),
	}
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

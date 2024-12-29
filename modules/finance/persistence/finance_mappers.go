package persistence

import (
	"errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyAccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/persistence/models"
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
		TransactionType:      entity.TransactionType.String(),
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

func toDBPayment(entity *payment.Payment) (*models.Payment, *models.Transaction) {
	dbTransaction := &models.Transaction{
		ID:                   entity.TransactionID,
		Amount:               entity.Amount,
		Comment:              entity.Comment,
		AccountingPeriod:     entity.AccountingPeriod,
		TransactionDate:      entity.TransactionDate,
		OriginAccountID:      nil,
		DestinationAccountID: &entity.Account.ID,
		TransactionType:      transaction.Income.String(),
		CreatedAt:            entity.CreatedAt,
	}
	dbPayment := &models.Payment{
		ID:            entity.ID,
		TransactionID: entity.TransactionID,
		Transaction:   dbTransaction,
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
	}
	return dbPayment, dbTransaction
}

func toDomainPayment(dbPayment *models.Payment) (*payment.Payment, error) {
	if dbPayment.Transaction == nil {
		return nil, errors.New("transaction is nil")
	}
	t, err := toDomainTransaction(dbPayment.Transaction)
	if err != nil {
		return nil, err
	}
	return &payment.Payment{
		ID:               dbPayment.ID,
		Amount:           t.Amount,
		Comment:          t.Comment,
		TransactionDate:  t.TransactionDate,
		AccountingPeriod: t.AccountingPeriod,
		User:             &user.User{},
		TransactionID:    dbPayment.TransactionID,
		Account:          moneyAccount.Account{ID: *t.DestinationAccountID},
		CreatedAt:        dbPayment.CreatedAt,
		UpdatedAt:        dbPayment.UpdatedAt,
	}, nil
}

func toDBExpenseCategory(entity *category.ExpenseCategory) *models.ExpenseCategory {
	return &models.ExpenseCategory{
		ID:               entity.ID,
		Name:             entity.Name,
		Description:      &entity.Description,
		Amount:           entity.Amount,
		AmountCurrencyID: string(entity.Currency.Code),
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func toDomainExpenseCategory(dbCategory *models.ExpenseCategory) (*category.ExpenseCategory, error) {
	return &category.ExpenseCategory{
		ID:          dbCategory.ID,
		Name:        dbCategory.Name,
		Description: mapping.Value(dbCategory.Description),
		Amount:      dbCategory.Amount,
		CreatedAt:   dbCategory.CreatedAt,
		UpdatedAt:   dbCategory.UpdatedAt,
	}, nil
}

func toDomainMoneyAccount(dbAccount *models.MoneyAccount) (*moneyAccount.Account, error) {
	currencyEntity, err := corepersistence.ToDomainCurrency(dbAccount.Currency)
	if err != nil {
		return nil, err
	}
	return &moneyAccount.Account{
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

func toDBMoneyAccount(entity *moneyAccount.Account) *models.MoneyAccount {
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

func toDomainExpense(dbExpense *models.Expense) (*expense.Expense, error) {
	return &expense.Expense{
		ID:               dbExpense.ID,
		Amount:           -1 * dbExpense.Transaction.Amount,
		Account:          moneyAccount.Account{ID: *dbExpense.Transaction.OriginAccountID},
		Comment:          dbExpense.Transaction.Comment,
		TransactionID:    dbExpense.TransactionID,
		AccountingPeriod: dbExpense.Transaction.AccountingPeriod,
		Date:             dbExpense.Transaction.TransactionDate,
		CreatedAt:        dbExpense.CreatedAt,
		UpdatedAt:        dbExpense.UpdatedAt,
	}, nil
}

func toDBExpense(entity *expense.Expense) (*models.Expense, *transaction.Transaction) {
	transaction := &transaction.Transaction{
		ID:                   entity.TransactionID,
		Amount:               -1 * entity.Amount,
		Comment:              entity.Comment,
		AccountingPeriod:     entity.AccountingPeriod,
		TransactionDate:      entity.Date,
		OriginAccountID:      &entity.Account.ID,
		DestinationAccountID: nil,
		TransactionType:      transaction.Expense,
		CreatedAt:            entity.CreatedAt,
	}
	dbExpense := &models.Expense{
		ID:            entity.ID,
		CategoryID:    entity.Category.ID,
		TransactionID: entity.TransactionID,
		Transaction:   nil,
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
	}
	return dbExpense, transaction
}

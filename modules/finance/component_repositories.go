package finance

import (
	debt "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	expense "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	payment "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	counterparty "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	inventory "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	transaction "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
)

func newMoneyAccountRepository() moneyaccount.Repository {
	return persistence.NewMoneyAccountRepository()
}

func newTransactionRepository() transaction.Repository {
	return persistence.NewTransactionRepository()
}

func newExpenseCategoryRepository() category.Repository {
	return persistence.NewExpenseCategoryRepository()
}

func newPaymentCategoryRepository() paymentcategory.Repository {
	return persistence.NewPaymentCategoryRepository()
}

func newPaymentRepository() payment.Repository {
	return persistence.NewPaymentRepository()
}

func newExpenseRepository(categoryRepo category.Repository, transactionRepo transaction.Repository) expense.Repository {
	return persistence.NewExpenseRepository(categoryRepo, transactionRepo)
}

func newCounterpartyRepository() counterparty.Repository {
	return persistence.NewCounterpartyRepository()
}

func newInventoryRepository() inventory.Repository {
	return persistence.NewInventoryRepository()
}

func newDebtRepository() debt.Repository {
	return persistence.NewDebtRepository()
}

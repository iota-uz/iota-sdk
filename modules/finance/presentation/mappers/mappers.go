package mappers

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
)

func ExpenseCategoryToViewModel(entity *category.ExpenseCategory) *viewmodels.ExpenseCategory {
	return &viewmodels.ExpenseCategory{
		ID:                 strconv.FormatUint(uint64(entity.ID), 10),
		Name:               entity.Name,
		Amount:             fmt.Sprintf("%.2f", entity.Amount),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount, entity.Currency.Symbol),
		CurrencyCode:       string(entity.Currency.Code),
		Description:        entity.Description,
		UpdatedAt:          entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:          entity.CreatedAt.Format(time.RFC3339),
	}
}

func MoneyAccountToViewModel(entity *moneyaccount.Account) *viewmodels.MoneyAccount {
	return &viewmodels.MoneyAccount{
		ID:                  strconv.FormatUint(uint64(entity.ID), 10),
		Name:                entity.Name,
		AccountNumber:       entity.AccountNumber,
		Balance:             fmt.Sprintf("%.2f", entity.Balance),
		BalanceWithCurrency: fmt.Sprintf("%.2f %s", entity.Balance, entity.Currency.Symbol),
		CurrencyCode:        string(entity.Currency.Code),
		CurrencySymbol:      string(entity.Currency.Symbol),
		Description:         entity.Description,
		UpdatedAt:           entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:           entity.CreatedAt.Format(time.RFC3339),
	}
}

func MoneyAccountToViewUpdateModel(entity *moneyaccount.Account) *viewmodels.MoneyAccountUpdateDTO {
	return &viewmodels.MoneyAccountUpdateDTO{
		Name:          entity.Name,
		Description:   entity.Description,
		AccountNumber: entity.AccountNumber,
		Balance:       fmt.Sprintf("%.2f", entity.Balance),
		CurrencyCode:  string(entity.Currency.Code),
	}
}

func PaymentToViewModel(entity payment.Payment) *viewmodels.Payment {
	currency := entity.Account().Currency
	return &viewmodels.Payment{
		ID:                 strconv.FormatUint(uint64(entity.ID()), 10),
		Amount:             fmt.Sprintf("%.2f", entity.Amount()),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount(), currency.Symbol),
		AccountID:          strconv.FormatUint(uint64(entity.Account().ID), 10),
		TransactionID:      strconv.FormatUint(uint64(entity.TransactionID()), 10),
		TransactionDate:    entity.TransactionDate().Format(time.RFC3339),
		AccountingPeriod:   entity.AccountingPeriod().Format(time.RFC3339),
		Comment:            entity.Comment(),
		CreatedAt:          entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:          entity.UpdatedAt().Format(time.RFC3339),
	}
}

func ExpenseToViewModel(entity *expense.Expense) *viewmodels.Expense {
	currencyEntity := entity.Category.Currency
	return &viewmodels.Expense{
		ID:                 strconv.FormatUint(uint64(entity.ID), 10),
		Amount:             fmt.Sprintf("%.2f", entity.Amount),
		AccountID:          strconv.FormatUint(uint64(entity.Account.ID), 10),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount, currencyEntity.Symbol),
		CategoryID:         strconv.FormatUint(uint64(entity.Category.ID), 10),
		Category:           ExpenseCategoryToViewModel(&entity.Category),
		Comment:            entity.Comment,
		TransactionID:      strconv.FormatUint(uint64(entity.TransactionID), 10),
		AccountingPeriod:   entity.AccountingPeriod.Format(time.RFC3339),
		Date:               entity.Date.Format(time.RFC3339),
		CreatedAt:          entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          entity.UpdatedAt.Format(time.RFC3339),
	}
}

func CounterpartyToViewModel(entity counterparty.Counterparty) *viewmodels.Counterparty {
	return &viewmodels.Counterparty{
		ID:           strconv.FormatUint(uint64(entity.ID()), 10),
		TIN:          entity.TIN(),
		Name:         entity.Name(),
		Type:         string(entity.Type()),
		LegalType:    string(entity.LegalType()),
		LegalAddress: entity.LegalAddress(),
		CreatedAt:    entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:    entity.UpdatedAt().Format(time.RFC3339),
	}
}

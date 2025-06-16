package mappers

import (
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
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

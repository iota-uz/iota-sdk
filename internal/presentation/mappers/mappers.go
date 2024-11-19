package mappers

import (
	"fmt"
	"github.com/iota-agency/iota-sdk/internal/domain/aggregates/expense"
	category "github.com/iota-agency/iota-sdk/internal/domain/aggregates/expense_category"
	"github.com/iota-agency/iota-sdk/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-sdk/internal/domain/aggregates/payment"
	"github.com/iota-agency/iota-sdk/internal/domain/aggregates/project"
	"github.com/iota-agency/iota-sdk/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/internal/domain/entities/currency"
	"github.com/iota-agency/iota-sdk/internal/domain/entities/employee"
	stage "github.com/iota-agency/iota-sdk/internal/domain/entities/project_stages"
	"github.com/iota-agency/iota-sdk/internal/presentation/viewmodels"
	"strconv"
	"time"
)

func UserToViewModel(entity *user.User) *viewmodels.User {
	return &viewmodels.User{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		FirstName:  entity.FirstName,
		LastName:   entity.LastName,
		MiddleName: entity.MiddleName,
		Email:      entity.Email,
		UILanguage: string(entity.UILanguage),
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
	}
}

func ExpenseCategoryToViewModel(entity *category.ExpenseCategory) *viewmodels.ExpenseCategory {
	return &viewmodels.ExpenseCategory{
		ID:                 strconv.FormatUint(uint64(entity.ID), 10),
		Name:               entity.Name,
		Amount:             fmt.Sprintf("%.2f", entity.Amount),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount, entity.Currency.Symbol),
		Description:        entity.Description,
		UpdatedAt:          entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:          entity.CreatedAt.Format(time.RFC3339),
	}
}

func MoneyAccountToViewModel(entity *moneyaccount.Account, editURL string) *viewmodels.MoneyAccount {
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
		EditURL:             editURL,
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

func ProjectStageToViewModel(entity *stage.ProjectStage) *viewmodels.ProjectStage {
	return &viewmodels.ProjectStage{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		Name:      entity.Name,
		ProjectID: strconv.FormatUint(uint64(entity.ProjectID), 10),
		Margin:    fmt.Sprintf("%.2f", entity.Margin),
		Risks:     fmt.Sprintf("%.2f", entity.Risks),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
	}
}

func PaymentToViewModel(entity *payment.Payment) *viewmodels.Payment {
	currency := entity.Account.Currency
	return &viewmodels.Payment{
		ID:                 strconv.FormatUint(uint64(entity.ID), 10),
		Amount:             fmt.Sprintf("%.2f", entity.Amount),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount, currency.Symbol),
		AccountID:          strconv.FormatUint(uint64(entity.Account.ID), 10),
		TransactionID:      strconv.FormatUint(uint64(entity.TransactionID), 10),
		StageID:            strconv.FormatUint(uint64(entity.StageID), 10),
		TransactionDate:    entity.TransactionDate.Format(time.RFC3339),
		AccountingPeriod:   entity.AccountingPeriod.Format(time.RFC3339),
		Comment:            entity.Comment,
		CreatedAt:          entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          entity.UpdatedAt.Format(time.RFC3339),
	}
}

func ExpenseToViewModel(entity *expense.Expense) *viewmodels.Expense {
	currency := entity.Category.Currency
	return &viewmodels.Expense{
		ID:                 strconv.FormatUint(uint64(entity.ID), 10),
		Amount:             fmt.Sprintf("%.2f", entity.Amount),
		AccountID:          strconv.FormatUint(uint64(entity.Account.ID), 10),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount, currency.Symbol),
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

func CurrencyToViewModel(entity *currency.Currency) *viewmodels.Currency {
	return &viewmodels.Currency{
		Code:   string(entity.Code),
		Name:   entity.Name,
		Symbol: string(entity.Symbol),
	}
}

func ProjectToViewModel(entity *project.Project) *viewmodels.Project {
	return &viewmodels.Project{
		ID:          strconv.FormatUint(uint64(entity.ID), 10),
		Name:        entity.Name,
		Description: entity.Description,
		UpdatedAt:   entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:   entity.CreatedAt.Format(time.RFC3339),
	}
}

func EmployeeToViewModel(entity *employee.Employee) *viewmodels.Employee {
	return &viewmodels.Employee{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		FirstName: entity.FirstName,
		LastName:  entity.LastName,
		Email:     entity.Email,
		Phone:     entity.Phone,
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
	}
}

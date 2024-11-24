package mappers

import (
	"fmt"
	"strconv"
	"time"

	category "github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/project"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/employee"
	stage "github.com/iota-agency/iota-sdk/pkg/domain/entities/project_stages"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/pkg/presentation/viewmodels"
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

func UploadToViewModel(entity *upload.Upload) *viewmodels.Upload {
	return &viewmodels.Upload{
		ID:        entity.ID,
		URL:       entity.URL,
		Name:      entity.Name,
		Mimetype:  entity.Mimetype.String(),
		Size:      strconv.Itoa(entity.Size),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}
}

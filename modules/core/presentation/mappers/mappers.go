package mappers

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/employee"
	stage "github.com/iota-uz/iota-sdk/modules/core/domain/entities/project_stages"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	viewmodels2 "github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"slices"
	"strconv"
	"time"

	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
)

func UserToViewModel(entity *user.User) *viewmodels2.User {
	var avatarId string
	if v := entity.AvatarID; v != nil {
		avatarId = strconv.Itoa(int(*v))
	}
	var avatar viewmodels2.Upload
	if entity.Avatar != nil {
		avatar = *UploadToViewModel(entity.Avatar)
	}
	return &viewmodels2.User{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		FirstName:  entity.FirstName,
		LastName:   entity.LastName,
		MiddleName: entity.MiddleName,
		Email:      entity.Email,
		Avatar:     &avatar,
		UILanguage: string(entity.UILanguage),
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
		AvatarID:   avatarId,
	}
}

func ExpenseCategoryToViewModel(entity *category.ExpenseCategory) *viewmodels2.ExpenseCategory {
	return &viewmodels2.ExpenseCategory{
		ID:                 strconv.FormatUint(uint64(entity.ID), 10),
		Name:               entity.Name,
		Amount:             fmt.Sprintf("%.2f", entity.Amount),
		AmountWithCurrency: fmt.Sprintf("%.2f %s", entity.Amount, entity.Currency.Symbol),
		Description:        entity.Description,
		UpdatedAt:          entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:          entity.CreatedAt.Format(time.RFC3339),
	}
}

func MoneyAccountToViewModel(entity *moneyaccount.Account) *viewmodels2.MoneyAccount {
	return &viewmodels2.MoneyAccount{
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

func MoneyAccountToViewUpdateModel(entity *moneyaccount.Account) *viewmodels2.MoneyAccountUpdateDTO {
	return &viewmodels2.MoneyAccountUpdateDTO{
		Name:          entity.Name,
		Description:   entity.Description,
		AccountNumber: entity.AccountNumber,
		Balance:       fmt.Sprintf("%.2f", entity.Balance),
		CurrencyCode:  string(entity.Currency.Code),
	}
}

func ProjectStageToViewModel(entity *stage.ProjectStage) *viewmodels2.ProjectStage {
	return &viewmodels2.ProjectStage{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		Name:      entity.Name,
		ProjectID: strconv.FormatUint(uint64(entity.ProjectID), 10),
		Margin:    fmt.Sprintf("%.2f", entity.Margin),
		Risks:     fmt.Sprintf("%.2f", entity.Risks),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
	}
}

func ProjectToViewModel(entity *project.Project) *viewmodels2.Project {
	return &viewmodels2.Project{
		ID:          strconv.FormatUint(uint64(entity.ID), 10),
		Name:        entity.Name,
		Description: entity.Description,
		UpdatedAt:   entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt:   entity.CreatedAt.Format(time.RFC3339),
	}
}

func EmployeeToViewModel(entity *employee.Employee) *viewmodels2.Employee {
	return &viewmodels2.Employee{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		FirstName: entity.FirstName,
		LastName:  entity.LastName,
		Email:     entity.Email,
		Phone:     entity.Phone,
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
	}
}

func UploadToViewModel(entity *upload.Upload) *viewmodels2.Upload {
	var url string
	if slices.Contains([]string{".xls", ".xlsx"}, entity.Mimetype.Extension()) {
		url = "/assets/" + assets.HashFS.HashName("images/excel-logo.svg")
	} else {
		url = "/" + entity.Path
	}

	return &viewmodels2.Upload{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		Hash:      entity.Hash,
		URL:       url,
		Mimetype:  entity.Mimetype.String(),
		Size:      strconv.Itoa(entity.Size),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}
}

func CurrencyToViewModel(entity *currency.Currency) *viewmodels2.Currency {
	return &viewmodels2.Currency{
		Code:   string(entity.Code),
		Name:   entity.Name,
		Symbol: string(entity.Symbol),
	}
}

func TabToViewModel(entity *tab.Tab) *viewmodels2.Tab {
	return &viewmodels2.Tab{
		ID:   strconv.FormatUint(uint64(entity.ID), 10),
		Href: entity.Href,
	}
}

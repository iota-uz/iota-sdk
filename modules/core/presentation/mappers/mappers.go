package mappers

import (
	"slices"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func UserToViewModel(entity user.User) *viewmodels.User {
	var avatar *viewmodels.Upload
	if entity.Avatar() != nil {
		avatar = UploadToViewModel(entity.Avatar())
	}
	return &viewmodels.User{
		ID:         strconv.FormatUint(uint64(entity.ID()), 10),
		FirstName:  entity.FirstName(),
		LastName:   entity.LastName(),
		MiddleName: entity.MiddleName(),
		Email:      entity.Email(),
		Avatar:     avatar,
		UILanguage: string(entity.UILanguage()),
		LastAction: entity.LastAction().Format(time.RFC3339),
		CreatedAt:  entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt().Format(time.RFC3339),
		Roles:      mapping.MapViewModels(entity.Roles(), RoleToViewModel),
		AvatarID:   strconv.Itoa(int(entity.AvatarID())),
	}
}

func EmployeeToViewModel(entity employee.Employee) *viewmodels.Employee {
	var email string
	if entity.Email() != nil {
		email = entity.Email().Value()
	}
	return &viewmodels.Employee{
		ID:              strconv.FormatUint(uint64(entity.ID()), 10),
		FirstName:       entity.FirstName(),
		LastName:        entity.LastName(),
		Email:           email,
		Salary:          strconv.FormatFloat(entity.Salary().Value(), 'f', 2, 64),
		Phone:           entity.Phone(),
		BirthDate:       entity.BirthDate().Format(time.DateOnly),
		HireDate:        entity.HireDate().Format(time.DateOnly),
		ResignationDate: entity.BirthDate().Format(time.DateOnly),
		Notes:           entity.Notes(),
		UpdatedAt:       entity.UpdatedAt().Format(time.RFC3339),
		CreatedAt:       entity.CreatedAt().Format(time.RFC3339),
	}
}

func UploadToViewModel(entity *upload.Upload) *viewmodels.Upload {
	var url string
	// TODO: this is gotta be implemented better
	if slices.Contains([]string{".xls", ".xlsx"}, entity.Mimetype.Extension()) {
		url = "/assets/" + assets.HashFS.HashName("images/excel-logo.svg")
	} else if entity.Path != "" {
		url = "/" + entity.Path
	}

	return &viewmodels.Upload{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		Hash:      entity.Hash,
		URL:       url,
		Mimetype:  entity.Mimetype.String(),
		Size:      strconv.Itoa(entity.Size),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}
}

func CurrencyToViewModel(entity *currency.Currency) *viewmodels.Currency {
	return &viewmodels.Currency{
		Code:   string(entity.Code),
		Name:   entity.Name,
		Symbol: string(entity.Symbol),
	}
}

func TabToViewModel(entity *tab.Tab) *viewmodels.Tab {
	return &viewmodels.Tab{
		ID:   strconv.FormatUint(uint64(entity.ID), 10),
		Href: entity.Href,
	}
}

func RoleToViewModel(entity role.Role) *viewmodels.Role {
	return &viewmodels.Role{
		ID:          strconv.FormatUint(uint64(entity.ID()), 10),
		Name:        entity.Name(),
		Description: entity.Description(),
		CreatedAt:   entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   entity.UpdatedAt().Format(time.RFC3339),
	}
}

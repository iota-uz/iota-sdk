package mappers

import (
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
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
		Email:      entity.Email().Value(),
		Avatar:     avatar,
		UILanguage: string(entity.UILanguage()),
		LastAction: entity.LastAction().Format(time.RFC3339),
		CreatedAt:  entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt().Format(time.RFC3339),
		Roles:      mapping.MapViewModels(entity.Roles(), RoleToViewModel),
		AvatarID:   strconv.Itoa(int(entity.AvatarID())),
	}
}

func UploadToViewModel(entity upload.Upload) *viewmodels.Upload {
	return &viewmodels.Upload{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Hash:      entity.Hash(),
		URL:       entity.PreviewURL(),
		Mimetype:  entity.Mimetype().String(),
		Size:      entity.Size().String(),
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt().Format(time.RFC3339),
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

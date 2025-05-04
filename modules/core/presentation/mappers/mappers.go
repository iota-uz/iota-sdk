package mappers

import (
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
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

	phone := ""
	if entity.Phone() != nil {
		phone = entity.Phone().Value()
	}

	var groupIDs []string
	if entity.GroupIDs() != nil {
		groupIDs = make([]string, len(entity.GroupIDs()))
		for i, groupID := range entity.GroupIDs() {
			groupIDs[i] = groupID.String()
		}
	}

	return &viewmodels.User{
		ID:          strconv.FormatUint(uint64(entity.ID()), 10),
		Type:        string(entity.Type()),
		FirstName:   entity.FirstName(),
		LastName:    entity.LastName(),
		MiddleName:  entity.MiddleName(),
		Email:       entity.Email().Value(),
		Phone:       phone,
		Avatar:      avatar,
		Language:    string(entity.UILanguage()),
		LastAction:  entity.LastAction().Format(time.RFC3339),
		CreatedAt:   entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   entity.UpdatedAt().Format(time.RFC3339),
		Roles:       mapping.MapViewModels(entity.Roles(), RoleToViewModel),
		GroupIDs:    groupIDs,
		Permissions: mapping.MapViewModels(entity.Permissions(), PermissionToViewModel),
		AvatarID:    strconv.Itoa(int(entity.AvatarID())),
		CanUpdate:   entity.CanUpdate(),
		CanDelete:   entity.CanDelete(),
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
		Type:        string(entity.Type()),
		Name:        entity.Name(),
		Description: entity.Description(),
		CreatedAt:   entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   entity.UpdatedAt().Format(time.RFC3339),
		CanUpdate:   entity.CanUpdate(),
		CanDelete:   entity.CanDelete(),
	}
}

func PermissionToViewModel(entity *permission.Permission) *viewmodels.Permission {
	return &viewmodels.Permission{
		ID:       entity.ID.String(),
		Name:     entity.Name,
		Resource: string(entity.Resource),
		Action:   string(entity.Action),
		Modifier: string(entity.Modifier),
	}
}

func GroupToViewModel(entity group.Group) *viewmodels.Group {
	return &viewmodels.Group{
		ID:          entity.ID().String(),
		Type:        string(entity.Type()),
		Name:        entity.Name(),
		Description: entity.Description(),
		Roles:       mapping.MapViewModels(entity.Roles(), RoleToViewModel),
		Users:       mapping.MapViewModels(entity.Users(), UserToViewModel),
		CreatedAt:   entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   entity.UpdatedAt().Format(time.RFC3339),
		CanUpdate:   entity.CanUpdate(),
		CanDelete:   entity.CanDelete(),
	}
}

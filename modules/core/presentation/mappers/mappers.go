// Package mappers provides this package.
package mappers

import (
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
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

	blockedAt := ""
	if !entity.BlockedAt().IsZero() {
		blockedAt = entity.BlockedAt().Format(time.RFC3339)
	}

	blockedBy := ""
	if entity.BlockedBy() != 0 {
		blockedBy = strconv.FormatUint(uint64(entity.BlockedBy()), 10)
	}

	vm := &viewmodels.User{
		ID:            strconv.FormatUint(uint64(entity.ID()), 10),
		Type:          string(entity.Type()),
		FirstName:     entity.FirstName(),
		LastName:      entity.LastName(),
		MiddleName:    entity.MiddleName(),
		Email:         entity.Email().Value(),
		Phone:         phone,
		Avatar:        avatar,
		LastAction:    "",
		Language:      string(entity.UILanguage()),
		CreatedAt:     entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:     entity.UpdatedAt().Format(time.RFC3339),
		Roles:         mapping.MapViewModels(entity.Roles(), RoleToViewModel),
		GroupIDs:      groupIDs,
		Permissions:   mapping.MapViewModels(entity.Permissions(), PermissionToViewModel),
		AvatarID:      strconv.Itoa(int(entity.AvatarID())),
		CanUpdate:     entity.CanUpdate(),
		CanDelete:     entity.CanDelete(),
		IsBlocked:     entity.IsBlocked(),
		BlockReason:   entity.BlockReason(),
		BlockedAt:     blockedAt,
		BlockedBy:     blockedBy,
		BlockedByUser: "",
		CanBeBlocked:  entity.CanBeBlocked(),
	}

	if v := entity.LastAction(); !v.IsZero() {
		vm.LastAction = v.Format(time.RFC3339)
	}
	return vm
}

func UploadToViewModel(entity upload.Upload) *viewmodels.Upload {
	upload := &viewmodels.Upload{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Hash:      entity.Hash(),
		Slug:      entity.Slug(),
		Name:      entity.Name(),
		URL:       entity.PreviewURL(),
		Mimetype:  "",
		Size:      entity.Size().String(),
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt().Format(time.RFC3339),
	}
	if mime := entity.Mimetype(); mime != nil {
		upload.Mimetype = mime.String()
	}
	return upload
}

func CurrencyToViewModel(entity currency.Currency) *viewmodels.Currency {
	return &viewmodels.Currency{
		Code:   string(entity.Code()),
		Name:   entity.Name(),
		Symbol: string(entity.Symbol()),
	}
}

func RoleToViewModel(entity role.Role) *viewmodels.Role {
	return &viewmodels.Role{
		ID:          strconv.FormatUint(uint64(entity.ID()), 10),
		Type:        string(entity.Type()),
		Name:        entity.Name(),
		Description: entity.Description(),
		UsersCount:  0,
		CreatedAt:   entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   entity.UpdatedAt().Format(time.RFC3339),
		CanUpdate:   entity.CanUpdate(),
		CanDelete:   entity.CanDelete(),
	}
}

func PermissionToViewModel(entity permission.Permission) *viewmodels.Permission {
	return &viewmodels.Permission{
		ID:       entity.ID().String(),
		Name:     entity.Name(),
		Resource: string(entity.Resource()),
		Action:   string(entity.Action()),
		Modifier: string(entity.Modifier()),
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

// multiLangToMap returns the per-locale values of a MultiLang as a plain map,
// tolerating a nil value so callers can pre-fill forms without nil checks.
func multiLangToMap(ml interface{ GetAll() map[string]string }) map[string]string {
	if ml == nil {
		return map[string]string{}
	}
	all := ml.GetAll()
	if all == nil {
		return map[string]string{}
	}
	return all
}

// DepartmentToViewModel maps a department aggregate to its presentation model.
// locale resolves the display name; parentNames resolves the parent department
// label for the list view (nil-safe — an unknown parent renders blank).
func DepartmentToViewModel(
	entity department.Department,
	locale string,
	parentNames map[string]string,
) *viewmodels.Department {
	parentID := ""
	parentName := ""
	if entity.ParentID() != nil {
		parentID = entity.ParentID().String()
		if parentNames != nil {
			parentName = parentNames[parentID]
		}
	}
	return &viewmodels.Department{
		ID:         entity.ID().String(),
		Code:       entity.Code(),
		Name:       entity.Name(locale),
		NameI18n:   multiLangToMap(entity.NameI18n()),
		ParentID:   parentID,
		ParentName: parentName,
		Order:      strconv.Itoa(entity.Order()),
		Status:     string(entity.Status()),
		CreatedAt:  entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt().Format(time.RFC3339),
		CanUpdate:  true,
		CanDelete:  true,
	}
}

// UserPositionToViewModel maps a user position aggregate to its presentation
// model. locale resolves the display title; userNames and departmentNames
// resolve the linked labels for the list view (nil-safe).
func UserPositionToViewModel(
	entity userposition.UserPosition,
	locale string,
	userNames map[string]string,
	departmentNames map[string]string,
) *viewmodels.UserPosition {
	userID := strconv.FormatUint(uint64(entity.UserID()), 10)
	departmentID := entity.DepartmentID().String()
	userName := ""
	if userNames != nil {
		userName = userNames[userID]
	}
	departmentName := ""
	if departmentNames != nil {
		departmentName = departmentNames[departmentID]
	}
	return &viewmodels.UserPosition{
		ID:             entity.ID().String(),
		UserID:         userID,
		UserName:       userName,
		DepartmentID:   departmentID,
		DepartmentName: departmentName,
		Title:          entity.Title(locale),
		TitleI18n:      multiLangToMap(entity.TitleI18n()),
		IsManager:      entity.IsManager(),
		IsPrimary:      entity.IsPrimary(),
		Status:         string(entity.Status()),
		CreatedAt:      entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:      entity.UpdatedAt().Format(time.RFC3339),
		CanUpdate:      true,
		CanDelete:      true,
	}
}

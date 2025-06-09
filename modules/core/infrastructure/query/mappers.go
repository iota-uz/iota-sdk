package query

import (
	"fmt"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
)

func mapToUserViewModel(dbUser models.User, hasAvatar bool, avatar *models.Upload) viewmodels.User {
	user := viewmodels.User{
		ID:         strconv.FormatUint(uint64(dbUser.ID), 10),
		Type:       dbUser.Type,
		FirstName:  dbUser.FirstName,
		LastName:   dbUser.LastName,
		MiddleName: dbUser.MiddleName.String,
		Email:      dbUser.Email,
		Language:   dbUser.UILanguage,
		CreatedAt:  dbUser.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  dbUser.UpdatedAt.Format(time.RFC3339),
	}

	if dbUser.Phone.Valid {
		user.Phone = dbUser.Phone.String
	}

	if dbUser.LastAction.Valid {
		user.LastAction = dbUser.LastAction.Time.Format(time.RFC3339)
	}

	if dbUser.AvatarID.Valid {
		user.AvatarID = strconv.Itoa(int(dbUser.AvatarID.Int32))
	}

	// Map avatar if exists
	if hasAvatar && avatar != nil {
		user.Avatar = &viewmodels.Upload{
			ID:        strconv.FormatUint(uint64(avatar.ID), 10),
			Hash:      avatar.Hash,
			URL:       fmt.Sprintf("/uploads/%s", avatar.Path),
			Mimetype:  avatar.Mimetype,
			Size:      strconv.Itoa(avatar.Size),
			CreatedAt: avatar.CreatedAt.Format(time.RFC3339),
			UpdatedAt: avatar.UpdatedAt.Format(time.RFC3339),
		}
	}

	return user
}

func mapToRoleViewModel(dbRole models.Role) *viewmodels.Role {
	return &viewmodels.Role{
		ID:          strconv.FormatUint(uint64(dbRole.ID), 10),
		Type:        dbRole.Type,
		Name:        dbRole.Name,
		Description: dbRole.Description.String,
		CreatedAt:   dbRole.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dbRole.UpdatedAt.Format(time.RFC3339),
	}
}

func mapToPermissionViewModel(dbPerm models.Permission) *viewmodels.Permission {
	return &viewmodels.Permission{
		ID:       dbPerm.ID,
		Name:     dbPerm.Name,
		Resource: dbPerm.Resource,
		Action:   dbPerm.Action,
		Modifier: dbPerm.Modifier,
	}
}

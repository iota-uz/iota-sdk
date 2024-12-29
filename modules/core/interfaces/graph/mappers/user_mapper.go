package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	model "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph/gqlmodels"
)

func UserToGraphModel(u *user.User) *model.User {
	return &model.User{
		ID:         int64(u.ID),
		Email:      u.Email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		UILanguage: string(u.UILanguage),
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
	}
}

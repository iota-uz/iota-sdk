package mappers

import (
	model "github.com/iota-agency/iota-sdk/modules/core/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
)

func UserToGraphModel(u *user.User) *model.User {
	return &model.User{
		ID:        int64(u.ID),
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

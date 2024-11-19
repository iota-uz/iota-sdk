package mappers

import (
	"github.com/iota-agency/iota-sdk/elxolding/viewmodels"
	"github.com/iota-agency/iota-sdk/internal/domain/aggregates/user"
	"strconv"
	"time"
)

func UserToViewModel(entity *user.User) *viewmodels.User {
	return &viewmodels.User{
		ID:         strconv.FormatUint(uint64(entity.ID), 10),
		FirstName:  entity.FirstName,
		LastName:   entity.LastName,
		MiddleName: entity.MiddleName,
		UILanguage: string(entity.UILanguage),
		CreatedAt:  entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt.Format(time.RFC3339),
	}
}

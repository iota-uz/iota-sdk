package mappers

import (
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/modules/elxolding/viewmodels"
	"strconv"
	"time"
)

func UserToViewModel(entity *user.User) *viewmodels.User {
	return &viewmodels.User{
		ID:        strconv.FormatUint(uint64(entity.ID), 10),
		FirstName: entity.FirstName,
		LastName:  entity.LastName,
		Email:     entity.Email,
		//Phone:     entity.Phone,
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}
}

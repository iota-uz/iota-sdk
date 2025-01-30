package mappers

import (
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/viewmodels"
)

func ClientToViewModel(entity client.Client) *viewmodels.Client {
	return &viewmodels.Client{
		ID:         strconv.FormatUint(uint64(entity.ID()), 10),
		FirstName:  entity.FirstName(),
		LastName:   entity.LastName(),
		MiddleName: entity.MiddleName(),
		Phone:      entity.Phone().Value(),
		CreatedAt:  entity.CreatedAt().Format(time.RFC3339),
		UpdatedAt:  entity.UpdatedAt().Format(time.RFC3339),
	}
}

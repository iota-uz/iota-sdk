package mappers

import (
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
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

func MessageToViewModel(entity message.Message) *viewmodels.Message {
	return &viewmodels.Message{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Message:   entity.Message(),
		CreatedAt: entity.CreatedAt(),
	}
}

func ChatToViewModel(entity chat.Chat) *viewmodels.Chat {
	return &viewmodels.Chat{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Client:    ClientToViewModel(entity.Client()),
		Messages:  mapping.MapViewModels(entity.Messages(), MessageToViewModel),
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
	}
}

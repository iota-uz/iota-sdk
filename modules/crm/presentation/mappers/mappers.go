package mappers

import (
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message-template"
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

func initialsFromFullName(firstName, lastName string) string {
	res := ""
	if len(firstName) > 0 {
		res += string(firstName[0])
	}
	if len(lastName) > 0 {
		res += string(lastName[0])
	}
	return res
}

func SenderToViewModel(entity chat.Sender) viewmodels.MessageSender {
	senderID := strconv.FormatUint(uint64(entity.ID()), 10)
	initials := initialsFromFullName(entity.FirstName(), entity.LastName())
	if entity.IsClient() {
		return viewmodels.NewClientMessageSender(senderID, initials)
	}
	return viewmodels.NewUserMessageSender(senderID, initials)
}

func MessageToViewModel(entity chat.Message) *viewmodels.Message {
	return &viewmodels.Message{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Sender:    SenderToViewModel(entity.Sender()),
		Message:   entity.Message(),
		CreatedAt: entity.CreatedAt(),
	}
}

func ChatToViewModel(entity chat.Chat, clientEntity client.Client) *viewmodels.Chat {
	return &viewmodels.Chat{
		ID:             strconv.FormatUint(uint64(entity.ID()), 10),
		Client:         ClientToViewModel(clientEntity),
		Messages:       mapping.MapViewModels(entity.Messages(), MessageToViewModel),
		UnreadMessages: entity.UnreadMessages(),
		CreatedAt:      entity.CreatedAt().Format(time.RFC3339),
	}
}

func MessageTemplateToViewModel(entity messagetemplate.MessageTemplate) *viewmodels.MessageTemplate {
	return &viewmodels.MessageTemplate{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Template:  entity.Template(),
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
	}
}

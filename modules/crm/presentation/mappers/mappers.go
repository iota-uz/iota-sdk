package mappers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message-template"
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

func UserMessageToViewModel(entity message.Message, user user.User) *viewmodels.Message {
	senderID := strconv.FormatUint(uint64(user.ID()), 10)
	initials := strings.ToTitle(fmt.Sprintf("%s%s", user.FirstName()[0:1], user.LastName()[0:1]))
	return &viewmodels.Message{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Sender:    viewmodels.NewUserMessageSender(senderID, initials),
		Message:   entity.Message(),
		CreatedAt: entity.CreatedAt(),
	}
}

func ClientMessageToViewModel(entity message.Message, client client.Client) *viewmodels.Message {
	senderID := strconv.FormatUint(uint64(client.ID()), 10)
	initials := strings.ToTitle(fmt.Sprintf("%s%s", client.FirstName()[0:1], client.LastName()[0:1]))
	return &viewmodels.Message{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Sender:    viewmodels.NewClientMessageSender(senderID, initials),
		Message:   entity.Message(),
		CreatedAt: entity.CreatedAt(),
	}
}

func ChatToViewModel(entity chat.Chat, messages []*viewmodels.Message) *viewmodels.Chat {
	return &viewmodels.Chat{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Client:    ClientToViewModel(entity.Client()),
		Messages:  messages,
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
	}
}

func MessageTemplateToViewModel(entity messagetemplate.MessageTemplate) *viewmodels.MessageTemplate {
	return &viewmodels.MessageTemplate{
		ID:        strconv.FormatUint(uint64(entity.ID()), 10),
		Template:  entity.Template(),
		CreatedAt: entity.CreatedAt().Format(time.RFC3339),
	}
}

package persistence

import (
	"github.com/iota-uz/iota-sdk/pkg/mapping"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
)

func toDomainClient(dbRow *models.Client) (client.Client, error) {
	p, err := phone.New(dbRow.PhoneNumber, country.UnitedStates)
	if err != nil {
		return nil, err
	}
	return client.NewWithID(
		dbRow.ID,
		dbRow.FirstName,
		dbRow.LastName,
		dbRow.MiddleName.String,
		p,
		dbRow.CreatedAt,
		dbRow.UpdatedAt,
	)
}

func toDBClient(domainEntity client.Client) *models.Client {
	return &models.Client{
		ID:          domainEntity.ID(),
		FirstName:   domainEntity.FirstName(),
		LastName:    domainEntity.LastName(),
		MiddleName:  mapping.ValueToSQLNullString(domainEntity.MiddleName()),
		PhoneNumber: domainEntity.Phone().Value(),
		CreatedAt:   domainEntity.CreatedAt(),
		UpdatedAt:   domainEntity.UpdatedAt(),
	}
}

func toDBMessage(domainEntity message.Message) *models.Message {
	return &models.Message{
		ID:        domainEntity.ID(),
		ChatID:    domainEntity.ChatID(),
		Message:   domainEntity.Message(),
		CreatedAt: domainEntity.CreatedAt(),
	}
}

func toDomainMessage(dbRow *models.Message) message.Message {
	var sender message.Sender
	if dbRow.SenderUserID.Valid {
		sender = message.NewUserSender(uint(dbRow.SenderUserID.Int64))
	} else {
		sender = message.NewClientSender(uint(dbRow.SenderClientID.Int64))
	}
	return message.NewMessageWithID(
		dbRow.ID,
		dbRow.ChatID,
		dbRow.Message,
		sender,
		dbRow.IsActive,
		dbRow.CreatedAt,
	)
}

func toDBChat(domainEntity chat.Chat) (*models.Chat, []*models.Message) {
	dbMessages := make([]*models.Message, 0, len(domainEntity.Messages()))
	for _, m := range domainEntity.Messages() {
		dbMessages = append(dbMessages, toDBMessage(m))
	}
	return &models.Chat{
		ID:        domainEntity.ID(),
		ClientID:  domainEntity.Client().ID(),
		CreatedAt: domainEntity.CreatedAt(),
	}, dbMessages
}

func toDomainChat(dbRow *models.Chat, dbClient *models.Client, dbMessages []*models.Message) (chat.Chat, error) {
	messages := make([]message.Message, 0, len(dbMessages))
	for _, m := range dbMessages {
		messages = append(messages, toDomainMessage(m))
	}
	c, err := toDomainClient(dbClient)
	if err != nil {
		return nil, err
	}
	domainChat := chat.NewWithID(
		dbRow.ID,
		c,
		messages,
		dbRow.CreatedAt,
	)
	return domainChat, nil
}

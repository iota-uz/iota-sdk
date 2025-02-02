package persistence

import (
	"database/sql"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message-template"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func toDomainClient(dbRow *models.Client) (client.Client, error) {
	p, err := phone.NewFromE164(dbRow.PhoneNumber)
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
	dbMessage := &models.Message{
		ID:             domainEntity.ID(),
		ChatID:         domainEntity.ChatID(),
		Message:        domainEntity.Message(),
		SenderUserID:   sql.NullInt64{},
		SenderClientID: sql.NullInt64{},
		IsActive:       domainEntity.IsActive(),
		CreatedAt:      domainEntity.CreatedAt(),
	}
	if domainEntity.Sender().IsUser() {
		dbMessage.SenderUserID = mapping.ValueToSQLNullInt64(int64(domainEntity.Sender().ID()))
	} else {
		dbMessage.SenderClientID = mapping.ValueToSQLNullInt64(int64(domainEntity.Sender().ID()))
	}
	return dbMessage
}

func toDomainMessage(dbRow *models.Message) (message.Message, error) {
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
	), nil
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
	messages, err := mapping.MapDBModels(dbMessages, toDomainMessage)
	if err != nil {
		return nil, err
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

func toDomainMessageTemplate(dbTemplate *models.MessageTemplate) (messagetemplate.MessageTemplate, error) {
	return messagetemplate.NewWithID(
		dbTemplate.ID,
		dbTemplate.Template,
		dbTemplate.CreatedAt,
	), nil
}

func toDBMessageTemplate(domainTemplate messagetemplate.MessageTemplate) *models.MessageTemplate {
	return &models.MessageTemplate{
		ID:        domainTemplate.ID(),
		Template:  domainTemplate.Template(),
		CreatedAt: domainTemplate.CreatedAt(),
	}
}

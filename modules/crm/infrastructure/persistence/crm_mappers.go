package persistence

import (
	"database/sql"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
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
		dbRow.LastName.String,
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
		LastName:    mapping.ValueToSQLNullString(domainEntity.LastName()),
		MiddleName:  mapping.ValueToSQLNullString(domainEntity.MiddleName()),
		PhoneNumber: domainEntity.Phone().Value(),
		CreatedAt:   domainEntity.CreatedAt(),
		UpdatedAt:   domainEntity.UpdatedAt(),
	}
}

func toDBMessage(domainEntity message.Message) *models.Message {
	dbMessage := &models.Message{
		ID:      domainEntity.ID(),
		ChatID:  domainEntity.ChatID(),
		Message: domainEntity.Message(),
		SenderUserID: sql.NullInt64{
			Int64: 0,
			Valid: false,
		},
		SenderClientID: sql.NullInt64{
			Int64: 0,
			Valid: false,
		},
		IsRead:    domainEntity.IsRead(),
		CreatedAt: domainEntity.CreatedAt(),
	}
	if domainEntity.Sender().IsUser() {
		dbMessage.SenderUserID = mapping.ValueToSQLNullInt64(int64(domainEntity.Sender().ID()))
	} else {
		dbMessage.SenderClientID = mapping.ValueToSQLNullInt64(int64(domainEntity.Sender().ID()))
	}
	return dbMessage
}

func toDomainMessage(
	dbRow *models.Message,
	dbUploads []*coremodels.Upload,
	sender message.Sender,
) (message.Message, error) {
	uploads := make([]*upload.Upload, 0, len(dbUploads))
	for _, u := range dbUploads {
		uploads = append(uploads, corepersistence.ToDomainUpload(u))
	}
	return message.NewWithID(
		dbRow.ID,
		dbRow.ChatID,
		dbRow.Message,
		sender,
		dbRow.IsRead,
		uploads,
		dbRow.CreatedAt,
	), nil
}

func toDBChat(domainEntity chat.Chat) *models.Chat {
	return &models.Chat{
		ID:        domainEntity.ID(),
		ClientID:  domainEntity.Client().ID(),
		CreatedAt: domainEntity.CreatedAt(),
	}
}

func toDomainChat(dbRow *models.Chat, dbClient *models.Client) (chat.Chat, error) {
	c, err := toDomainClient(dbClient)
	if err != nil {
		return nil, err
	}
	domainChat := chat.NewWithID(
		dbRow.ID,
		c,
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

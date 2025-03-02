package persistence

import (
	"database/sql"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/passport"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
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

func toDomainClientComplete(dbRow *models.Client, passportData passport.Passport) (client.Client, error) {
	p, err := phone.NewFromE164(dbRow.PhoneNumber)
	if err != nil {
		return nil, err
	}
	
	return client.NewComplete(
		dbRow.ID,
		dbRow.FirstName,
		dbRow.LastName.String,
		dbRow.MiddleName.String,
		p,
		dbRow.Address.String,
		dbRow.Email.String,
		dbRow.HourlyRate.Float64,
		mapping.SQLNullTimeToPointer(dbRow.DateOfBirth),
		dbRow.Gender.String,
		passportData,
		dbRow.PIN.String,
		dbRow.CreatedAt,
		dbRow.UpdatedAt,
	)
}

func toDBClient(domainEntity client.Client) *models.Client {
	// Passport ID will be handled in separate transaction logic
	// This just creates the client record
	var passportID sql.NullString
	if domainEntity.Passport() != nil && (domainEntity.Passport().Series() != "" || domainEntity.Passport().Number() != "") {
		// If passport is linked to this client, you would set the passport ID here
		// But that would typically be managed in the repository logic
	}
	
	return &models.Client{
		ID:            domainEntity.ID(),
		FirstName:     domainEntity.FirstName(),
		LastName:      mapping.ValueToSQLNullString(domainEntity.LastName()),
		MiddleName:    mapping.ValueToSQLNullString(domainEntity.MiddleName()),
		PhoneNumber:   domainEntity.Phone().Value(),
		Address:       mapping.ValueToSQLNullString(domainEntity.Address()),
		Email:         mapping.ValueToSQLNullString(domainEntity.Email()),
		HourlyRate:    mapping.ValueToSQLNullFloat64(domainEntity.HourlyRate()),
		DateOfBirth:   mapping.PointerToSQLNullTime(domainEntity.DateOfBirth()),
		Gender:        mapping.ValueToSQLNullString(domainEntity.Gender()),
		PassportID:    passportID,
		PIN:           mapping.ValueToSQLNullString(domainEntity.PIN()),
		CreatedAt:     domainEntity.CreatedAt(),
		UpdatedAt:     domainEntity.UpdatedAt(),
	}
}

func toDBMessage(entity chat.Message, chatID uint) *models.Message {
	dbMessage := &models.Message{
		ID:      entity.ID(),
		Message: entity.Message(),
		ChatID:  chatID,
		SenderUserID: sql.NullInt64{
			Int64: 0,
			Valid: false,
		},
		SenderClientID: sql.NullInt64{
			Int64: 0,
			Valid: false,
		},
		IsRead:    entity.IsRead(),
		ReadAt:    mapping.PointerToSQLNullTime(entity.ReadAt()),
		CreatedAt: entity.CreatedAt(),
	}
	if entity.Sender().IsUser() {
		dbMessage.SenderUserID = mapping.ValueToSQLNullInt64(int64(entity.Sender().ID()))
	} else {
		dbMessage.SenderClientID = mapping.ValueToSQLNullInt64(int64(entity.Sender().ID()))
	}
	return dbMessage
}

func toDomainMessage(
	dbRow *models.Message,
	dbUploads []*coremodels.Upload,
	sender chat.Sender,
) (chat.Message, error) {
	uploads := make([]upload.Upload, 0, len(dbUploads))
	for _, u := range dbUploads {
		uploads = append(uploads, corepersistence.ToDomainUpload(u))
	}
	return chat.NewMessageWithID(
		dbRow.ID,
		dbRow.Message,
		sender,
		dbRow.IsRead,
		uploads,
		dbRow.CreatedAt,
	), nil
}

func toDBChat(domainEntity chat.Chat) (*models.Chat, []*models.Message) {
	dbMessages := make([]*models.Message, 0, len(domainEntity.Messages()))
	for _, m := range domainEntity.Messages() {
		dbMessages = append(dbMessages, toDBMessage(m, domainEntity.ID()))
	}
	return &models.Chat{
		ID:            domainEntity.ID(),
		ClientID:      domainEntity.ClientID(),
		CreatedAt:     domainEntity.CreatedAt(),
		LastMessageAt: mapping.PointerToSQLNullTime(domainEntity.LastMessageAt()),
	}, dbMessages
}

func toDomainChat(dbRow *models.Chat, messages []chat.Message) (chat.Chat, error) {
	domainChat := chat.NewWithID(
		dbRow.ID,
		dbRow.ClientID,
		dbRow.CreatedAt,
		messages,
		mapping.SQLNullTimeToPointer(dbRow.LastMessageAt),
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

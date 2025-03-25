package persistence

import (
	"database/sql"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/general"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	messagetemplate "github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message-template"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func ToDomainClientComplete(dbRow *models.Client, passportData passport.Passport) (client.Client, error) {

	options := []client.Option{
		client.WithID(dbRow.ID),
		client.WithCreatedAt(dbRow.CreatedAt),
		client.WithUpdatedAt(dbRow.UpdatedAt),
	}

	if dbRow.PhoneNumber.Valid {
		p, err := phone.NewFromE164(dbRow.PhoneNumber.String)
		if err != nil {
			return nil, err
		}
		options = append(options, client.WithPhone(p))
	}

	if dbRow.Address.Valid {
		options = append(options, client.WithAddress(dbRow.Address.String))
	}

	if dbRow.Email.Valid && dbRow.Email.String != "" {
		e, err := internet.NewEmail(dbRow.Email.String)
		if err == nil {
			options = append(options, client.WithEmail(e))
		}
	}

	if dbRow.Gender.Valid && dbRow.Gender.String != "" {
		g, err := general.NewGender(dbRow.Gender.String)
		if err == nil {
			options = append(options, client.WithGender(g))
		}
	}

	if dbRow.Pin.Valid && dbRow.Pin.String != "" {
		tPin, err := tax.NewPin(dbRow.Pin.String, country.Afghanistan)
		if err == nil {
			options = append(options, client.WithPin(tPin))
		}
	}

	if dbRow.DateOfBirth.Valid {
		options = append(options, client.WithDateOfBirth(mapping.SQLNullTimeToPointer(dbRow.DateOfBirth)))
	}

	if passportData != nil {
		options = append(options, client.WithPassport(passportData))
	}

	return client.New(
		dbRow.FirstName,
		dbRow.LastName.String,
		dbRow.MiddleName.String,
		options...,
	)
}

func ToDBClient(domainEntity client.Client) *models.Client {
	// First check if we need to create a passport
	var passportID sql.NullString

	if domainEntity.Passport() != nil {
		passportID = sql.NullString{
			String: domainEntity.Passport().ID().String(),
			Valid:  true,
		}
	}

	var email sql.NullString

	if domainEntity.Email() != nil && domainEntity.Email() != nil {
		email = mapping.ValueToSQLNullString(domainEntity.Email().Value())
	}

	var gender sql.NullString
	if domainEntity.Gender() != nil {
		gender = mapping.ValueToSQLNullString(domainEntity.Gender().String())
	}

	var pin sql.NullString
	if domainEntity.Pin() != nil && domainEntity.Pin().Value() != "" {
		pin = mapping.ValueToSQLNullString(domainEntity.Pin().Value())
	}

	var phone sql.NullString
	if domainEntity.Phone() != nil && domainEntity.Phone().Value() != "" {
		phone = mapping.ValueToSQLNullString(domainEntity.Phone().Value())
	}

	return &models.Client{
		ID:          domainEntity.ID(),
		FirstName:   domainEntity.FirstName(),
		LastName:    mapping.ValueToSQLNullString(domainEntity.LastName()),
		MiddleName:  mapping.ValueToSQLNullString(domainEntity.MiddleName()),
		PhoneNumber: phone,
		Address:     mapping.ValueToSQLNullString(domainEntity.Address()),
		Email:       email,
		DateOfBirth: mapping.PointerToSQLNullTime(domainEntity.DateOfBirth()),
		Gender:      gender,
		PassportID:  passportID,
		Pin:         pin,
		CreatedAt:   domainEntity.CreatedAt(),
		UpdatedAt:   domainEntity.UpdatedAt(),
	}
}

func ToDBMessage(entity chat.Message, chatID uint) *models.Message {
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

func ToDomainMessage(
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

func ToDBChat(domainEntity chat.Chat) (*models.Chat, []*models.Message) {
	dbMessages := make([]*models.Message, 0, len(domainEntity.Messages()))
	for _, m := range domainEntity.Messages() {
		dbMessages = append(dbMessages, ToDBMessage(m, domainEntity.ID()))
	}
	return &models.Chat{
		ID:            domainEntity.ID(),
		ClientID:      domainEntity.ClientID(),
		CreatedAt:     domainEntity.CreatedAt(),
		LastMessageAt: mapping.PointerToSQLNullTime(domainEntity.LastMessageAt()),
	}, dbMessages
}

func ToDomainChat(dbRow *models.Chat, messages []chat.Message) (chat.Chat, error) {
	domainChat := chat.NewWithID(
		dbRow.ID,
		dbRow.ClientID,
		dbRow.CreatedAt,
		messages,
		mapping.SQLNullTimeToPointer(dbRow.LastMessageAt),
	)
	return domainChat, nil
}

func ToDomainMessageTemplate(dbTemplate *models.MessageTemplate) (messagetemplate.MessageTemplate, error) {
	return messagetemplate.NewWithID(
		dbTemplate.ID,
		dbTemplate.Template,
		dbTemplate.CreatedAt,
	), nil
}

func ToDBMessageTemplate(domainTemplate messagetemplate.MessageTemplate) *models.MessageTemplate {
	return &models.MessageTemplate{
		ID:        domainTemplate.ID(),
		Template:  domainTemplate.Template(),
		CreatedAt: domainTemplate.CreatedAt(),
	}
}

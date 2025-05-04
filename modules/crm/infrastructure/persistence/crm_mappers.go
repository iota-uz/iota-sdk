package persistence

import (
	"database/sql"
	"encoding/json"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
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

func ToDomainClient(dbRow *models.Client, passportData passport.Passport) (client.Client, error) {
	options := []client.Option{
		client.WithID(dbRow.ID),
		client.WithCreatedAt(dbRow.CreatedAt),
		client.WithUpdatedAt(dbRow.UpdatedAt),
	}

	if dbRow.LastName.Valid {
		options = append(options, client.WithLastName(dbRow.LastName.String))
	}

	if dbRow.MiddleName.Valid {
		options = append(options, client.WithMiddleName(dbRow.MiddleName.String))
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

	if dbRow.Comments.Valid {
		options = append(options, client.WithComments(dbRow.Comments.String))
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

	if domainEntity.Email() != nil {
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

	var comments sql.NullString
	if domainEntity.Comments() != "" {
		comments = mapping.ValueToSQLNullString(domainEntity.Comments())
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
		Comments:    comments,
		CreatedAt:   domainEntity.CreatedAt(),
		UpdatedAt:   domainEntity.UpdatedAt(),
	}
}

func ToDBMessage(entity chat.Message) *models.Message {
	dbMessage := &models.Message{
		ID:        entity.ID(),
		Message:   entity.Message(),
		ChatID:    entity.ChatID(),
		ReadAt:    mapping.PointerToSQLNullTime(entity.ReadAt()),
		SenderID:  entity.Sender().ID().String(),
		CreatedAt: entity.CreatedAt(),
	}
	return dbMessage
}

func ToDomainMessage(
	dbRow *models.Message,
	sender chat.Member,
	dbUploads []*coremodels.Upload,
) (chat.Message, error) {
	uploads := make([]upload.Upload, 0, len(dbUploads))
	for _, u := range dbUploads {
		uploads = append(uploads, corepersistence.ToDomainUpload(u))
	}
	return chat.NewMessage(
		dbRow.Message,
		sender,
		chat.WithMessageChatID(dbRow.ChatID),
		chat.WithMessageID(dbRow.ID),
		chat.WithReadAt(mapping.SQLNullTimeToPointer(dbRow.ReadAt)),
		chat.WithAttachments(uploads),
		chat.WithMessageCreatedAt(dbRow.CreatedAt),
	), nil
}

func ToDBChat(domainEntity chat.Chat) (*models.Chat, []*models.Message) {
	dbMessages := make([]*models.Message, 0, len(domainEntity.Messages()))
	for _, m := range domainEntity.Messages() {
		dbMessages = append(dbMessages, ToDBMessage(m))
	}
	return &models.Chat{
		ID:            domainEntity.ID(),
		ClientID:      domainEntity.ClientID(),
		CreatedAt:     domainEntity.CreatedAt(),
		LastMessageAt: mapping.PointerToSQLNullTime(domainEntity.LastMessageAt()),
	}, dbMessages
}

func ToDomainChat(dbRow *models.Chat, messages []chat.Message, members []chat.Member) (chat.Chat, error) {
	return chat.New(
		dbRow.ClientID,
		chat.WithChatID(dbRow.ID),
		chat.WithCreatedAt(dbRow.CreatedAt),
		chat.WithMessages(messages),
		chat.WithMembers(members),
		chat.WithLastMessageAt(mapping.SQLNullTimeToPointer(dbRow.LastMessageAt)),
	), nil
}

func ToDBChatMember(chatID uint, entity chat.Member) *models.ChatMember {
	dbRow := &models.ChatMember{
		ID:        entity.ID().String(),
		ChatID:    chatID,
		Transport: string(entity.Transport()),
		CreatedAt: entity.CreatedAt(),
		UpdatedAt: entity.UpdatedAt(),
	}
	switch v := entity.Sender().(type) {
	case chat.ClientSender:
		dbRow.ClientID = mapping.ValueToSQLNullInt32(int32(v.ClientID()))
		dbRow.ClientContactID = mapping.ValueToSQLNullInt32(int32(v.ContactID()))
	case chat.UserSender:
		dbRow.UserID = mapping.ValueToSQLNullInt32(int32(v.UserID()))
	}
	return dbRow
}

func ToDomainChatMember(dbMember *models.ChatMember) (chat.Member, error) {
	transport := chat.Transport(dbMember.Transport)
	var sender chat.Sender

	if dbMember.UserID.Valid {
		sender = chat.NewUserSender(transport, uint(dbMember.UserID.Int32), "", "")
	} else if dbMember.ClientID.Valid {
		sender = chat.NewClientSender(
			transport,
			uint(dbMember.ClientID.Int32),
			uint(dbMember.ClientContactID.Int32),
			"", "",
		)
	} else {
		baseSender := chat.NewUserSender(transport, 0, "", "")
		sender = chat.NewOtherSender(baseSender)
	}

	// Process transport-specific metadata if available
	if dbMember.TransportMeta != nil {
		metaStr, ok := dbMember.TransportMeta.Interface().(string)
		if ok {
			switch transport {
			case chat.TelegramTransport:
				var meta models.TelegramMeta
				if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
					sender, err = TelegramMetaToSender(sender, &meta)
					if err != nil {
						return nil, errors.Wrap(err, "failed to process telegram metadata")
					}
				}
			case chat.WhatsAppTransport:
				var meta models.WhatsAppMeta
				if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
					sender, err = WhatsAppMetaToSender(sender, &meta)
					if err != nil {
						return nil, errors.Wrap(err, "failed to process whatsapp metadata")
					}
				}
			case chat.InstagramTransport:
				var meta models.InstagramMeta
				if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
					sender, err = InstagramMetaToSender(sender, &meta)
					if err != nil {
						return nil, errors.Wrap(err, "failed to process instagram metadata")
					}
				}
			case chat.EmailTransport:
				var meta models.EmailMeta
				if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
					sender, err = EmailMetaToSender(sender, &meta)
					if err != nil {
						return nil, errors.Wrap(err, "failed to process email metadata")
					}
				}
			case chat.PhoneTransport:
				var meta models.PhoneMeta
				if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
					sender, err = PhoneMetaToSender(sender, &meta)
					if err != nil {
						return nil, errors.Wrap(err, "failed to process phone metadata")
					}
				}
			case chat.SMSTransport:
				var meta models.SMSMeta
				if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
					sender, err = SMSMetaToSender(sender, &meta)
					if err != nil {
						return nil, errors.Wrap(err, "failed to process sms metadata")
					}
				}
			case chat.WebsiteTransport:
				var meta models.WebsiteMeta
				if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
					sender, err = WebsiteMetaToSender(sender, &meta)
					if err != nil {
						return nil, errors.Wrap(err, "failed to process website metadata")
					}
				}
			case chat.OtherTransport:
				panic("other transport")
			}
		}
	}

	uid, err := uuid.Parse(dbMember.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse member ID")
	}
	return chat.NewMember(
		sender,
		chat.WithMemberID(uid),
		chat.WithMemberCreatedAt(dbMember.CreatedAt),
		chat.WithMemberUpdatedAt(dbMember.UpdatedAt),
	), nil
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

// ToDomainClientContact converts a database client contact model to a domain client contact entity
func ToDomainClientContact(dbContact *models.ClientContact) client.Contact {
	return client.NewContact(
		client.ContactType(dbContact.ContactType),
		dbContact.ContactValue,
		client.WithContactID(dbContact.ID),
		client.WithContactCreatedAt(dbContact.CreatedAt),
		client.WithContactUpdatedAt(dbContact.UpdatedAt),
	)
}

// ToDBClientContact converts a domain client contact entity to a database client contact model
func ToDBClientContact(clientID uint, domainContact client.Contact) *models.ClientContact {
	return &models.ClientContact{
		ID:           domainContact.ID(),
		ClientID:     clientID,
		ContactType:  string(domainContact.Type()),
		ContactValue: domainContact.Value(),
		CreatedAt:    domainContact.CreatedAt(),
		UpdatedAt:    domainContact.UpdatedAt(),
	}
}

func TelegramMetaToSender(baseSender chat.Sender, meta *models.TelegramMeta) (chat.Sender, error) {
	if meta == nil {
		return baseSender, nil
	}
	var p phone.Phone
	var err error
	if meta.Phone != "" {
		p, err = phone.NewFromE164(meta.Phone)
		if err != nil {
			p = nil
		}
	} else {
		p = nil
	}
	return chat.NewTelegramSender(baseSender, meta.ChatID, meta.Username, p), nil
}

func WhatsAppMetaToSender(baseSender chat.Sender, meta *models.WhatsAppMeta) (chat.Sender, error) {
	if meta == nil {
		return chat.NewWhatsAppSender(baseSender, nil), nil
	}
	var p phone.Phone
	var err error
	if meta.Phone != "" {
		p, err = phone.NewFromE164(meta.Phone)
		if err != nil {
			p = nil
		}
	} else {
		p = nil
	}
	return chat.NewWhatsAppSender(baseSender, p), nil
}

func InstagramMetaToSender(baseSender chat.Sender, meta *models.InstagramMeta) (chat.Sender, error) {
	if meta == nil {
		return chat.NewInstagramSender(baseSender, ""), nil
	}
	return chat.NewInstagramSender(baseSender, meta.Username), nil
}

func EmailMetaToSender(baseSender chat.Sender, meta *models.EmailMeta) (chat.Sender, error) {
	if meta == nil {
		return chat.NewEmailSender(baseSender, nil), nil
	}
	var emailObj internet.Email
	var err error
	if meta.Email != "" {
		emailObj, err = internet.NewEmail(meta.Email)
		if err != nil {
			emailObj = nil
		}
	} else {
		emailObj = nil
	}
	return chat.NewEmailSender(baseSender, emailObj), nil
}

func PhoneMetaToSender(baseSender chat.Sender, meta *models.PhoneMeta) (chat.Sender, error) {
	if meta == nil {
		return chat.NewPhoneSender(baseSender, nil), nil
	}
	var p phone.Phone
	var err error
	if meta.Phone != "" {
		p, err = phone.NewFromE164(meta.Phone)
		if err != nil {
			p = nil
		}
	} else {
		p = nil
	}
	return chat.NewPhoneSender(baseSender, p), nil
}

func SMSMetaToSender(baseSender chat.Sender, meta *models.SMSMeta) (chat.Sender, error) {
	if meta == nil {
		return chat.NewSMSSender(baseSender, nil), nil
	}
	var p phone.Phone
	var err error
	if meta.Phone != "" {
		p, err = phone.NewFromE164(meta.Phone)
		if err != nil {
			p = nil
		}
	} else {
		p = nil
	}
	return chat.NewSMSSender(baseSender, p), nil
}

func WebsiteMetaToSender(baseSender chat.Sender, meta *models.WebsiteMeta) (chat.Sender, error) {
	if meta == nil {
		return chat.NewWebsiteSender(baseSender, nil, nil), nil
	}

	var phoneObj phone.Phone
	var emailObj internet.Email
	var err error

	if meta.Phone != "" {
		phoneObj, err = phone.NewFromE164(meta.Phone)
		if err != nil {
			phoneObj = nil
		}
	} else {
		phoneObj = nil
	}

	if meta.Email != "" {
		emailObj, err = internet.NewEmail(meta.Email)
		if err != nil {
			emailObj = nil
		}
	} else {
		emailObj = nil
	}

	return chat.NewWebsiteSender(baseSender, phoneObj, emailObj), nil
}

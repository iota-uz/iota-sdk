package persistence

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-faster/errors"

	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrChatNotFound    = errors.New("chat not found")
	ErrMessageNotFound = errors.New("message not found")
)

const (
	selectChatQuery = `
		SELECT 
			c.id,
			c.created_at,
			c.last_message_at,
			c.client_id
		FROM chats c
	`

	countChatQuery = `SELECT COUNT(*) as count FROM chats`

	insertChatQuery = `
		INSERT INTO chats (
			client_id,
			created_at
		) VALUES ($1, $2) RETURNING id
	`

	updateChatQuery = `UPDATE chats SET
		client_id = $1,
		created_at = $2,
		last_message_at = $3
		WHERE id = $4`

	deleteChatQuery = `DELETE FROM chats WHERE id = $1`

	selectMessagesQuery = `
		SELECT 
			m.id,
			m.chat_id,
			m.message,
			m.transport,
			m.sender_user_id,
			m.sender_client_id,
			m.is_read,
			m.read_at,
			m.transport_meta,
			m.created_at
		FROM messages m
	`

	selectMessageUserSender = `SELECT id, first_name, last_name FROM users WHERE id = $1`

	selectMessageClientSender = `SELECT id, first_name, last_name FROM clients WHERE id = $1`

	selectMessageAttachmentsQuery = `
		SELECT 
			u.id AS upload_id,
			u.hash,
			u.path,
			u.size,
			u.mimetype,
			u.created_at AS upload_created_at,
			u.updated_at AS upload_updated_at
		FROM messages AS m
		JOIN message_media AS mm ON m.id = mm.message_id
		JOIN uploads AS u ON mm.upload_id = u.id
		WHERE m.id = $1
	`

	insertMessageQuery = `
		INSERT INTO messages (
			chat_id,
			message,
			transport,
			sender_user_id,
			sender_client_id,
			is_read,
			transport_meta,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`

	updateMessageQuery = `
		UPDATE messages SET 
			chat_id = $1,
			message = $2,
			transport = $3,
			sender_user_id = $4,
			sender_client_id = $5,
			is_read = $6,
			transport_meta = $7,
			read_at = $8
		WHERE id = $9
	`

	deleteMessageQuery = `DELETE FROM messages WHERE id = $1`
)

type ChatRepository struct {
}

func NewChatRepository() chat.Repository {
	return &ChatRepository{}
}

func (g *ChatRepository) queryChats(ctx context.Context, query string, args ...interface{}) ([]chat.Chat, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute chat query")
	}
	defer rows.Close()

	dbChats := make([]*models.Chat, 0)
	for rows.Next() {
		var c models.Chat
		if err := rows.Scan(
			&c.ID,
			&c.CreatedAt,
			&c.LastMessageAt,
			&c.ClientID,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan chat")
		}
		dbChats = append(dbChats, &c)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error occurred while iterating chat rows")
	}

	chats := make([]chat.Chat, 0, len(dbChats))

	for _, c := range dbChats {
		messages, err := g.queryMessages(
			ctx,
			repo.Join(
				selectMessagesQuery,
				"WHERE m.chat_id = $1",
				"ORDER BY m.created_at ASC",
			),
			c.ID,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get messages for chat")
		}
		domainChat, err := ToDomainChat(c, messages)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert to domain chat")
		}
		chats = append(chats, domainChat)
	}

	return chats, nil
}

func (g *ChatRepository) queryMessages(ctx context.Context, query string, args ...interface{}) ([]chat.Message, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute message query")
	}
	defer rows.Close()

	var dbMessages []*models.Message
	for rows.Next() {
		var msg models.Message
		var transportMetaBytes []byte
		if err := rows.Scan(
			&msg.ID,
			&msg.ChatID,
			&msg.Message,
			&msg.Transport,
			&msg.SenderUserID,
			&msg.SenderClientID,
			&msg.IsRead,
			&msg.ReadAt,
			&transportMetaBytes,
			&msg.CreatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan message")
		}

		// Process transport meta based on the transport type
		if len(transportMetaBytes) > 0 {
			transportType := chat.Transport(msg.Transport)

			// Parse JSON based on transport type
			switch transportType {
			case chat.TelegramTransport:
				var metaData models.TelegramMeta
				if err := json.Unmarshal(transportMetaBytes, &metaData); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal telegram meta")
				}
				msg.TransportMeta = models.NewTransportMeta(&metaData)
			case chat.WhatsAppTransport:
				var metaData models.WhatsAppMeta
				if err := json.Unmarshal(transportMetaBytes, &metaData); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal whatsapp meta")
				}
				msg.TransportMeta = models.NewTransportMeta(&metaData)
			case chat.InstagramTransport:
				var metaData models.InstagramMeta
				if err := json.Unmarshal(transportMetaBytes, &metaData); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal instagram meta")
				}
				msg.TransportMeta = models.NewTransportMeta(&metaData)
			case chat.EmailTransport:
				var metaData models.EmailMeta
				if err := json.Unmarshal(transportMetaBytes, &metaData); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal email meta")
				}
				msg.TransportMeta = models.NewTransportMeta(&metaData)
			case chat.SMSTransport:
				var metaData models.SMSMeta
				if err := json.Unmarshal(transportMetaBytes, &metaData); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal sms meta")
				}
				msg.TransportMeta = models.NewTransportMeta(&metaData)
			case chat.PhoneTransport:
				var metaData models.PhoneMeta
				if err := json.Unmarshal(transportMetaBytes, &metaData); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal phone meta")
				}
				msg.TransportMeta = models.NewTransportMeta(&metaData)
			case chat.WebsiteTransport:
				var metaData models.WebsiteMeta
				if err := json.Unmarshal(transportMetaBytes, &metaData); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal website meta")
				}
				msg.TransportMeta = models.NewTransportMeta(&metaData)
			default:
				// For other transports, store as generic map
				var metaData map[string]interface{}
				if err := json.Unmarshal(transportMetaBytes, &metaData); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal generic meta")
				}
				msg.TransportMeta = models.NewTransportMeta(metaData)
			}
		}

		dbMessages = append(dbMessages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error occurred while iterating message rows")
	}

	messages := make([]chat.Message, 0, len(dbMessages))
	for _, message := range dbMessages {
		var sender chat.Sender
		if message.SenderUserID.Valid {
			var user coremodels.User
			if err := pool.QueryRow(ctx, selectMessageUserSender, message.SenderUserID.Int64).Scan(
				&user.ID,
				&user.FirstName,
				&user.LastName,
			); err != nil {
				return nil, errors.Wrap(err, "failed to scan user sender")
			}
			sender = chat.NewUserSender(chat.Transport(message.Transport), user.ID, user.FirstName, user.LastName)
		}

		if message.SenderClientID.Valid {
			var client models.Client
			if err := pool.QueryRow(ctx, selectMessageClientSender, message.SenderClientID.Int64).Scan(
				&client.ID,
				&client.FirstName,
				&client.LastName,
			); err != nil {
				return nil, errors.Wrap(err, "failed to scan client sender")
			}
			baseSender := chat.NewClientSender(
				chat.Transport(message.Transport),
				client.ID,
				client.FirstName,
				client.LastName.String,
			)

			var err error
			switch chat.Transport(message.Transport) {
			case chat.TelegramTransport:
				if message.TransportMeta != nil && message.TransportMeta.Interface() != nil {
					telegramMeta, ok := message.TransportMeta.Interface().(*models.TelegramMeta)
					if ok {
						sender, err = TelegramMetaToSender(baseSender, telegramMeta)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create telegram sender")
						}
					} else {
						sender, err = TelegramMetaToSender(baseSender, nil)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create telegram sender")
						}
					}
				} else {
					sender, err = TelegramMetaToSender(baseSender, nil)
					if err != nil {
						return nil, errors.Wrap(err, "failed to create telegram sender")
					}
				}
			case chat.WhatsAppTransport:
				if message.TransportMeta != nil && message.TransportMeta.Interface() != nil {
					whatsappMeta, ok := message.TransportMeta.Interface().(*models.WhatsAppMeta)
					if ok {
						sender, err = WhatsAppMetaToSender(baseSender, whatsappMeta)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create whatsapp sender")
						}
					} else {
						sender, err = WhatsAppMetaToSender(baseSender, nil)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create whatsapp sender")
						}
					}
				} else {
					sender, err = WhatsAppMetaToSender(baseSender, nil)
					if err != nil {
						return nil, errors.Wrap(err, "failed to create whatsapp sender")
					}
				}
			case chat.InstagramTransport:
				if message.TransportMeta != nil && message.TransportMeta.Interface() != nil {
					instagramMeta, ok := message.TransportMeta.Interface().(*models.InstagramMeta)
					if ok {
						sender, err = InstagramMetaToSender(baseSender, instagramMeta)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create instagram sender")
						}
					} else {
						sender, err = InstagramMetaToSender(baseSender, nil)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create instagram sender")
						}
					}
				} else {
					sender, err = InstagramMetaToSender(baseSender, nil)
					if err != nil {
						return nil, errors.Wrap(err, "failed to create instagram sender")
					}
				}
			case chat.SMSTransport:
				if message.TransportMeta != nil && message.TransportMeta.Interface() != nil {
					smsMeta, ok := message.TransportMeta.Interface().(*models.SMSMeta)
					if ok {
						sender, err = SMSMetaToSender(baseSender, smsMeta)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create sms sender")
						}
					} else {
						sender, err = SMSMetaToSender(baseSender, nil)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create sms sender")
						}
					}
				} else {
					sender, err = SMSMetaToSender(baseSender, nil)
					if err != nil {
						return nil, errors.Wrap(err, "failed to create sms sender")
					}
				}
			case chat.EmailTransport:
				if message.TransportMeta != nil && message.TransportMeta.Interface() != nil {
					emailMeta, ok := message.TransportMeta.Interface().(*models.EmailMeta)
					if ok {
						sender, err = EmailMetaToSender(baseSender, emailMeta)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create email sender")
						}
					} else {
						sender, err = EmailMetaToSender(baseSender, nil)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create email sender")
						}
					}
				} else {
					sender, err = EmailMetaToSender(baseSender, nil)
					if err != nil {
						return nil, errors.Wrap(err, "failed to create email sender")
					}
				}
			case chat.PhoneTransport:
				if message.TransportMeta != nil && message.TransportMeta.Interface() != nil {
					phoneMeta, ok := message.TransportMeta.Interface().(*models.PhoneMeta)
					if ok {
						sender, err = PhoneMetaToSender(baseSender, phoneMeta)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create phone sender")
						}
					} else {
						sender, err = PhoneMetaToSender(baseSender, nil)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create phone sender")
						}
					}
				} else {
					sender, err = PhoneMetaToSender(baseSender, nil)
					if err != nil {
						return nil, errors.Wrap(err, "failed to create phone sender")
					}
				}
			case chat.WebsiteTransport:
				if message.TransportMeta != nil && message.TransportMeta.Interface() != nil {
					websiteMeta, ok := message.TransportMeta.Interface().(*models.WebsiteMeta)
					if ok {
						sender, err = WebsiteMetaToSender(baseSender, websiteMeta)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create website sender")
						}
					} else {
						sender, err = WebsiteMetaToSender(baseSender, nil)
						if err != nil {
							return nil, errors.Wrap(err, "failed to create website sender")
						}
					}
				} else {
					sender, err = WebsiteMetaToSender(baseSender, nil)
					if err != nil {
						return nil, errors.Wrap(err, "failed to create website sender")
					}
				}
			case chat.OtherTransport:
				sender = chat.NewOtherSender(baseSender)
			default:
				sender = baseSender
			}
		}

		uploads, err := pool.Query(ctx, selectMessageAttachmentsQuery, message.ID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to query attachments for message %d", message.ID)
		}
		defer uploads.Close()

		var dbUploads []*coremodels.Upload
		for uploads.Next() {
			var upload coremodels.Upload
			if err := uploads.Scan(
				&upload.ID,
				&upload.Hash,
				&upload.Path,
				&upload.Size,
				&upload.Mimetype,
				&upload.CreatedAt,
				&upload.UpdatedAt,
			); err != nil {
				return nil, errors.Wrap(err, "failed to scan upload")
			}
			dbUploads = append(dbUploads, &upload)
		}

		if err := uploads.Err(); err != nil {
			return nil, errors.Wrap(err, "error occurred while iterating uploads")
		}

		domainMessage, err := ToDomainMessage(message, dbUploads, sender)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert to domain message")
		}
		messages = append(messages, domainMessage)
	}

	return messages, nil
}

func (g *ChatRepository) GetPaginated(
	ctx context.Context, params *chat.FindParams,
) ([]chat.Chat, error) {
	sortFields := []string{}
	for _, f := range params.SortBy.Fields {
		switch f {
		case chat.LastMessageAt:
			sortFields = append(sortFields, "c.last_message_at")
		case chat.CreatedAt:
			sortFields = append(sortFields, "c.created_at")
		default:
			return nil, fmt.Errorf("unknown sort field: %v", f)
		}
	}

	where, args := []string{"1 = 1"}, []interface{}{}
	if params.Search != "" {
		where = append(
			where,
			"cl.first_name ILIKE $1 OR cl.last_name ILIKE $1 OR cl.middle_name ILIKE $1 OR cl.phone_number ILIKE $1",
		)
		args = append(args, "%"+params.Search+"%")
	}
	return g.queryChats(
		ctx,
		repo.Join(
			selectChatQuery,
			repo.JoinWhere(where...),
			repo.OrderBy(sortFields, params.SortBy.Ascending),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
		args...,
	)
}

func (g *ChatRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}
	var count int64
	if err := pool.QueryRow(ctx, countChatQuery).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "failed to count chats")
	}
	return count, nil
}

func (g *ChatRepository) GetAll(ctx context.Context) ([]chat.Chat, error) {
	chats, err := g.queryChats(ctx, selectChatQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all chats")
	}
	return chats, nil
}

func (g *ChatRepository) GetByID(ctx context.Context, id uint) (chat.Chat, error) {
	chats, err := g.queryChats(ctx, selectChatQuery+" WHERE c.id = $1", id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get chat with id %d", id)
	}
	if len(chats) == 0 {
		return nil, ErrChatNotFound
	}
	return chats[0], nil
}

func (g *ChatRepository) GetByClientID(ctx context.Context, clientID uint) (chat.Chat, error) {
	chats, err := g.queryChats(ctx, selectChatQuery+" WHERE c.client_id = $1", clientID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get chat for client %d", clientID)
	}
	if len(chats) == 0 {
		return nil, ErrChatNotFound
	}
	return chats[0], nil
}

func (g *ChatRepository) GetMessageByID(ctx context.Context, id uint) (chat.Message, error) {
	messages, err := g.queryMessages(ctx, selectMessagesQuery+" WHERE m.id = $1", id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get message with id %d", id)
	}
	if len(messages) == 0 {
		return nil, ErrMessageNotFound
	}
	return messages[0], nil
}

func (g *ChatRepository) AddMessage(ctx context.Context, chatID uint, message chat.Message) (chat.Message, error) {
	id, err := g.insertMessage(ctx, ToDBMessage(message, chatID))
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert message")
	}
	return g.GetMessageByID(ctx, id)
}

func (g *ChatRepository) insertMessage(ctx context.Context, message *models.Message) (uint, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}
	if err := tx.QueryRow(
		ctx,
		insertMessageQuery,
		message.ChatID,
		message.Message,
		message.Transport,
		message.SenderUserID,
		message.SenderClientID,
		message.IsRead,
		&message.CreatedAt,
	).Scan(&message.ID); err != nil {
		return 0, errors.Wrap(err, "failed to insert message")
	}
	return message.ID, nil
}

func (g *ChatRepository) updateMessage(ctx context.Context, message *models.Message) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}
	if _, err := tx.Exec(
		ctx,
		updateMessageQuery,
		message.ChatID,
		message.Message,
		message.Transport,
		message.SenderUserID,
		message.SenderClientID,
		message.IsRead,
		message.ReadAt,
		message.ID,
	); err != nil {
		return errors.Wrap(err, "failed to update message")
	}
	return nil
}

func (g *ChatRepository) DeleteMessage(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}
	if _, err := tx.Exec(ctx, deleteMessageQuery, id); err != nil {
		return errors.Wrapf(err, "failed to delete message with id %d", id)
	}
	return nil
}

func (g *ChatRepository) saveMessages(ctx context.Context, dbMessages []*models.Message) error {
	for _, m := range dbMessages {
		if m.ID == 0 {
			if _, err := g.insertMessage(ctx, m); err != nil {
				return errors.Wrap(err, "failed to add message")
			}
		} else {
			if err := g.updateMessage(ctx, m); err != nil {
				return errors.Wrap(err, "failed to update message")
			}
		}
	}
	return nil
}

func (g *ChatRepository) Create(ctx context.Context, data chat.Chat) (chat.Chat, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbChat, dbMessages := ToDBChat(data)
	if err := tx.QueryRow(
		ctx,
		insertChatQuery,
		dbChat.ClientID,
		&dbChat.CreatedAt,
	).Scan(&dbChat.ID); err != nil {
		return nil, errors.Wrap(err, "failed to insert chat")
	}
	if err := g.saveMessages(ctx, dbMessages); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, dbChat.ID)
}

func (g *ChatRepository) Update(ctx context.Context, data chat.Chat) (chat.Chat, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbChat, dbMessages := ToDBChat(data)
	if _, err := tx.Exec(
		ctx,
		updateChatQuery,
		dbChat.ClientID,
		&dbChat.CreatedAt,
		&dbChat.LastMessageAt,
		dbChat.ID,
	); err != nil {
		return nil, errors.Wrap(err, "failed to update chat")
	}
	if err := g.saveMessages(ctx, dbMessages); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, dbChat.ID)
}

func (g *ChatRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}
	if _, err := tx.Exec(ctx, deleteChatQuery, id); err != nil {
		return errors.Wrapf(err, "failed to delete chat with id %d", id)
	}
	return nil
}

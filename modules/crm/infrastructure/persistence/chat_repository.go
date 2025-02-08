package persistence

import (
	"context"
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
			m.sender_user_id,
			m.sender_client_id,
			m.is_read,
			m.read_at,
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

	countMessagesQuery = `SELECT COUNT(*) as count FROM messages`

	insertMessageQuery = `
		INSERT INTO messages (
			chat_id,
			message,
			sender_user_id,
			sender_client_id,
			is_read,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	updateMessageQuery = `
		UPDATE messages SET 
			chat_id = $1,
			message = $2,
			sender_user_id = $3,
			sender_client_id = $4,
			is_read = $5, 
			read_at = $6
		WHERE id = $7
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
		domainChat, err := toDomainChat(c, messages)
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
		if err := rows.Scan(
			&msg.ID,
			&msg.ChatID,
			&msg.Message,
			&msg.SenderUserID,
			&msg.SenderClientID,
			&msg.IsRead,
			&msg.ReadAt,
			&msg.CreatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan message")
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
			sender = chat.NewUserSender(user.ID, user.FirstName, user.LastName)
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
			sender = chat.NewClientSender(client.ID, client.FirstName, client.LastName.String)
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

		domainMessage, err := toDomainMessage(message, dbUploads, sender)
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
	id, err := g.insertMessage(ctx, toDBMessage(message, chatID))
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

	dbChat, dbMessages := toDBChat(data)
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

	dbChat, dbMessages := toDBChat(data)
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

package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"

	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrMessageNotFound = errors.New("message not found")
)

const (
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

	updateMessageQuery = `UPDATE messages SET 
		chat_id = $1,
		message = $2,
		sender_user_id = $3,
		sender_client_id = $4,
		is_read = $5, 
		created_at = $6
		WHERE id = $7`

	deleteMessageQuery = `DELETE FROM messages WHERE id = $1`
)

type MessageRepository struct {
}

func NewMessageRepository() message.Repository {
	return &MessageRepository{}
}

func (g *MessageRepository) queryMessages(ctx context.Context, query string, args ...interface{}) ([]message.Message, error) {
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

	messages := make([]message.Message, 0, len(dbMessages))
	for _, dbMessage := range dbMessages {
		var sender message.Sender
		if dbMessage.SenderUserID.Valid {
			var user coremodels.User
			if err := pool.QueryRow(ctx, selectMessageUserSender, dbMessage.SenderUserID.Int64).Scan(
				&user.ID,
				&user.FirstName,
				&user.LastName,
			); err != nil {
				return nil, errors.Wrap(err, "failed to scan user sender")
			}
			sender = message.NewUserSender(user.ID, user.FirstName, user.LastName)
		}

		if dbMessage.SenderClientID.Valid {
			var client models.Client
			if err := pool.QueryRow(ctx, selectMessageClientSender, dbMessage.SenderClientID.Int64).Scan(
				&client.ID,
				&client.FirstName,
				&client.LastName,
			); err != nil {
				return nil, errors.Wrap(err, "failed to scan client sender")
			}
			sender = message.NewClientSender(client.ID, client.FirstName, client.LastName.String)
		}

		uploads, err := pool.Query(ctx, selectMessageAttachmentsQuery, dbMessage.ID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to query attachments for message %d", dbMessage.ID)
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

		domainMessage, err := toDomainMessage(dbMessage, dbUploads, sender)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert to domain message")
		}
		messages = append(messages, domainMessage)
	}

	return messages, nil
}

func (g *MessageRepository) GetPaginated(
	ctx context.Context, params *message.FindParams,
) ([]message.Message, error) {
	sortFields := []string{}
	for _, f := range params.SortBy.Fields {
		switch f {
		case message.CreatedAt:
			sortFields = append(sortFields, "m.created_at")
		default:
			return nil, errors.Wrapf(fmt.Errorf("unknown sort field"), "invalid sort field: %v", f)
		}
	}

	where, args := []string{"1 = 1"}, []interface{}{}
	if params.Search != "" {
		where = append(
			where,
			"m.message ILIKE $1",
		)
		args = append(args, "%"+params.Search+"%")
	}
	return g.queryMessages(
		ctx,
		repo.Join(
			selectMessagesQuery,
			repo.JoinWhere(where...),
			repo.OrderBy(sortFields, params.SortBy.Ascending),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
		args...,
	)
}

func (g *MessageRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}
	var count int64
	if err := pool.QueryRow(ctx, countMessagesQuery).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "failed to count messages")
	}
	return count, nil
}

func (g *MessageRepository) GetAll(ctx context.Context) ([]message.Message, error) {
	messages, err := g.queryMessages(ctx, selectMessagesQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all messages")
	}
	return messages, nil
}

func (g *MessageRepository) GetByID(ctx context.Context, id uint) (message.Message, error) {
	messages, err := g.queryMessages(ctx, selectMessagesQuery+" WHERE m.id = $1", id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get message with id %d", id)
	}
	if len(messages) == 0 {
		return nil, ErrMessageNotFound
	}
	return messages[0], nil
}

func (g *MessageRepository) GetByChatID(ctx context.Context, chatID uint) ([]message.Message, error) {
	messages, err := g.queryMessages(ctx, selectMessagesQuery+" WHERE m.chat_id = $1", chatID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get messages for chat %d", chatID)
	}
	return messages, nil
}

func (g *MessageRepository) Create(ctx context.Context, data message.Message) (message.Message, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbMessage := toDBMessage(data)
	if err := tx.QueryRow(
		ctx,
		insertMessageQuery,
		dbMessage.ChatID,
		dbMessage.Message,
		dbMessage.SenderUserID,
		dbMessage.SenderClientID,
		dbMessage.IsRead,
		dbMessage.CreatedAt,
	).Scan(&dbMessage.ID); err != nil {
		return nil, errors.Wrap(err, "failed to insert message")
	}
	return g.GetByID(ctx, dbMessage.ID)
}

func (g *MessageRepository) Update(ctx context.Context, data message.Message) (message.Message, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}
	dbMessage := toDBMessage(data)

	if _, err := tx.Exec(
		ctx,
		updateMessageQuery,
		dbMessage.ChatID,
		dbMessage.Message,
		dbMessage.SenderUserID,
		dbMessage.SenderClientID,
		dbMessage.IsRead,
		dbMessage.CreatedAt,
		dbMessage.ID,
	); err != nil {
		return nil, errors.Wrapf(err, "failed to update message %d", dbMessage.ID)
	}
	return g.GetByID(ctx, data.ID())
}

func (g *MessageRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}
	if _, err := tx.Exec(ctx, deleteMessageQuery, id); err != nil {
		return errors.Wrapf(err, "failed to delete message with id %d", id)
	}
	return nil
}

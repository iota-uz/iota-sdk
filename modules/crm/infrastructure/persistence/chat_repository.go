package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrChatNotFound = errors.New("chat not found")
)

const (
	selectChatQuery = `
		SELECT 
			c.id,
			c.created_at,
			c.client_id,
			cl.id,
			cl.first_name,
			cl.last_name,
			cl.middle_name,
			cl.phone_number,
			cl.created_at,
			cl.updated_at
		FROM chats c LEFT JOIN clients cl ON c.client_id = cl.id
	`

	selectMessagesForChatsQuery = `
		SELECT 
			m.id,
			m.created_at,
			m.chat_id,
			m.message,
			m.sender_user_id,
			m.sender_client_id,
			m.is_active
		FROM messages m
		WHERE m.chat_id = ANY($1) ORDER BY m.created_at DESC
	`

	countChatQuery = `SELECT COUNT(*) as count FROM chats`

	insertChatQuery = `
		INSERT INTO chats (
			client_id,
			created_at
		) VALUES ($1, $2) RETURNING id`

	insertMessageQuery = `
		INSERT INTO messages (
			chat_id,
			message,
			sender_user_id,
			sender_client_id,
			is_active,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	updateMessageQuery = `UPDATE messages SET is_active = $1 WHERE id = $2`

	deleteChatQuery = `DELETE FROM chats WHERE id = $1`
)

type ChatRepository struct {
}

func NewChatRepository() chat.Repository {
	return &ChatRepository{}
}

func (g *ChatRepository) queryChats(ctx context.Context, query string, args ...interface{}) ([]chat.Chat, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	// First, query all chats
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make([]chat.Chat, 0)
	chatIDs := make([]uint, 0)

	// Collect all chats and their IDs
	for rows.Next() {
		var c models.Chat
		var dbClient models.Client
		if err := rows.Scan(
			&c.ID,
			&c.CreatedAt,
			&c.ClientID,
			&dbClient.ID,
			&dbClient.FirstName,
			&dbClient.LastName,
			&dbClient.MiddleName,
			&dbClient.PhoneNumber,
			&dbClient.CreatedAt,
			&dbClient.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan chat")
		}
		entity, err := toDomainChat(&c, &dbClient, []*models.Message{})
		if err != nil {
			return nil, err
		}
		chats = append(chats, entity)
		chatIDs = append(chatIDs, c.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(chats) == 0 {
		return []chat.Chat{}, nil
	}

	// Query all messages for these chats in a single query
	messageRows, err := pool.Query(ctx, selectMessagesForChatsQuery, chatIDs)
	if err != nil {
		return nil, err
	}
	defer messageRows.Close()

	// Create a map to store messages by chat ID
	messagesByChatID := make(map[uint][]*models.Message)

	// Collect all messages
	for messageRows.Next() {
		var m models.Message
		if err := messageRows.Scan(
			&m.ID,
			&m.CreatedAt,
			&m.ChatID,
			&m.Message,
			&m.SenderUserID,
			&m.SenderClientID,
			&m.IsActive,
		); err != nil {
			return nil, err
		}
		messagesByChatID[m.ChatID] = append(messagesByChatID[m.ChatID], &m)
	}

	if err := messageRows.Err(); err != nil {
		return nil, err
	}

	for i, c := range chats {
		domainMessages, err := mapping.MapDBModels(messagesByChatID[c.ID()], toDomainMessage)
		if err != nil {
			return nil, err
		}
		chats[i] = c.AddMessages(domainMessages...)
	}
	return chats, nil
}

func (g *ChatRepository) createMessage(ctx context.Context, msg *models.Message) (message.Message, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(
		ctx,
		insertMessageQuery,
		msg.ChatID,
		msg.Message,
		msg.SenderUserID,
		msg.SenderClientID,
		msg.IsActive,
		&msg.CreatedAt,
	).Scan(&msg.ID); err != nil {
		return nil, err
	}
	return toDomainMessage(msg)
}

func (g *ChatRepository) GetPaginated(
	ctx context.Context, params *chat.FindParams,
) ([]chat.Chat, error) {
	sortFields := []string{}
	for _, f := range params.SortBy.Fields {
		switch f {
		case chat.CreatedAt:
			sortFields = append(sortFields, "c.created_at")
		default:
			return nil, fmt.Errorf("unknown sort field: %s", f)
		}
	}
	return g.queryChats(
		ctx,
		repo.Join(
			selectChatQuery,
			repo.FormatLimitOffset(params.Limit, params.Offset),
			repo.OrderBy(sortFields, params.SortBy.Ascending),
		),
	)
}

func (g *ChatRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, countChatQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *ChatRepository) GetAll(ctx context.Context) ([]chat.Chat, error) {
	return g.queryChats(ctx, selectChatQuery)
}

func (g *ChatRepository) GetByID(ctx context.Context, id uint) (chat.Chat, error) {
	chats, err := g.queryChats(ctx, selectChatQuery+" WHERE c.id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(chats) == 0 {
		return nil, ErrChatNotFound
	}
	return chats[0], nil
}

func (g *ChatRepository) GetByClientID(ctx context.Context, clientID uint) (chat.Chat, error) {
	chats, err := g.queryChats(ctx, selectChatQuery+" WHERE c.client_id = $1", clientID)
	if err != nil {
		return nil, err
	}
	if len(chats) == 0 {
		return nil, ErrChatNotFound
	}
	return chats[0], nil
}

func (g *ChatRepository) Create(ctx context.Context, data chat.Chat) (chat.Chat, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	dbChat, dbMessages := toDBChat(data)
	if err := tx.QueryRow(
		ctx,
		insertChatQuery,
		dbChat.ClientID,
		&dbChat.CreatedAt,
	).Scan(&dbChat.ID); err != nil {
		return nil, err
	}

	for i := range dbMessages {
		dbMessages[i].ChatID = dbChat.ID
		if _, err := g.createMessage(ctx, dbMessages[i]); err != nil {
			return nil, err
		}
	}

	return g.GetByID(ctx, dbChat.ID)
}

func (g *ChatRepository) Update(ctx context.Context, data chat.Chat) (chat.Chat, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	dbChat, dbMessages := toDBChat(data)
	for i := range dbMessages {
		if dbMessages[i].ID == 0 {
			if _, err := g.createMessage(ctx, dbMessages[i]); err != nil {
				return nil, errors.Wrap(err, "failed to create message")
			}
		} else {
			if _, err := tx.Exec(
				ctx,
				updateMessageQuery,
				dbMessages[i].IsActive,
				dbMessages[i].ID,
			); err != nil {
				return nil, errors.Wrap(err, "failed to update message")
			}
		}

	}
	return g.GetByID(ctx, dbChat.ID)
}

func (g *ChatRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteChatQuery, id); err != nil {
		return err
	}
	return nil
}

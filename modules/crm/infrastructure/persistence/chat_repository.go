package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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
			c.last_message_at,
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

	countChatQuery = `SELECT COUNT(*) as count FROM chats`

	insertChatQuery = `
		INSERT INTO chats (
			client_id,
			created_at
		) VALUES ($1, $2) RETURNING id
	`

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
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute chat query")
	}
	defer rows.Close()

	chats := make([]chat.Chat, 0)

	for rows.Next() {
		var c models.Chat
		var dbClient models.Client
		if err := rows.Scan(
			&c.ID,
			&c.CreatedAt,
			&c.LastMessageAt,
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
		entity, err := toDomainChat(&c, &dbClient)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert to domain chat")
		}
		chats = append(chats, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error occurred while iterating chat rows")
	}

	return chats, nil
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

func (g *ChatRepository) Create(ctx context.Context, data chat.Chat) (chat.Chat, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbChat := toDBChat(data)
	if err := tx.QueryRow(
		ctx,
		insertChatQuery,
		dbChat.ClientID,
		&dbChat.CreatedAt,
	).Scan(&dbChat.ID); err != nil {
		return nil, errors.Wrap(err, "failed to insert chat")
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

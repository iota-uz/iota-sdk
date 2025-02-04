package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrClientNotFound = errors.New("client not found")
)

const (
	selectClientQuery = `
		SELECT 
			c.id,
			c.first_name,
			c.last_name,
			c.middle_name,
			c.phone_number,
			c.created_at,
			c.updated_at
		FROM clients c
	`
	countClientQuery  = `SELECT COUNT(*) as count FROM clients`
	insertClientQuery = `
		INSERT INTO clients (
			first_name, 
			last_name, 
			middle_name, 
			phone_number
		) VALUES ($1, $2, $3, $4) RETURNING id`
	updateClientQuery = `
		UPDATE clients 
		SET first_name = $1, last_name = $2, middle_name = $3, phone_number = $4
		WHERE id = $5`
	deleteChatMessagesQuery = `DELETE FROM messages WHERE chat_id IN (SELECT id FROM chats WHERE client_id = $1)`
	deleteClientChatsQuery  = `DELETE FROM chats WHERE client_id = $1`
	deleteClientQuery       = `DELETE FROM clients WHERE id = $1`
)

type ClientRepository struct {
}

func NewClientRepository() client.Repository {
	return &ClientRepository{}
}

func (g *ClientRepository) queryClients(ctx context.Context, query string, args ...interface{}) ([]client.Client, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clients := make([]client.Client, 0)

	for rows.Next() {
		var c models.Client
		if err := rows.Scan(
			&c.ID,
			&c.FirstName,
			&c.LastName,
			&c.MiddleName,
			&c.PhoneNumber,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		entity, err := toDomainClient(&c)
		if err != nil {
			return nil, err
		}
		clients = append(clients, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return clients, nil
}

func (g *ClientRepository) GetPaginated(
	ctx context.Context, params *client.FindParams,
) ([]client.Client, error) {
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("c.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}
	if params.Query != "" && params.Field != "" {
		where, args = append(where, fmt.Sprintf("c.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1)), append(args, "%"+params.Query+"%")
	}
	sortFields := make([]string, 0, len(params.SortBy.Fields))
	for _, f := range params.SortBy.Fields {
		switch f {
		case client.FirstName:
			sortFields = append(sortFields, "c.first_name")
		case client.LastName:
			sortFields = append(sortFields, "c.last_name")
		case client.MiddleName:
			sortFields = append(sortFields, "c.middle_name")
		case client.PhoneNumber:
			sortFields = append(sortFields, "c.phone_number")
		case client.UpdatedAt:
			sortFields = append(sortFields, "c.updated_at")
		case client.CreatedAt:
			sortFields = append(sortFields, "c.created_at")
		}
	}
	sql := repo.Join(
		selectClientQuery,
		repo.JoinWhere(where...),
		repo.OrderBy(sortFields, params.SortBy.Ascending),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryClients(
		ctx,
		sql,
		args...,
	)
}

func (g *ClientRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, countClientQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *ClientRepository) GetAll(ctx context.Context) ([]client.Client, error) {
	return g.queryClients(ctx, selectClientQuery)
}

func (g *ClientRepository) GetByID(ctx context.Context, id uint) (client.Client, error) {
	clients, err := g.queryClients(ctx, selectClientQuery+" WHERE c.id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return nil, ErrClientNotFound
	}
	return clients[0], nil
}

func (g *ClientRepository) GetByPhone(ctx context.Context, phoneNumber string) (client.Client, error) {
	clients, err := g.queryClients(ctx, selectClientQuery+" WHERE c.phone_number = $1", phoneNumber)
	if err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return nil, ErrClientNotFound
	}
	return clients[0], nil
}

func (g *ClientRepository) Create(ctx context.Context, data client.Client) (client.Client, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	dbRow := toDBClient(data)
	if err := tx.QueryRow(
		ctx,
		insertClientQuery,
		dbRow.FirstName,
		dbRow.LastName,
		dbRow.MiddleName,
		dbRow.PhoneNumber,
	).Scan(&dbRow.ID); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, dbRow.ID)
}

func (g *ClientRepository) Update(ctx context.Context, data client.Client) (client.Client, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	dbRow := toDBClient(data)
	if _, err := tx.Exec(
		ctx,
		updateClientQuery,
		dbRow.FirstName,
		dbRow.LastName,
		dbRow.MiddleName,
		dbRow.PhoneNumber,
		data.ID(),
	); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, data.ID())
}

func (g *ClientRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteChatMessagesQuery, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteClientChatsQuery, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteClientQuery, id); err != nil {
		return err
	}
	return nil
}

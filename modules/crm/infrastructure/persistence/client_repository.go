package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
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
			c.address,
			c.email,
			c.date_of_birth,
			c.gender,
			c.passport_id,
			c.pin,
			c.created_at,
			c.updated_at
		FROM clients c
	`
	countClientQuery        = `SELECT COUNT(*) as count FROM clients`
	deleteChatMessagesQuery = `DELETE FROM messages WHERE chat_id IN (SELECT id FROM chats WHERE client_id = $1)`
	deleteClientChatsQuery  = `DELETE FROM chats WHERE client_id = $1`
	deleteClientQuery       = `DELETE FROM clients WHERE id = $1`
)

type ClientRepository struct {
	passportRepo passport.Repository
}

func NewClientRepository(passportRepo passport.Repository) client.Repository {
	return &ClientRepository{
		passportRepo: passportRepo,
	}
}

func (g *ClientRepository) queryClients(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]client.Client, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	// First collect all client records
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Temporary slice to store client records and passport IDs
	clientRecords := make([]models.Client, 0)
	passportIDs := make([]string, 0)

	// Collect all client records and passport IDs first
	for rows.Next() {
		var c models.Client
		if err := rows.Scan(
			&c.ID,
			&c.FirstName,
			&c.LastName,
			&c.MiddleName,
			&c.PhoneNumber,
			&c.Address,
			&c.Email,
			&c.DateOfBirth,
			&c.Gender,
			&c.PassportID,
			&c.Pin,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}

		clientRecords = append(clientRecords, c)

		// Collect passport IDs for later retrieval
		if c.PassportID.Valid {
			passportIDs = append(passportIDs, c.PassportID.String)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// If no clients found, return empty slice
	if len(clientRecords) == 0 {
		return []client.Client{}, nil
	}

	// Create a map to store passport data by ID
	passportMap := make(map[string]passport.Passport)

	// Fetch passport data if there are any passport IDs
	if len(passportIDs) > 0 {
		// Fetch passports individually using passport repository
		// In a real implementation, the passport repository should have a GetByIDs batch method
		for _, passportID := range passportIDs {
			passportEntity, err := g.passportRepo.GetByID(ctx, uuid.MustParse(passportID))
			if err != nil {
				// If we can't get a passport, skip it (don't fail the whole operation)
				continue
			}
			passportMap[passportID] = passportEntity
		}
	}

	// Create domain client entities
	clients := make([]client.Client, 0, len(clientRecords))
	for _, c := range clientRecords {
		var passportData passport.Passport

		// Get passport from map if it exists, otherwise create an empty one
		if c.PassportID.Valid {
			var ok bool
			passportData, ok = passportMap[c.PassportID.String]
			if !ok {
				passportData = passport.New("", "")
			}
		} else {
			passportData = passport.New("", "")
		}

		// Create complete client with passport data
		entity, err := ToDomainClientComplete(&c, passportData)
		if err != nil {
			return nil, err
		}

		clients = append(clients, entity)
	}

	return clients, nil
}

func (g *ClientRepository) exists(ctx context.Context, id uint) (bool, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return false, err
	}
	var exists bool
	q := "SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1)"
	if err := pool.QueryRow(ctx, q, id).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
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

	if params.Search != "" {
		where = append(where, "c.first_name ILIKE $1 OR c.last_name ILIKE $1 OR c.middle_name ILIKE $1 OR c.phone_number ILIKE $1")
		args = append(args, "%"+params.Search+"%")
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

	dbRow := ToDBClient(data)

	if data.Passport() != nil {
		p, err := g.passportRepo.Save(ctx, data.Passport())
		if err != nil {
			return nil, err
		}
		dbRow.PassportID = sql.NullString{
			String: p.ID().String(),
			Valid:  true,
		}
	}

	fields := []string{
		"first_name",
		"last_name",
		"middle_name",
		"phone_number",
		"address",
		"email",
		"date_of_birth",
		"gender",
		"passport_id",
		"pin",
		"created_at",
		"updated_at",
	}

	values := []interface{}{
		dbRow.FirstName,
		dbRow.LastName,
		dbRow.MiddleName,
		dbRow.PhoneNumber,
		dbRow.Address,
		dbRow.Email,
		dbRow.DateOfBirth,
		dbRow.Gender,
		dbRow.PassportID,
		dbRow.Pin,
		dbRow.CreatedAt,
		dbRow.UpdatedAt,
	}

	if efs, ok := data.(repo.ExtendedFieldSet); ok {
		fields = append(fields, efs.Fields()...)
		for _, k := range efs.Fields() {
			values = append(values, efs.Value(k))
		}
	}

	if err := tx.QueryRow(ctx, repo.Insert("clients", fields, "id"), values...).Scan(&dbRow.ID); err != nil {
		return nil, err
	}

	return g.GetByID(ctx, dbRow.ID)
}

func (g *ClientRepository) Update(ctx context.Context, data client.Client) (client.Client, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	dbRow := ToDBClient(data)
	if data.Passport() != nil {
		p, err := g.passportRepo.Save(ctx, data.Passport())
		if err != nil {
			return nil, err
		}
		dbRow.PassportID = sql.NullString{
			String: p.ID().String(),
			Valid:  true,
		}
	}

	fields := []string{
		"first_name",
		"last_name",
		"middle_name",
		"phone_number",
		"address",
		"email",
		"date_of_birth",
		"gender",
		"passport_id",
		"pin",
		"updated_at",
	}

	values := []interface{}{
		dbRow.FirstName,
		dbRow.LastName,
		dbRow.MiddleName,
		dbRow.PhoneNumber,
		dbRow.Address,
		dbRow.Email,
		dbRow.DateOfBirth,
		dbRow.Gender,
		dbRow.PassportID,
		dbRow.Pin,
		dbRow.UpdatedAt,
	}

	if efs, ok := data.(repo.ExtendedFieldSet); ok {
		fields = append(fields, efs.Fields()...)
		for _, k := range efs.Fields() {
			values = append(values, efs.Value(k))
		}
	}

	values = append(values, data.ID())

	if _, err := tx.Exec(
		ctx,
		repo.Update("clients", fields, fmt.Sprintf("id = $%d", len(values))),
		values...,
	); err != nil {
		return nil, err
	}

	return g.GetByID(ctx, data.ID())
}

func (g *ClientRepository) Save(ctx context.Context, data client.Client) (client.Client, error) {
	exists, err := g.exists(ctx, data.ID())
	if err != nil {
		return nil, err
	}
	if exists {
		return g.Update(ctx, data)
	}
	return g.Create(ctx, data)
}

func (g *ClientRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	var passportID sql.NullString
	err = tx.QueryRow(ctx, "SELECT passport_id FROM clients WHERE id = $1", id).Scan(&passportID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Delete all related chat messages
	if _, err := tx.Exec(ctx, deleteChatMessagesQuery, id); err != nil {
		return err
	}

	// Delete client chats
	if _, err := tx.Exec(ctx, deleteClientChatsQuery, id); err != nil {
		return err
	}

	// Delete the client record
	if _, err := tx.Exec(ctx, deleteClientQuery, id); err != nil {
		return err
	}

	// If client had a passport, delete it using passport repository
	if passportID.Valid {
		err = g.passportRepo.Delete(ctx, uuid.MustParse(passportID.String))
		if err != nil {
			return fmt.Errorf("failed to delete passport: %w", err)
		}
	}

	return nil
}

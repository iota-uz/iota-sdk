package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
			c.comments,
			c.created_at,
			c.updated_at
		FROM clients c
	`
	countClientQuery        = `SELECT COUNT(*) as count FROM clients`
	deleteChatMessagesQuery = `DELETE FROM messages WHERE chat_id IN (SELECT id FROM chats WHERE client_id = $1)`
	deleteClientChatsQuery  = `DELETE FROM chats WHERE client_id = $1`
	deleteClientQuery       = `DELETE FROM clients WHERE id = $1`

	selectClientContactsQuery        = `SELECT id, contact_type, contact_value, created_at, updated_at FROM client_contacts WHERE client_id = $1`
	insertClientContactQuery         = `INSERT INTO client_contacts (client_id, contact_type, contact_value, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`
	insertClientContactReturnIDQuery = `INSERT INTO client_contacts (client_id, contact_type, contact_value, created_at, updated_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	selectExistingContactsQuery      = `SELECT id, contact_type, contact_value FROM client_contacts WHERE client_id = $1`
	updateContactTimestampQuery      = `UPDATE client_contacts SET updated_at = $1 WHERE id = $2`
	deleteClientContactQuery         = `DELETE FROM client_contacts`

	clientExistsQuery = `SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1)`

	selectClientByContactQuery = `
		SELECT c.id FROM clients c
		JOIN client_contacts cc ON c.id = cc.client_id
		WHERE cc.contact_type = $1 AND cc.contact_value = $2
		LIMIT 1
	`

	selectClientPassportQuery = `SELECT passport_id FROM clients WHERE id = $1`
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
			&c.Comments,
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
			if p, ok := passportMap[c.PassportID.String]; ok {
				passportData = p
			}
		}

		// Create complete client with passport data
		entity, err := ToDomainClient(&c, passportData)
		if err != nil {
			return nil, err
		}

		// Load client contacts
		contactRows, err := pool.Query(ctx, selectClientContactsQuery, c.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load client contacts: %w", err)
		}

		var contacts []client.Contact
		for contactRows.Next() {
			var id uint
			var contactType, contactValue string
			var createdAt, updatedAt time.Time

			if err := contactRows.Scan(&id, &contactType, &contactValue, &createdAt, &updatedAt); err != nil {
				contactRows.Close()
				return nil, fmt.Errorf("failed to scan contact row: %w", err)
			}

			dbContact := &models.ClientContact{
				ID:           id,
				ContactType:  contactType,
				ContactValue: contactValue,
				CreatedAt:    createdAt,
				UpdatedAt:    updatedAt,
			}

			contact := ToDomainClientContact(dbContact)
			contacts = append(contacts, contact)
		}
		contactRows.Close()

		if len(contacts) > 0 {
			entity = entity.SetContacts(contacts)
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
	if err := pool.QueryRow(ctx, clientExistsQuery, id).Scan(&exists); err != nil {
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

func (g *ClientRepository) create(ctx context.Context, data client.Client) (client.Client, error) {
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

	// Process client contacts if any (excluding email and phone which are stored in the clients table)
	for _, contact := range data.Contacts() {
		// Skip email and phone contacts as they're stored directly in clients table
		if contact.Type() == client.ContactTypeEmail || contact.Type() == client.ContactTypePhone {
			continue
		}

		// Convert to DB model and insert the contact
		dbContact := ToDBClientContact(dbRow.ID, contact)
		_, err := tx.Exec(ctx, insertClientContactQuery,
			dbContact.ClientID, dbContact.ContactType, dbContact.ContactValue, dbContact.CreatedAt, dbContact.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to insert contact: %w", err)
		}
	}

	return g.GetByID(ctx, dbRow.ID)
}

func (g *ClientRepository) update(ctx context.Context, data client.Client) (client.Client, error) {
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
		"comments",
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
		dbRow.Comments,
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

	if contacts := data.Contacts(); len(contacts) > 0 {
		existingContactsRows, err := tx.Query(ctx, selectExistingContactsQuery, data.ID())
		if err != nil {
			return nil, err
		}
		defer existingContactsRows.Close()

		existingContacts := make(map[string]uint)
		for existingContactsRows.Next() {
			var id uint
			var contactType, contactValue string
			if err := existingContactsRows.Scan(&id, &contactType, &contactValue); err != nil {
				return nil, err
			}
			key := contactType + ":" + contactValue
			existingContacts[key] = id
		}

		for _, contact := range contacts {
			// TODO: remove
			if contact.Type() == client.ContactTypeEmail || contact.Type() == client.ContactTypePhone {
				continue
			}

			// Create a composite key for lookup
			key := string(contact.Type()) + ":" + contact.Value()

			if existingContactID, exists := existingContacts[key]; exists {
				delete(existingContacts, key)

				if contact.ID() != 0 && contact.ID() != existingContactID {
					if _, err := tx.Exec(ctx, updateContactTimestampQuery,
						time.Now(), existingContactID); err != nil {
						return nil, err
					}
				}
			} else {
				dbContact := ToDBClientContact(data.ID(), contact)
				var contactID uint
				err := tx.QueryRow(
					ctx,
					insertClientContactReturnIDQuery,
					dbContact.ClientID,
					dbContact.ContactType,
					dbContact.ContactValue,
					dbContact.CreatedAt,
					dbContact.UpdatedAt,
				).Scan(&contactID)
				if err != nil {
					return nil, err
				}
			}
		}

		for _, contactID := range existingContacts {
			if _, err := tx.Exec(ctx, repo.Join(deleteClientContactQuery, "WHERE id = $1"), contactID); err != nil {
				return nil, err
			}
		}
	}

	return g.GetByID(ctx, data.ID())
}

func (g *ClientRepository) Save(ctx context.Context, data client.Client) (client.Client, error) {
	exists, err := g.exists(ctx, data.ID())
	if err != nil {
		return nil, err
	}
	if exists {
		return g.update(ctx, data)
	}
	return g.create(ctx, data)
}

func (g *ClientRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	var passportID sql.NullString
	err = tx.QueryRow(ctx, selectClientPassportQuery, id).Scan(&passportID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Delete chat messages related to this client
	if _, err := tx.Exec(ctx, deleteChatMessagesQuery, id); err != nil {
		return err
	}

	// Delete client chats
	if _, err := tx.Exec(ctx, deleteClientChatsQuery, id); err != nil {
		return err
	}

	// Delete client contacts (will be handled by CASCADE on client delete, but explicit for clarity)
	if _, err := tx.Exec(ctx, repo.Join(deleteClientContactQuery, "WHERE client_id = $1"), id); err != nil {
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

func (g *ClientRepository) GetByContactValue(
	ctx context.Context,
	contactType client.ContactType,
	value string,
) (client.Client, error) {
	if contactType == client.ContactTypePhone {
		return g.GetByPhone(ctx, value)
	}

	if contactType == client.ContactTypeEmail {
		clients, err := g.queryClients(ctx, repo.Join(selectClientQuery, "WHERE c.email = $1"), value)
		if err != nil {
			return nil, err
		}
		if len(clients) == 0 {
			return nil, ErrClientNotFound
		}
		return clients[0], nil
	}

	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var clientID uint
	err = pool.QueryRow(ctx, selectClientByContactQuery, string(contactType), value).Scan(&clientID)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, err
	}

	return g.GetByID(ctx, clientID)
}

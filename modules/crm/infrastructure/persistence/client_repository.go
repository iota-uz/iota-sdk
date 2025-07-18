package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
			c.tenant_id,
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
	countClientQuery        = `SELECT COUNT(*) as count FROM clients c`
	deleteChatMessagesQuery = `DELETE FROM messages WHERE chat_id IN (SELECT id FROM chats WHERE client_id = $1)`
	deleteClientChatsQuery  = `DELETE FROM chats WHERE client_id = $1`
	deleteClientQuery       = `DELETE FROM clients WHERE id = $1`

	selectClientContactsQuery = `SELECT id, contact_type, contact_value, created_at, updated_at FROM client_contacts WHERE client_id = $1`
	insertClientContactQuery  = `INSERT INTO client_contacts (client_id, contact_type, contact_value, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`
	updateClientContactQuery  = `UPDATE client_contacts SET contact_type = $1, contact_value = $2, updated_at = $3 WHERE id = $4`

	selectExistingContactsQuery = `SELECT id, contact_type, contact_value FROM client_contacts WHERE client_id = $1`
	updateContactTimestampQuery = `UPDATE client_contacts SET updated_at = $1 WHERE id = $2`
	deleteClientContactQuery    = `DELETE FROM client_contacts`

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
	fieldMap     map[client.Field]string
}

func NewClientRepository(passportRepo passport.Repository) client.Repository {
	return &ClientRepository{
		passportRepo: passportRepo,
		fieldMap: map[client.Field]string{
			client.ID:          "c.id",
			client.FirstName:   "c.first_name",
			client.LastName:    "c.last_name",
			client.MiddleName:  "c.middle_name",
			client.PhoneNumber: "c.phone_number",
			client.UpdatedAt:   "c.updated_at",
			client.CreatedAt:   "c.created_at",
			client.TenantID:    "c.tenant_id",
		},
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

	clientRecords := make([]models.Client, 0)
	passportIDs := make([]string, 0)

	for rows.Next() {
		var c models.Client
		if err := rows.Scan(
			&c.ID,
			&c.TenantID,
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

	passportMap := make(map[string]passport.Passport)

	if len(passportIDs) > 0 {
		for _, passportID := range passportIDs {
			passportEntity, err := g.passportRepo.GetByID(ctx, uuid.MustParse(passportID))
			if err != nil {
				continue
			}
			passportMap[passportID] = passportEntity
		}
	}

	// Create domain client entities
	clients := make([]client.Client, 0, len(clientRecords))
	for _, c := range clientRecords {
		var passportData passport.Passport

		if c.PassportID.Valid {
			if p, ok := passportMap[c.PassportID.String]; ok {
				passportData = p
			}
		}

		entity, err := ToDomainClient(&c, passportData)
		if err != nil {
			return nil, err
		}

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
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	where := make([]string, 0)
	args := make([]interface{}, 0)

	// Add tenant filter first
	where = append(where, fmt.Sprintf("c.tenant_id = $%d", len(args)+1))
	args = append(args, tenantID)

	// Apply filters
	for _, filter := range params.Filters {
		column, ok := g.fieldMap[filter.Column]
		if !ok {
			continue
		}
		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	if params.Search != "" {
		searchPlaceholder := fmt.Sprintf("$%d", len(args)+1)

		// Primary full text search using ts_vector
		searchConditions := []string{
			// Full text search with ts_vector (supports partial matching with prefix search)
			fmt.Sprintf("c.search_vector @@ plainto_tsquery('english', %s)", searchPlaceholder),
			// Prefix search for partial matching
			fmt.Sprintf("c.search_vector @@ to_tsquery('english', %s || ':*')", searchPlaceholder),
		}

		// Fallback to ILIKE for backwards compatibility and special cases
		searchConditions = append(searchConditions, fmt.Sprintf("c.first_name ILIKE %s OR c.last_name ILIKE %s OR c.middle_name ILIKE %s OR c.phone_number ILIKE %s", searchPlaceholder, searchPlaceholder, searchPlaceholder, searchPlaceholder))

		// Word-based search for multi-word queries
		words := strings.Fields(params.Search)
		if len(words) > 1 {
			// Full text search with multi-word query
			multiWordQuery := fmt.Sprintf("$%d", len(args)+2)
			searchConditions = append(searchConditions, fmt.Sprintf("c.search_vector @@ plainto_tsquery('english', %s)", multiWordQuery))
			args = append(args, params.Search)

			// Individual word matching for fallback
			wordConditions := make([]string, 0, len(words))
			for _, word := range words {
				wordPlaceholder := fmt.Sprintf("$%d", len(args)+2+len(wordConditions))
				wordConditions = append(wordConditions, fmt.Sprintf("(c.first_name ILIKE %s OR c.last_name ILIKE %s OR c.middle_name ILIKE %s)", wordPlaceholder, wordPlaceholder, wordPlaceholder))
				args = append(args, "%"+word+"%")
			}
			searchConditions = append(searchConditions, strings.Join(wordConditions, " AND "))
		}

		where = append(where, fmt.Sprintf("(%s)", strings.Join(searchConditions, " OR ")))
		args = append(args, params.Search, params.Search)
	}

	sql := repo.Join(
		selectClientQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(g.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	return g.queryClients(
		ctx,
		sql,
		args...,
	)
}

func (g *ClientRepository) Count(ctx context.Context, params *client.FindParams) (int64, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, err
	}

	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	where := make([]string, 0)
	args := make([]interface{}, 0)

	// Add tenant filter first
	where = append(where, fmt.Sprintf("c.tenant_id = $%d", len(args)+1))
	args = append(args, tenantID)

	// Apply filters
	for _, filter := range params.Filters {
		column, ok := g.fieldMap[filter.Column]
		if !ok {
			continue
		}
		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	if params.Search != "" {
		searchPlaceholder := fmt.Sprintf("$%d", len(args)+1)

		// Primary full text search using ts_vector
		searchConditions := []string{
			// Full text search with ts_vector (supports partial matching with prefix search)
			fmt.Sprintf("c.search_vector @@ plainto_tsquery('english', %s)", searchPlaceholder),
			// Prefix search for partial matching
			fmt.Sprintf("c.search_vector @@ to_tsquery('english', %s || ':*')", searchPlaceholder),
		}

		// Fallback to ILIKE for backwards compatibility and special cases
		searchConditions = append(searchConditions, fmt.Sprintf("c.first_name ILIKE %s OR c.last_name ILIKE %s OR c.middle_name ILIKE %s OR c.phone_number ILIKE %s", searchPlaceholder, searchPlaceholder, searchPlaceholder, searchPlaceholder))

		// Word-based search for multi-word queries
		words := strings.Fields(params.Search)
		if len(words) > 1 {
			// Full text search with multi-word query
			multiWordQuery := fmt.Sprintf("$%d", len(args)+2)
			searchConditions = append(searchConditions, fmt.Sprintf("c.search_vector @@ plainto_tsquery('english', %s)", multiWordQuery))
			args = append(args, params.Search)

			// Individual word matching for fallback
			wordConditions := make([]string, 0, len(words))
			for _, word := range words {
				wordPlaceholder := fmt.Sprintf("$%d", len(args)+2+len(wordConditions))
				wordConditions = append(wordConditions, fmt.Sprintf("(c.first_name ILIKE %s OR c.last_name ILIKE %s OR c.middle_name ILIKE %s)", wordPlaceholder, wordPlaceholder, wordPlaceholder))
				args = append(args, "%"+word+"%")
			}
			searchConditions = append(searchConditions, strings.Join(wordConditions, " AND "))
		}

		where = append(where, fmt.Sprintf("(%s)", strings.Join(searchConditions, " OR ")))
		args = append(args, params.Search, params.Search)
	}

	query := countClientQuery + " " + repo.JoinWhere(where...)

	var count int64
	if err := pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *ClientRepository) GetByID(ctx context.Context, id uint) (client.Client, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	clients, err := g.queryClients(ctx, selectClientQuery+" WHERE c.id = $1 AND c.tenant_id = $2", id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return nil, ErrClientNotFound
	}
	return clients[0], nil
}

// getByID is an internal helper method for getting a client by ID without tenant filtering
func (g *ClientRepository) getByID(ctx context.Context, id uint) (client.Client, error) {
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
		"tenant_id",
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
		dbRow.TenantID,
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
	q := repo.Insert("clients", fields, "id")
	if err := tx.QueryRow(ctx, q, values...).Scan(&dbRow.ID); err != nil {
		return nil, err
	}

	for _, contact := range data.Contacts() {
		if err := g.saveContact(ctx, dbRow.ID, contact); err != nil {
			return nil, err
		}
	}

	return g.getByID(ctx, dbRow.ID)
}

func (g *ClientRepository) saveContact(
	ctx context.Context,
	clientID uint,
	contact client.Contact,
) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	dbContact := ToDBClientContact(clientID, contact)

	if contact.ID() == 0 {
		if _, err = tx.Exec(
			ctx,
			insertClientContactQuery,
			dbContact.ClientID,
			dbContact.ContactType,
			dbContact.ContactValue,
			dbContact.CreatedAt,
			dbContact.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert contact: %w", err)
		}
	} else {
		if _, err := tx.Exec(
			ctx,
			updateClientContactQuery,
			dbContact.ContactType,
			dbContact.ContactValue,
			dbContact.UpdatedAt,
			contact.ID(),
		); err != nil {
			return fmt.Errorf("failed to update contact: %w", err)
		}
	}

	return nil
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

	values = append(values, data.ID(), data.TenantID())

	if _, err := tx.Exec(
		ctx,
		repo.Update("clients", fields, fmt.Sprintf("id = $%d AND tenant_id = $%d", len(values)-1, len(values))),
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
			key := string(contact.Type()) + ":" + contact.Value()

			if existingContactID, exists := existingContacts[key]; exists {
				delete(existingContacts, key)

				if contact.ID() != 0 && contact.ID() != existingContactID {
					if _, err := tx.Exec(
						ctx,
						updateContactTimestampQuery,
						time.Now(),
						existingContactID,
					); err != nil {
						return nil, err
					}
				}
			} else {
				if err := g.saveContact(ctx, data.ID(), contact); err != nil {
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

	return g.getByID(ctx, data.ID())
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
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	var passportID sql.NullString
	err = tx.QueryRow(ctx, selectClientPassportQuery, id).Scan(&passportID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if _, err := tx.Exec(ctx, deleteChatMessagesQuery, id); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, deleteClientChatsQuery, id); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, repo.Join(deleteClientContactQuery, "WHERE client_id = $1"), id); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, deleteClientQuery+" AND tenant_id = $2", id, tenantID); err != nil {
		return err
	}

	if passportID.Valid {
		err = g.passportRepo.Delete(ctx, uuid.MustParse(passportID.String))
		if err != nil {
			return fmt.Errorf("failed to delete passport: %w", err)
		}
	}

	return nil
}

func (g *ClientRepository) getByContactValue(
	ctx context.Context,
	contactType client.ContactType,
	value string,
) (client.Client, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var clientID uint
	err = tx.QueryRow(ctx, selectClientByContactQuery, string(contactType), value).Scan(&clientID)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, err
	}

	return g.getByID(ctx, clientID)
}

func (g *ClientRepository) GetByContactValue(
	ctx context.Context,
	contactType client.ContactType,
	value string,
) (client.Client, error) {
	if contactType == client.ContactTypePhone {
		v, err := g.GetByPhone(ctx, value)
		if err == nil {
			return v, nil
		}
		if err != nil && !errors.Is(err, ErrClientNotFound) {
			return nil, err
		}
	}

	if contactType == client.ContactTypeEmail {
		clients, err := g.queryClients(ctx, repo.Join(selectClientQuery, "WHERE c.email = $1"), value)
		if err != nil {
			return nil, err
		}
		if len(clients) > 0 {
			return clients[0], nil
		}
	}
	return g.getByContactValue(ctx, contactType, value)
}

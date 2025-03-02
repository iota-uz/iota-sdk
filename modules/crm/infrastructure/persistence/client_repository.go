package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/passport"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
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
			c.hourly_rate,
			c.date_of_birth,
			c.gender,
			c.passport_id,
			c.pin,
			c.created_at,
			c.updated_at
		FROM clients c
	`
	// For getting passport data when needed
	selectPassportQuery = `
		SELECT 
			id,
			first_name,
			last_name,
			middle_name,
			gender,
			birth_date,
			birth_place,
			nationality,
			passport_type,
			passport_number,
			series,
			issuing_country,
			issued_at,
			issued_by,
			expires_at,
			machine_readable_zone,
			biometric_data,
			signature_image,
			remarks
		FROM passports
		WHERE id = $1
	`
	countClientQuery  = `SELECT COUNT(*) as count FROM clients`
	insertClientQuery = `
		INSERT INTO clients (
			first_name, 
			last_name, 
			middle_name, 
			phone_number,
			address,
			email,
			hourly_rate,
			date_of_birth,
			gender,
			passport_id,
			pin
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`
	updateClientQuery = `
		UPDATE clients 
		SET first_name = $1, last_name = $2, middle_name = $3, phone_number = $4,
		    address = $5, email = $6, hourly_rate = $7, date_of_birth = $8,
		    gender = $9, passport_id = $10, pin = $11
		WHERE id = $12`
	deleteChatMessagesQuery = `DELETE FROM messages WHERE chat_id IN (SELECT id FROM chats WHERE client_id = $1)`
	deleteClientChatsQuery  = `DELETE FROM chats WHERE client_id = $1`
	deleteClientQuery       = `DELETE FROM clients WHERE id = $1`
)

type ClientRepository struct {
}

func NewClientRepository() client.Repository {
	return &ClientRepository{}
}

// Helper method to load a passport for a client if it has a passport ID
func (g *ClientRepository) loadPassportForClient(ctx context.Context, clientPassportID string) (passport.Passport, error) {
	// If no passport ID, return empty passport
	if clientPassportID == "" {
		return passport.New("", ""), nil
	}

	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	// Query the passport data
	var dbPassport coremodels.Passport

	err = pool.QueryRow(ctx, selectPassportQuery, clientPassportID).Scan(
		&dbPassport.ID,
		&dbPassport.FirstName,
		&dbPassport.LastName,
		&dbPassport.MiddleName,
		&dbPassport.Gender,
		&dbPassport.BirthDate,
		&dbPassport.BirthPlace,
		&dbPassport.Nationality,
		&dbPassport.PassportType,
		&dbPassport.PassportNumber,
		&dbPassport.Series,
		&dbPassport.IssuingCountry,
		&dbPassport.IssuedAt,
		&dbPassport.IssuedBy,
		&dbPassport.ExpiresAt,
		&dbPassport.MachineReadableZone,
		&dbPassport.BiometricData,
		&dbPassport.SignatureImage,
		&dbPassport.Remarks,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// No passport found with this ID, return empty passport
			return passport.New("", ""), nil
		}
		return nil, err
	}

	// Convert to domain passport
	return corepersistence.ToDomainPassport(&dbPassport), nil
}

func (g *ClientRepository) queryClients(ctx context.Context, query string, args ...interface{}) ([]client.Client, error) {
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
			&c.HourlyRate,
			&c.DateOfBirth,
			&c.Gender,
			&c.PassportID,
			&c.PIN,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}

		clientRecords = append(clientRecords, c)
		
		// Collect passport IDs for later batch query
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
		// Build query for IN clause
		passportQuery := `
			SELECT 
				id,
				first_name,
				last_name,
				middle_name,
				gender,
				birth_date,
				birth_place,
				nationality,
				passport_type,
				passport_number,
				series,
				issuing_country,
				issued_at,
				issued_by,
				expires_at,
				machine_readable_zone,
				biometric_data,
				signature_image,
				remarks
			FROM passports
			WHERE id = ANY($1)
		`
		
		// Execute batch query for passports
		passportRows, err := pool.Query(ctx, passportQuery, passportIDs)
		if err != nil {
			return nil, err
		}
		defer passportRows.Close()
		
		// Process passport rows
		for passportRows.Next() {
			var dbPassport coremodels.Passport
			err = passportRows.Scan(
				&dbPassport.ID,
				&dbPassport.FirstName,
				&dbPassport.LastName,
				&dbPassport.MiddleName,
				&dbPassport.Gender,
				&dbPassport.BirthDate,
				&dbPassport.BirthPlace,
				&dbPassport.Nationality,
				&dbPassport.PassportType,
				&dbPassport.PassportNumber,
				&dbPassport.Series,
				&dbPassport.IssuingCountry,
				&dbPassport.IssuedAt,
				&dbPassport.IssuedBy,
				&dbPassport.ExpiresAt,
				&dbPassport.MachineReadableZone,
				&dbPassport.BiometricData,
				&dbPassport.SignatureImage,
				&dbPassport.Remarks,
			)
			if err != nil {
				return nil, err
			}
			
			// Add passport to map
			passportMap[dbPassport.ID] = corepersistence.ToDomainPassport(&dbPassport)
		}
		
		if err := passportRows.Err(); err != nil {
			return nil, err
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
				// Create empty passport if the passport was not found for some reason
				passportData = passport.New("", "")
			}
		} else {
			// Create empty passport if no passport ID
			passportData = passport.New("", "")
		}
		
		// Create complete client with passport data
		entity, err := toDomainClientComplete(&c, passportData)
		if err != nil {
			return nil, err
		}
		
		clients = append(clients, entity)
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

// SQL queries for passport operations
const (
	insertPassportQuery = `
		INSERT INTO passports (
			first_name, last_name, middle_name, gender, birth_date, birth_place,
			nationality, passport_type, passport_number, series, issuing_country,
			issued_at, issued_by, expires_at, machine_readable_zone,
			biometric_data, signature_image, remarks
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id`

	updatePassportQuery = `
		UPDATE passports
		SET first_name = $1, last_name = $2, middle_name = $3, gender = $4,
			birth_date = $5, birth_place = $6, nationality = $7, passport_type = $8,
			passport_number = $9, series = $10, issuing_country = $11, issued_at = $12,
			issued_by = $13, expires_at = $14, machine_readable_zone = $15,
			biometric_data = $16, signature_image = $17, remarks = $18, updated_at = now()
		WHERE id = $19`
)

func (g *ClientRepository) Create(ctx context.Context, data client.Client) (client.Client, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	// First check if we need to create a passport
	var passportID sql.NullString

	if data.Passport() != nil && (data.Passport().Series() != "" || data.Passport().Number() != "") {
		// Create passport record
		dbPassport := corepersistence.ToDBPassport(data.Passport())

		// Save passport to database
		var passportUUID string
		err := tx.QueryRow(
			ctx,
			insertPassportQuery,
			dbPassport.FirstName,
			dbPassport.LastName,
			dbPassport.MiddleName,
			dbPassport.Gender,
			dbPassport.BirthDate,
			dbPassport.BirthPlace,
			dbPassport.Nationality,
			dbPassport.PassportType,
			dbPassport.PassportNumber,
			dbPassport.Series,
			dbPassport.IssuingCountry,
			dbPassport.IssuedAt,
			dbPassport.IssuedBy,
			dbPassport.ExpiresAt,
			dbPassport.MachineReadableZone,
			dbPassport.BiometricData,
			dbPassport.SignatureImage,
			dbPassport.Remarks,
		).Scan(&passportUUID)

		if err != nil {
			return nil, err
		}

		passportID = sql.NullString{
			String: passportUUID,
			Valid:  true,
		}
	}

	// Now create the client with passport reference
	dbRow := toDBClient(data)
	dbRow.PassportID = passportID

	if err := tx.QueryRow(
		ctx,
		insertClientQuery,
		dbRow.FirstName,
		dbRow.LastName,
		dbRow.MiddleName,
		dbRow.PhoneNumber,
		dbRow.Address,
		dbRow.Email,
		dbRow.HourlyRate,
		dbRow.DateOfBirth,
		dbRow.Gender,
		dbRow.PassportID,
		dbRow.PIN,
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

	// First, get the existing client to check passport status
	existingClient, err := g.GetByID(ctx, data.ID())
	if err != nil {
		return nil, err
	}

	// Check if we need to handle passport data
	var passportID sql.NullString

	// Check if client has passport data
	if data.Passport() != nil && (data.Passport().Series() != "" || data.Passport().Number() != "") {
		dbPassport := corepersistence.ToDBPassport(data.Passport())

		// Check if existing client already has a passport ID
		if existingClient.Passport() != nil && (existingClient.Passport().Series() != "" || existingClient.Passport().Number() != "") {
			// Get the existing client's passport ID by querying the database
			var existingPassportID string
			err := tx.QueryRow(ctx, "SELECT passport_id FROM clients WHERE id = $1", data.ID()).Scan(&existingPassportID)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}

			if existingPassportID != "" {
				// Update existing passport
				_, err = tx.Exec(
					ctx,
					updatePassportQuery,
					dbPassport.FirstName,
					dbPassport.LastName,
					dbPassport.MiddleName,
					dbPassport.Gender,
					dbPassport.BirthDate,
					dbPassport.BirthPlace,
					dbPassport.Nationality,
					dbPassport.PassportType,
					dbPassport.PassportNumber,
					dbPassport.Series,
					dbPassport.IssuingCountry,
					dbPassport.IssuedAt,
					dbPassport.IssuedBy,
					dbPassport.ExpiresAt,
					dbPassport.MachineReadableZone,
					dbPassport.BiometricData,
					dbPassport.SignatureImage,
					dbPassport.Remarks,
					existingPassportID,
				)
				if err != nil {
					return nil, err
				}

				passportID = sql.NullString{
					String: existingPassportID,
					Valid:  true,
				}
			} else {
				// Create new passport
				var passportUUID string
				err := tx.QueryRow(
					ctx,
					insertPassportQuery,
					dbPassport.FirstName,
					dbPassport.LastName,
					dbPassport.MiddleName,
					dbPassport.Gender,
					dbPassport.BirthDate,
					dbPassport.BirthPlace,
					dbPassport.Nationality,
					dbPassport.PassportType,
					dbPassport.PassportNumber,
					dbPassport.Series,
					dbPassport.IssuingCountry,
					dbPassport.IssuedAt,
					dbPassport.IssuedBy,
					dbPassport.ExpiresAt,
					dbPassport.MachineReadableZone,
					dbPassport.BiometricData,
					dbPassport.SignatureImage,
					dbPassport.Remarks,
				).Scan(&passportUUID)

				if err != nil {
					return nil, err
				}

				passportID = sql.NullString{
					String: passportUUID,
					Valid:  true,
				}
			}
		} else {
			// Create new passport
			var passportUUID string
			err := tx.QueryRow(
				ctx,
				insertPassportQuery,
				dbPassport.FirstName,
				dbPassport.LastName,
				dbPassport.MiddleName,
				dbPassport.Gender,
				dbPassport.BirthDate,
				dbPassport.BirthPlace,
				dbPassport.Nationality,
				dbPassport.PassportType,
				dbPassport.PassportNumber,
				dbPassport.Series,
				dbPassport.IssuingCountry,
				dbPassport.IssuedAt,
				dbPassport.IssuedBy,
				dbPassport.ExpiresAt,
				dbPassport.MachineReadableZone,
				dbPassport.BiometricData,
				dbPassport.SignatureImage,
				dbPassport.Remarks,
			).Scan(&passportUUID)

			if err != nil {
				return nil, err
			}

			passportID = sql.NullString{
				String: passportUUID,
				Valid:  true,
			}
		}
	}

	// Now update the client with passport reference
	dbRow := toDBClient(data)
	dbRow.PassportID = passportID

	if _, err := tx.Exec(
		ctx,
		updateClientQuery,
		dbRow.FirstName,
		dbRow.LastName,
		dbRow.MiddleName,
		dbRow.PhoneNumber,
		dbRow.Address,
		dbRow.Email,
		dbRow.HourlyRate,
		dbRow.DateOfBirth,
		dbRow.Gender,
		dbRow.PassportID,
		dbRow.PIN,
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

	// First get the client's passport ID before deleting (if any)
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

	if passportID.Valid {
		_, err = tx.Exec(ctx, "DELETE FROM passports WHERE id = $1", passportID.String)
		if err != nil {
			return err
		}
	}

	return nil
}

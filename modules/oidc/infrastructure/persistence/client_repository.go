package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	selectClientQuery = `
		SELECT
			id, client_id, client_secret_hash, name, application_type,
			redirect_uris, grant_types, response_types, scopes, auth_method,
			access_token_lifetime, id_token_lifetime, refresh_token_lifetime,
			require_pkce, is_active, created_at, updated_at
		FROM oidc_clients
	`

	countClientQuery = `SELECT COUNT(*) FROM oidc_clients`

	insertClientQuery = `
		INSERT INTO oidc_clients (
			id, client_id, client_secret_hash, name, application_type,
			redirect_uris, grant_types, response_types, scopes, auth_method,
			access_token_lifetime, id_token_lifetime, refresh_token_lifetime,
			require_pkce, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id, client_id, client_secret_hash, name, application_type,
			redirect_uris, grant_types, response_types, scopes, auth_method,
			access_token_lifetime, id_token_lifetime, refresh_token_lifetime,
			require_pkce, is_active, created_at, updated_at
	`

	updateClientQuery = `
		UPDATE oidc_clients
		SET client_secret_hash = $1, name = $2, application_type = $3,
			redirect_uris = $4, grant_types = $5, response_types = $6,
			scopes = $7, auth_method = $8, access_token_lifetime = $9,
			id_token_lifetime = $10, refresh_token_lifetime = $11,
			require_pkce = $12, is_active = $13, updated_at = $14
		WHERE id = $15
	`

	deleteClientQuery = `DELETE FROM oidc_clients WHERE id = $1`

	clientIDExistsQuery = `SELECT EXISTS(SELECT 1 FROM oidc_clients WHERE client_id = $1)`
)

type ClientRepository struct {
	fieldMap map[client.Field]string
}

func NewClientRepository() client.Repository {
	return &ClientRepository{
		fieldMap: map[client.Field]string{
			client.ClientIDField:        "oidc_clients.client_id",
			client.NameField:            "oidc_clients.name",
			client.ApplicationTypeField: "oidc_clients.application_type",
			client.IsActiveField:        "oidc_clients.is_active",
			client.CreatedAtField:       "oidc_clients.created_at",
			client.UpdatedAtField:       "oidc_clients.updated_at",
		},
	}
}

func (r *ClientRepository) Count(ctx context.Context, params *client.FindParams) (int64, error) {
	const op serrors.Op = "ClientRepository.Count"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	where, args := r.buildWhereClause(params)
	var query string
	if len(where) > 0 {
		query = repo.Join(countClientQuery, repo.JoinWhere(where...))
	} else {
		query = countClientQuery
	}

	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, serrors.E(op, err)
	}

	return count, nil
}

func (r *ClientRepository) GetAll(ctx context.Context) ([]client.Client, error) {
	const op serrors.Op = "ClientRepository.GetAll"

	query := selectClientQuery + " WHERE is_active = true ORDER BY created_at DESC"
	return r.queryClients(ctx, op, query)
}

func (r *ClientRepository) GetPaginated(ctx context.Context, params *client.FindParams) ([]client.Client, error) {
	const op serrors.Op = "ClientRepository.GetPaginated"

	where, args := r.buildWhereClause(params)

	var query string
	if len(where) > 0 {
		query = repo.Join(
			selectClientQuery,
			repo.JoinWhere(where...),
			params.SortBy.ToSQL(r.fieldMap),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		)
	} else {
		query = repo.Join(
			selectClientQuery,
			params.SortBy.ToSQL(r.fieldMap),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		)
	}

	return r.queryClients(ctx, op, query, args...)
}

func (r *ClientRepository) GetByID(ctx context.Context, id uuid.UUID) (client.Client, error) {
	const op serrors.Op = "ClientRepository.GetByID"

	query := selectClientQuery + " WHERE id = $1"
	clients, err := r.queryClients(ctx, op, query, id)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if len(clients) == 0 {
		return nil, serrors.E(op, serrors.NotFound, "client not found")
	}

	return clients[0], nil
}

func (r *ClientRepository) GetByClientID(ctx context.Context, clientID string) (client.Client, error) {
	const op serrors.Op = "ClientRepository.GetByClientID"

	query := selectClientQuery + " WHERE client_id = $1"
	clients, err := r.queryClients(ctx, op, query, clientID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if len(clients) == 0 {
		return nil, serrors.E(op, serrors.NotFound, "client not found")
	}

	return clients[0], nil
}

func (r *ClientRepository) ClientIDExists(ctx context.Context, clientID string) (bool, error) {
	const op serrors.Op = "ClientRepository.ClientIDExists"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, serrors.E(op, err)
	}

	var exists bool
	if err := tx.QueryRow(ctx, clientIDExistsQuery, clientID).Scan(&exists); err != nil {
		return false, serrors.E(op, err)
	}

	return exists, nil
}

func (r *ClientRepository) Create(ctx context.Context, c client.Client) (client.Client, error) {
	const op serrors.Op = "ClientRepository.Create"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	dbClient := ToDBClient(c)

	var result models.Client
	err = tx.QueryRow(
		ctx,
		insertClientQuery,
		dbClient.ID,
		dbClient.ClientID,
		dbClient.ClientSecretHash,
		dbClient.Name,
		dbClient.ApplicationType,
		dbClient.RedirectURIs,
		dbClient.GrantTypes,
		dbClient.ResponseTypes,
		dbClient.Scopes,
		dbClient.AuthMethod,
		dbClient.AccessTokenLifetime,
		dbClient.IDTokenLifetime,
		dbClient.RefreshTokenLifetime,
		dbClient.RequirePKCE,
		dbClient.IsActive,
		dbClient.CreatedAt,
		dbClient.UpdatedAt,
	).Scan(
		&result.ID,
		&result.ClientID,
		&result.ClientSecretHash,
		&result.Name,
		&result.ApplicationType,
		&result.RedirectURIs,
		&result.GrantTypes,
		&result.ResponseTypes,
		&result.Scopes,
		&result.AuthMethod,
		&result.AccessTokenLifetime,
		&result.IDTokenLifetime,
		&result.RefreshTokenLifetime,
		&result.RequirePKCE,
		&result.IsActive,
		&result.CreatedAt,
		&result.UpdatedAt,
	)

	if err != nil {
		return nil, serrors.E(op, err)
	}

	return ToDomainClient(&result)
}

func (r *ClientRepository) Update(ctx context.Context, c client.Client) error {
	const op serrors.Op = "ClientRepository.Update"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	dbClient := ToDBClient(c)

	result, err := tx.Exec(
		ctx,
		updateClientQuery,
		dbClient.ClientSecretHash,
		dbClient.Name,
		dbClient.ApplicationType,
		dbClient.RedirectURIs,
		dbClient.GrantTypes,
		dbClient.ResponseTypes,
		dbClient.Scopes,
		dbClient.AuthMethod,
		dbClient.AccessTokenLifetime,
		dbClient.IDTokenLifetime,
		dbClient.RefreshTokenLifetime,
		dbClient.RequirePKCE,
		dbClient.IsActive,
		dbClient.UpdatedAt,
		dbClient.ID,
	)

	if err != nil {
		return serrors.E(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "client not found")
	}

	return nil
}

func (r *ClientRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "ClientRepository.Delete"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, deleteClientQuery, id)
	if err != nil {
		return serrors.E(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "client not found")
	}

	return nil
}

// Helper methods

func (r *ClientRepository) buildWhereClause(params *client.FindParams) ([]string, []interface{}) {
	where := []string{}
	args := []interface{}{}

	// Apply filters
	for _, filter := range params.Filters {
		fieldName, ok := r.fieldMap[filter.Column]
		if !ok {
			continue
		}

		// Phase 2: Add support for filter operators (contains, startsWith, etc.)
		// For now, only exact match (equals) is supported
		where = append(where, fmt.Sprintf("%s = $%d", fieldName, len(args)+1))
		args = append(args, filter.Filter)
	}

	// Apply search
	if params.Search != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR client_id ILIKE $%d)", len(args)+1, len(args)+2))
		args = append(args, "%"+params.Search+"%", "%"+params.Search+"%")
	}

	return where, args
}

func (r *ClientRepository) queryClients(ctx context.Context, op serrors.Op, query string, args ...interface{}) ([]client.Client, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var clients []client.Client
	for rows.Next() {
		var dbClient models.Client
		if err := rows.Scan(
			&dbClient.ID,
			&dbClient.ClientID,
			&dbClient.ClientSecretHash,
			&dbClient.Name,
			&dbClient.ApplicationType,
			&dbClient.RedirectURIs,
			&dbClient.GrantTypes,
			&dbClient.ResponseTypes,
			&dbClient.Scopes,
			&dbClient.AuthMethod,
			&dbClient.AccessTokenLifetime,
			&dbClient.IDTokenLifetime,
			&dbClient.RefreshTokenLifetime,
			&dbClient.RequirePKCE,
			&dbClient.IsActive,
			&dbClient.CreatedAt,
			&dbClient.UpdatedAt,
		); err != nil {
			return nil, serrors.E(op, err)
		}

		domainClient, err := ToDomainClient(&dbClient)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		clients = append(clients, domainClient)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return clients, nil
}

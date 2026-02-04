package persistence

import (
	"database/sql"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

// copySlice creates a copy of a string slice to avoid aliasing
func copySlice(s []string) []string {
	if s == nil {
		return nil
	}
	result := make([]string, len(s))
	copy(result, s)
	return result
}

// Client mappers

func ToDBClient(c client.Client) *models.Client {
	var clientSecretHash sql.NullString
	if c.ClientSecretHash() != nil {
		clientSecretHash = mapping.ValueToSQLNullString(*c.ClientSecretHash())
	}

	return &models.Client{
		ID:                   c.ID().String(),
		ClientID:             c.ClientID(),
		ClientSecretHash:     clientSecretHash,
		Name:                 c.Name(),
		ApplicationType:      c.ApplicationType(),
		RedirectURIs:         copySlice(c.RedirectURIs()),
		GrantTypes:           copySlice(c.GrantTypes()),
		ResponseTypes:        copySlice(c.ResponseTypes()),
		Scopes:               copySlice(c.Scopes()),
		AuthMethod:           c.AuthMethod(),
		AccessTokenLifetime:  c.AccessTokenLifetime(),
		IDTokenLifetime:      c.IDTokenLifetime(),
		RefreshTokenLifetime: c.RefreshTokenLifetime(),
		RequirePKCE:          c.RequirePKCE(),
		IsActive:             c.IsActive(),
		CreatedAt:            c.CreatedAt(),
		UpdatedAt:            c.UpdatedAt(),
	}
}

func ToDomainClient(dbClient *models.Client) (client.Client, error) {
	id, err := uuid.Parse(dbClient.ID)
	if err != nil {
		return nil, err
	}

	opts := []client.Option{
		client.WithID(id),
		client.WithGrantTypes(dbClient.GrantTypes),
		client.WithResponseTypes(dbClient.ResponseTypes),
		client.WithScopes(dbClient.Scopes),
		client.WithAuthMethod(dbClient.AuthMethod),
		client.WithAccessTokenLifetime(dbClient.AccessTokenLifetime),
		client.WithIDTokenLifetime(dbClient.IDTokenLifetime),
		client.WithRefreshTokenLifetime(dbClient.RefreshTokenLifetime),
		client.WithRequirePKCE(dbClient.RequirePKCE),
		client.WithIsActive(dbClient.IsActive),
		client.WithCreatedAt(dbClient.CreatedAt),
		client.WithUpdatedAt(dbClient.UpdatedAt),
	}

	if dbClient.ClientSecretHash.Valid {
		opts = append(opts, client.WithClientSecretHash(dbClient.ClientSecretHash.String))
	}

	return client.New(
		dbClient.ClientID,
		dbClient.Name,
		dbClient.ApplicationType,
		dbClient.RedirectURIs,
		opts...,
	), nil
}

// AuthRequest mappers

func ToDBAuthRequest(ar authrequest.AuthRequest) *models.AuthRequest {
	dbAR := &models.AuthRequest{
		ID:           ar.ID().String(),
		ClientID:     ar.ClientID(),
		RedirectURI:  ar.RedirectURI(),
		Scopes:       copySlice(ar.Scopes()),
		ResponseType: ar.ResponseType(),
		CreatedAt:    ar.CreatedAt(),
		ExpiresAt:    ar.ExpiresAt(),
	}

	if ar.State() != nil {
		dbAR.State = mapping.ValueToSQLNullString(*ar.State())
	}

	if ar.Nonce() != nil {
		dbAR.Nonce = mapping.ValueToSQLNullString(*ar.Nonce())
	}

	if ar.CodeChallenge() != nil {
		dbAR.CodeChallenge = mapping.ValueToSQLNullString(*ar.CodeChallenge())
	}

	if ar.CodeChallengeMethod() != nil {
		dbAR.CodeChallengeMethod = mapping.ValueToSQLNullString(*ar.CodeChallengeMethod())
	}

	if ar.UserID() != nil {
		dbAR.UserID = sql.NullInt64{Int64: int64(*ar.UserID()), Valid: true}
	}

	if ar.TenantID() != nil {
		dbAR.TenantID = mapping.ValueToSQLNullString(ar.TenantID().String())
	}

	if ar.AuthTime() != nil {
		dbAR.AuthTime = mapping.ValueToSQLNullTime(*ar.AuthTime())
	}

	return dbAR
}

func ToDomainAuthRequest(dbAR *models.AuthRequest) (authrequest.AuthRequest, error) {
	id, err := uuid.Parse(dbAR.ID)
	if err != nil {
		return nil, err
	}

	opts := []authrequest.Option{
		authrequest.WithID(id),
		authrequest.WithCreatedAt(dbAR.CreatedAt),
		authrequest.WithExpiresAt(dbAR.ExpiresAt),
	}

	if dbAR.State.Valid {
		opts = append(opts, authrequest.WithState(dbAR.State.String))
	}

	if dbAR.Nonce.Valid {
		opts = append(opts, authrequest.WithNonce(dbAR.Nonce.String))
	}

	if dbAR.CodeChallenge.Valid && dbAR.CodeChallengeMethod.Valid {
		opts = append(opts, authrequest.WithCodeChallenge(
			dbAR.CodeChallenge.String,
			dbAR.CodeChallengeMethod.String,
		))
	}

	if dbAR.UserID.Valid {
		userID := int(dbAR.UserID.Int64)
		opts = append(opts, authrequest.WithUserID(userID))
	}

	if dbAR.TenantID.Valid {
		tenantID, err := uuid.Parse(dbAR.TenantID.String)
		if err != nil {
			return nil, err
		}
		opts = append(opts, authrequest.WithTenantID(tenantID))
	}

	if dbAR.AuthTime.Valid {
		opts = append(opts, authrequest.WithAuthTime(dbAR.AuthTime.Time))
	}

	return authrequest.New(
		dbAR.ClientID,
		dbAR.RedirectURI,
		dbAR.Scopes,
		dbAR.ResponseType,
		opts...,
	), nil
}

// RefreshToken mappers

func ToDBRefreshToken(t token.RefreshToken) *models.RefreshToken {
	return &models.RefreshToken{
		ID:        t.ID().String(),
		TokenHash: t.TokenHash(),
		ClientID:  t.ClientID(),
		UserID:    t.UserID(),
		TenantID:  t.TenantID().String(),
		Scopes:    copySlice(t.Scopes()),
		Audience:  copySlice(t.Audience()),
		AuthTime:  t.AuthTime(),
		AMR:       copySlice(t.AMR()),
		ExpiresAt: t.ExpiresAt(),
		CreatedAt: t.CreatedAt(),
	}
}

func ToDomainRefreshToken(dbToken *models.RefreshToken) (token.RefreshToken, error) {
	id, err := uuid.Parse(dbToken.ID)
	if err != nil {
		return nil, err
	}

	tenantID, err := uuid.Parse(dbToken.TenantID)
	if err != nil {
		return nil, err
	}

	// Calculate lifetime from expires_at and created_at
	lifetime := dbToken.ExpiresAt.Sub(dbToken.CreatedAt)

	opts := []token.Option{
		token.WithID(id),
		token.WithAudience(dbToken.Audience),
		token.WithAMR(dbToken.AMR),
		token.WithExpiresAt(dbToken.ExpiresAt),
		token.WithCreatedAt(dbToken.CreatedAt),
	}

	return token.New(
		dbToken.TokenHash,
		dbToken.ClientID,
		dbToken.UserID,
		tenantID,
		dbToken.Scopes,
		dbToken.AuthTime,
		lifetime,
		opts...,
	), nil
}

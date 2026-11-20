package oidc_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	oidcpersistence "github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
)

type grantRequest struct {
	subject string
	client  string
}

func (r grantRequest) GetSubject() string    { return r.subject }
func (r grantRequest) GetAudience() []string { return []string{r.client} }
func (r grantRequest) GetScopes() []string   { return []string{"openid", "offline_access"} }

// Falsely green if the authorization code grant returns no refresh token, if
// every grant stores the same token hash (the second grant then fails on the
// unique index), or if a refreshed token can be replayed.
func TestStorage_CreateAccessAndRefreshTokens_IssuesAndRotates(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	require.NoError(t, oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey))
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	storage := oidc.NewStorage(nil, nil, oidcpersistence.NewTokenRepository(), userRepo, env.Pool, testCryptoKey,
		"https://issuer.example.com/oidc", time.Hour, 24*time.Hour)

	subject := func() string {
		email, err := internet.NewEmail("refresh-" + uuid.NewString() + "@example.test")
		require.NoError(t, err)
		u, err := user.New("Refresh", "Holder", email, user.UILanguageEN, user.WithTenantID(env.TenantID())).SetPassword("Permanent123!")
		require.NoError(t, err)
		created, err := userRepo.Create(env.Ctx, u)
		require.NoError(t, err)
		return strconv.FormatUint(uint64(created.ID()), 10)
	}
	clientID := "mcp-" + uuid.NewString()
	_, err := oidcpersistence.NewClientRepository().Create(env.Ctx,
		client.New(clientID, "MCP client", "web", []string{"https://client.example.test/callback"}))
	require.NoError(t, err)
	first, second := grantRequest{subject: subject(), client: clientID}, grantRequest{subject: subject(), client: clientID}

	access, refresh, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, first, "")
	require.NoError(t, err)
	require.NotEmpty(t, access)
	require.NotEmpty(t, refresh, "authorization code grant must return a refresh token")

	_, other, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, second, "")
	require.NoError(t, err, "a second grant must not collide with the first")
	require.NotEqual(t, refresh, other)

	presented, err := storage.TokenRequestByRefreshToken(env.Ctx, refresh)
	require.NoError(t, err)
	require.Equal(t, first.subject, presented.GetSubject())

	_, rotated, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, first, refresh)
	require.NoError(t, err)
	require.NotEmpty(t, rotated)
	require.NotEqual(t, refresh, rotated)

	_, err = storage.TokenRequestByRefreshToken(env.Ctx, refresh)
	require.Error(t, err, "a rotated refresh token must not be accepted again")
	_, _, _, err = storage.CreateAccessAndRefreshTokens(env.Ctx, first, refresh)
	require.Error(t, err, "replaying a rotated refresh token must fail")

	_, err = storage.TokenRequestByRefreshToken(env.Ctx, rotated)
	require.NoError(t, err)
}

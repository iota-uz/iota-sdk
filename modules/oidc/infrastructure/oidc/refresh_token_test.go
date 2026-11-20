package oidc_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	oidcpersistence "github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
)

// Falsely green if the storage hands back the token it was given: the code
// exchange passes no current token, so every login would get the same empty
// refresh token and the second one would collide on its hash.
func TestStorage_CreateAccessAndRefreshTokens_IssuesAndRotates(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	require.NoError(t, oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey))
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	storage := oidc.NewStorage(nil, nil, oidcpersistence.NewTokenRepository(), userRepo, env.Pool, testCryptoKey, "https://issuer.example.com/oidc", time.Hour, 24*time.Hour)

	_, err := oidcpersistence.NewClientRepository().Create(env.Ctx, client.New("client", "Client", "web", []string{"https://client.example.com/callback"}))
	require.NoError(t, err)
	email, err := internet.NewEmail("oidc-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	u, err := user.New("Token", "Holder", email, user.UILanguageEN, user.WithTenantID(env.TenantID())).SetPassword("Permanent123!")
	require.NoError(t, err)
	u, err = userRepo.Create(env.Ctx, u)
	require.NoError(t, err)
	req := tokenRequest{subject: strconv.FormatUint(uint64(u.ID()), 10)}

	_, first, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, req, "")
	require.NoError(t, err)
	_, second, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, req, "")
	require.NoError(t, err)
	assert.NotEmpty(t, first)
	assert.NotEqual(t, first, second, "each code exchange issues its own refresh token")

	refreshReq, err := storage.TokenRequestByRefreshToken(env.Ctx, first)
	require.NoError(t, err)
	_, rotated, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, refreshReq, first)
	require.NoError(t, err)
	assert.NotEqual(t, first, rotated)

	_, err = storage.TokenRequestByRefreshToken(env.Ctx, first)
	require.Error(t, err, "a used refresh token is not accepted again")
	_, err = storage.TokenRequestByRefreshToken(env.Ctx, rotated)
	require.NoError(t, err)
}

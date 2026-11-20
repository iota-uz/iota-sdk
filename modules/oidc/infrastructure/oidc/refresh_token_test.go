package oidc_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	oidcpersistence "github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/constants"
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
	replayed, _, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, first, refresh)
	require.Error(t, err, "replaying a rotated refresh token must fail")
	require.Empty(t, replayed)

	// The replay above revoked the chain: whoever holds the replacement of a
	// leaked token loses it too (RFC 9700 4.14.2).
	_, err = storage.TokenRequestByRefreshToken(env.Ctx, rotated)
	require.Error(t, err, "reusing a consumed token must revoke its replacement")
}

// Falsely green if the access token carries no granted scopes: a resource
// server (mcpgateway) then rejects every token as lacking scope.
func TestStorage_GetPrivateClaimsFromScopes_CarriesScope(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	storage := oidc.NewStorage(nil, nil, nil, userRepo, env.Pool, testCryptoKey,
		"https://issuer.example.com/oidc", time.Hour, 24*time.Hour)
	email, err := internet.NewEmail("scope-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	u, err := user.New("Scope", "Holder", email, user.UILanguageEN, user.WithTenantID(env.TenantID())).SetPassword("Permanent123!")
	require.NoError(t, err)
	created, err := userRepo.Create(env.Ctx, u)
	require.NoError(t, err)

	claims, err := storage.GetPrivateClaimsFromScopes(env.Ctx, strconv.FormatUint(uint64(created.ID()), 10), "client",
		[]string{"offline_access", "erp:read"})
	require.NoError(t, err)
	require.Equal(t, "offline_access erp:read", claims["scope"])
}

// Falsely green if two simultaneous redemptions of one refresh token both
// rotate it: the zero-row delete must fail the loser inside its transaction.
func TestStorage_CreateAccessAndRefreshTokens_ConcurrentRedemption(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	require.NoError(t, oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey))
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	tokens := oidcpersistence.NewTokenRepository()
	storage := oidc.NewStorage(oidcpersistence.NewClientRepository(), nil, tokens, userRepo, env.Pool, testCryptoKey,
		"https://issuer.example.com/oidc", time.Hour, 24*time.Hour)

	clientID := "mcp-" + uuid.NewString()
	_, err := oidcpersistence.NewClientRepository().Create(env.Ctx,
		client.New(clientID, "MCP client", "web", []string{"https://client.example.test/callback"}))
	require.NoError(t, err)
	email, err := internet.NewEmail("race-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	u, err := user.New("Race", "Holder", email, user.UILanguageEN, user.WithTenantID(env.TenantID())).SetPassword("Permanent123!")
	require.NoError(t, err)
	created, err := userRepo.Create(env.Ctx, u)
	require.NoError(t, err)
	grant := grantRequest{subject: strconv.FormatUint(uint64(created.ID()), 10), client: clientID}
	_, refresh, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, grant, "")
	require.NoError(t, err)
	// Make the grant visible to separate connections.
	env.CommitTx(t)

	// Each redemption runs on its own connection, outside the test transaction.
	detached := context.WithValue(env.Ctx, constants.TxKey, nil)
	for _, ctx := range []context.Context{detached, detached} {
		_, err := storage.TokenRequestByRefreshToken(ctx, refresh)
		require.NoError(t, err, "both requests load the token before either rotates it")
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			access, _, _, err := storage.CreateAccessAndRefreshTokens(detached, grant, refresh)
			if err == nil && access == "" {
				err = errors.New("rotation returned no access token")
			}
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	require.Equal(t, 1, succeeded, "exactly one concurrent redemption may rotate the token")
}

// Falsely green if a deactivated public client keeps refreshing: it never
// passes AuthorizeClientIDSecret, where activity used to be checked.
func TestStorage_TokenRequestByRefreshToken_RejectsInactiveClient(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	require.NoError(t, oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey))
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	clients := oidcpersistence.NewClientRepository()
	storage := oidc.NewStorage(clients, nil, oidcpersistence.NewTokenRepository(), userRepo, env.Pool, testCryptoKey,
		"https://issuer.example.com/oidc", time.Hour, 24*time.Hour)

	for _, method := range []string{"none", "client_secret_post"} {
		clientID := "mcp-" + method + "-" + uuid.NewString()
		saved, err := clients.Create(env.Ctx, client.New(clientID, "MCP client", "web",
			[]string{"https://client.example.test/callback"}, client.WithAuthMethod(method)))
		require.NoError(t, err)
		email, err := internet.NewEmail("inactive-" + uuid.NewString() + "@example.test")
		require.NoError(t, err)
		u, err := user.New("Inactive", "Client", email, user.UILanguageEN, user.WithTenantID(env.TenantID())).SetPassword("Permanent123!")
		require.NoError(t, err)
		created, err := userRepo.Create(env.Ctx, u)
		require.NoError(t, err)
		grant := grantRequest{subject: strconv.FormatUint(uint64(created.ID()), 10), client: clientID}
		_, refresh, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, grant, "")
		require.NoError(t, err)

		_, err = storage.TokenRequestByRefreshToken(env.Ctx, refresh)
		require.NoError(t, err, method)
		require.NoError(t, clients.Update(env.Ctx, saved.Deactivate()))
		_, err = storage.TokenRequestByRefreshToken(env.Ctx, refresh)
		require.Error(t, err, "deactivated %s client must not refresh", method)
	}
}

// Falsely green if a refresh that asks for fewer scopes still gets the
// original ones in the new access and refresh tokens.
func TestRefreshTokenRequest_SetCurrentScopesNarrows(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	require.NoError(t, oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey))
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	clients := oidcpersistence.NewClientRepository()
	storage := oidc.NewStorage(clients, nil, oidcpersistence.NewTokenRepository(), userRepo, env.Pool, testCryptoKey,
		"https://issuer.example.com/oidc", time.Hour, 24*time.Hour)
	clientID := "mcp-" + uuid.NewString()
	_, err := clients.Create(env.Ctx, client.New(clientID, "MCP client", "web", []string{"https://client.example.test/callback"}))
	require.NoError(t, err)
	email, err := internet.NewEmail("narrow-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	u, err := user.New("Narrow", "Scopes", email, user.UILanguageEN, user.WithTenantID(env.TenantID())).SetPassword("Permanent123!")
	require.NoError(t, err)
	created, err := userRepo.Create(env.Ctx, u)
	require.NoError(t, err)
	_, refresh, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx,
		grantRequest{subject: strconv.FormatUint(uint64(created.ID()), 10), client: clientID}, "")
	require.NoError(t, err)

	req, err := storage.TokenRequestByRefreshToken(env.Ctx, refresh)
	require.NoError(t, err)
	require.Equal(t, []string{"openid", "offline_access"}, req.GetScopes())
	req.SetCurrentScopes([]string{"offline_access"})
	require.Equal(t, []string{"offline_access"}, req.GetScopes())

	_, rotated, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, req, refresh)
	require.NoError(t, err)
	next, err := storage.TokenRequestByRefreshToken(env.Ctx, rotated)
	require.NoError(t, err)
	require.Equal(t, []string{"offline_access"}, next.GetScopes(), "the replacement token keeps the narrowed scopes")
}

// failingCreate stores nothing: it makes the replacement insert fail after
// the presented token has been deleted in the same transaction.
type failingCreate struct{ token.Repository }

func (failingCreate) Create(context.Context, token.RefreshToken) error {
	return errors.New("insert failed")
}

// Falsely green if a failed replacement insert leaves the client without
// any refresh token: the delete of the presented token must roll back.
func TestStorage_CreateAccessAndRefreshTokens_FailedInsertKeepsToken(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	require.NoError(t, oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey))
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	clients := oidcpersistence.NewClientRepository()
	tokens := oidcpersistence.NewTokenRepository()
	storage := oidc.NewStorage(clients, nil, tokens, userRepo, env.Pool, testCryptoKey,
		"https://issuer.example.com/oidc", time.Hour, 24*time.Hour)
	broken := oidc.NewStorage(clients, nil, failingCreate{tokens}, userRepo, env.Pool, testCryptoKey,
		"https://issuer.example.com/oidc", time.Hour, 24*time.Hour)

	clientID := "mcp-" + uuid.NewString()
	_, err := clients.Create(env.Ctx, client.New(clientID, "MCP client", "web", []string{"https://client.example.test/callback"}))
	require.NoError(t, err)
	email, err := internet.NewEmail("rollback-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	u, err := user.New("Rollback", "Holder", email, user.UILanguageEN, user.WithTenantID(env.TenantID())).SetPassword("Permanent123!")
	require.NoError(t, err)
	created, err := userRepo.Create(env.Ctx, u)
	require.NoError(t, err)
	grant := grantRequest{subject: strconv.FormatUint(uint64(created.ID()), 10), client: clientID}
	_, refresh, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, grant, "")
	require.NoError(t, err)
	env.CommitTx(t)

	// Outside the test transaction, so the storage opens its own one.
	detached := context.WithValue(env.Ctx, constants.TxKey, nil)
	access, _, _, err := broken.CreateAccessAndRefreshTokens(detached, grant, refresh)
	require.Error(t, err)
	require.Empty(t, access)

	_, err = storage.TokenRequestByRefreshToken(detached, refresh)
	require.NoError(t, err, "the presented token stays valid when its replacement is not stored")
}

// Falsely green if reuse revokes only the direct replacement: an attacker who
// already rotated twice must lose the latest token as well.
func TestStorage_TokenRequestByRefreshToken_ReuseRevokesWholeChain(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	require.NoError(t, oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey))
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	clients := oidcpersistence.NewClientRepository()
	storage := oidc.NewStorage(clients, nil, oidcpersistence.NewTokenRepository(), userRepo, env.Pool, testCryptoKey,
		"https://issuer.example.com/oidc", time.Hour, 24*time.Hour)
	clientID := "mcp-" + uuid.NewString()
	_, err := clients.Create(env.Ctx, client.New(clientID, "MCP client", "web", []string{"https://client.example.test/callback"}))
	require.NoError(t, err)
	email, err := internet.NewEmail("chain-" + uuid.NewString() + "@example.test")
	require.NoError(t, err)
	u, err := user.New("Chain", "Holder", email, user.UILanguageEN, user.WithTenantID(env.TenantID())).SetPassword("Permanent123!")
	require.NoError(t, err)
	created, err := userRepo.Create(env.Ctx, u)
	require.NoError(t, err)
	grant := grantRequest{subject: strconv.FormatUint(uint64(created.ID()), 10), client: clientID}

	_, first, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, grant, "")
	require.NoError(t, err)
	_, second, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, grant, first)
	require.NoError(t, err)
	_, third, _, err := storage.CreateAccessAndRefreshTokens(env.Ctx, grant, second)
	require.NoError(t, err)
	_, err = storage.TokenRequestByRefreshToken(env.Ctx, third)
	require.NoError(t, err)

	_, err = storage.TokenRequestByRefreshToken(env.Ctx, first)
	require.Error(t, err)
	_, err = storage.TokenRequestByRefreshToken(env.Ctx, third)
	require.Error(t, err, "the latest token of a reused chain must be revoked")
}

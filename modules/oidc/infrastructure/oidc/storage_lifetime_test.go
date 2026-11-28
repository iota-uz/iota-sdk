package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type stubClientRepo struct {
	client.Repository
	clients map[string]client.Client
}

func (r *stubClientRepo) GetByClientID(_ context.Context, clientID string) (client.Client, error) {
	c, ok := r.clients[clientID]
	if !ok {
		return nil, serrors.New(serrors.NotFound, "client not found")
	}
	return c, nil
}

type stubUserRepo struct {
	user.Repository
	users map[uint]user.User
}

func (r *stubUserRepo) GetByID(_ context.Context, id uint) (user.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

type stubTokenRepo struct {
	token.Repository
	mu       sync.Mutex
	byHash   map[string]token.RefreshToken
	consumed map[string]uuid.UUID
	created  []token.RefreshToken
}

func newStubTokenRepo() *stubTokenRepo {
	return &stubTokenRepo{
		byHash:   make(map[string]token.RefreshToken),
		consumed: make(map[string]uuid.UUID),
	}
}

func (r *stubTokenRepo) LockGrant(context.Context, int, string) error { return nil }

func (r *stubTokenRepo) Create(_ context.Context, t token.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byHash[t.TokenHash()] = t
	r.created = append(r.created, t)
	return nil
}

func (r *stubTokenRepo) GetByTokenHash(_ context.Context, tokenHash string) (token.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, wasConsumed := r.consumed[tokenHash]; wasConsumed {
		return nil, serrors.New(serrors.NotFound, "token consumed")
	}
	t, ok := r.byHash[tokenHash]
	if !ok {
		return nil, serrors.New(serrors.NotFound, "token not found")
	}
	return t, nil
}

func (r *stubTokenRepo) Consume(_ context.Context, tokenHash string, replacedBy uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byHash[tokenHash]; !ok {
		return serrors.New(serrors.NotFound, "token not found")
	}
	if _, wasConsumed := r.consumed[tokenHash]; wasConsumed {
		return serrors.New(serrors.NotFound, "already consumed")
	}
	r.consumed[tokenHash] = replacedBy
	return nil
}

type lifetimeTokenReq struct {
	subject  string
	clientID string
	audience []string
	scopes   []string
}

func (r lifetimeTokenReq) GetSubject() string    { return r.subject }
func (r lifetimeTokenReq) GetAudience() []string { return r.audience }
func (r lifetimeTokenReq) GetScopes() []string   { return r.scopes }
func (r lifetimeTokenReq) GetClientID() string   { return r.clientID }

type audienceOnlyTokenReq struct {
	subject  string
	audience []string
	scopes   []string
}

func (r audienceOnlyTokenReq) GetSubject() string    { return r.subject }
func (r audienceOnlyTokenReq) GetAudience() []string { return r.audience }
func (r audienceOnlyTokenReq) GetScopes() []string   { return r.scopes }

func newLifetimeTestStorage(t *testing.T, clients map[string]client.Client, globalAccess, globalRefresh time.Duration) (*Storage, *stubTokenRepo, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	email, err := internet.NewEmail("ttl@example.test")
	require.NoError(t, err)
	u := user.New("TTL", "User", email, user.UILanguageEN, user.WithID(42), user.WithTenantID(uuid.New()))

	tokenRepo := newStubTokenRepo()
	var clientRepo client.Repository
	if clients != nil {
		clientRepo = &stubClientRepo{clients: clients}
	}
	storage := NewStorage(
		clientRepo,
		nil,
		tokenRepo,
		&stubUserRepo{users: map[uint]user.User{42: u}},
		nil,
		"test-crypto-key",
		"https://issuer.example.test/oidc",
		globalAccess,
		globalRefresh,
	)
	storage.signingKeyLoader = func(context.Context) (*rsa.PrivateKey, string, error) {
		return key, "kid-test", nil
	}
	return storage, tokenRepo, key
}

func assertJWTLifetime(t *testing.T, tokenStr string, pub *rsa.PublicKey, expected time.Duration) {
	t.Helper()
	parsed, err := jwt.Parse(tokenStr, func(*jwt.Token) (any, error) {
		return pub, nil
	})
	require.NoError(t, err)
	claims, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)
	iat, ok := claims["iat"].(float64)
	require.True(t, ok)
	exp, ok := claims["exp"].(float64)
	require.True(t, ok)
	actual := time.Duration(exp-iat) * time.Second
	require.InDelta(t, expected.Seconds(), actual.Seconds(), 2, "JWT exp-iat must match expected access token lifetime")
}

// Falsely green if Storage ignored per-client AccessTokenLifetime / RefreshTokenLifetime
// and always applied the global Storage defaults on minting or refresh rotation.
func TestStorage_PerClientTokenLifetimes(t *testing.T) {
	t.Parallel()

	const (
		globalAccess  = time.Hour
		globalRefresh = 30 * 24 * time.Hour
		customAccess  = 15 * time.Minute
		customRefresh = 24 * time.Hour
	)

	customClient := client.New(
		"client-custom",
		"Custom TTL Client",
		"web",
		[]string{"https://app.example.test/cb"},
		client.WithAccessTokenLifetime(customAccess),
		client.WithRefreshTokenLifetime(customRefresh),
	)
	zeroClient := client.New(
		"client-zero",
		"Zero TTL Client",
		"web",
		[]string{"https://app.example.test/cb"},
		client.WithAccessTokenLifetime(0),
		client.WithRefreshTokenLifetime(0),
	)
	partialClient := client.New(
		"client-partial",
		"Partial TTL Client",
		"web",
		[]string{"https://app.example.test/cb"},
		client.WithAccessTokenLifetime(10*time.Minute),
		client.WithRefreshTokenLifetime(0),
	)

	storage, tokenRepo, key := newLifetimeTestStorage(
		t,
		map[string]client.Client{
			"client-custom":  customClient,
			"client-zero":    zeroClient,
			"client-partial": partialClient,
		},
		globalAccess,
		globalRefresh,
	)
	ctx := context.Background()

	t.Run("custom client gets per-client access and refresh lifetimes", func(t *testing.T) {
		req := audienceOnlyTokenReq{
			subject:  "42",
			audience: []string{"client-custom"},
			scopes:   []string{"openid", "offline_access"},
		}

		// CreateAccessToken honors 15m (not global 1h).
		before := time.Now()
		accessOnly, accessExp, err := storage.CreateAccessToken(ctx, req)
		require.NoError(t, err)
		require.InDelta(t, customAccess.Seconds(), accessExp.Sub(before).Seconds(), 2)
		assertJWTLifetime(t, accessOnly, &key.PublicKey, customAccess)

		// CreateAccessAndRefreshTokens honors 15m access / 24h refresh (not global 1h / 30d).
		before = time.Now()
		accessToken, refreshToken, exp, err := storage.CreateAccessAndRefreshTokens(ctx, req, "")
		require.NoError(t, err)
		require.NotEmpty(t, refreshToken)
		require.InDelta(t, customAccess.Seconds(), exp.Sub(before).Seconds(), 2)
		assertJWTLifetime(t, accessToken, &key.PublicKey, customAccess)

		stored, err := tokenRepo.GetByTokenHash(ctx, hashRefreshToken(refreshToken))
		require.NoError(t, err)
		require.InDelta(t, customRefresh.Seconds(), stored.ExpiresAt().Sub(stored.CreatedAt()).Seconds(), 2)

		// Refresh rotation preserves the client's 15m access and 24h refresh TTLs.
		refreshReq, err := storage.TokenRequestByRefreshToken(ctx, refreshToken)
		require.NoError(t, err)
		beforeRotate := time.Now()
		rotatedAccess, rotatedRefresh, rotatedExp, err := storage.CreateAccessAndRefreshTokens(ctx, refreshReq, refreshToken)
		require.NoError(t, err)
		require.NotEqual(t, refreshToken, rotatedRefresh)
		require.InDelta(t, customAccess.Seconds(), rotatedExp.Sub(beforeRotate).Seconds(), 2)
		assertJWTLifetime(t, rotatedAccess, &key.PublicKey, customAccess)

		storedRotated, err := tokenRepo.GetByTokenHash(ctx, hashRefreshToken(rotatedRefresh))
		require.NoError(t, err)
		require.InDelta(t, customRefresh.Seconds(), storedRotated.ExpiresAt().Sub(storedRotated.CreatedAt()).Seconds(), 2)
	})

	t.Run("GetClientID takes precedence over extra audiences", func(t *testing.T) {
		req := lifetimeTokenReq{
			subject:  "42",
			clientID: "client-custom",
			audience: []string{"https://resource.example.test", "client-custom"},
			scopes:   []string{"openid", "offline_access"},
		}
		before := time.Now()
		accessToken, refreshToken, exp, err := storage.CreateAccessAndRefreshTokens(ctx, req, "")
		require.NoError(t, err)
		require.InDelta(t, customAccess.Seconds(), exp.Sub(before).Seconds(), 2)
		assertJWTLifetime(t, accessToken, &key.PublicKey, customAccess)

		stored, err := tokenRepo.GetByTokenHash(ctx, hashRefreshToken(refreshToken))
		require.NoError(t, err)
		require.Equal(t, "client-custom", stored.ClientID())
		require.InDelta(t, customRefresh.Seconds(), stored.ExpiresAt().Sub(stored.CreatedAt()).Seconds(), 2)
	})

	t.Run("client with zero lifetimes falls back to global defaults", func(t *testing.T) {
		req := audienceOnlyTokenReq{
			subject:  "42",
			audience: []string{"client-zero"},
			scopes:   []string{"openid", "offline_access"},
		}
		before := time.Now()
		accessToken, refreshToken, exp, err := storage.CreateAccessAndRefreshTokens(ctx, req, "")
		require.NoError(t, err)
		require.InDelta(t, globalAccess.Seconds(), exp.Sub(before).Seconds(), 2)
		assertJWTLifetime(t, accessToken, &key.PublicKey, globalAccess)

		stored, err := tokenRepo.GetByTokenHash(ctx, hashRefreshToken(refreshToken))
		require.NoError(t, err)
		require.InDelta(t, globalRefresh.Seconds(), stored.ExpiresAt().Sub(stored.CreatedAt()).Seconds(), 2)
	})

	t.Run("partial client override falls back only for unset lifetime", func(t *testing.T) {
		req := audienceOnlyTokenReq{
			subject:  "42",
			audience: []string{"client-partial"},
			scopes:   []string{"openid", "offline_access"},
		}
		before := time.Now()
		accessToken, refreshToken, exp, err := storage.CreateAccessAndRefreshTokens(ctx, req, "")
		require.NoError(t, err)
		require.InDelta(t, (10 * time.Minute).Seconds(), exp.Sub(before).Seconds(), 2)
		assertJWTLifetime(t, accessToken, &key.PublicKey, 10*time.Minute)

		stored, err := tokenRepo.GetByTokenHash(ctx, hashRefreshToken(refreshToken))
		require.NoError(t, err)
		require.InDelta(t, globalRefresh.Seconds(), stored.ExpiresAt().Sub(stored.CreatedAt()).Seconds(), 2)
	})

	t.Run("unknown client or nil client repo falls back to global defaults", func(t *testing.T) {
		unknownReq := audienceOnlyTokenReq{
			subject:  "42",
			audience: []string{"missing-client"},
			scopes:   []string{"openid", "offline_access"},
		}
		before := time.Now()
		accessToken, refreshToken, exp, err := storage.CreateAccessAndRefreshTokens(ctx, unknownReq, "")
		require.NoError(t, err)
		require.InDelta(t, globalAccess.Seconds(), exp.Sub(before).Seconds(), 2)
		assertJWTLifetime(t, accessToken, &key.PublicKey, globalAccess)

		stored, err := tokenRepo.GetByTokenHash(ctx, hashRefreshToken(refreshToken))
		require.NoError(t, err)
		require.InDelta(t, globalRefresh.Seconds(), stored.ExpiresAt().Sub(stored.CreatedAt()).Seconds(), 2)

		noClientRepoStorage, noClientTokens, noClientKey := newLifetimeTestStorage(t, nil, globalAccess, globalRefresh)
		before = time.Now()
		accessToken, refreshToken, exp, err = noClientRepoStorage.CreateAccessAndRefreshTokens(ctx, unknownReq, "")
		require.NoError(t, err)
		require.InDelta(t, globalAccess.Seconds(), exp.Sub(before).Seconds(), 2)
		assertJWTLifetime(t, accessToken, &noClientKey.PublicKey, globalAccess)

		stored, err = noClientTokens.GetByTokenHash(ctx, hashRefreshToken(refreshToken))
		require.NoError(t, err)
		require.InDelta(t, globalRefresh.Seconds(), stored.ExpiresAt().Sub(stored.CreatedAt()).Seconds(), 2)
	})
}

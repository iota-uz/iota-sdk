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
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
)

type tokenRequest struct{ subject string }

func (r tokenRequest) GetSubject() string    { return r.subject }
func (r tokenRequest) GetAudience() []string { return []string{"client"} }
func (r tokenRequest) GetScopes() []string   { return []string{"openid"} }

// Falsely green if a user who has not finished onboarding, or a blocked user,
// can still obtain or refresh OIDC tokens.
func TestStorage_CreateAccessToken_RequiresEligibleUser(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	require.NoError(t, oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey))
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	storage := oidc.NewStorage(nil, nil, nil, userRepo, env.Pool, testCryptoKey, "https://issuer.example.com/oidc", time.Hour, 24*time.Hour)

	create := func(build func(user.User) user.User) user.User {
		email, err := internet.NewEmail("oidc-" + uuid.NewString() + "@example.test")
		require.NoError(t, err)
		u, err := user.New("Token", "Holder", email, user.UILanguageEN, user.WithTenantID(env.TenantID())).SetPassword("Permanent123!")
		require.NoError(t, err)
		created, err := userRepo.Create(env.Ctx, build(u))
		require.NoError(t, err)
		return created
	}
	subject := func(u user.User) tokenRequest {
		return tokenRequest{subject: strconv.FormatUint(uint64(u.ID()), 10)}
	}

	active := create(func(u user.User) user.User { return u })
	_, _, err := storage.CreateAccessToken(env.Ctx, subject(active))
	require.NoError(t, err)

	pending := create(func(u user.User) user.User {
		out, err := u.IssueTemporaryPassword("Temporary-1", time.Now().Add(time.Hour))
		require.NoError(t, err)
		return out
	})
	_, _, err = storage.CreateAccessToken(env.Ctx, subject(pending))
	require.Error(t, err)

	blocked := create(func(u user.User) user.User { return u.Block("left", active.ID(), active.TenantID()) })
	_, _, err = storage.CreateAccessToken(env.Ctx, subject(blocked))
	require.Error(t, err)
}
